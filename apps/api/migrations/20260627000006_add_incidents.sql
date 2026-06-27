-- migrate:up

-- Allow the technician role (code already uses it but the CHECK was too narrow).
ALTER TABLE team_members DROP CONSTRAINT IF EXISTS team_members_role_check;
ALTER TABLE team_members ADD CONSTRAINT team_members_role_check
  CHECK (role IN ('lead', 'member', 'technician'));

-- Incidents — reported problems on a machine. When an incident is created the
-- machine status flips to 'down'; when resolved it flips back to 'operational'.
CREATE TABLE IF NOT EXISTS incidents (
    id              TEXT        PRIMARY KEY,
    machine_id      TEXT        NOT NULL REFERENCES machines(id) ON DELETE CASCADE,
    org_id          TEXT        NOT NULL REFERENCES organizations(id),
    reported_by     BIGINT      NOT NULL REFERENCES users(id),
    title           TEXT        NOT NULL,
    description     TEXT,
    severity        TEXT        NOT NULL CHECK (severity IN ('low','medium','high','critical')),
    status          TEXT        NOT NULL DEFAULT 'open'
                    CHECK (status IN ('open','in_progress','resolved')),
    resolved_at     TIMESTAMPTZ,
    resolution_notes TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Incident messages — chat thread between technicians and the maintenance agent.
CREATE TABLE IF NOT EXISTS incident_messages (
    id          TEXT        PRIMARY KEY,
    incident_id TEXT        NOT NULL REFERENCES incidents(id) ON DELETE CASCADE,
    sender_id   BIGINT      REFERENCES users(id),
    sender_name TEXT        NOT NULL,
    sender_role TEXT        NOT NULL DEFAULT 'technician',
    content     TEXT        NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_incidents_machine ON incidents(machine_id);
CREATE INDEX IF NOT EXISTS idx_incidents_org ON incidents(org_id);
CREATE INDEX IF NOT EXISTS idx_incident_messages_incident ON incident_messages(incident_id);

-- migrate:down

ALTER TABLE team_members DROP CONSTRAINT IF EXISTS team_members_role_check;
ALTER TABLE team_members ADD CONSTRAINT team_members_role_check CHECK (role IN ('lead', 'member'));
DROP TABLE IF EXISTS incident_messages;
DROP TABLE IF EXISTS incidents;
