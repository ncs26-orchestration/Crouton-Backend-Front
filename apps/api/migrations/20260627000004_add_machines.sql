-- migrate:up

-- Machine registry. Each machine belongs to an org and may be assigned
-- to a technician (user with the technician team-role). Status tracks
-- the operational lifecycle; metadata holds machine-specific config
-- such as thresholds, error-code lookups, and the optional API key for
-- telemetry ingestion (M-F5).
CREATE TABLE machines (
    id               TEXT        PRIMARY KEY,
    org_id           TEXT        NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    assigned_user_id BIGINT      REFERENCES users(id),
    name             TEXT        NOT NULL,
    machine_type     TEXT        NOT NULL DEFAULT '',
    location         TEXT        NOT NULL DEFAULT '',
    serial_number    TEXT        NOT NULL DEFAULT '',
    status           TEXT        NOT NULL DEFAULT 'operational'
                                CHECK (status IN ('operational', 'degraded', 'down', 'maintenance')),
    metadata         JSONB       NOT NULL DEFAULT '{}',
    last_service_at  TIMESTAMPTZ,
    next_service_due TIMESTAMPTZ,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_machines_org      ON machines(org_id);
CREATE INDEX idx_machines_assigned ON machines(assigned_user_id);

-- Backfill a Maintenance team for every org that doesn't have one yet,
-- so existing orgs get the new team without manual setup. New orgs are
-- handled by the CreateOrg handler in Go (which seeds Maintenance on
-- creation).
INSERT INTO teams (id, org_id, name, description)
SELECT 'team_maint_' || id, id, 'Maintenance', 'Equipment maintenance and repair'
FROM organizations
WHERE NOT EXISTS (
    SELECT 1 FROM teams t WHERE t.org_id = organizations.id AND t.name = 'Maintenance'
);

-- migrate:down

DROP TABLE IF EXISTS machines;
