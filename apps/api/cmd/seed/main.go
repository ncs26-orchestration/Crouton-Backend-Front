// Command seed fills the database with a realistic demo organization so the
// app has something to show on first boot: one org ("Acme Corp"), a handful of
// users across roles, the five department teams, and a spread of requests in
// different lifecycle states, each carrying a planned workflow graph.
//
// It is safe to run repeatedly. Every run first removes the demo org (by slug)
// and the demo users (by @acme.test email), then re-inserts. Only demo-tagged
// data is touched.
//
// Run it inside the dev api container: `make seed`.
package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"

	"github.com/ncs26-orchestration/solution/apps/api/internal/agentclient"
	"github.com/ncs26-orchestration/solution/apps/api/internal/repo"
)

const (
	demoOrgID    = "org_seed_acme"
	demoOrgName  = "Acme Corp"
	demoOrgSlug  = "acme"
	demoPassword = "password"
	emailDomain  = "@acme.test"
)

// userSpec is one seeded user plus the org role they get.
type userSpec struct {
	email   string
	name    string
	orgRole string // admin | executor | employee
}

var users = []userSpec{
	{"founder@acme.test", "Dana Founder", "admin"},
	{"coo@acme.test", "Omar Operations", "executor"},
	{"finance.lead@acme.test", "Farah Finance", "employee"},
	{"legal.lead@acme.test", "Leo Legal", "employee"},
	{"it.lead@acme.test", "Ivy IT", "employee"},
	{"hr.lead@acme.test", "Hana HR", "employee"},
	{"ops.lead@acme.test", "Otto Ops", "employee"},
}

// teamSpec is a department team plus the lead's email.
type teamSpec struct {
	id          string
	name        string
	description string
	leadEmail   string
}

var teams = []teamSpec{
	{"team_seed_finance", "Finance", "Budgets, spend approval, financial risk", "finance.lead@acme.test"},
	{"team_seed_legal", "Legal", "Contracts, compliance, regulatory review", "legal.lead@acme.test"},
	{"team_seed_it", "IT", "Systems, security, provisioning", "it.lead@acme.test"},
	{"team_seed_hr", "HR", "Hiring, onboarding, people ops", "hr.lead@acme.test"},
	{"team_seed_ops", "Operations", "Execution, logistics, delivery", "ops.lead@acme.test"},
}

// graphProfile describes how a request's workflow graph should look: which
// plan-node keys are done, which are mid-flight (with a status line), and a
// headline progress percent. Everything not listed is left pending.
type graphProfile struct {
	completed  []string
	inProgress map[string]string
}

// requestSpec is one seeded request and the shape of its workflow graph.
type requestSpec struct {
	id          string
	title       string
	description string
	priority    string
	status      string
	progress    int
	requester   string // email
	ageDays     int    // how long ago it was submitted
	etaDays     int    // estimated completion from now; 0 means no ETA
	profile     graphProfile
}

var requests = []requestSpec{
	{
		id:          "req_seed_berlin",
		title:       "Open a new office in Berlin",
		description: "Expand into the EU market with a 30-person satellite office in Berlin by Q4.",
		priority:    "high",
		status:      "in_progress",
		progress:    35,
		requester:   "founder@acme.test",
		ageDays:     6,
		etaDays:     20,
		profile: graphProfile{
			completed: []string{"intake", "planning"},
			inProgress: map[string]string{
				"finance_review": "Validating the 3-year budget and EU tax exposure",
				"legal_review":   "Reviewing GmbH incorporation and German employment law",
				"it_assessment":  "Scoping office network and device provisioning",
			},
		},
	},
	{
		id:          "req_seed_laptops",
		title:       "Procure 50 engineering laptops",
		description: "Refresh the engineering fleet with 50 new laptops ahead of the Q3 hiring wave.",
		priority:    "medium",
		status:      "awaiting_approval",
		progress:    55,
		requester:   "it.lead@acme.test",
		ageDays:     9,
		etaDays:     8,
		profile: graphProfile{
			completed: []string{"intake", "planning", "finance_review", "legal_review", "it_assessment"},
			inProgress: map[string]string{
				"exec_approval": "Awaiting CFO sign-off on the $92k spend",
			},
		},
	},
	{
		id:          "req_seed_contractors",
		title:       "Onboard Q3 contractor cohort",
		description: "Bring on 12 contractors for the Q3 delivery push, fully provisioned and compliant.",
		priority:    "medium",
		status:      "completed",
		progress:    100,
		requester:   "hr.lead@acme.test",
		ageDays:     21,
		etaDays:     0,
		profile: graphProfile{
			completed: []string{
				"intake", "planning", "finance_review", "legal_review", "it_assessment",
				"exec_approval", "hr_planning", "ops_planning", "implementation", "report",
			},
		},
	},
	{
		id:          "req_seed_crm",
		title:       "Migrate CRM to new vendor",
		description: "Move off the legacy CRM to a new vendor with zero data loss and minimal downtime.",
		priority:    "urgent",
		status:      "in_progress",
		progress:    15,
		requester:   "coo@acme.test",
		ageDays:     2,
		etaDays:     30,
		profile: graphProfile{
			completed: []string{"intake"},
			inProgress: map[string]string{
				"planning": "Mapping the data model and the cutover plan",
			},
		},
	},
	{
		id:          "req_seed_policy",
		title:       "Update remote-work policy",
		description: "Refresh the remote-work policy to cover hybrid schedules and home-office stipends.",
		priority:    "low",
		status:      "submitted",
		progress:    0,
		requester:   "hr.lead@acme.test",
		ageDays:     0,
		etaDays:     0,
		profile:     graphProfile{},
	},
}

