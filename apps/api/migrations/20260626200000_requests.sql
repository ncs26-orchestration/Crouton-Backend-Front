-- migrate:up

CREATE TABLE requests (
    id              TEXT PRIMARY KEY,
    org_id          TEXT NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    title           TEXT NOT NULL,
    description     TEXT NOT NULL DEFAULT '',
    requester_user_id BIGINT NOT NULL REFERENCES users(id),
    priority        TEXT NOT NULL DEFAULT 'medium' CHECK (priority IN ('low', 'medium', 'high', 'critical')),
    status          TEXT NOT NULL DEFAULT 'submitted' CHECK (status IN ('submitted', 'in_progress', 'awaiting_approval', 'approved', 'rejected', 'completed')),
    progress        INTEGER NOT NULL DEFAULT 0 CHECK (progress BETWEEN 0 AND 100),
    estimated_completion TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_requests_org_id ON requests(org_id);
CREATE INDEX idx_requests_status ON requests(status);

-- migrate:down

DROP TABLE IF EXISTS requests;
