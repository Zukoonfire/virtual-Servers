# Virtual Server Lifecycle Simulator

A backend service in **Go** that simulates cloud-style server lifecycle management:
- Provisioning virtual servers
- Lifecycle state transitions (start, stop, reboot, terminate)
- Billing accrual based on uptime
- Logging lifecycle events
- Automatic idle server reaping

Built to mimic how cloud providers like AWS/GCP handle virtual machine lifecycles — without managing real infrastructure.

---

## Features

### Core API
- **POST /server** – Provision a new server (allocate IP from pool).
- **GET /servers** – List servers (filter by region, type, status; with pagination).
- **GET /servers/{id}** – Fetch detailed server metadata (with live uptime & billing).
- **POST /servers/{id}/action** – Lifecycle actions (`start`, `stop`, `reboot`, `terminate`).
- **GET /servers/{id}/logs** – Retrieve last 100 lifecycle events for a server.

### Bonus Features
- **Billing Daemon** – Background task accrues billing for RUNNING servers in real time.
- **Idle Reaper** – Automatically terminates servers that have been STOPPED for >30 minutes.

---

## Tech Stack

- **Language**: Go 1.22+
- **Framework**: [chi](https://github.com/go-chi/chi)
- **Database**: PostgreSQL (with Docker)
- **Migrations/Seed Data**: SQL files in `db/init`
- **Logging**: Structured logs with request IDs
- **Configuration**: `.env` + envconfig
- **Concurrency**: goroutines + context for billing/reaper

---

##  Project Structure

├── cmd/
│ ├── server/ # API entrypoint
│ └── dbcheck/ # DB connection test tool
├── internal/
│ ├── api/ # HTTP handlers
│ ├── repository/ # DB queries
│ ├── service/ # background daemons (billing, reaper)
│ └── domain/ # domain models + FSM
├── db/
│ └── init/ # schema.sql + seed.sql
├── docs/
│ └── runbook.md # operational notes
├── docker-compose.yml
├── .env
└── README.md



