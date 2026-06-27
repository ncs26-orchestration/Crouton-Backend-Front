-- migrate:up
ALTER TABLE workflow_nodes
  ADD COLUMN IF NOT EXISTS decision_reasoning TEXT NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS decision_key_factors JSONB NOT NULL DEFAULT '[]';

-- migrate:down
ALTER TABLE workflow_nodes
  DROP COLUMN IF EXISTS decision_key_factors,
  DROP COLUMN IF EXISTS decision_reasoning;
