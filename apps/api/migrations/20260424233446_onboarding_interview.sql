-- migrate:up
-- Add the onboarding-interview surface alongside the existing
-- workflow-design chats. Two tiny additions:
--
--   1. chats.kind  — 'workflow' (existing behavior) or 'interview'
--      (new). The Go AppendMessage handler dispatches per kind:
--      workflow chats hit /extract; interview chats hit /interview
--      and update the project's overview snapshot.
--
--   2. projects.overview_json — the latest organisation overview
--      built from interview turns (size, sectors, key systems,
--      stakeholders, …). One snapshot per project; older snapshots
--      live in chat_messages history. Future: a sibling versions
--      table when audit becomes a requirement.

ALTER TABLE chats
  ADD COLUMN kind TEXT NOT NULL DEFAULT 'workflow'
  CHECK (kind IN ('workflow', 'interview'));

-- A project owns at most one active interview chat; enforce uniqueness
-- so the UI's "open onboarding" call can be a get-or-create without
-- racing.
CREATE UNIQUE INDEX chats_one_interview_per_project
  ON chats (project_id)
  WHERE kind = 'interview';

ALTER TABLE projects
  ADD COLUMN overview_json JSONB;


-- migrate:down
DROP INDEX IF EXISTS chats_one_interview_per_project;
ALTER TABLE chats DROP COLUMN IF EXISTS kind;
ALTER TABLE projects DROP COLUMN IF EXISTS overview_json;
