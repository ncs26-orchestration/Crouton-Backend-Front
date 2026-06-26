-- migrate:up

-- Business requests submitted into the org. The spine of the AI Org OS:
-- a request is planned into a workflow, worked by department agents, and
-- approved by a human. Status enum tracks its lifecycle; progress is a
-- 0-100 percentage the engine updates as nodes complete.
CREATE TABLE requests (
  id                   TEXT        PRIMARY KEY,
  org_id               TEXT        NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
  title                TEXT        NOT NULL,
  description          TEXT        NOT NULL DEFAULT '',
  requester_user_id    BIGINT      NOT NULL REFERENCES users(id),
  priority             TEXT        NOT NULL CHECK (priority IN ('low', 'medium', 'high', 'urgent')),
  status               TEXT        NOT NULL DEFAULT 'submitted'
                                   CHECK (status IN ('submitted', 'in_progress', 'awaiting_approval', 'approved', 'rejected', 'completed')),
  progress             INT         NOT NULL DEFAULT 0 CHECK (progress BETWEEN 0 AND 100),
  estimated_completion TIMESTAMPTZ,
  created_at           TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX requests_by_org_idx ON requests (org_id, created_at DESC);

-- migrate:down
DROP TABLE IF EXISTS requests;