func main() {
	log.SetFlags(0)

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://app:app@postgres:5432/app?sslmode=disable"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	pool, err := repo.NewPostgres(ctx, dbURL)
	if err != nil {
		log.Fatalf("seed: connect postgres: %v", err)
	}
	defer pool.Close()

	counts, err := seed(ctx, pool)
	if err != nil {
		log.Fatalf("seed: %v", err)
	}

	log.Println("seed complete:")
	log.Printf("  org      %s (slug %q)", demoOrgName, demoOrgSlug)
	log.Printf("  users    %d", len(users))
	log.Printf("  teams    %d", len(teams))
	log.Printf("  requests %d (%d nodes, %d edges, %d tasks)", len(requests), counts.nodes, counts.edges, counts.tasks)
	log.Println("  login    founder@acme.test / password  (same password for every @acme.test user)")
}

// seed runs the full demo seed in one go: it removes any prior demo data, then
// inserts users, the org, teams, and requests with their workflow graphs and
// agent tasks. Safe to run repeatedly (reset clears the prior demo first).
func seed(ctx context.Context, pool *pgxpool.Pool) (seedCounts, error) {
	if err := reset(ctx, pool); err != nil {
		return seedCounts{}, fmt.Errorf("reset: %w", err)
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(demoPassword), 12)
	if err != nil {
		return seedCounts{}, fmt.Errorf("hash password: %w", err)
	}

	userIDs, err := seedUsers(ctx, pool, string(hash))
	if err != nil {
		return seedCounts{}, fmt.Errorf("users: %w", err)
	}
	if err := seedOrg(ctx, pool, userIDs); err != nil {
		return seedCounts{}, fmt.Errorf("org: %w", err)
	}
	if err := seedTeams(ctx, pool, userIDs); err != nil {
		return seedCounts{}, fmt.Errorf("teams: %w", err)
	}
	counts, err := seedRequests(ctx, pool, userIDs)
	if err != nil {
		return seedCounts{}, fmt.Errorf("requests: %w", err)
	}
	return counts, nil
}

// reset removes any prior demo data. The org delete cascades to its members,
// teams, requests, and workflow graph; users are removed afterwards so the
// requests' requester FK is already gone.
func reset(ctx context.Context, pool *pgxpool.Pool) error {
	if _, err := pool.Exec(ctx, `DELETE FROM organizations WHERE slug = $1`, demoOrgSlug); err != nil {
		return fmt.Errorf("delete org: %w", err)
	}
	if _, err := pool.Exec(ctx, `DELETE FROM users WHERE email LIKE '%' || $1`, emailDomain); err != nil {
		return fmt.Errorf("delete users: %w", err)
	}
	return nil
}

func seedUsers(ctx context.Context, pool *pgxpool.Pool, hash string) (map[string]int64, error) {
	ids := make(map[string]int64, len(users))
	for _, u := range users {
		var id int64
		err := pool.QueryRow(ctx,
			`INSERT INTO users (email, name, password_hash) VALUES ($1, $2, $3) RETURNING id`,
			u.email, u.name, hash,
		).Scan(&id)
		if err != nil {
			return nil, fmt.Errorf("insert %s: %w", u.email, err)
		}
		ids[u.email] = id
	}
	return ids, nil
}

