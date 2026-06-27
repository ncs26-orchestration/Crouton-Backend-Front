-- migrate:up
-- node_messages is the per-node conversation between a human verifier and the
-- department agent: questions, the agent's answers, and change requests that make
-- the agent revise its decision. author_user_id is NULL for the agent.
CREATE TABLE node_messages (
    id             TEXT        PRIMARY KEY,
    request_id     TEXT        NOT NULL REFERENCES requests(id) ON DELETE CASCADE,
    node_id        TEXT        NOT NULL REFERENCES workflow_nodes(id) ON DELETE CASCADE,
    author_user_id BIGINT      REFERENCES users(id) ON DELETE SET NULL,
    author_name    TEXT        NOT NULL DEFAULT '',
    role           TEXT        NOT NULL CHECK (role IN ('human', 'agent', 'system')),
    body           TEXT        NOT NULL,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_node_messages_node ON node_messages(node_id, created_at);

-- migrate:down
DROP TABLE IF EXISTS node_messages;
