-- migrate:up
-- Widen the workflow_versions.stage check constraint so the operator
-- can explicitly Approve a ready workflow. Approval is per-version:
-- subsequent modifying turns write a new row at ready/drafting, and
-- the approved snapshot stays immutable in history.

ALTER TABLE workflow_versions
  DROP CONSTRAINT IF EXISTS workflow_versions_stage_check;

ALTER TABLE workflow_versions
  ADD CONSTRAINT workflow_versions_stage_check
  CHECK (stage IN ('drafting', 'ready', 'approved'));


-- migrate:down
ALTER TABLE workflow_versions
  DROP CONSTRAINT IF EXISTS workflow_versions_stage_check;

-- Downgrade path: any 'approved' rows get collapsed to 'ready' so
-- the narrower constraint applies cleanly.
UPDATE workflow_versions SET stage = 'ready' WHERE stage = 'approved';

ALTER TABLE workflow_versions
  ADD CONSTRAINT workflow_versions_stage_check
  CHECK (stage IN ('drafting', 'ready'));
