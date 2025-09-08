-- Instance types (with pricing)
INSERT INTO instance_types (type, hourly_rate) VALUES
  ('t2.micro', 0.01),
  ('t2.small', 0.02),
  ('t2.medium', 0.04)
ON CONFLICT DO NOTHING;

-- Populate IP pool for regions
INSERT INTO ip_pool (region, ip)
SELECT 'us-east-1', ('192.168.10.' || g)::inet
FROM generate_series(1, 100) g
ON CONFLICT DO NOTHING;

INSERT INTO ip_pool (region, ip)
SELECT 'eu-west-1', ('192.168.20.' || g)::inet
FROM generate_series(1, 100) g
ON CONFLICT DO NOTHING;

-- Insert base servers
WITH ip AS (
  SELECT id FROM ip_pool WHERE region='us-east-1' AND allocated=false LIMIT 1
),
s AS (
  INSERT INTO servers (id, name, region, type, status, ip_id, stopped_since, billing_last_at)
  VALUES (gen_random_uuid(), 'web-01', 'us-east-1', 't2.micro', 'STOPPED', (SELECT id FROM ip), now(), now())
  RETURNING id
)
UPDATE ip_pool p
SET allocated=true, server_id=(SELECT id FROM s), allocated_at=now()
WHERE p.id=(SELECT id FROM ip);

WITH ip AS (
  SELECT id FROM ip_pool WHERE region='eu-west-1' AND allocated=false LIMIT 1
),
s AS (
  INSERT INTO servers (id, name, region, type, status, ip_id, last_started_at, billing_last_at)
  VALUES (gen_random_uuid(), 'api-01', 'eu-west-1', 't2.micro', 'RUNNING', (SELECT id FROM ip), now() - interval '2 hours', now())
  RETURNING id
)
UPDATE ip_pool p
SET allocated=true, server_id=(SELECT id FROM s), allocated_at=now()
WHERE p.id=(SELECT id FROM ip);

-- Insert 10 random test servers
DO $$
DECLARE
    r TEXT;
    st TEXT;
    t TEXT := 't2.micro';
BEGIN
  FOR i IN 1..10 LOOP
    r := (ARRAY['us-east-1','eu-west-1'])[1 + floor(random()*2)::int];
    st := (ARRAY['STOPPED','RUNNING'])[1 + floor(random()*2)::int];

    WITH ip AS (
      SELECT id FROM ip_pool
      WHERE region = r AND allocated = FALSE
      ORDER BY random() LIMIT 1
    ),
    ins AS (
      INSERT INTO servers (id, name, region, type, status, ip_id, stopped_since, last_started_at, billing_last_at, accrued_seconds, accrued_cost)
      SELECT gen_random_uuid(),
             'test-'||i,
             r,
             t,
             st::server_status,
             (SELECT id FROM ip),
             CASE WHEN st='STOPPED' THEN now() - interval '5 minutes' END,
             CASE WHEN st='RUNNING' THEN now() - interval '1 hour' END,
             CASE WHEN st='RUNNING' THEN now() ELSE NULL END,
             0, 0
      RETURNING id
    )
    UPDATE ip_pool p
    SET allocated = TRUE, server_id = (SELECT id FROM ins), allocated_at = now()
    WHERE p.id = (SELECT ip.id FROM ip);
  END LOOP;
END
$$ LANGUAGE plpgsql;
