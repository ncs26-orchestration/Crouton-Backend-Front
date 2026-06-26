-- migrate:up

-- node_dependencies records a cross-department dependency an agent declared
-- (F5). When an agent returns a decision with blocked_on set, the engine
-- inserts a row here and marks the dependent node blocked. When the blocking
-- node completes, the engine resolves all dependencies pointing at it (sets
-- resolved_at) and re-runs the formerly blocked node.
CREATE TABLE node_dependencies (
    id                TEXT PRIMARY KEY,
    request_id        TEXT NOT NULL REFERENCES requests(id) ON DELETE CASCADE,
    dependent_node_id TEXT NOT NULL REFERENCES workflow_nodes(id) ON DELETE CASCADE,
    blocking_node_id  TEXT NOT NULL REFERENCES workflow_nodes(id) ON DELETE CASCADE,
    reason            TEXT NOT NULL DEFAULT '',
    run_count         INTEGER NOT NULL DEFAULT 1,
    resolved_at       TIMESTAMPTZ,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_node_deps_dependent ON node_dependencies(dependent_node_id) WHERE resolved_at IS NULL;
CREATE INDEX idx_node_deps_blocking   ON node_dependencies(blocking_node_id)  WHERE resolved_at IS NULL;

-- migrate:down

DROP TABLE IF EXISTS node_dependencies;
