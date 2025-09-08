-- Instance types and pricing
CREATE TABLE IF NOT EXISTS instance_types (
  type         TEXT PRIMARY KEY,          -- e.g., 't2.micro'
  hourly_rate  NUMERIC(10,4) NOT NULL CHECK (hourly_rate >= 0)
);

-- Server status as an enum (keeps states constrained)
DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'server_status') THEN
    CREATE TYPE server_status AS ENUM ('PENDING','STOPPED','RUNNING','REBOOTING','TERMINATED');
  END IF;
END$$;

-- Pool of private IPs per region
CREATE TABLE IF NOT EXISTS ip_pool (
  id           BIGSERIAL PRIMARY KEY,
  region       TEXT NOT NULL,
  ip           INET NOT NULL UNIQUE,
  allocated    BOOLEAN NOT NULL DEFAULT FALSE,
  server_id    UUID,
  allocated_at TIMESTAMPTZ,
  released_at  TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS ip_pool_region_idx ON ip_pool(region);
CREATE INDEX IF NOT EXISTS ip_pool_allocated_idx ON ip_pool(allocated);

-- Servers
CREATE TABLE IF NOT EXISTS servers (
  id               UUID PRIMARY KEY,
  name             TEXT,
  region           TEXT NOT NULL,
  type             TEXT NOT NULL REFERENCES instance_types(type),
  status           server_status NOT NULL,
  ip_id            BIGINT REFERENCES ip_pool(id),
  created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
  terminated_at    TIMESTAMPTZ,
  last_started_at  TIMESTAMPTZ,
  last_stopped_at  TIMESTAMPTZ,
  stopped_since    TIMESTAMPTZ,              -- for idle reaper
  billing_last_at  TIMESTAMPTZ,              -- last accrual tick
  accrued_seconds  BIGINT NOT NULL DEFAULT 0,
  accrued_cost     NUMERIC(12,6) NOT NULL DEFAULT 0
);
CREATE INDEX IF NOT EXISTS servers_region_idx ON servers(region);
CREATE INDEX IF NOT EXISTS servers_status_idx ON servers(status);
CREATE INDEX IF NOT EXISTS servers_created_idx ON servers(created_at DESC);

-- Running sessions (uptime segments)
CREATE TABLE IF NOT EXISTS server_sessions (
  id         BIGSERIAL PRIMARY KEY,
  server_id  UUID NOT NULL REFERENCES servers(id) ON DELETE CASCADE,
  start_at   TIMESTAMPTZ NOT NULL,
  end_at     TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS server_sessions_svr_idx ON server_sessions(server_id, start_at DESC);

-- Lifecycle events (we’ll query newest 100)
CREATE TABLE IF NOT EXISTS server_events (
  id         BIGSERIAL PRIMARY KEY,
  server_id  UUID NOT NULL REFERENCES servers(id) ON DELETE CASCADE,
  ts         TIMESTAMPTZ NOT NULL DEFAULT now(),
  event      TEXT NOT NULL,        -- e.g., 'created','ip_allocated','start','stop','terminate'
  message    TEXT,
  data       JSONB
);
CREATE INDEX IF NOT EXISTS server_events_svr_ts_idx ON server_events(server_id, ts DESC);
