-- migrate:up

-- agents is a registry of department agents for an org. One agent is seeded
-- per department team; the engine looks up agents by org + agent_type to
-- determine which department a workflow node runs in. Capabilities is a
-- free-text list so the UI can display what each agent is equipped for.
CREATE TABLE agents (
    id            TEXT PRIMARY KEY,
    org_id        TEXT NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    team_id       TEXT NOT NULL REFERENCES teams(id) ON DELETE CASCADE,
    agent_type    TEXT NOT NULL,
    name          TEXT NOT NULL,
    avatar        TEXT NOT NULL DEFAULT '',
    capabilities  TEXT NOT NULL DEFAULT '',
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    -- One agent per type per org; the seed inserts exactly one of each.
    UNIQUE (org_id, agent_type)
);

CREATE INDEX idx_agents_org ON agents(org_id);

-- department_policies are the written guidelines each department agent consults
-- when making decisions. Seeded per org on creation; read-only from the UI.
CREATE TABLE department_policies (
    id         TEXT PRIMARY KEY,
    org_id     TEXT NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    team_id    TEXT NOT NULL REFERENCES teams(id) ON DELETE CASCADE,
    title      TEXT NOT NULL,
    body       TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_policies_org ON department_policies(org_id);

-- migrate:down

DROP TABLE IF EXISTS department_policies;
DROP TABLE IF EXISTS agents;
