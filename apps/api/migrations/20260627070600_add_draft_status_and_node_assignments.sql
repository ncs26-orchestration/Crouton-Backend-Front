-- migrate:up
-- A request now waits in 'draft' after intake plans it, until a human assigns
-- verifiers and launches it.
ALTER TABLE requests DROP CONSTRAINT IF EXISTS requests_status_check;
ALTER TABLE requests ADD CONSTRAINT requests_status_check
    CHECK (status IN ('draft', 'submitted', 'in_progress', 'awaiting_approval', 'approved', 'rejected', 'completed'));

-- A node with an assigned verifier pauses at 'awaiting_review' after its agent
-- runs, until the verifier (or the executive) signs off.
ALTER TABLE workflow_nodes DROP CONSTRAINT IF EXISTS workflow_nodes_status_check;
ALTER TABLE workflow_nodes ADD CONSTRAINT workflow_nodes_status_check
    CHECK (status IN ('pending', 'in_progress', 'awaiting_review', 'completed', 'blocked'));

-- node_assignments: who must verify a node's agent output before the workflow
-- advances past it. A node with no assignment auto-completes.
CREATE TABLE node_assignments (
    id          TEXT        PRIMARY KEY,
    request_id  TEXT        NOT NULL REFERENCES requests(id) ON DELETE CASCADE,
    node_id     TEXT        NOT NULL REFERENCES workflow_nodes(id) ON DELETE CASCADE,
    user_id     BIGINT      NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    assigned_by BIGINT      REFERENCES users(id) ON DELETE SET NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (node_id, user_id)
);

CREATE INDEX idx_node_assignments_node ON node_assignments(node_id);
CREATE INDEX idx_node_assignments_request ON node_assignments(request_id);

-- migrate:down
DROP TABLE IF EXISTS node_assignments;
ALTER TABLE workflow_nodes DROP CONSTRAINT IF EXISTS workflow_nodes_status_check;
ALTER TABLE workflow_nodes ADD CONSTRAINT workflow_nodes_status_check
    CHECK (status IN ('pending', 'in_progress', 'completed', 'blocked'));
ALTER TABLE requests DROP CONSTRAINT IF EXISTS requests_status_check;
ALTER TABLE requests ADD CONSTRAINT requests_status_check
    CHECK (status IN ('submitted', 'in_progress', 'awaiting_approval', 'approved', 'rejected', 'completed'));
