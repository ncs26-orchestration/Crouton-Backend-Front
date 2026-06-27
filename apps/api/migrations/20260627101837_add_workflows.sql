-- migrate:up
CREATE TABLE IF NOT EXISTS workflows (
    id          TEXT PRIMARY KEY,
    org_id      TEXT NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    team_id     TEXT REFERENCES teams(id) ON DELETE CASCADE,
    scope       TEXT NOT NULL DEFAULT 'global' CHECK (scope IN ('global', 'team')),
    name        TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    category    TEXT NOT NULL DEFAULT 'general',
    nodes       JSONB NOT NULL DEFAULT '[]',
    edges       JSONB NOT NULL DEFAULT '[]',
    created_by  BIGINT REFERENCES users(id) ON DELETE SET NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (org_id, name)
);

CREATE INDEX IF NOT EXISTS workflows_org_idx ON workflows (org_id);
CREATE INDEX IF NOT EXISTS workflows_team_idx ON workflows (team_id);

-- A request row is the unit of execution for both ad-hoc requests and workflow
-- runs; kind distinguishes them and workflow_id links a run to its definition.
ALTER TABLE requests
    ADD COLUMN IF NOT EXISTS kind TEXT NOT NULL DEFAULT 'request' CHECK (kind IN ('request', 'workflow_run')),
    ADD COLUMN IF NOT EXISTS workflow_id TEXT REFERENCES workflows(id) ON DELETE SET NULL;

CREATE INDEX IF NOT EXISTS requests_workflow_idx ON requests (workflow_id);

-- migrate:down
ALTER TABLE requests
    DROP COLUMN IF EXISTS workflow_id,
    DROP COLUMN IF EXISTS kind;

DROP TABLE IF EXISTS workflows;
