package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/joho/godotenv"
)

func main() {
	_ = godotenv.Load() // loads .env if present

	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		log.Fatal("DATABASE_URL not set (put it in .env or export it)")
	}

	db, err := sql.Open("pgx", dsn)
	if err != nil {
		log.Fatal("open:", err)
	}
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		log.Fatal("ping:", err)
	}
	fmt.Println("✅ Connected to Postgres")

	var types int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM instance_types`).Scan(&types); err != nil {
		log.Fatal("instance_types:", err)
	}
	fmt.Printf("instance_types: %d\n", types)

	// IP pool by region
	rows, err := db.QueryContext(ctx, `
		SELECT region,
		       COUNT(*) FILTER (WHERE allocated) AS allocated,
		       COUNT(*) FILTER (WHERE NOT allocated) AS free
		FROM ip_pool
		GROUP BY region
		ORDER BY region`)
	if err != nil {
		log.Fatal("ip_pool:", err)
	}
	defer rows.Close()
	fmt.Println("ip_pool:")
	for rows.Next() {
		var region string
		var allocated, free int
		_ = rows.Scan(&region, &allocated, &free)
		fmt.Printf("  - %s allocated=%d free=%d\n", region, allocated, free)
	}

	// Sample servers
	rows2, err := db.QueryContext(ctx, `
		SELECT id, name, region, status, accrued_seconds, accrued_cost
		FROM servers
		ORDER BY created_at DESC
		LIMIT 5`)
	if err != nil {
		log.Fatal("servers:", err)
	}
	defer rows2.Close()
	fmt.Println("servers:")
	for rows2.Next() {
		var id, name, region, status string
		var secs int64
		var cost float64
		_ = rows2.Scan(&id, &name, &region, &status, &secs, &cost)
		fmt.Printf("  - %s  %s  %s  %s  uptime=%ds  cost=%.6f\n", id, name, region, status, secs, cost)
	}

	fmt.Println("✅ DB check complete")
}