func seedOrg(ctx context.Context, pool *pgxpool.Pool, userIDs map[string]int64) error {
	if _, err := pool.Exec(ctx,
		`INSERT INTO organizations (id, name, slug) VALUES ($1, $2, $3)`,
		demoOrgID, demoOrgName, demoOrgSlug,
	); err != nil {
		return fmt.Errorf("insert org: %w", err)
	}
	for _, u := range users {
		if _, err := pool.Exec(ctx,
			`INSERT INTO org_members (org_id, user_id, role) VALUES ($1, $2, $3)`,
			demoOrgID, userIDs[u.email], u.orgRole,
		); err != nil {
			return fmt.Errorf("insert member %s: %w", u.email, err)
		}
	}
	return nil
}

func seedTeams(ctx context.Context, pool *pgxpool.Pool, userIDs map[string]int64) error {
	for _, t := range teams {
		if _, err := pool.Exec(ctx,
			`INSERT INTO teams (id, org_id, name, description) VALUES ($1, $2, $3, $4)`,
			t.id, demoOrgID, t.name, t.description,
		); err != nil {
			return fmt.Errorf("insert team %s: %w", t.name, err)
		}
		if _, err := pool.Exec(ctx,
			`INSERT INTO team_members (team_id, user_id, role) VALUES ($1, $2, 'lead')`,
			t.id, userIDs[t.leadEmail],
		); err != nil {
			return fmt.Errorf("insert team lead %s: %w", t.leadEmail, err)
		}
	}
	return nil
}

// seedCounts is what a seed run produced, for the summary log.
type seedCounts struct {
	nodes int
	edges int
	tasks int
}

func seedRequests(ctx context.Context, pool *pgxpool.Pool, userIDs map[string]int64) (seedCounts, error) {
	plan := agentclient.DefaultPlan()
	now := time.Now()

	var counts seedCounts
	for _, r := range requests {
		nodes, edges := buildGraph(r, plan, now)
		tasks := buildTasks(nodes)
		if err := insertRequestGraph(ctx, pool, r, userIDs[r.requester], nodes, edges, tasks, now); err != nil {
			return seedCounts{}, fmt.Errorf("request %q: %w", r.title, err)
		}
		counts.nodes += len(nodes)
		counts.edges += len(edges)
		counts.tasks += len(tasks)
	}
	return counts, nil
}

// buildTasks gives each worked node believable agent_tasks drawn from the same
// deterministic department playbook the engine uses: completed nodes carry the
// full task set (all completed), in-progress nodes carry the first task still
// running. Pending nodes have no tasks yet.
func buildTasks(nodes []repo.WorkflowNode) []repo.AgentTask {
	var tasks []repo.AgentTask
	for _, n := range nodes {
		dec := agentclient.DefaultDecision(n.AgentType)
		switch n.Status {
		case "completed":
			for _, t := range dec.Tasks {
				tasks = append(tasks, repo.AgentTask{
					ID:          fmt.Sprintf("at_%s", shortID()),
					NodeID:      n.ID,
					Title:       t.Title,
					Status:      "completed",
					StartedAt:   n.StartedAt,
					CompletedAt: n.CompletedAt,
				})
			}
		case "in_progress":
			if len(dec.Tasks) > 0 {
				tasks = append(tasks, repo.AgentTask{
					ID:        fmt.Sprintf("at_%s", shortID()),
					NodeID:    n.ID,
					Title:     dec.Tasks[0].Title,
					Status:    "in_progress",
					StartedAt: n.StartedAt,
				})
			}
		}
	}
	return tasks
}

