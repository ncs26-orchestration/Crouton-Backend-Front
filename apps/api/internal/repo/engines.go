package repo

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// EngineConnection is one row from engine_connections.
type EngineConnection struct {
	ID           int64
	TenantID     string
	ExternalID   string // user-facing id, e.g. "prod-camunda"
	Kind         string // camunda7 | camunda8 | elsa3
	Endpoint     string
	AuthKind     string // basic | bearer | none
	AuthUsername string
	AuthSecret   string // retained in memory; NOT persisted as plaintext in v0.1
	LastSyncedAt *time.Time
}

// ProjectedUser / Group / Member / Form are the read-only projections.
type ProjectedUser struct {
	EngineConnectionID int64
	ExternalID         string
	DisplayName        string
	Email              string
}

type ProjectedGroup struct {
	EngineConnectionID int64
	ExternalID         string
	DisplayName        string
}

type ProjectedGroupMember struct {
	EngineConnectionID int64
	GroupExternalID    string
	UserExternalID     string
}

type ProjectedForm struct {
	EngineConnectionID int64
	FormKey            string
	SourceResource     string
}

// DeclaredSystem is a tenant-declared system with a capability catalog.
// Identities don't live here; only the endpoint + capabilities.
type DeclaredSystem struct {
	ID           int64
	TenantID     string
	ExternalID   string
	Name         string
	Kind         string
	Endpoint     string
	Capabilities []string
}

// EngineRepo encapsulates all SQL against the IS projection schema.
// Methods are safe to call concurrently; pgxpool handles that.
type EngineRepo struct {
	pg *pgxpool.Pool
}

func NewEngineRepo(pg *pgxpool.Pool) *EngineRepo {
	return &EngineRepo{pg: pg}
}

// ErrNotFound is returned when a lookup misses. Callers map it to a
// 404 at the HTTP boundary.
var ErrNotFound = errors.New("not found")

// UpsertEngineConnection inserts a new engine_connection or updates
// the endpoint/auth fields if (tenant_id, external_id) already exists.
// The secret is intentionally not persisted in v0.1 — we keep it in
// memory for the session and reconnect on restart. Future work: wire a
// vault.
func (r *EngineRepo) UpsertEngineConnection(ctx context.Context, ec *EngineConnection) (*EngineConnection, error) {
	row := r.pg.QueryRow(ctx, `
        INSERT INTO engine_connections
          (tenant_id, external_id, kind, endpoint, auth_kind, auth_username)
        VALUES ($1, $2, $3, $4, $5, $6)
        ON CONFLICT (tenant_id, external_id) DO UPDATE SET
          kind           = EXCLUDED.kind,
          endpoint       = EXCLUDED.endpoint,
          auth_kind      = EXCLUDED.auth_kind,
          auth_username  = EXCLUDED.auth_username
        RETURNING id, last_synced_at
    `, ec.TenantID, ec.ExternalID, ec.Kind, ec.Endpoint, ec.AuthKind, ec.AuthUsername)
	var lastSynced *time.Time
	if err := row.Scan(&ec.ID, &lastSynced); err != nil {
		return nil, fmt.Errorf("upsert engine_connection: %w", err)
	}
	ec.LastSyncedAt = lastSynced
	return ec, nil
}

