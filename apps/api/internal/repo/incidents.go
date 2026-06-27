package repo

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Incident struct {
	ID              string
	MachineID       string
	OrgID           string
	ReportedBy      *int64
	Title           string
	Description     string
	Severity        string
	Status          string
	ResolvedAt      *time.Time
	ResolutionNotes string
	CreatedAt       time.Time
}

type IncidentMessage struct {
	ID         string
	IncidentID string
	SenderID   *int64
	SenderName string
	SenderRole string
	Content    string
	CreatedAt  time.Time
}

type IncidentRepo struct {
	pg *pgxpool.Pool
}

func NewIncidentRepo(pg *pgxpool.Pool) *IncidentRepo {
	return &IncidentRepo{pg: pg}
}

func (r *IncidentRepo) Create(ctx context.Context, inc Incident) (*Incident, error) {
	row := r.pg.QueryRow(ctx, `
		INSERT INTO incidents (id, machine_id, org_id, reported_by, title, description, severity, status)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id, machine_id, org_id, reported_by, title, description, severity, status, resolved_at, COALESCE(resolution_notes, ''), created_at
	`, inc.ID, inc.MachineID, inc.OrgID, inc.ReportedBy, inc.Title, inc.Description, inc.Severity, inc.Status)
	var out Incident
	if err := row.Scan(
		&out.ID, &out.MachineID, &out.OrgID, &out.ReportedBy,
		&out.Title, &out.Description, &out.Severity, &out.Status,
		&out.ResolvedAt, &out.ResolutionNotes, &out.CreatedAt,
	); err != nil {
		return nil, err
	}
	return &out, nil
}

func (r *IncidentRepo) GetByID(ctx context.Context, id string) (*Incident, error) {
	row := r.pg.QueryRow(ctx, `
		SELECT id, machine_id, org_id, reported_by, title, description, severity, status, resolved_at, COALESCE(resolution_notes, ''), created_at
		FROM incidents
		WHERE id = $1
	`, id)
	var out Incident
	if err := row.Scan(
		&out.ID, &out.MachineID, &out.OrgID, &out.ReportedBy,
		&out.Title, &out.Description, &out.Severity, &out.Status,
		&out.ResolvedAt, &out.ResolutionNotes, &out.CreatedAt,
	); err != nil {
		if err == pgx.ErrNoRows {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &out, nil
}

func (r *IncidentRepo) ListByMachine(ctx context.Context, machineID string) ([]Incident, error) {
	rows, err := r.pg.Query(ctx, `
		SELECT id, machine_id, org_id, reported_by, title, description, severity, status, resolved_at, COALESCE(resolution_notes, ''), created_at
		FROM incidents
		WHERE machine_id = $1
		ORDER BY created_at DESC
	`, machineID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Incident
	for rows.Next() {
		var inc Incident
		if err := rows.Scan(
			&inc.ID, &inc.MachineID, &inc.OrgID, &inc.ReportedBy,
			&inc.Title, &inc.Description, &inc.Severity, &inc.Status,
			&inc.ResolvedAt, &inc.ResolutionNotes, &inc.CreatedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, inc)
	}
	return out, rows.Err()
}

func (r *IncidentRepo) ListByOrg(ctx context.Context, orgID string) ([]Incident, error) {
	rows, err := r.pg.Query(ctx, `
		SELECT id, machine_id, org_id, reported_by, title, description, severity, status, resolved_at, COALESCE(resolution_notes, ''), created_at
		FROM incidents
		WHERE org_id = $1
		ORDER BY created_at DESC
	`, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Incident
	for rows.Next() {
		var inc Incident
		if err := rows.Scan(
			&inc.ID, &inc.MachineID, &inc.OrgID, &inc.ReportedBy,
			&inc.Title, &inc.Description, &inc.Severity, &inc.Status,
			&inc.ResolvedAt, &inc.ResolutionNotes, &inc.CreatedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, inc)
	}
	return out, rows.Err()
}

func (r *IncidentRepo) Resolve(ctx context.Context, id string, notes string) error {
	tag, err := r.pg.Exec(ctx, `
		UPDATE incidents
		SET status = 'resolved', resolved_at = now(), resolution_notes = $2
		WHERE id = $1 AND status != 'resolved'
	`, id, notes)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// --- Messages ---

func (r *IncidentRepo) AppendMessage(ctx context.Context, msg IncidentMessage) (*IncidentMessage, error) {
	row := r.pg.QueryRow(ctx, `
		INSERT INTO incident_messages (id, incident_id, sender_id, sender_name, sender_role, content)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, incident_id, sender_id, sender_name, sender_role, content, created_at
	`, msg.ID, msg.IncidentID, msg.SenderID, msg.SenderName, msg.SenderRole, msg.Content)
	var out IncidentMessage
	if err := row.Scan(
		&out.ID, &out.IncidentID, &out.SenderID, &out.SenderName, &out.SenderRole, &out.Content, &out.CreatedAt,
	); err != nil {
		return nil, err
	}
	return &out, nil
}

func (r *IncidentRepo) ListMessages(ctx context.Context, incidentID string) ([]IncidentMessage, error) {
	rows, err := r.pg.Query(ctx, `
		SELECT id, incident_id, sender_id, sender_name, sender_role, content, created_at
		FROM incident_messages
		WHERE incident_id = $1
		ORDER BY created_at ASC
	`, incidentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []IncidentMessage
	for rows.Next() {
		var m IncidentMessage
		if err := rows.Scan(
			&m.ID, &m.IncidentID, &m.SenderID, &m.SenderName, &m.SenderRole, &m.Content, &m.CreatedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}
