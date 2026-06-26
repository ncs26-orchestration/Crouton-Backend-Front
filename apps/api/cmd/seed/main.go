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

// agentSpec is one seeded department agent linked to a team.
type agentSpec struct {
	id         string
	agentType  string
	name       string
	capabilities string
}

var agents = []agentSpec{
	{"agent_seed_finance", "finance", "Finance Agent", "Budget analysis, spend approval, financial risk assessment, ROI calculation"},
	{"agent_seed_legal", "legal", "Legal Agent", "Contract review, regulatory compliance, risk flagging, policy advisory"},
	{"agent_seed_it", "it", "IT Agent", "Technical feasibility, security assessment, infrastructure planning, systems integration"},
	{"agent_seed_hr", "hr", "HR Agent", "Staffing assessment, hiring plan, onboarding logistics, people ops"},
	{"agent_seed_ops", "ops", "Operations Agent", "Logistics planning, facilities, timeline management, execution coordination"},
}

// policySpec is one seeded department policy.
type policySpec struct {
	id     string
	teamID string
	title  string
	body   string
}

var policies = []policySpec{
	{"pol_seed_finance", "team_seed_finance", "Finance Policy",
		"All expenditures over $10k require executive approval. Budget allocations must align with quarterly planning."},
	{"pol_seed_legal", "team_seed_legal", "Legal Policy",
		"All contracts must be reviewed for regulatory compliance. Non-disclosure agreements follow the standard template."},
	{"pol_seed_it", "team_seed_it", "IT Policy",
		"New systems must pass a security assessment. Software procurement follows the approved vendor list."},
	{"pol_seed_hr", "team_seed_hr", "HR Policy",
		"New headcount requires approved job descriptions and budget allocation. Onboarding includes equipment provisioning and system access."},
	{"pol_seed_ops", "team_seed_ops", "Operations Policy",
		"Project timelines must account for dependencies and buffer time. Vendor onboarding follows the standard integration checklist."},
}

// graphProfile describes how a request's workflow graph should look: which
// plan-node keys are done, which are mid-flight (with a status line), which are
// blocked (F5), and headline progress percent. Everything not listed is left pending.
type graphProfile struct {
	completed  []string
	inProgress map[string]string
	blocked    map[string]string // key → reason for the blocked dependency (F5)
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
				"legal_review":  "Reviewing GmbH incorporation and German employment law",
				"it_assessment": "Scoping office network and device provisioning",
			},
			blocked: map[string]string{
				"finance_review": "Need the IT security assessment and infrastructure cost estimate before the budget can be finalized.",
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
	log.Printf("  org          %s (slug %q)", demoOrgName, demoOrgSlug)
	log.Printf("  users        %d", len(users))
	log.Printf("  teams        %d", len(teams))
	log.Printf("  agents       %d", len(agents))
	log.Printf("  policies     %d", len(policies))
	log.Printf("  requests     %d (%d nodes, %d edges, %d tasks, %d deps, %d audit events)", len(requests), counts.nodes, counts.edges, counts.tasks, counts.deps, counts.auditEvents)
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
	if err := seedAgents(ctx, pool); err != nil {
		return seedCounts{}, fmt.Errorf("agents: %w", err)
	}
	if err := seedPolicies(ctx, pool); err != nil {
		return seedCounts{}, fmt.Errorf("policies: %w", err)
	}
	counts, err := seedRequests(ctx, pool, userIDs)
	if err != nil {
		return seedCounts{}, fmt.Errorf("requests: %w", err)
	}
	auditCount, err := seedAuditEvents(ctx, pool, userIDs)
	if err != nil {
		return seedCounts{}, fmt.Errorf("audit events: %w", err)
	}
	counts.auditEvents = auditCount
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
	nodes       int
	edges       int
	tasks       int
	deps        int
	auditEvents int
}