func (r *EngineRepo) GetEngineConnection(ctx context.Context, tenantID, externalID string) (*EngineConnection, error) {
	row := r.pg.QueryRow(ctx, `
        SELECT id, kind, endpoint, auth_kind, COALESCE(auth_username,''), last_synced_at
        FROM engine_connections
        WHERE tenant_id = $1 AND external_id = $2
    `, tenantID, externalID)
	ec := &EngineConnection{TenantID: tenantID, ExternalID: externalID}
	if err := row.Scan(&ec.ID, &ec.Kind, &ec.Endpoint, &ec.AuthKind, &ec.AuthUsername, &ec.LastSyncedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return ec, nil
}

// DeleteEngineConnection removes an engine and cascades through the
// projected_* tables (FK ON DELETE CASCADE). Returns ErrNotFound when
// the engine does not exist for this tenant.
func (r *EngineRepo) DeleteEngineConnection(ctx context.Context, tenantID, externalID string) error {
	tag, err := r.pg.Exec(ctx, `
        DELETE FROM engine_connections WHERE tenant_id = $1 AND external_id = $2
    `, tenantID, externalID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *EngineRepo) ListEngineConnections(ctx context.Context, tenantID string) ([]EngineConnection, error) {
	rows, err := r.pg.Query(ctx, `
        SELECT id, external_id, kind, endpoint, auth_kind, COALESCE(auth_username,''), last_synced_at
        FROM engine_connections
        WHERE tenant_id = $1
        ORDER BY external_id
    `, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []EngineConnection
	for rows.Next() {
		ec := EngineConnection{TenantID: tenantID}
		if err := rows.Scan(&ec.ID, &ec.ExternalID, &ec.Kind, &ec.Endpoint, &ec.AuthKind, &ec.AuthUsername, &ec.LastSyncedAt); err != nil {
			return nil, err
		}
		out = append(out, ec)
	}
	return out, rows.Err()
}

// ReplaceProjection clears the projection for one engine connection and
// inserts the fresh snapshot in a single transaction. This is the only
// supported write path to projected_* tables — no partial/incremental
// sync in v0.1. The whole-snapshot semantics keep things predictable:
// deleting a user in the engine results in that user disappearing from
// AUP on the next sync, no stale rows.
func (r *EngineRepo) ReplaceProjection(
	ctx context.Context,
	ecID int64,
	users []ProjectedUser,
	groups []ProjectedGroup,
	members []ProjectedGroupMember,
	forms []ProjectedForm,
) error {
	tx, err := r.pg.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx, `DELETE FROM projected_group_members WHERE engine_connection_id = $1`, ecID); err != nil {
		return fmt.Errorf("clear projected_group_members: %w", err)
	}
	if _, err := tx.Exec(ctx, `DELETE FROM projected_groups WHERE engine_connection_id = $1`, ecID); err != nil {
		return fmt.Errorf("clear projected_groups: %w", err)
	}
	if _, err := tx.Exec(ctx, `DELETE FROM projected_users WHERE engine_connection_id = $1`, ecID); err != nil {
		return fmt.Errorf("clear projected_users: %w", err)
	}
	if _, err := tx.Exec(ctx, `DELETE FROM projected_forms WHERE engine_connection_id = $1`, ecID); err != nil {
		return fmt.Errorf("clear projected_forms: %w", err)
	}

	for _, u := range users {
		if _, err := tx.Exec(ctx, `
            INSERT INTO projected_users (engine_connection_id, external_id, display_name, email)
            VALUES ($1, $2, NULLIF($3,''), NULLIF($4,''))
        `, ecID, u.ExternalID, u.DisplayName, u.Email); err != nil {
			return fmt.Errorf("insert projected_user %s: %w", u.ExternalID, err)
		}
	}
	for _, g := range groups {
		if _, err := tx.Exec(ctx, `
            INSERT INTO projected_groups (engine_connection_id, external_id, display_name)
            VALUES ($1, $2, NULLIF($3,''))
        `, ecID, g.ExternalID, g.DisplayName); err != nil {
			return fmt.Errorf("insert projected_group %s: %w", g.ExternalID, err)
		}
	}
	for _, m := range members {
		if _, err := tx.Exec(ctx, `
            INSERT INTO projected_group_members (engine_connection_id, group_external_id, user_external_id)
            VALUES ($1, $2, $3)
        `, ecID, m.GroupExternalID, m.UserExternalID); err != nil {
			return fmt.Errorf("insert projected_group_member %s/%s: %w", m.GroupExternalID, m.UserExternalID, err)
		}
	}
	for _, f := range forms {
		if _, err := tx.Exec(ctx, `
            INSERT INTO projected_forms (engine_connection_id, form_key, source_resource)
            VALUES ($1, $2, NULLIF($3,''))
        `, ecID, f.FormKey, f.SourceResource); err != nil {
			return fmt.Errorf("insert projected_form %s: %w", f.FormKey, err)
		}
	}

	if _, err := tx.Exec(ctx, `UPDATE engine_connections SET last_synced_at = NOW() WHERE id = $1`, ecID); err != nil {
		return fmt.Errorf("stamp last_synced_at: %w", err)
	}
	return tx.Commit(ctx)
}

// ReadTenantProjection returns everything AUP knows about a tenant's
// IS — projected entities across all engine connections plus declared
// systems. Shaped for direct conversion into ir.ISRegistry at the
// service layer.
type TenantProjection struct {
	Tenant          string
	EngineConnections []EngineConnection
	Users           []ProjectedUser
	Groups          []ProjectedGroup
	Memberships     []ProjectedGroupMember
	DeployedForms   []ProjectedForm
	DeclaredSystems []DeclaredSystem
}

func (r *EngineRepo) ReadTenantProjection(ctx context.Context, tenantID string) (*TenantProjection, error) {
	out := &TenantProjection{Tenant: tenantID}

	ecs, err := r.ListEngineConnections(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	out.EngineConnections = ecs

	ecIDs := make([]int64, 0, len(ecs))
	for _, ec := range ecs {
		ecIDs = append(ecIDs, ec.ID)
	}

	if len(ecIDs) > 0 {
		// Users.
		rows, err := r.pg.Query(ctx, `
            SELECT engine_connection_id, external_id, COALESCE(display_name,''), COALESCE(email,'')
            FROM projected_users
            WHERE engine_connection_id = ANY($1)
            ORDER BY external_id
        `, ecIDs)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var u ProjectedUser
			if err := rows.Scan(&u.EngineConnectionID, &u.ExternalID, &u.DisplayName, &u.Email); err != nil {
				rows.Close()
				return nil, err
			}
			out.Users = append(out.Users, u)
		}
		rows.Close()

		// Groups.
		rows, err = r.pg.Query(ctx, `
            SELECT engine_connection_id, external_id, COALESCE(display_name,'')
            FROM projected_groups
            WHERE engine_connection_id = ANY($1)
            ORDER BY external_id
        `, ecIDs)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var g ProjectedGroup
			if err := rows.Scan(&g.EngineConnectionID, &g.ExternalID, &g.DisplayName); err != nil {
				rows.Close()
				return nil, err
			}
			out.Groups = append(out.Groups, g)
		}
		rows.Close()

		// Memberships.
		rows, err = r.pg.Query(ctx, `
            SELECT engine_connection_id, group_external_id, user_external_id
            FROM projected_group_members
            WHERE engine_connection_id = ANY($1)
            ORDER BY group_external_id, user_external_id
        `, ecIDs)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var m ProjectedGroupMember
			if err := rows.Scan(&m.EngineConnectionID, &m.GroupExternalID, &m.UserExternalID); err != nil {
				rows.Close()
				return nil, err
			}
			out.Memberships = append(out.Memberships, m)
		}
		rows.Close()

		// Forms.
		rows, err = r.pg.Query(ctx, `
            SELECT engine_connection_id, form_key, COALESCE(source_resource,'')
            FROM projected_forms
            WHERE engine_connection_id = ANY($1)
            ORDER BY form_key
        `, ecIDs)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var f ProjectedForm
			if err := rows.Scan(&f.EngineConnectionID, &f.FormKey, &f.SourceResource); err != nil {
				rows.Close()
				return nil, err
			}
			out.DeployedForms = append(out.DeployedForms, f)
		}
		rows.Close()
	}

	// Declared systems.
	rows, err := r.pg.Query(ctx, `
        SELECT ds.id, ds.external_id, COALESCE(ds.name,''), ds.kind, COALESCE(ds.endpoint,''),
               COALESCE(array_agg(dsc.capability ORDER BY dsc.capability) FILTER (WHERE dsc.capability IS NOT NULL), '{}') AS caps
        FROM declared_systems ds
        LEFT JOIN declared_system_capabilities dsc ON dsc.declared_system_id = ds.id
        WHERE ds.tenant_id = $1
        GROUP BY ds.id, ds.external_id, ds.name, ds.kind, ds.endpoint
        ORDER BY ds.external_id
    `, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var d DeclaredSystem
		d.TenantID = tenantID
		if err := rows.Scan(&d.ID, &d.ExternalID, &d.Name, &d.Kind, &d.Endpoint, &d.Capabilities); err != nil {
			return nil, err
		}
		out.DeclaredSystems = append(out.DeclaredSystems, d)
	}
	return out, rows.Err()
}

