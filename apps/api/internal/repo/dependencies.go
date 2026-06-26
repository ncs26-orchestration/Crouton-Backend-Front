package repo

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type NodeDependency struct {
	ID              string
	RequestID       string
	DependentNodeID string
	BlockingNodeID  string
	Reason          string
	RunCount        int
	ResolvedAt      *time.Time
	CreatedAt       time.Time
}

type DependencyRepo struct {
	pg *pgxpool.Pool
}

func NewDependencyRepo(pg *pgxpool.Pool) *DependencyRepo {
	return &DependencyRepo{pg: pg}
}

func (r *DependencyRepo) Insert(ctx context.Context, dep NodeDependency) error {
	_, err := r.pg.Exec(ctx, `
		INSERT INTO node_dependencies (id, request_id, dependent_node_id, blocking_node_id, reason, run_count)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, dep.ID, dep.RequestID, dep.DependentNodeID, dep.BlockingNodeID, dep.Reason, dep.RunCount)
	return err
}

func (r *DependencyRepo) ListUnresolvedByDependent(ctx context.Context, dependentNodeID string) ([]NodeDependency, error) {
	rows, err := r.pg.Query(ctx, `
		SELECT id, request_id, dependent_node_id, blocking_node_id, reason, run_count, resolved_at, created_at
		FROM node_dependencies
		WHERE dependent_node_id = $1 AND resolved_at IS NULL
		ORDER BY created_at ASC
	`, dependentNodeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []NodeDependency
	for rows.Next() {
		var d NodeDependency
		if err := rows.Scan(&d.ID, &d.RequestID, &d.DependentNodeID, &d.BlockingNodeID,
			&d.Reason, &d.RunCount, &d.ResolvedAt, &d.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

func (r *DependencyRepo) ListUnresolvedByRequest(ctx context.Context, requestID string) ([]NodeDependency, error) {
	rows, err := r.pg.Query(ctx, `
		SELECT id, request_id, dependent_node_id, blocking_node_id, reason, run_count, resolved_at, created_at
		FROM node_dependencies
		WHERE request_id = $1 AND resolved_at IS NULL
		ORDER BY created_at ASC
	`, requestID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []NodeDependency
	for rows.Next() {
		var d NodeDependency
		if err := rows.Scan(&d.ID, &d.RequestID, &d.DependentNodeID, &d.BlockingNodeID,
			&d.Reason, &d.RunCount, &d.ResolvedAt, &d.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

func (r *DependencyRepo) ResolveByBlockingNode(ctx context.Context, blockingNodeID string) ([]string, error) {
	rows, err := r.pg.Query(ctx, `
		UPDATE node_dependencies
		SET resolved_at = now()
		WHERE blocking_node_id = $1 AND resolved_at IS NULL
		RETURNING dependent_node_id
	`, blockingNodeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var unblocked []string
	for rows.Next() {
		var depID string
		if err := rows.Scan(&depID); err != nil {
			return nil, err
		}
		unblocked = append(unblocked, depID)
	}
	return unblocked, rows.Err()
}

func (r *DependencyRepo) IncrementRunCount(ctx context.Context, dependentNodeID string) error {
	_, err := r.pg.Exec(ctx, `
		UPDATE node_dependencies
		SET run_count = run_count + 1
		WHERE dependent_node_id = $1 AND resolved_at IS NOT NULL
	`, dependentNodeID)
	return err
}

func (r *DependencyRepo) MaxRunCount(ctx context.Context, dependentNodeID string) (int, error) {
	var maxCount int
	err := r.pg.QueryRow(ctx, `
		SELECT COALESCE(MAX(run_count), 0)
		FROM node_dependencies
		WHERE dependent_node_id = $1
	`, dependentNodeID).Scan(&maxCount)
	if err != nil {
		if err == pgx.ErrNoRows {
			return 0, nil
		}
		return 0, err
	}
	return maxCount, nil
}
