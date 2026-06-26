package repo

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// requestsTestDB connects to TEST_DATABASE_URL or skips. The DB-less
// `api` CI job skips this; the migrations/e2e jobs and local runs (which
// have a migrated Postgres) exercise it.
func requestsTestDB(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set TEST_DATABASE_URL to run repo round-trip tests")
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

func TestRequestRepoRoundTrip(t *testing.T) {
	pool := requestsTestDB(t)
	ctx := context.Background()
	r := NewRequestRepo(pool)

	// Seed the FK parents (user + org) and clean them up after.
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	var userID int64
	if err := pool.QueryRow(ctx,
		`INSERT INTO users (email, name, password_hash) VALUES ($1, 'Repo Tester', 'x') RETURNING id`,
		"repo+"+suffix+"@example.com",
	).Scan(&userID); err != nil {
		t.Fatalf("seed user: %v", err)
	}
	orgID := "org_test_" + suffix
	if _, err := pool.Exec(ctx,
		`INSERT INTO organizations (id, name, slug) VALUES ($1, 'Repo Org', $2)`,
		orgID, "repo-"+suffix,
	); err != nil {
		t.Fatalf("seed org: %v", err)
	}
	t.Cleanup(func() {
		// Cascades to requests; then remove the user.
		_, _ = pool.Exec(context.Background(), `DELETE FROM organizations WHERE id = $1`, orgID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM users WHERE id = $1`, userID)
	})

	// Create returns the row with DB defaults applied.
	created, err := r.Create(ctx, Request{
		ID:              "req_test_" + suffix,
		OrgID:           orgID,
		Title:           "Open a new office in Berlin",
		Description:     "Expand into the EU market",
		RequesterUserID: userID,
		Priority:        "high",
		Status:          "submitted",
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if created.Status != "submitted" || created.Progress != 0 {
		t.Errorf("defaults: status=%q progress=%d, want submitted/0", created.Status, created.Progress)
	}
	if created.CreatedAt.IsZero() {
		t.Error("created_at not populated")
	}

	// GetByID returns the same row.
	got, err := r.GetByID(ctx, created.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Title != "Open a new office in Berlin" || got.Priority != "high" || got.OrgID != orgID {
		t.Errorf("get mismatch: %+v", got)
	}

	// ListByOrg includes it.
	list, err := r.ListByOrg(ctx, orgID, 50)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list) != 1 || list[0].ID != created.ID {
		t.Errorf("list = %d rows, want 1 containing %s", len(list), created.ID)
	}

	// UpdateStatusProgress moves it forward.
	if err := r.UpdateStatusProgress(ctx, created.ID, "in_progress", 40); err != nil {
		t.Fatalf("update: %v", err)
	}
	got, err = r.GetByID(ctx, created.ID)
	if err != nil {
		t.Fatalf("get after update: %v", err)
	}
	if got.Status != "in_progress" || got.Progress != 40 {
		t.Errorf("after update: status=%q progress=%d, want in_progress/40", got.Status, got.Progress)
	}
}

func TestRequestRepoGetByIDNotFound(t *testing.T) {
	pool := requestsTestDB(t)
	r := NewRequestRepo(pool)
	if _, err := r.GetByID(context.Background(), "req_does_not_exist"); err != ErrNotFound {
		t.Fatalf("GetByID(missing) err = %v, want ErrNotFound", err)
	}
}
