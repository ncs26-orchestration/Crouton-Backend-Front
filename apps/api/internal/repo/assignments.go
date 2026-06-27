package repo

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// NodeAssignment links a user to a workflow node they must verify before the
// workflow advances past it.
type NodeAssignment struct {
	ID         string
	RequestID  string
	NodeID     string
	UserID     int64
	AssignedBy *int64
	CreatedAt  time.Time
}

// NodeAssignmentWithUser is an assignment plus the assignee's display fields,
// for rendering avatars/chips.
type NodeAssignmentWithUser struct {
	ID        string `json:"id"`
	NodeID    string `json:"node_id"`
	UserID    int64  `json:"user_id"`
	UserName  string `json:"user_name"`
	UserEmail string `json:"user_email"`
}

type AssignmentRepo struct {
	pg *pgxpool.Pool
}

func NewAssignmentRepo(pg *pgxpool.Pool) *AssignmentRepo {
	return &AssignmentRepo{pg: pg}
}

// Create inserts an assignment, ignoring a duplicate (node, user) pair.
func (r *AssignmentRepo) Create(ctx context.Context, a NodeAssignment) error {
	_, err := r.pg.Exec(ctx, `
		INSERT INTO node_assignments (id, request_id, node_id, user_id, assigned_by)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (node_id, user_id) DO NOTHING
	`, a.ID, a.RequestID, a.NodeID, a.UserID, a.AssignedBy)
	return err
}

// Delete removes an assignment scoped to its request.
func (r *AssignmentRepo) Delete(ctx context.Context, id, requestID string) error {
	_, err := r.pg.Exec(ctx, `DELETE FROM node_assignments WHERE id = $1 AND request_id = $2`, id, requestID)
	return err
}

// ListByRequest returns all assignments for a request with assignee display fields.
func (r *AssignmentRepo) ListByRequest(ctx context.Context, requestID string) ([]NodeAssignmentWithUser, error) {
	rows, err := r.pg.Query(ctx, `
		SELECT na.id, na.node_id, na.user_id, COALESCE(u.name, ''), u.email
		FROM node_assignments na
		JOIN users u ON u.id = na.user_id
		WHERE na.request_id = $1
		ORDER BY na.created_at ASC
	`, requestID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]NodeAssignmentWithUser, 0)
	for rows.Next() {
		var a NodeAssignmentWithUser
		if err := rows.Scan(&a.ID, &a.NodeID, &a.UserID, &a.UserName, &a.UserEmail); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// CountByNode returns how many verifiers are assigned to a node. The engine uses
// it to decide whether a node pauses for review.
func (r *AssignmentRepo) CountByNode(ctx context.Context, nodeID string) (int, error) {
	var n int
	err := r.pg.QueryRow(ctx, `SELECT count(*) FROM node_assignments WHERE node_id = $1`, nodeID).Scan(&n)
	return n, err
}

// IsAssigned reports whether a user is assigned to a node.
func (r *AssignmentRepo) IsAssigned(ctx context.Context, nodeID string, userID int64) (bool, error) {
	var ok bool
	err := r.pg.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM node_assignments WHERE node_id = $1 AND user_id = $2)`,
		nodeID, userID,
	).Scan(&ok)
	return ok, err
}

// UserInOrg reports whether a user is a member of the org.
func (r *AssignmentRepo) UserInOrg(ctx context.Context, orgID string, userID int64) (bool, error) {
	var ok bool
	err := r.pg.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM org_members WHERE org_id = $1 AND user_id = $2)`,
		orgID, userID,
	).Scan(&ok)
	return ok, err
}

// NodeVerification is a node parked at awaiting_review that a given user is
// allowed to sign off, with enough context to render it in their work queue.
type NodeVerification struct {
	NodeID       string `json:"node_id"`
	RequestID    string `json:"request_id"`
	NodeName     string `json:"node_name"`
	Department   string `json:"department"`
	RequestTitle string `json:"request_title"`
	AssignedToMe bool   `json:"assigned_to_me"`
}

// ListVerificationsForUser returns the awaiting_review nodes in an org that the
// user may verify: nodes assigned to them, nodes in their department, and (when
// isAdmin) every awaiting_review node. Mirrors the VerifyNode RBAC.
func (r *AssignmentRepo) ListVerificationsForUser(ctx context.Context, orgID string, userID int64, isAdmin bool) ([]NodeVerification, error) {
	rows, err := r.pg.Query(ctx, `
		SELECT wn.id, wn.request_id, wn.name, wn.department, r.title,
			EXISTS(SELECT 1 FROM node_assignments na2 WHERE na2.node_id = wn.id AND na2.user_id = $2) AS assigned_to_me
		FROM workflow_nodes wn
		JOIN requests r ON r.id = wn.request_id
		WHERE r.org_id = $1
			AND wn.status = 'awaiting_review'
			AND (
				$3
				OR EXISTS(SELECT 1 FROM node_assignments na WHERE na.node_id = wn.id AND na.user_id = $2)
				OR EXISTS(
					SELECT 1 FROM team_members tm
					JOIN teams t ON t.id = tm.team_id
					WHERE tm.user_id = $2 AND t.org_id = $1 AND LOWER(t.name) = LOWER(wn.department)
				)
			)
		ORDER BY wn.started_at ASC NULLS LAST
	`, orgID, userID, isAdmin)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]NodeVerification, 0)
	for rows.Next() {
		var v NodeVerification
		if err := rows.Scan(&v.NodeID, &v.RequestID, &v.NodeName, &v.Department, &v.RequestTitle, &v.AssignedToMe); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

// UserInDepartment reports whether a user belongs to the team whose name matches
// a node's department, within an org. Used for RBAC on verification.
func (r *AssignmentRepo) UserInDepartment(ctx context.Context, orgID string, userID int64, department string) (bool, error) {
	var ok bool
	err := r.pg.QueryRow(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM team_members tm
			JOIN teams t ON t.id = tm.team_id
			WHERE tm.user_id = $1 AND t.org_id = $2 AND LOWER(t.name) = LOWER($3)
		)
	`, userID, orgID, department).Scan(&ok)
	return ok, err
}
