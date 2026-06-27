-- migrate:up
-- decision_outcome records the call a department agent reached on a node:
-- approve, approve_with_conditions, flag, reject, or block. Defaults to pending
-- until the node runs.
ALTER TABLE workflow_nodes
    ADD COLUMN decision_outcome TEXT NOT NULL DEFAULT 'pending';

-- migrate:down
ALTER TABLE workflow_nodes
    DROP COLUMN decision_outcome;
