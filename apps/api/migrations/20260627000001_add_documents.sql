-- migrate:up

CREATE TABLE documents (
    id           TEXT PRIMARY KEY,
    request_id   TEXT NOT NULL REFERENCES requests(id) ON DELETE CASCADE,
    node_id      TEXT REFERENCES workflow_nodes(id) ON DELETE SET NULL,
    filename     TEXT NOT NULL,
    mime         TEXT NOT NULL DEFAULT 'text/plain',
    content_text TEXT NOT NULL DEFAULT '',
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_documents_request_id ON documents(request_id);

ALTER TABLE audit_events ADD COLUMN document_id TEXT REFERENCES documents(id) ON DELETE SET NULL;

-- migrate:down

ALTER TABLE audit_events DROP COLUMN document_id;
DROP TABLE IF EXISTS documents;
