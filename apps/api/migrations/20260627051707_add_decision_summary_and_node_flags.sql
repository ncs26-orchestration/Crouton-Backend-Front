-- migrate:up
-- decision_summary holds the agent's full reasoning for a node (not just the
-- one-line status_text), so the UI can show why a department decided what it did.
ALTER TABLE workflow_nodes
    ADD COLUMN decision_summary TEXT NOT NULL DEFAULT '';

-- node_flags are the risks/notes an agent raised on a node (severity + message,
-- which may cite a policy). Mirrors agent_tasks: one row per flag, ordered.
CREATE TABLE node_flags (
    id         TEXT        PRIMARY KEY,
    request_id TEXT        NOT NULL REFERENCES requests(id) ON DELETE CASCADE,
    node_id    TEXT        NOT NULL REFERENCES workflow_nodes(id) ON DELETE CASCADE,
    severity   TEXT        NOT NULL DEFAULT 'info'
                           CHECK (severity IN ('info', 'warning', 'critical')),
    message    TEXT        NOT NULL,
    ordinal    INT         NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_node_flags_node ON node_flags(node_id);
CREATE INDEX idx_node_flags_request ON node_flags(request_id);

-- migrate:down
DROP TABLE IF EXISTS node_flags;
ALTER TABLE workflow_nodes DROP COLUMN decision_summary;
