-- migrate:up

CREATE TABLE workflow_nodes (
    id              TEXT PRIMARY KEY,
    request_id      TEXT NOT NULL REFERENCES requests(id) ON DELETE CASCADE,
    key             TEXT NOT NULL,
    name            TEXT NOT NULL,
    agent_type      TEXT NOT NULL DEFAULT '',
    department      TEXT NOT NULL DEFAULT '',
    status          TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'in_progress', 'completed', 'blocked')),
    description     TEXT NOT NULL DEFAULT '',
    progress_percent INTEGER NOT NULL DEFAULT 0 CHECK (progress_percent BETWEEN 0 AND 100),
    status_text     TEXT NOT NULL DEFAULT '',
    started_at      TIMESTAMPTZ,
    completed_at    TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_workflow_nodes_request_id ON workflow_nodes(request_id);

CREATE TABLE workflow_edges (
    id              TEXT PRIMARY KEY,
    request_id      TEXT NOT NULL REFERENCES requests(id) ON DELETE CASCADE,
    source_node_id  TEXT NOT NULL REFERENCES workflow_nodes(id) ON DELETE CASCADE,
    target_node_id  TEXT NOT NULL REFERENCES workflow_nodes(id) ON DELETE CASCADE,
    edge_type       TEXT NOT NULL DEFAULT 'sequence'
);

CREATE INDEX idx_workflow_edges_request_id ON workflow_edges(request_id);

-- migrate:down

DROP TABLE IF EXISTS workflow_edges;
DROP TABLE IF EXISTS workflow_nodes;