func seedRequests(ctx context.Context, pool *pgxpool.Pool, userIDs map[string]int64) (seedCounts, error) {
	plan := agentclient.DefaultPlan()
	now := time.Now()

	var counts seedCounts
	for _, r := range requests {
		nodes, edges := buildGraph(r, plan, now)
		tasks := buildTasks(nodes)
		deps := buildDeps(r, plan, nodes)
		if err := insertRequestGraph(ctx, pool, r, userIDs[r.requester], nodes, edges, tasks, deps, now); err != nil {
			return seedCounts{}, fmt.Errorf("request %q: %w", r.title, err)
		}
		counts.nodes += len(nodes)
		counts.edges += len(edges)
		counts.tasks += len(tasks)
		counts.deps += len(deps)
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
			for i, t := range dec.Tasks {
				tasks = append(tasks, repo.AgentTask{
					ID:          fmt.Sprintf("at_%s", shortID()),
					NodeID:      n.ID,
					Title:       t.Title,
					Status:      "completed",
					Ordinal:     i,
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
					Ordinal:   0,
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
		case r.profile.blocked[pn.Key] != "":
			n.Status = "blocked"
			n.ProgressPercent = 50
			n.StatusText = "Waiting for IT: " + r.profile.blocked[pn.Key]
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

// buildDeps creates node_dependencies rows for blocked nodes in the seed (F5).
// It maps the dependent node (the blocked one) to its blocking department node.
func buildDeps(r requestSpec, plan *agentclient.Plan, nodes []repo.WorkflowNode) []repo.NodeDependency {
	if len(r.profile.blocked) == 0 {
		return nil
	}
	keyToNode := make(map[string]repo.WorkflowNode, len(nodes))
	for _, n := range nodes {
		keyToNode[n.Key] = n
	}
	var out []repo.NodeDependency
	for blockedKey, reason := range r.profile.blocked {
		depNode, ok := keyToNode[blockedKey]
		if !ok {
			continue
		}
		// The blocking node is assumed to be the IT assessment in the same request.
		blockingKey := "it_assessment"
		blockingNode, ok := keyToNode[blockingKey]
		if !ok {
			continue
		}
		out = append(out, repo.NodeDependency{
			ID:              "nd_" + shortID(),
			RequestID:       r.id,
			DependentNodeID: depNode.ID,
			BlockingNodeID:  blockingNode.ID,
			Reason:          reason,
			RunCount:        1,
		})
	}
	return out
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
	deps []repo.NodeDependency,
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
			INSERT INTO agent_tasks (id, node_id, title, status, ordinal, started_at, completed_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7)
		`, t.ID, t.NodeID, t.Title, t.Status, t.Ordinal, t.StartedAt, t.CompletedAt)
	}
	// node_dependencies (F5) — reference workflow_nodes, queued above.
	for _, d := range deps {
		batch.Queue(`
			INSERT INTO node_dependencies (id, request_id, dependent_node_id, blocking_node_id, reason, run_count)
			VALUES ($1, $2, $3, $4, $5, $6)
		`, d.ID, d.RequestID, d.DependentNodeID, d.BlockingNodeID, d.Reason, d.RunCount)
	}

	br := tx.SendBatch(ctx, batch)
	for range len(nodes) + len(edges) + len(tasks) + len(deps) {
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

// seedAuditEvents creates believable audit events for each seeded request.
// Completed nodes get node.started + node.completed; in_progress nodes get
// node.started; completed requests get request.completed.
func seedAuditEvents(ctx context.Context, pool *pgxpool.Pool, _ map[string]int64) (int, error) {
	now := time.Now()
	var count int

	for _, r := range requests {
		if len(r.profile.completed) == 0 && len(r.profile.inProgress) == 0 {
			continue
		}

		var nodeIDs []struct {
			ID     string
			Key    string
			Name   string
			Status string
		}
		nrows, err := pool.Query(ctx, `
			SELECT id, key, name, status FROM workflow_nodes WHERE request_id = $1 ORDER BY created_at ASC
		`, r.id)
		if err != nil {
			return 0, fmt.Errorf("query nodes for %s: %w", r.id, err)
		}
		for nrows.Next() {
			var n struct {
				ID     string
				Key    string
				Name   string
				Status string
			}
			if err := nrows.Scan(&n.ID, &n.Key, &n.Name, &n.Status); err != nil {
				nrows.Close()
				return 0, fmt.Errorf("scan node for %s: %w", r.id, err)
			}
			nodeIDs = append(nodeIDs, n)
		}
		nrows.Close()

		submitted := now.AddDate(0, 0, -r.ageDays)

		for _, n := range nodeIDs {
			if n.Status != "completed" && n.Status != "in_progress" {
				continue
			}

			// node.started
			if _, err := pool.Exec(ctx, `
				INSERT INTO audit_events (id, request_id, node_id, actor, action, reason, created_at)
				VALUES ($1, $2, $3, $4, $5, $6, $7)
			`, "aev_"+shortID(), r.id, &n.ID, "engine", "node.started",
				n.Name+" started", submitted); err != nil {
				return 0, fmt.Errorf("insert audit node.started for %s: %w", n.ID, err)
			}
			count++

			if n.Status == "completed" {
				completedAt := submitted.Add(4 * time.Hour)
				if _, err := pool.Exec(ctx, `
					INSERT INTO audit_events (id, request_id, node_id, actor, action, reason, created_at)
					VALUES ($1, $2, $3, $4, $5, $6, $7)
				`, "aev_"+shortID(), r.id, &n.ID, n.Name, "node.completed",
					n.Name+" complete", completedAt); err != nil {
					return 0, fmt.Errorf("insert audit node.completed for %s: %w", n.ID, err)
				}
				count++
			}
		}

		// request status events
		if r.status == "completed" || r.status == "in_progress" {
			if _, err := pool.Exec(ctx, `
				INSERT INTO audit_events (id, request_id, node_id, actor, action, reason, created_at)
				VALUES ($1, $2, NULL, $3, $4, $5, $6)
			`, "aev_"+shortID(), r.id, "engine", "request.created",
				r.title+" submitted", submitted); err != nil {
				return 0, fmt.Errorf("insert audit request.created for %s: %w", r.id, err)
			}
			count++
		}
		if r.status == "completed" {
			if _, err := pool.Exec(ctx, `
				INSERT INTO audit_events (id, request_id, node_id, actor, action, reason, created_at)
				VALUES ($1, $2, NULL, $3, $4, $5, $6)
			`, "aev_"+shortID(), r.id, "engine", "request.completed",
				"All stages complete", now); err != nil {
				return 0, fmt.Errorf("insert audit request.completed for %s: %w", r.id, err)
			}
			count++
		}
	}
	return count, nil
}

// seedAgents creates one agent row per seeded department team.
func seedAgents(ctx context.Context, pool *pgxpool.Pool) error {
	for i, a := range agents {
		if i >= len(teams) {
			break
		}
		if _, err := pool.Exec(ctx, `
			INSERT INTO agents (id, org_id, team_id, agent_type, name, capabilities)
			VALUES ($1, $2, $3, $4, $5, $6)
		`, a.id, demoOrgID, teams[i].id, a.agentType, a.name, a.capabilities); err != nil {
			return fmt.Errorf("insert agent %s: %w", a.id, err)
		}
	}
	return nil
}

// seedPolicies creates starter department policies for each seeded team.
func seedPolicies(ctx context.Context, pool *pgxpool.Pool) error {
	for _, p := range policies {
		if _, err := pool.Exec(ctx, `
			INSERT INTO department_policies (id, org_id, team_id, title, body)
			VALUES ($1, $2, $3, $4, $5)
		`, p.id, demoOrgID, p.teamID, p.title, p.body); err != nil {
			return fmt.Errorf("insert policy %s: %w", p.id, err)
		}
	}
	return nil
}
