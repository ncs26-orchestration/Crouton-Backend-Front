package repo

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Machine struct {
	ID             string
	OrgID          string
	AssignedUserID *int64
	Name           string
	MachineType    string
	Location       string
	SerialNumber   string
	Status         string
	Metadata       []byte
	LastServiceAt  *time.Time
	NextServiceDue *time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type MachineRepo struct {
	pg *pgxpool.Pool
}

func NewMachineRepo(pg *pgxpool.Pool) *MachineRepo {
	return &MachineRepo{pg: pg}
}

func (r *MachineRepo) Create(ctx context.Context, m Machine) (*Machine, error) {
	row := r.pg.QueryRow(ctx, `
		INSERT INTO machines (id, org_id, assigned_user_id, name, machine_type, location, serial_number, status, metadata, last_service_at, next_service_due)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		RETURNING id, org_id, assigned_user_id, name, machine_type, location, serial_number, status, metadata, last_service_at, next_service_due, created_at, updated_at
	`, m.ID, m.OrgID, m.AssignedUserID, m.Name, m.MachineType, m.Location, m.SerialNumber, m.Status, m.Metadata, m.LastServiceAt, m.NextServiceDue)
	var out Machine
	if err := row.Scan(
		&out.ID, &out.OrgID, &out.AssignedUserID, &out.Name, &out.MachineType,
		&out.Location, &out.SerialNumber, &out.Status, &out.Metadata,
		&out.LastServiceAt, &out.NextServiceDue, &out.CreatedAt, &out.UpdatedAt,
	); err != nil {
		return nil, err
	}
	return &out, nil
}

func (r *MachineRepo) GetByID(ctx context.Context, id string) (*Machine, error) {
	row := r.pg.QueryRow(ctx, `
		SELECT id, org_id, assigned_user_id, name, machine_type, location, serial_number, status, metadata, last_service_at, next_service_due, created_at, updated_at
		FROM machines
		WHERE id = $1
	`, id)
	var m Machine
	if err := row.Scan(
		&m.ID, &m.OrgID, &m.AssignedUserID, &m.Name, &m.MachineType,
		&m.Location, &m.SerialNumber, &m.Status, &m.Metadata,
		&m.LastServiceAt, &m.NextServiceDue, &m.CreatedAt, &m.UpdatedAt,
	); err != nil {
		if err == pgx.ErrNoRows {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &m, nil
}

type ListMachinesFilter struct {
	Status       string
	AssignedToMe *int64
	TeamID       string
}

func (r *MachineRepo) ListByOrg(ctx context.Context, orgID string, filter ListMachinesFilter) ([]Machine, error) {
	query := `SELECT id, org_id, assigned_user_id, name, machine_type, location, serial_number, status, metadata, last_service_at, next_service_due, created_at, updated_at
		FROM machines
		WHERE org_id = $1`
	args := []any{orgID}

	if filter.Status != "" {
		args = append(args, filter.Status)
		query += ` AND status = $` + fmt.Sprint(len(args))
	}
	if filter.AssignedToMe != nil {
		args = append(args, *filter.AssignedToMe)
		query += ` AND assigned_user_id = $` + fmt.Sprint(len(args))
	}

	query += ` ORDER BY created_at DESC`

	rows, err := r.pg.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Machine
	for rows.Next() {
		var m Machine
		if err := rows.Scan(
			&m.ID, &m.OrgID, &m.AssignedUserID, &m.Name, &m.MachineType,
			&m.Location, &m.SerialNumber, &m.Status, &m.Metadata,
			&m.LastServiceAt, &m.NextServiceDue, &m.CreatedAt, &m.UpdatedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

func (r *MachineRepo) Update(ctx context.Context, m Machine) error {
	_, err := r.pg.Exec(ctx, `
		UPDATE machines
		SET name = $2, machine_type = $3, location = $4, serial_number = $5,
		    status = $6, assigned_user_id = $7, metadata = $8,
		    last_service_at = $9, next_service_due = $10,
		    updated_at = now()
		WHERE id = $1
	`, m.ID, m.Name, m.MachineType, m.Location, m.SerialNumber,
		m.Status, m.AssignedUserID, m.Metadata,
		m.LastServiceAt, m.NextServiceDue)
	return err
}
