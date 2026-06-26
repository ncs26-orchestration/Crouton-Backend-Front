-- migrate:up

-- Extend users with auth fields
ALTER TABLE users
  ADD COLUMN IF NOT EXISTS name          TEXT        NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS password_hash TEXT        NOT NULL DEFAULT '';

-- Organizations (tenants)
CREATE TABLE organizations (
  id          TEXT        PRIMARY KEY,
  name        TEXT        NOT NULL,
  slug        TEXT        NOT NULL UNIQUE,
  created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Org members — role governs what surface they see
--   admin     → full executor UI + org management
--   executor  → executor UI (build/monitor workflows)
--   employee  → mobile app only (submit, inbox, status)
CREATE TABLE org_members (
  org_id    TEXT    NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
  user_id   BIGINT  NOT NULL REFERENCES users(id)         ON DELETE CASCADE,
  role      TEXT    NOT NULL CHECK (role IN ('admin', 'executor', 'employee')),
  joined_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  PRIMARY KEY (org_id, user_id)
);

CREATE INDEX org_members_user_idx ON org_members (user_id);

-- Teams — departments inside an org (Finance, Legal, HR…)
CREATE TABLE teams (
  id          TEXT        PRIMARY KEY,
  org_id      TEXT        NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
  name        TEXT        NOT NULL,
  description TEXT        NOT NULL DEFAULT '',
  created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (org_id, name)
);

CREATE INDEX teams_by_org_idx ON teams (org_id, created_at);

-- Team members
CREATE TABLE team_members (
  team_id   TEXT    NOT NULL REFERENCES teams(id)  ON DELETE CASCADE,
  user_id   BIGINT  NOT NULL REFERENCES users(id)  ON DELETE CASCADE,
  role      TEXT    NOT NULL CHECK (role IN ('lead', 'member')),
  joined_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  PRIMARY KEY (team_id, user_id)
);

CREATE INDEX team_members_user_idx ON team_members (user_id);

-- migrate:down
DROP TABLE IF EXISTS team_members;
DROP TABLE IF EXISTS teams;
DROP TABLE IF EXISTS org_members;
DROP TABLE IF EXISTS organizations;
ALTER TABLE users DROP COLUMN IF EXISTS password_hash;
ALTER TABLE users DROP COLUMN IF EXISTS name;
