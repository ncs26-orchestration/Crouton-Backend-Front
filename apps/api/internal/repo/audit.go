package repo

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// AuditEvent is an append-only record of a state change. Every transition a
// node or request makes — status change, agent decision, task created,
// dependency blocked or unblocked, approval granted or rejected — is logged
// with who did it, what they did, why, and when.
type AuditEvent struct {
	ID         string
	RequestID  string
	NodeID     *string
	Actor      string
	Action     string
	Reason     string
	DocumentID *string
	CreatedAt  time.Time
}

type AuditRepo struct {
	pg *pgxpool.Pool
}

func NewAuditRepo(pg *pgxpool.Pool) *AuditRepo {
	return &AuditRepo{pg: pg}
}

// Append writes an audit event. Because audit_events is append-only there is
// no update or delete — once written the record is immutable.
func (r *AuditRepo) Append(ctx context.Context, e AuditEvent) error {
	_, err := r.pg.Exec(ctx, `
		INSERT INTO audit_events (id, request_id, node_id, actor, action, reason, document_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`, e.ID, e.RequestID, e.NodeID, e.Actor, e.Action, e.Reason, e.DocumentID)
	return err
}

// ListByRequest returns audit events for a request, newest first.
func (r *AuditRepo) ListByRequest(ctx context.Context, requestID string) ([]AuditEvent, error) {
	rows, err := r.pg.Query(ctx, `
		SELECT id, request_id, node_id, actor, action, reason, document_id, created_at
		FROM audit_events
		WHERE request_id = $1
		ORDER BY created_at DESC
	`, requestID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []AuditEvent
	for rows.Next() {
		var e AuditEvent
		if err := rows.Scan(&e.ID, &e.RequestID, &e.NodeID, &e.Actor, &e.Action, &e.Reason, &e.DocumentID, &e.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// ListByNode returns audit events scoped to a single node, newest first.
func (r *AuditRepo) ListByNode(ctx context.Context, nodeID string) ([]AuditEvent, error) {
	rows, err := r.pg.Query(ctx, `
		SELECT id, request_id, node_id, actor, action, reason, document_id, created_at
		FROM audit_events
		WHERE node_id = $1
		ORDER BY created_at DESC
	`, nodeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []AuditEvent
	for rows.Next() {
		var e AuditEvent
		if err := rows.Scan(&e.ID, &e.RequestID, &e.NodeID, &e.Actor, &e.Action, &e.Reason, &e.DocumentID, &e.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// ListByOrg returns audit events for all requests in an org, newest first.
func (r *AuditRepo) ListByOrg(ctx context.Context, orgID string) ([]AuditEvent, error) {
	rows, err := r.pg.Query(ctx, `
		SELECT ae.id, ae.request_id, ae.node_id, ae.actor, ae.action, ae.reason, ae.document_id, ae.created_at
		FROM audit_events ae
		JOIN requests rq ON rq.id = ae.request_id
		WHERE rq.org_id = $1
		ORDER BY ae.created_at DESC
	`, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []AuditEvent
	for rows.Next() {
		var e AuditEvent
		if err := rows.Scan(&e.ID, &e.RequestID, &e.NodeID, &e.Actor, &e.Action, &e.Reason, &e.DocumentID, &e.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}
