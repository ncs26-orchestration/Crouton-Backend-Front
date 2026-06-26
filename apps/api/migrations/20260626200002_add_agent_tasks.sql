-- migrate:up

-- Per-node work items produced by a department agent when it runs a
-- workflow node. The orchestration engine (F3) writes one row per task
-- the agent reports, so the node detail panel can show what was done.
CREATE TABLE agent_tasks (
    id           TEXT        PRIMARY KEY,
    node_id      TEXT        NOT NULL REFERENCES workflow_nodes(id) ON DELETE CASCADE,
    title        TEXT        NOT NULL,
    status       TEXT        NOT NULL DEFAULT 'completed'
                             CHECK (status IN ('pending', 'in_progress', 'completed')),
    -- Display order within a node. created_at is transaction time and ties
    -- when tasks are inserted together, so ordinal gives a stable sort.
    ordinal      INT         NOT NULL DEFAULT 0,
    started_at   TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_agent_tasks_node_id ON agent_tasks(node_id);

-- migrate:down

DROP TABLE IF EXISTS agent_tasks;
