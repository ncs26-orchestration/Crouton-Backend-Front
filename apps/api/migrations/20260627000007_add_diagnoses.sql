-- migrate:up

-- Machine documents — manuals, spec sheets, and maintenance guides uploaded
-- during onboarding. The agent reads extracted_text when diagnosing incidents.
CREATE TABLE IF NOT EXISTS machine_documents (
    id              TEXT        PRIMARY KEY,
    machine_id      TEXT        NOT NULL REFERENCES machines(id) ON DELETE CASCADE,
    org_id          TEXT        NOT NULL REFERENCES organizations(id),
    uploaded_by     BIGINT      NOT NULL REFERENCES users(id),
    filename        TEXT        NOT NULL,
    content_type    TEXT        NOT NULL DEFAULT 'application/pdf',
    file_size_bytes BIGINT      NOT NULL DEFAULT 0,
    doc_type        TEXT        NOT NULL DEFAULT 'manual'
                    CHECK (doc_type IN ('manual','spec_sheet','maintenance_guide','safety_sheet','other')),
    extracted_text  TEXT        NOT NULL DEFAULT '',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_machine_documents_machine ON machine_documents(machine_id);

-- AI-generated diagnostic plans for machine incidents.
-- When a technician requests a diagnosis, the agent produces an ordered list
-- of checkpoint steps the technician follows to resolve the issue.
CREATE TABLE IF NOT EXISTS incident_diagnoses (
    id            TEXT        PRIMARY KEY,
    incident_id   TEXT        NOT NULL REFERENCES incidents(id) ON DELETE CASCADE,
    machine_id    TEXT        NOT NULL REFERENCES machines(id) ON DELETE CASCADE,
    agent_model   TEXT        NOT NULL DEFAULT 'diagnostic',
    summary       TEXT        NOT NULL,
    root_cause    TEXT,
    status        TEXT        NOT NULL DEFAULT 'in_progress'
                  CHECK (status IN ('in_progress','completed','failed')),
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    completed_at  TIMESTAMPTZ
);

CREATE TABLE IF NOT EXISTS diagnosis_steps (
    id            TEXT        PRIMARY KEY,
    diagnosis_id  TEXT        NOT NULL REFERENCES incident_diagnoses(id) ON DELETE CASCADE,
    step_order    INT         NOT NULL,
    title         TEXT        NOT NULL,
    description   TEXT        NOT NULL,
    action_type   TEXT        NOT NULL DEFAULT 'check'
                  CHECK (action_type IN ('check','measure','replace','restart','calibrate','inspect','clean','test')),
    expected_outcome TEXT,
    warning       TEXT,
    status        TEXT        NOT NULL DEFAULT 'pending'
                  CHECK (status IN ('pending','in_progress','completed','skipped')),
    notes         TEXT,
    completed_at  TIMESTAMPTZ,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_diagnoses_incident ON incident_diagnoses(incident_id);
CREATE INDEX IF NOT EXISTS idx_diagnosis_steps_diagnosis ON diagnosis_steps(diagnosis_id);

-- migrate:down

DROP TABLE IF EXISTS diagnosis_steps;
DROP TABLE IF EXISTS incident_diagnoses;
DROP TABLE IF EXISTS machine_documents;
