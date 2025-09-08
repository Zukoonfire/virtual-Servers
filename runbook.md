# Runbook — Virtual Server Lifecycle Simulator

This runbook describes how to operate, monitor, and recover the **Virtual Server Lifecycle Simulator** backend service.

---

## 1. System Overview

The service simulates virtual server lifecycles similar to a cloud provider (AWS EC2 / GCP Compute):
- Provision servers (`POST /server`)
- Lifecycle transitions (`start`, `stop`, `reboot`, `terminate`)
- Billing based on uptime (Billing Daemon)
- Idle cleanup (Idle Reaper)
- Lifecycle event logging

### Components
- **API service** (Go 1.24+, chi framework)
- **Postgres 16** (with schema + seed data)
- **Background daemons** (billing & reaper)
- **Docker Compose** used for deployment

---

## 2. Setup & Startup

### Local development
```bash
docker compose up --build
Postgres is seeded from db/init/schema.sql and db/init/seed.sql

API starts on http://localhost:8080

Useful endpoints
GET /servers – list servers

GET /servers/{id} – server details

POST /server – provision new server

POST /servers/{id}/action – lifecycle actions

GET /servers/{id}/logs – lifecycle events

GET /healthz – health check (to be implemented)

GET /readyz – readiness check (to be implemented)

3. Monitoring
Logs
The API logs to stdout:

billing daemon updated X servers

idle reaper terminated X servers

Request logs for API endpoints

Errors are logged with stack traces

View logs:

bash
Copy code
docker compose logs -f api
docker compose logs -f db
unt.

4. Alerts


Billing Daemon not updating for >10m → costs not accruing.

Idle Reaper not terminating → STOPPED servers pile up.

DB connection errors → API not ready.

High error rate (HTTP 5xx) → API instability.

5. Failure Modes
a.Database unavailable
Symptoms: /servers returns 500, logs show connection refused.
Recovery: Restart Postgres container:
docker compose restart db

b.No free IPs in pool
Symptoms: POST /server returns 409 Conflict.
Recovery: Extend IP pool in seed.sql or run:

INSERT INTO ip_pool (region, ip)
SELECT 'us-east-1', ('192.168.10.' || g)::inet
FROM generate_series(101, 200) g;

c.Billing not accruing
Symptoms: live_cost not increasing for RUNNING servers.
Recovery: Check API logs for billing daemon errors, restart API.

d.Idle reaper not terminating
Symptoms: STOPPED servers older than 30m not cleaned up.
Recovery: Check reaper logs, ensure timestamps in stopped_since are set.

6. Recovery Steps
Restart API only:
docker compose restart api

Reset entire system:
docker compose down -v
docker compose up --build

Verify DB connectivity:
docker exec -it virt-postgres psql -U postgres -d virt -c "SELECT COUNT(*) FROM servers;"

7. Scaling Notes
API is stateless → can run multiple replicas behind a load balancer.

Postgres is single-node in this setup (scale by using managed DB service).

Daemons (billing & reaper) run in each API process → use leader election or run single API replica for consistency.

8. Contacts
Owner: Prashant Singh

Repo:https://github.com/Zukoonfire/virtual-Servers