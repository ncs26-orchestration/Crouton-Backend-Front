package repo

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Telemetry struct {
	ID        string
	MachineID string
	Metrics   []byte
	ErrorCode string
	Source    string
	CreatedAt time.Time
}

type TelemetryRepo struct {
	pg *pgxpool.Pool
}

func NewTelemetryRepo(pg *pgxpool.Pool) *TelemetryRepo {
	return &TelemetryRepo{pg: pg}
}

func (r *TelemetryRepo) Insert(ctx context.Context, t Telemetry) (*Telemetry, error) {
	row := r.pg.QueryRow(ctx, `
		INSERT INTO machine_telemetry (id, machine_id, metrics, error_code, source)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, machine_id, metrics, error_code, source, created_at
	`, t.ID, t.MachineID, t.Metrics, t.ErrorCode, t.Source)
	var out Telemetry
	if err := row.Scan(&out.ID, &out.MachineID, &out.Metrics, &out.ErrorCode, &out.Source, &out.CreatedAt); err != nil {
		return nil, err
	}
	return &out, nil
}

func (r *TelemetryRepo) ListByMachine(ctx context.Context, machineID string, limit int) ([]Telemetry, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := r.pg.Query(ctx, `
		SELECT id, machine_id, metrics, error_code, source, created_at
		FROM machine_telemetry
		WHERE machine_id = $1
		ORDER BY created_at DESC
		LIMIT $2
	`, machineID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Telemetry
	for rows.Next() {
		var t Telemetry
		if err := rows.Scan(&t.ID, &t.MachineID, &t.Metrics, &t.ErrorCode, &t.Source, &t.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// ListByMachineSince returns telemetry entries after a given timestamp.
func (r *TelemetryRepo) ListByMachineSince(ctx context.Context, machineID string, since time.Time) ([]Telemetry, error) {
	rows, err := r.pg.Query(ctx, `
		SELECT id, machine_id, metrics, error_code, source, created_at
		FROM machine_telemetry
		WHERE machine_id = $1 AND created_at > $2
		ORDER BY created_at DESC
	`, machineID, since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Telemetry
	for rows.Next() {
		var t Telemetry
		if err := rows.Scan(&t.ID, &t.MachineID, &t.Metrics, &t.ErrorCode, &t.Source, &t.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}
