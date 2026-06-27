-- migrate:up

-- Make agent_tasks work for both workflow tasks and incident tasks.
-- node_id is now nullable: null means this is an incident task.
-- incident_id links to the machine incident when applicable.
ALTER TABLE agent_tasks ALTER COLUMN node_id DROP NOT NULL;
ALTER TABLE agent_tasks ADD COLUMN incident_id TEXT REFERENCES machine_incidents(id) ON DELETE CASCADE;

CREATE INDEX IF NOT EXISTS idx_agent_tasks_incident ON agent_tasks(incident_id);

-- migrate:down

DROP INDEX IF EXISTS idx_agent_tasks_incident;
ALTER TABLE agent_tasks DROP COLUMN IF EXISTS incident_id;
ALTER TABLE agent_tasks ALTER COLUMN node_id SET NOT NULL;