// UpsertDeclaredSystem inserts or updates a tenant-declared system
// and replaces its capability set. Used by the onboarding flow when a
// tenant declares OpenBee/Odoo/M365 etc.
func (r *EngineRepo) UpsertDeclaredSystem(ctx context.Context, ds *DeclaredSystem) (*DeclaredSystem, error) {
	tx, err := r.pg.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	row := tx.QueryRow(ctx, `
        INSERT INTO declared_systems (tenant_id, external_id, name, kind, endpoint)
        VALUES ($1, $2, NULLIF($3,''), $4, NULLIF($5,''))
        ON CONFLICT (tenant_id, external_id) DO UPDATE SET
          name     = EXCLUDED.name,
          kind     = EXCLUDED.kind,
          endpoint = EXCLUDED.endpoint
        RETURNING id
    `, ds.TenantID, ds.ExternalID, ds.Name, ds.Kind, ds.Endpoint)
	if err := row.Scan(&ds.ID); err != nil {
		return nil, fmt.Errorf("upsert declared_system: %w", err)
	}
	if _, err := tx.Exec(ctx, `DELETE FROM declared_system_capabilities WHERE declared_system_id = $1`, ds.ID); err != nil {
		return nil, err
	}
	for _, c := range ds.Capabilities {
		if _, err := tx.Exec(ctx, `
            INSERT INTO declared_system_capabilities (declared_system_id, capability)
            VALUES ($1, $2)
        `, ds.ID, c); err != nil {
			return nil, fmt.Errorf("insert capability %s: %w", c, err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return ds, nil
}
