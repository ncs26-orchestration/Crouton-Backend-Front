-- migrate:up

-- The agent roster (GET /orgs/:orgId/agents) derives live status by aggregating
-- workflow_nodes per agent_type, and the Agents tab polls it. Index agent_type
-- so that aggregate does not scan the whole table on each poll.
CREATE INDEX IF NOT EXISTS idx_workflow_nodes_agent_type ON workflow_nodes(agent_type);

-- migrate:down

DROP INDEX IF EXISTS idx_workflow_nodes_agent_type;

