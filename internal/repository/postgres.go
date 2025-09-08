package repository

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

type Store struct {
	DB *sql.DB
}

type ServerListItem struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Region    string    `json:"region"`
	Type      string    `json:"type"`
	Status    string    `json:"status"`
	IP        *string   `json:"ip,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type ListFilters struct {
	Region string
	Status string
	Type   string
	Limit  int
	Offset int
}

type ServerDetail struct {
	ID             string     `json:"id"`
	Name           string     `json:"name"`
	Region         string     `json:"region"`
	Type           string     `json:"type"`
	Status         string     `json:"status"`
	IP             *string    `json:"ip,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
	AccruedSeconds int64      `json:"accrued_seconds"`
	AccruedCost    float64    `json:"accrued_cost"`
	LastStartedAt  *time.Time `json:"last_started_at,omitempty"`
	HourlyRate     float64    `json:"hourly_rate"`
	LiveUptime     int64      `json:"live_uptime_seconds"`
	LiveCost       float64    `json:"live_cost"`
}

type ServerEvent struct {
	Timestamp time.Time `json:"timestamp"`
	Event     string    `json:"event"`
	Message   string    `json:"message"`
}

func (s *Store) ListServers(ctx context.Context, f ListFilters) ([]ServerListItem, int, error) {
	limit := f.Limit
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	offset := f.Offset
	if offset < 0 {
		offset = 0
	}
	conds := []string{}
	args := []any{}
	argn := 1

	if f.Region != "" {
		conds = append(conds, fmt.Sprintf("s.region=$%d", argn))
		args = append(args, f.Region)
		argn++
	}
	if f.Status != "" {
		conds = append(conds, fmt.Sprintf("s.status=$%d::server_status", argn))
		args = append(args, strings.ToUpper(f.Status))
		argn++
	}
	if f.Type != "" {
		conds = append(conds, fmt.Sprintf("s.type=$%d", argn))
		args = append(args, f.Type)
		argn++
	}
	where := ""
	if len(conds) > 0 {
		where = "WHERE" + strings.Join(conds, "AND")
	}
	countSQL := "SELECT COUNT(*) FROM servers s" + where
	var total int
	if err := s.DB.QueryRowContext(ctx, countSQL, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	limitPlaceholder := fmt.Sprintf("$%d", argn)
	offsetPlaceholder := fmt.Sprintf("$%d", argn+1)
	listSQL := `
SELECT
  s.id,
  s.name,
  s.region,
  s.type,
  s.status::text AS status,
  (SELECT ip_pool.ip::text FROM ip_pool WHERE ip_pool.id = s.ip_id) AS ip,
  s.created_at,
  s.updated_at
FROM servers s
` + where + `
ORDER BY s.created_at DESC
LIMIT ` + limitPlaceholder + ` OFFSET ` + offsetPlaceholder

	args = append(args, limit, offset)

	rows, err := s.DB.QueryContext(ctx, listSQL, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var items []ServerListItem
	for rows.Next() {
		var it ServerListItem
		var ip sql.NullString // <-- temp holder for possibly-NULL ip

		if err := rows.Scan(
			&it.ID,
			&it.Name,
			&it.Region,
			&it.Type,
			&it.Status,
			&ip, // <-- scan into NullString, not &it.IP
			&it.CreatedAt,
			&it.UpdatedAt,
		); err != nil {
			return nil, 0, err
		}

		if ip.Valid {
			s := ip.String
			it.IP = &s // set pointer only when non-null
		} // else leave it.IP = nil

		items = append(items, it)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (s *Store) GetServerByID(ctx context.Context, id string) (*ServerDetail, error) {
	query := `
	SELECT
	s.id,
	s.name,
	s.region,
	s.type,
	s.status::text,
	(SELECT ip_pool.ip::text FROM ip_pool WHERE ip_pool.id=s.ip_id)AS ip,
	s.created_at,
	s.updated_at,
	s.accrued_seconds,
	s.accrued_cost,
	s.last_started_at,
	it.hourly_rate
	
FROM servers s
JOIN instance_types it ON it.type =s.type
WHERE s.id=$1
`
	row := s.DB.QueryRowContext(ctx, query, id)

	var d ServerDetail
	var ip sql.NullString
	var lastStarted sql.NullTime

	err := row.Scan(
		&d.ID, &d.Name, &d.Region, &d.Type, &d.Status, &ip,
		&d.CreatedAt, &d.UpdatedAt,
		&d.AccruedSeconds, &d.AccruedCost, &lastStarted,
		&d.HourlyRate,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	if ip.Valid {
		s := ip.String
		d.IP = &s
	}
	if lastStarted.Valid {
		t := lastStarted.Time
		d.LastStartedAt = &t
	}

	//Computing live uptime/cost
	d.LiveUptime = d.AccruedSeconds
	if d.Status == "RUNNING" && d.LastStartedAt != nil {
		d.LiveUptime += int64(time.Since(*d.LastStartedAt).Seconds())

	}
	d.LiveCost = float64(d.LiveUptime) / 3600.0 * d.HourlyRate
	return &d, nil
}

func (s *Store) UpdateServerStatus(ctx context.Context, id string, newStatus string) error {
	_, err := s.DB.ExecContext(ctx, `
UPDATE servers
SET status=$1::server_status,
updated_at=now()
WHERE id=$2
`, newStatus, id)
	return err
}

//ApplyAction applies a lifecycle action,updates server state+timestamps, and logs event

func (s *Store) ApplyAction(ctx context.Context, id string, action string) (string, error) {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return "", err
	}
	defer tx.Rollback()

	//Getting current state
	row := tx.QueryRowContext(ctx, `
	SELECT status::text,last_started_at,last_stopped_at
	FROM servers
	WHERE id = $1
	FOR UPDATE
	`, id)
	var current string
	var lastStarted, lastStopped sql.NullTime
	if err := row.Scan(&current, &lastStarted, &lastStopped); err != nil {
		if err == sql.ErrNoRows {
			return "", sql.ErrNoRows
		}
		return "", err
	}
	var target string
	var updates []string
	var args []any
	argn := 1
	//FSM transitions and timestamp handling
	switch current {
	case "STOPPED":
		if action == "start" {
			target = "RUNNING"
			updates = append(updates, fmt.Sprint("status=$%d::server_status", argn))
			args = append(args, target)
			argn++
			updates = append(updates, fmt.Sprintf("last_started_at=now(),billing_last_at=now()"))
		} else if action == "TERMINATED" {
			target = "TERMINATED"
			updates = append(updates, fmt.Sprintf("status=$%d::server_status", argn))
			args = append(args, target)
			argn++
			updates = append(updates, "terminated_at=now()")
		}
	case "RUNNING":
		if action == "stop" {
			target = "STOPPED"
			updates = append(updates, fmt.Sprintf("status=$%d::server_status", argn))
			args = append(args, target)
			argn++
			updates = append(updates, "last_stopped_at=now(),stopped_since=now()")
			//billing data handle
			updates = append(updates, "accrued_seconds = COALESCE(accrued_seconds,0) + EXTRACT(EPOCH FROM (now()-billing_last_at))::bigint")
			updates = append(updates, "accrued_cost = COALESCE(accrued_cost,0) + (EXTRACT(EPOCH FROM (now() - billing_last_at)) / 3600.0 * (SELECT hourly_rate FROM instance_types it WHERE it.type = servers.type))")

		} else if action == "reboot" {
			target = "REBOOTING"
			updates = append(updates, fmt.Sprintf("status=$%d::server_status", argn))
			args = append(args, target)
			argn++
		}
	case "REBOOTING":
		if action == "complete-reboot" {
			target = "RUNNING"
			updates = append(updates, fmt.Sprintf("status=$%d::server_status", argn))
			args = append(args, target)
			argn++
			updates = append(updates, "last_started_at=now(),billing_last_at=now()")
		}
	}
	if target == "" {
		return "", fmt.Errorf("invalid transition")
	}
	updates = append(updates, "updated_at=now()")
	setClause := strings.Join(updates, ",")

	//Updating Server
	updateSQL := fmt.Sprintf("UPDATE servers SET %s WHERE id = $%d", setClause, argn)
	args = append(args, id)
	if _, err := tx.ExecContext(ctx, updateSQL, args...); err != nil {
		return "", err
	}
	//Inserting event
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO server_events (server_id, event, message) VALUES ($1, $2, $3)`,
		id, action, fmt.Sprintf("server %s", action)); err != nil {
		return "", err
	}
	if err := tx.Commit(); err != nil {
		return "", err
	}
	return target, nil
}

func (s *Store) GetServerLogs(ctx context.Context, id string) ([]ServerEvent, error) {
	rows, err := s.DB.QueryContext(ctx, `
	SELECT ts,event,message 
	FROM server_events
	WHERE server_id=$1
	ORDER BY ts DESC
	LIMIT 100
	`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []ServerEvent
	for rows.Next() {
		var ev ServerEvent
		if err := rows.Scan(&ev.Timestamp, &ev.Event, &ev.Message); err != nil {
			return nil, err
		}
		events = append(events, ev)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return events, nil
}

//AccrueBilling updates accrued_seconds/costs for all running serverss

func (s *Store) AccrueBilling(ctx context.Context) (int64, error) {
	res, err := s.DB.ExecContext(ctx, `
	UPDATE servers
	SET accrued_seconds =COALESCE(accrued_seconds,0)+
	EXTRACT (EPOCH FROM (now()-billing_last_at))::bigint,
	accrued_cost=COALESCE(accrued_cost,0)+
	(EXTRACT(EPOCH FROM (now()-billing_last_at))/ 3600.0 *
	(SELECT hourly_rate FROM instance_types it WHERE it.type=servers.type)),
	billing_last_at=now(),
	updated_at=now()
	WHERE status ='RUNNING'
	AND billing_last_at IS NOT NULL
	`)
	if err != nil {
		return 0, err
	}
	rows, _ := res.RowsAffected()
	return rows, nil
}

// ReapIdleServers terminates servers stopped for >30 minutes
// Returns number of servers stopped
func (s *Store) ReapIdleServers(ctx context.Context) (int64, error) {
	res, err := s.DB.ExecContext(ctx, `
UPDATE servers
SET status = 'TERMINATED',
    terminated_at = now(),
    updated_at = now()
WHERE status = 'STOPPED'
  AND stopped_since IS NOT NULL
  AND stopped_since < now() - interval '30 minutes'
`)

	if err != nil {
		return 0, err
	}
	rows, _ := res.RowsAffected()
	if rows > 0 {
		//Inserting event rows for reaped servers
		_, _ = s.DB.ExecContext(ctx, `
	INSERT INTO server_events(server_id,event,message)
	SELECT id,'reaped','server auto-termianted after 30m idle'
	FROM servers
	WHERE status='TERMINATED'
	AND terminated_at >now()-interval '1 minute'
	`)
	}
	return rows, nil
}

// CreateServer provisons a new server wwith a free ip from the pool
func (s *Store) CreateServer(ctx context.Context, name, region, stype string) (string, error) {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return "", err
	}
	defer tx.Rollback()

	//Allocating IP atomically
	var ipID int64
	err = tx.QueryRowContext(ctx, `
	SELECT id
	FROM ip_pool
	WHERE region =$1 AND allocated =FALSE
	ORDER BY id
	FOR UPDATE SKIP LOCKED
	LIMIT 1
	`, region).Scan(&ipID)
	if err != nil {
		return "", fmt.Errorf("no free IPs in region %s:%w", region, err)
	}

	//Insert INTO servers
	var serverID string
	err = tx.QueryRowContext(ctx, `
INSERT INTO servers (id, name, region, type, status, ip_id, stopped_since, billing_last_at)
VALUES (gen_random_uuid(), $1, $2, $3, 'STOPPED', $4, now(), now())
RETURNING id
`, name, region, stype, ipID).Scan(&serverID)

	if err != nil {
		return "", err
	}

	//Marking IP as allocated
	_, err = tx.ExecContext(ctx, `
	UPDATE ip_pool
	SET allocated =TRUE ,server_id=$1,allocated_at=now()
	WHERE id =$2

	`, serverID, ipID)
	if err != nil {
		return "", err
	}
	//Inserting lifecycle events
	_, err = tx.ExecContext(ctx, `
	INSERT INTO server_events(server_id,event,message)VALUES
	($1,'created','server created'),
	($1,'ip_allocated','private IP assigned'),
	($1,'stopped','server is stopped and ready')
	`, serverID)

	if err != nil {
		return "", err
	}

	if err := tx.Commit(); err != nil {
		return "", err
	}

	return serverID, nil

}
