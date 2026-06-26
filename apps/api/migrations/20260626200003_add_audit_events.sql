-- migrate:up

CREATE TABLE audit_events (
    id          TEXT PRIMARY KEY,
    request_id  TEXT NOT NULL REFERENCES requests(id) ON DELETE CASCADE,
    node_id     TEXT REFERENCES workflow_nodes(id) ON DELETE SET NULL,
    actor       TEXT NOT NULL,
    action      TEXT NOT NULL,
    reason      TEXT NOT NULL DEFAULT '',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_audit_events_request_id ON audit_events(request_id, created_at DESC);
CREATE INDEX idx_audit_events_node_id ON audit_events(node_id, created_at DESC);

-- migrate:down

DROP TABLE IF EXISTS audit_events;
