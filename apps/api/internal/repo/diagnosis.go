package repo

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type MachineDocument struct {
	ID            string
	MachineID     string
	OrgID         string
	UploadedBy    int64
	Filename      string
	ContentType   string
	FileSizeBytes int64
	DocType       string
	ExtractedText string
	CreatedAt     time.Time
}

type DiagnosisRecord struct {
	ID          string
	IncidentID  string
	MachineID   string
	AgentModel  string
	Summary     string
	RootCause   *string
	Status      string
	CreatedAt   time.Time
	CompletedAt *time.Time
}

type DiagnosisStep struct {
	ID              string
	DiagnosisID     string
	StepOrder       int
	Title           string
	Description     string
	ActionType      string
	ExpectedOutcome *string
	Warning         *string
	Status          string
	Notes           *string
	CompletedAt     *time.Time
	CreatedAt       time.Time
}

type DiagnosisRepo struct {
	pg *pgxpool.Pool
}

func NewDiagnosisRepo(pg *pgxpool.Pool) *DiagnosisRepo {
	return &DiagnosisRepo{pg: pg}
}

// --- Machine Documents ---

func (r *DiagnosisRepo) InsertDocument(ctx context.Context, doc MachineDocument) (*MachineDocument, error) {
	row := r.pg.QueryRow(ctx, `
		INSERT INTO machine_documents (id, machine_id, org_id, uploaded_by, filename, content_type, file_size_bytes, doc_type, extracted_text)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id, machine_id, org_id, uploaded_by, filename, content_type, file_size_bytes, doc_type, extracted_text, created_at
	`, doc.ID, doc.MachineID, doc.OrgID, doc.UploadedBy, doc.Filename, doc.ContentType, doc.FileSizeBytes, doc.DocType, doc.ExtractedText)
	var out MachineDocument
	if err := row.Scan(
		&out.ID, &out.MachineID, &out.OrgID, &out.UploadedBy,
		&out.Filename, &out.ContentType, &out.FileSizeBytes, &out.DocType,
		&out.ExtractedText, &out.CreatedAt,
	); err != nil {
		return nil, err
	}
	return &out, nil
}

func (r *DiagnosisRepo) ListDocumentsByMachine(ctx context.Context, machineID string) ([]MachineDocument, error) {
	rows, err := r.pg.Query(ctx, `
		SELECT id, machine_id, org_id, uploaded_by, filename, content_type, file_size_bytes, doc_type, extracted_text, created_at
		FROM machine_documents
		WHERE machine_id = $1
		ORDER BY created_at DESC
	`, machineID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []MachineDocument
	for rows.Next() {
		var doc MachineDocument
		if err := rows.Scan(
			&doc.ID, &doc.MachineID, &doc.OrgID, &doc.UploadedBy,
			&doc.Filename, &doc.ContentType, &doc.FileSizeBytes, &doc.DocType,
			&doc.ExtractedText, &doc.CreatedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, doc)
	}
	return out, rows.Err()
}

// --- Incident Diagnoses ---

func (r *DiagnosisRepo) CreateDiagnosis(ctx context.Context, diag DiagnosisRecord) (*DiagnosisRecord, error) {
	row := r.pg.QueryRow(ctx, `
		INSERT INTO incident_diagnoses (id, incident_id, machine_id, agent_model, summary, root_cause, status)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, incident_id, machine_id, agent_model, summary, root_cause, status, created_at, completed_at
	`, diag.ID, diag.IncidentID, diag.MachineID, diag.AgentModel, diag.Summary, diag.RootCause, diag.Status)
	var out DiagnosisRecord
	if err := row.Scan(
		&out.ID, &out.IncidentID, &out.MachineID, &out.AgentModel,
		&out.Summary, &out.RootCause, &out.Status,
		&out.CreatedAt, &out.CompletedAt,
	); err != nil {
		return nil, err
	}
	return &out, nil
}

func (r *DiagnosisRepo) GetDiagnosisByIncident(ctx context.Context, incidentID string) (*DiagnosisRecord, error) {
	row := r.pg.QueryRow(ctx, `
		SELECT id, incident_id, machine_id, agent_model, summary, root_cause, status, created_at, completed_at
		FROM incident_diagnoses
		WHERE incident_id = $1
	`, incidentID)
	var out DiagnosisRecord
	if err := row.Scan(
		&out.ID, &out.IncidentID, &out.MachineID, &out.AgentModel,
		&out.Summary, &out.RootCause, &out.Status,
		&out.CreatedAt, &out.CompletedAt,
	); err != nil {
		if err == pgx.ErrNoRows {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &out, nil
}

func (r *DiagnosisRepo) CompleteDiagnosis(ctx context.Context, diagnosisID string) error {
	tag, err := r.pg.Exec(ctx, `
		UPDATE incident_diagnoses
		SET status = 'completed', completed_at = now()
		WHERE id = $1 AND status != 'completed'
	`, diagnosisID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// --- Diagnosis Steps ---

func (r *DiagnosisRepo) InsertSteps(ctx context.Context, steps []DiagnosisStep) error {
	for _, s := range steps {
		_, err := r.pg.Exec(ctx, `
			INSERT INTO diagnosis_steps (id, diagnosis_id, step_order, title, description, action_type, expected_outcome, warning, status)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		`, s.ID, s.DiagnosisID, s.StepOrder, s.Title, s.Description, s.ActionType, s.ExpectedOutcome, s.Warning, s.Status)
		if err != nil {
			return err
		}
	}
	return nil
}

func (r *DiagnosisRepo) ListSteps(ctx context.Context, diagnosisID string) ([]DiagnosisStep, error) {
	rows, err := r.pg.Query(ctx, `
		SELECT id, diagnosis_id, step_order, title, description, action_type, expected_outcome, warning, status, notes, completed_at, created_at
		FROM diagnosis_steps
		WHERE diagnosis_id = $1
		ORDER BY step_order ASC
	`, diagnosisID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []DiagnosisStep
	for rows.Next() {
		var s DiagnosisStep
		if err := rows.Scan(
			&s.ID, &s.DiagnosisID, &s.StepOrder, &s.Title,
			&s.Description, &s.ActionType, &s.ExpectedOutcome, &s.Warning,
			&s.Status, &s.Notes, &s.CompletedAt, &s.CreatedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

func (r *DiagnosisRepo) CompleteStep(ctx context.Context, stepID string, notes string) error {
	tag, err := r.pg.Exec(ctx, `
		UPDATE diagnosis_steps
		SET status = 'completed', notes = $2, completed_at = now()
		WHERE id = $1 AND status != 'completed'
	`, stepID, notes)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
