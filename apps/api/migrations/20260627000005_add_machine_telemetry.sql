-- migrate:up

-- Machine telemetry: append-only readings pushed by machines or sensors.
-- Each row carries the full metrics snapshot, an optional error code, and
-- the source (e.g. "manual", "sensor", "api"). The anomaly-detection
-- layer (M-F5) will read from this to auto-create incidents.
CREATE TABLE machine_telemetry (
    id         TEXT        PRIMARY KEY,
    machine_id TEXT        NOT NULL REFERENCES machines(id) ON DELETE CASCADE,
    metrics    JSONB       NOT NULL DEFAULT '{}',
    error_code TEXT,
    source     TEXT        NOT NULL DEFAULT 'api',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_telemetry_machine ON machine_telemetry(machine_id, created_at DESC);

-- migrate:down

DROP TABLE IF EXISTS machine_telemetry;
