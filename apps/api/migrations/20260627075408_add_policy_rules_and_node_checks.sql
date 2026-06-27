-- migrate:up
-- A policy can carry typed, machine-checkable rules evaluated against a request's
-- structured details. Each rule: {label, field, op, value, severity, message}.
ALTER TABLE department_policies
    ADD COLUMN rules JSONB NOT NULL DEFAULT '[]'::jsonb;

-- node_checks are the exact pass/warn/fail results of evaluating a department's
-- policy rules against a request, persisted per node. Mirrors node_flags.
CREATE TABLE node_checks (
    id           TEXT        PRIMARY KEY,
    request_id   TEXT        NOT NULL REFERENCES requests(id) ON DELETE CASCADE,
    node_id      TEXT        NOT NULL REFERENCES workflow_nodes(id) ON DELETE CASCADE,
    label        TEXT        NOT NULL,
    status       TEXT        NOT NULL DEFAULT 'pass'
                             CHECK (status IN ('pass', 'warn', 'fail')),
    detail       TEXT        NOT NULL DEFAULT '',
    policy_title TEXT        NOT NULL DEFAULT '',
    ordinal      INT         NOT NULL DEFAULT 0,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_node_checks_node ON node_checks(node_id);
CREATE INDEX idx_node_checks_request ON node_checks(request_id);

-- migrate:down
DROP TABLE IF EXISTS node_checks;
ALTER TABLE department_policies DROP COLUMN rules;