// buildGraph turns the canonical plan into concrete nodes/edges for a request,
// applying the request's profile so completed and in-flight stages carry
// believable status text, progress, and timestamps.
func buildGraph(r requestSpec, plan *agentclient.Plan, now time.Time) ([]repo.WorkflowNode, []repo.WorkflowEdge) {
	done := make(map[string]bool, len(r.profile.completed))
	for _, k := range r.profile.completed {
		done[k] = true
	}

	submitted := now.AddDate(0, 0, -r.ageDays)
	nodeID := make(map[string]string, len(plan.Nodes))
	nodes := make([]repo.WorkflowNode, 0, len(plan.Nodes))

	for i, pn := range plan.Nodes {
		id := fmt.Sprintf("wn_%s_%s", shortID(), pn.Key)
		nodeID[pn.Key] = id

		// Lay the stages out in time so the demo reads chronologically.
		started := submitted.Add(time.Duration(i) * 6 * time.Hour)
		completed := started.Add(4 * time.Hour)

		n := repo.WorkflowNode{
			ID:         id,
			RequestID:  r.id,
			Key:        pn.Key,
			Name:       pn.Name,
			AgentType:  pn.AgentType,
			Department: pn.Department,
			Status:     "pending",
		}

		switch {
		case done[pn.Key]:
			n.Status = "completed"
			n.ProgressPercent = 100
			n.StatusText = pn.Name + " complete"
			st, ct := started, completed
			n.StartedAt, n.CompletedAt = &st, &ct
		case r.profile.inProgress[pn.Key] != "":
			n.Status = "in_progress"
			n.ProgressPercent = 50
			n.StatusText = r.profile.inProgress[pn.Key]
			st := started
			n.StartedAt = &st
		}
		nodes = append(nodes, n)
	}

	edges := make([]repo.WorkflowEdge, 0, len(plan.Edges))
	for _, pe := range plan.Edges {
		src, ok1 := nodeID[pe.From]
		tgt, ok2 := nodeID[pe.To]
		if !ok1 || !ok2 {
			continue
		}
		edges = append(edges, repo.WorkflowEdge{
			ID:           fmt.Sprintf("we_%s", shortID()),
			RequestID:    r.id,
			SourceNodeID: src,
			TargetNodeID: tgt,
			EdgeType:     pe.EdgeType,
		})
	}
	return nodes, edges
}

// insertRequestGraph writes a request and its full graph in one transaction so
// a partial failure never leaves an orphaned request.
func insertRequestGraph(
	ctx context.Context,
	pool *pgxpool.Pool,
	r requestSpec,
	requesterID int64,
	nodes []repo.WorkflowNode,
	edges []repo.WorkflowEdge,
	tasks []repo.AgentTask,
	now time.Time,
) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var eta *time.Time
	if r.etaDays > 0 {
		t := now.AddDate(0, 0, r.etaDays)
		eta = &t
	}
	createdAt := now.AddDate(0, 0, -r.ageDays)

	if _, err := tx.Exec(ctx, `
		INSERT INTO requests
			(id, org_id, title, description, requester_user_id, priority, status, progress, estimated_completion, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`, r.id, demoOrgID, r.title, r.description, requesterID, r.priority, r.status, r.progress, eta, createdAt); err != nil {
		return fmt.Errorf("insert request: %w", err)
	}

	batch := &pgx.Batch{}
	for _, n := range nodes {
		batch.Queue(`
			INSERT INTO workflow_nodes
				(id, request_id, key, name, agent_type, department, status, description, progress_percent, status_text, started_at, completed_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		`, n.ID, n.RequestID, n.Key, n.Name, n.AgentType, n.Department, n.Status, n.Description, n.ProgressPercent, n.StatusText, n.StartedAt, n.CompletedAt)
	}
	for _, e := range edges {
		batch.Queue(`
			INSERT INTO workflow_edges (id, request_id, source_node_id, target_node_id, edge_type)
			VALUES ($1, $2, $3, $4, $5)
		`, e.ID, e.RequestID, e.SourceNodeID, e.TargetNodeID, e.EdgeType)
	}
	// agent_tasks reference workflow_nodes, queued above in the same tx.
	for _, t := range tasks {
		batch.Queue(`
			INSERT INTO agent_tasks (id, node_id, title, status, started_at, completed_at)
			VALUES ($1, $2, $3, $4, $5, $6)
		`, t.ID, t.NodeID, t.Title, t.Status, t.StartedAt, t.CompletedAt)
	}

	br := tx.SendBatch(ctx, batch)
	for range len(nodes) + len(edges) + len(tasks) {
		if _, err := br.Exec(); err != nil {
			_ = br.Close()
			return fmt.Errorf("insert graph: %w", err)
		}
	}
	if err := br.Close(); err != nil {
		return err
	}

	return tx.Commit(ctx)
}

// shortID returns 8 hex chars, matching the wn_/we_ id style elsewhere.
func shortID() string {
	b := make([]byte, 4)
	if _, err := rand.Read(b); err != nil {
		panic("crypto/rand unavailable: " + err.Error())
	}
	return hex.EncodeToString(b)
}
