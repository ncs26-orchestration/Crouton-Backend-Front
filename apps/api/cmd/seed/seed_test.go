package main

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// TestSeedPopulatesAgentTasks runs the demo seed against a migrated database
// and asserts it is idempotent and leaves agent_tasks non-empty. Gated on
// TEST_DATABASE_URL so the DB-less api CI job skips it; the migrations/e2e DBs
// and local runs exercise it.
func TestSeedPopulatesAgentTasks(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set TEST_DATABASE_URL to run the seed test")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer pool.Close()

	// Run twice: the second run proves the seed is idempotent (reset clears
	// the prior demo first), which is how `make seed` is used in practice.
	if _, err := seed(ctx, pool); err != nil {
		t.Fatalf("first seed run: %v", err)
	}
	counts, err := seed(ctx, pool)
	if err != nil {
		t.Fatalf("second seed run (idempotency): %v", err)
	}
	if counts.tasks == 0 {
		t.Fatal("seed produced no agent_tasks")
	}

	// The table is actually populated for the demo org's nodes.
	var n int
	err = pool.QueryRow(ctx, `
		SELECT count(*)
		FROM agent_tasks at
		JOIN workflow_nodes wn ON wn.id = at.node_id
		JOIN requests r ON r.id = wn.request_id
		WHERE r.org_id = $1
	`, demoOrgID).Scan(&n)
	if err != nil {
		t.Fatalf("count agent_tasks: %v", err)
	}
	if n == 0 {
		t.Fatal("agent_tasks empty after seed")
	}

	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM organizations WHERE slug = $1`, demoOrgSlug)
		_, _ = pool.Exec(context.Background(), `DELETE FROM users WHERE email LIKE '%' || $1`, emailDomain)
	})
}
