-- migrate:up

-- The IS Registry projection. Everything below is read-only from
-- AUP's perspective: rows are written only by the discovery adapter
-- (apps/api/internal/engine/*) when syncing from an external source of
-- truth. The UI and public API never insert into these tables.
--
-- Layout principles:
--   * Everything is tenant-scoped.
--   * Projected entities carry both a stable AUP-side surrogate id
--     (BIGSERIAL) and the external (engine-side) id that was pulled.
--   * `(engine_connection_id, external_id)` is the uniqueness guarantee
--     that makes re-sync idempotent.
--   * `declared_systems` and `declared_capabilities` describe systems
--     AUP cannot auto-discover (OpenBee, Odoo, M365...): they are
--     tenant-declared capability catalogs. No identities live here.

CREATE TABLE IF NOT EXISTS tenants (
    id          TEXT PRIMARY KEY,
    name        TEXT NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS engine_connections (
    id                BIGSERIAL PRIMARY KEY,
    tenant_id         TEXT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    external_id       TEXT NOT NULL,
    kind              TEXT NOT NULL CHECK (kind IN ('camunda7', 'camunda8', 'elsa3')),
    endpoint          TEXT NOT NULL,
    auth_kind         TEXT NOT NULL DEFAULT 'basic' CHECK (auth_kind IN ('basic', 'bearer', 'none')),
    auth_username     TEXT,
    auth_secret_ref   TEXT,
    last_synced_at    TIMESTAMPTZ,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (tenant_id, external_id)
);

CREATE INDEX IF NOT EXISTS idx_engine_connections_tenant ON engine_connections(tenant_id);

CREATE TABLE IF NOT EXISTS projected_users (
    id                     BIGSERIAL PRIMARY KEY,
    engine_connection_id   BIGINT NOT NULL REFERENCES engine_connections(id) ON DELETE CASCADE,
    external_id            TEXT NOT NULL,
    display_name           TEXT,
    email                  TEXT,
    last_seen_at           TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (engine_connection_id, external_id)
);

CREATE INDEX IF NOT EXISTS idx_projected_users_engine ON projected_users(engine_connection_id);

CREATE TABLE IF NOT EXISTS projected_groups (
    id                     BIGSERIAL PRIMARY KEY,
    engine_connection_id   BIGINT NOT NULL REFERENCES engine_connections(id) ON DELETE CASCADE,
    external_id            TEXT NOT NULL,
    display_name           TEXT,
    last_seen_at           TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (engine_connection_id, external_id)
);

CREATE INDEX IF NOT EXISTS idx_projected_groups_engine ON projected_groups(engine_connection_id);

CREATE TABLE IF NOT EXISTS projected_group_members (
    engine_connection_id   BIGINT NOT NULL REFERENCES engine_connections(id) ON DELETE CASCADE,
    group_external_id      TEXT NOT NULL,
    user_external_id       TEXT NOT NULL,
    PRIMARY KEY (engine_connection_id, group_external_id, user_external_id)
);

CREATE INDEX IF NOT EXISTS idx_projected_group_members_group ON projected_group_members(engine_connection_id, group_external_id);

CREATE TABLE IF NOT EXISTS projected_forms (
    id                     BIGSERIAL PRIMARY KEY,
    engine_connection_id   BIGINT NOT NULL REFERENCES engine_connections(id) ON DELETE CASCADE,
    form_key               TEXT NOT NULL,
    source_resource        TEXT,
    last_seen_at           TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (engine_connection_id, form_key)
);

-- Capability catalog. Global across tenants: every instance of OpenBee
-- exposes the same family of capabilities. Tenants then reference
-- these via declared_systems.
CREATE TABLE IF NOT EXISTS capability_catalog (
    id            BIGSERIAL PRIMARY KEY,
    system_kind   TEXT NOT NULL,
    capability    TEXT NOT NULL,
    description   TEXT,
    UNIQUE (system_kind, capability)
);

-- Systems declared by a tenant (OpenBee instance, Odoo tenant...).
-- Identities don't live here; only the fact that the system exists
-- and the subset of catalog capabilities it exposes.
CREATE TABLE IF NOT EXISTS declared_systems (
    id           BIGSERIAL PRIMARY KEY,
    tenant_id    TEXT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    external_id  TEXT NOT NULL,
    name         TEXT,
    kind         TEXT NOT NULL CHECK (kind IN ('ecm','erp','comms','idp','crm','signer','other')),
    endpoint     TEXT,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (tenant_id, external_id)
);

CREATE TABLE IF NOT EXISTS declared_system_capabilities (
    declared_system_id  BIGINT NOT NULL REFERENCES declared_systems(id) ON DELETE CASCADE,
    capability          TEXT NOT NULL,
    PRIMARY KEY (declared_system_id, capability)
);

-- Seed: catalog rows always available; tenants opt into them when they
-- declare a system of the matching kind.
INSERT INTO capability_catalog (system_kind, capability, description) VALUES
    ('ecm',   'document.store',      'Store a document at a target path'),
    ('ecm',   'document.archive',    'Archive a finalized document'),
    ('ecm',   'document.sign',       'Route a document for signature'),
    ('erp',   'expense.create',      'Create an expense record'),
    ('erp',   'expense.approve',     'Mark an expense record approved'),
    ('erp',   'invoice.emit',        'Emit an invoice to a customer'),
    ('comms', 'user.notify.email',   'Send an email notification to a user'),
    ('comms', 'user.notify.teams',   'Post a Teams message to a user'),
    ('comms', 'calendar.schedule',   'Schedule a calendar event'),
    ('idp',   'user.resolve',        'Resolve a name/email to an IdP user'),
    ('idp',   'group.resolve',       'Resolve a role name to an IdP group')
ON CONFLICT (system_kind, capability) DO NOTHING;

-- Demo tenant so bootstrap works out of the box.
INSERT INTO tenants (id, name) VALUES ('demo', 'Demo tenant')
ON CONFLICT (id) DO NOTHING;

-- migrate:down

DROP TABLE IF EXISTS declared_system_capabilities;
DROP TABLE IF EXISTS declared_systems;
DROP TABLE IF EXISTS capability_catalog;
DROP TABLE IF EXISTS projected_forms;
DROP TABLE IF EXISTS projected_group_members;
DROP TABLE IF EXISTS projected_groups;
DROP TABLE IF EXISTS projected_users;
DROP TABLE IF EXISTS engine_connections;
DROP TABLE IF EXISTS tenants;
