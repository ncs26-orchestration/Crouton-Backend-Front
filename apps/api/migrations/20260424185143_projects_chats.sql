-- migrate:up
-- Repositioning to operator-tool shape: Projects group related chats,
-- each chat is a persistent workflow-design thread with full
-- message + attachment history, and deploy targets (Camunda, Elsa)
-- are scoped per project. The old IS-grounding tables remain
-- (projected_users, declared_systems, …) but are unreferenced at
-- runtime; a later migration will drop them once the repositioning
-- sticks.

CREATE TABLE projects (
  id           TEXT        PRIMARY KEY,
  name         TEXT        NOT NULL,
  description  TEXT        NOT NULL DEFAULT '',
  created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  archived_at  TIMESTAMPTZ
);

CREATE INDEX projects_active_idx ON projects (created_at DESC) WHERE archived_at IS NULL;

CREATE TABLE chats (
  id          TEXT        PRIMARY KEY,
  project_id  TEXT        NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
  title       TEXT        NOT NULL,
  summary     TEXT        NOT NULL DEFAULT '',
  -- latest_workflow_version_id is set once the chat has produced its
  -- first IR; NULL until then. FK is nullable so chat creation can
  -- precede extraction.
  latest_workflow_version_id TEXT,
  created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX chats_by_project_idx ON chats (project_id, updated_at DESC);

-- Every chat message is persisted so the thread can be revisited.
-- The body is a JSON envelope so we can hold text + rich parts
-- (attachment refs, IR diffs, tool-call markers) without further
-- migrations.
CREATE TABLE chat_messages (
  id          TEXT        PRIMARY KEY,
  chat_id     TEXT        NOT NULL REFERENCES chats(id) ON DELETE CASCADE,
  role        TEXT        NOT NULL CHECK (role IN ('user', 'assistant', 'system')),
  body        JSONB       NOT NULL,
  created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX chat_messages_thread_idx ON chat_messages (chat_id, created_at);

-- Attachments feed the chat's context. Normalized text lives
-- directly on the row so the extractor can read it without touching
-- object storage. Binary payload retention is out of scope for v0.1;
-- we only keep the extracted text + metadata.
CREATE TABLE chat_attachments (
  id            TEXT        PRIMARY KEY,
  chat_id       TEXT        NOT NULL REFERENCES chats(id) ON DELETE CASCADE,
  -- message_id is filled once the attachment is attached to a
  -- specific message. Uploads land first, then bind to a message on
  -- send. Nullable so orphan uploads don't break the chain.
  message_id    TEXT        REFERENCES chat_messages(id) ON DELETE SET NULL,
  kind          TEXT        NOT NULL CHECK (kind IN ('document', 'voice', 'image')),
  filename      TEXT        NOT NULL,
  mime          TEXT        NOT NULL,
  size_bytes    BIGINT      NOT NULL DEFAULT 0,
  text_content  TEXT        NOT NULL DEFAULT '',
  created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX chat_attachments_by_chat_idx ON chat_attachments (chat_id, created_at);
CREATE INDEX chat_attachments_by_message_idx ON chat_attachments (message_id);

-- One workflow_version row per extraction or Copilot-patch event.
-- Holding the IR JSON + stage keeps the design history reviewable
-- and lets a user rewind to a prior shape without re-running Gemini.
CREATE TABLE workflow_versions (
  id               TEXT        PRIMARY KEY,
  chat_id          TEXT        NOT NULL REFERENCES chats(id) ON DELETE CASCADE,
  ir_json          JSONB       NOT NULL,
  stage            TEXT        NOT NULL CHECK (stage IN ('drafting', 'ready')),
  diagnostics_json JSONB       NOT NULL DEFAULT '[]'::jsonb,
  source_message_id TEXT       REFERENCES chat_messages(id) ON DELETE SET NULL,
  created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX workflow_versions_by_chat_idx ON workflow_versions (chat_id, created_at DESC);

ALTER TABLE chats
  ADD CONSTRAINT chats_latest_version_fk
  FOREIGN KEY (latest_workflow_version_id)
  REFERENCES workflow_versions(id)
  ON DELETE SET NULL;

-- Deploy targets replace the old engine_connections for the new
-- flow. Scoped per project rather than per tenant because each
-- client company typically runs its own Camunda/Elsa.
CREATE TABLE deploy_targets (
  id            TEXT        PRIMARY KEY,
  project_id    TEXT        NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
  kind          TEXT        NOT NULL CHECK (kind IN ('camunda7', 'elsa3')),
  name          TEXT        NOT NULL,
  endpoint      TEXT        NOT NULL,
  auth_kind     TEXT        NOT NULL DEFAULT 'none',
  auth_user     TEXT        NOT NULL DEFAULT '',
  auth_secret   TEXT        NOT NULL DEFAULT '',
  created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX deploy_targets_by_project_idx ON deploy_targets (project_id, created_at);

-- migrate:down
DROP TABLE IF EXISTS deploy_targets;
ALTER TABLE chats DROP CONSTRAINT IF EXISTS chats_latest_version_fk;
DROP TABLE IF EXISTS workflow_versions;
DROP TABLE IF EXISTS chat_attachments;
DROP TABLE IF EXISTS chat_messages;
DROP TABLE IF EXISTS chats;
DROP TABLE IF EXISTS projects;
