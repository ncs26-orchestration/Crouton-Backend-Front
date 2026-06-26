-- migrate:up

ALTER TABLE projects
  ADD COLUMN IF NOT EXISTS org_id TEXT REFERENCES organizations(id) ON DELETE CASCADE;

-- Existing rows get no org (they're from before multi-tenancy). They'll be
-- invisible until re-assigned. In dev this is fine; in prod you'd backfill.
CREATE INDEX IF NOT EXISTS projects_by_org_idx
  ON projects (org_id, created_at DESC)
  WHERE archived_at IS NULL;

-- migrate:down

DROP INDEX IF EXISTS projects_by_org_idx;
ALTER TABLE projects DROP COLUMN IF EXISTS org_id;
