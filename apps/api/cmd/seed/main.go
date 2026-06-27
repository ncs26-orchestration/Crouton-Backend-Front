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
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"

	"github.com/ncs26-orchestration/solution/apps/api/internal/agentclient"
	"github.com/ncs26-orchestration/solution/apps/api/internal/orgdir"
	"github.com/ncs26-orchestration/solution/apps/api/internal/policyrules"
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
	{"finance.lead@acme.test", "Farah Finance", "executor"},
	{"legal.lead@acme.test", "Leo Legal", "executor"},
	{"it.lead@acme.test", "Ivy IT", "executor"},
	{"hr.lead@acme.test", "Hana HR", "executor"},
	{"ops.lead@acme.test", "Otto Ops", "executor"},
	{"tech.lead@acme.test", "Tara Technician", "executor"},
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
	{"team_seed_maint", "Maintenance", "Equipment maintenance and repair", "tech.lead@acme.test"},
}

// graphProfile describes how a request's workflow graph should look: which
// plan-node keys are done, which are mid-flight (with a status line), which are
// blocked (F5), and headline progress percent. Everything not listed is left pending.
type graphProfile struct {
	completed  []string
	inProgress map[string]string
	blocked    map[string]string   // key → reason for the blocked dependency (F5)
	outcomes   map[string]string   // key → decision_outcome override (else derived from status)
	flags      map[string][]string // key → "severity: message" flags an agent raised
	summaries  map[string]string   // key → the agent's full reasoning for the node
}

// requestSpec is one seeded request and the shape of its workflow graph.
type requestSpec struct {
	id          string
	title       string
	description string
	priority    string
	status      string
	requestType string         // intake classification (hiring, procurement, ...)
	details     map[string]any // structured fields shown in the request panel
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
		requestType: "expansion",
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
			summaries: map[string]string{
				"legal_review":  "Opening a Berlin office means a German GmbH and local employment contracts under German labor law. Workable, but needs a registered entity and a works-council-aware contract template before the first hire.",
				"it_assessment": "Scoping office networking, a site-to-site VPN, and device provisioning for ~30 staff. Standard build; the main cost driver is the managed network and the new site's security baseline.",
			},
		},
	},
	{
		id:          "req_seed_laptops",
		title:       "Procure 50 engineering laptops",
		description: "Refresh the engineering fleet with 50 new laptops ahead of the Q3 hiring wave.",
		priority:    "medium",
		status:      "awaiting_approval",
		requestType: "procurement",
		details: map[string]any{
			"vendor":     "Dell",
			"quantity":   50,
			"unit_cost":  1840,
			"total_cost": 92000,
			"needed_by":  "2026-08-01",
		},
		progress:  55,
		requester: "it.lead@acme.test",
		ageDays:   9,
		etaDays:   8,
		profile: graphProfile{
			completed: []string{"intake", "planning", "finance_review", "legal_review", "it_assessment"},
			inProgress: map[string]string{
				"exec_approval": "Awaiting CFO sign-off on the $92k spend",
			},
			outcomes: map[string]string{
				"finance_review": "approve_with_conditions",
			},
			flags: map[string][]string{
				"finance_review": {"warning: $92k exceeds the $50k single-PO limit in Finance Policy; CFO sign-off required."},
				"it_assessment":  {"info: Standard MDM enrollment and imaging covers all 50 units; no new infra."},
			},
			summaries: map[string]string{
				"finance_review": "Total spend is $92k against a $50k single-PO limit. ROI is sound (fleet is 4+ years old, rising repair costs), and funding exists in the Q3 capex line, so this is fundable but needs CFO sign-off per Finance Policy before the PO is cut.",
				"it_assessment":  "50 units of the approved laptop SKU. Provisioning is standard: MDM enrollment, disk encryption, and the standard image. No new infrastructure or security review required.",
				"legal_review":   "Purchase from an approved vendor on standard terms. No contract redlines or regulatory concerns; the existing MSA covers warranty and returns.",
			},
		},
	},
	{
		id:          "req_seed_contractors",
		title:       "Onboard Q3 contractor cohort",
		description: "Bring on 12 contractors for the Q3 delivery push, fully provisioned and compliant.",
		priority:    "medium",
		status:      "completed",
		requestType: "hiring",
		details: map[string]any{
			"role":       "Delivery Contractor",
			"headcount":  12,
			"seniority":  "Mid",
			"comp_band":  "$80/hr",
			"start_date": "2026-07-15",
		},
		progress:  100,
		requester: "hr.lead@acme.test",
		ageDays:   21,
		etaDays:   0,
		profile: graphProfile{
			completed: []string{
				"intake", "planning", "finance_review", "legal_review", "it_assessment",
				"exec_approval", "hr_planning", "ops_planning", "implementation", "report",
			},
			outcomes: map[string]string{
				"legal_review": "approve_with_conditions",
			},
			flags: map[string][]string{
				"legal_review": {"info: Contractor agreements use the standard NDA template; IP assignment confirmed."},
			},
			summaries: map[string]string{
				"finance_review": "12 contractors at ~$80/hr for the quarter lands within the approved Q3 delivery budget. Spend is variable and capped by the SOW, so no funding risk.",
				"legal_review":   "Standard contractor agreements with the approved NDA and IP-assignment clauses. No regulatory blockers; classification reviewed and they are correctly engaged as contractors.",
				"hr_planning":    "Onboarding for 12: equipment, system access, and compliance training scheduled in two waves so delivery isn't disrupted.",
				"ops_planning":   "Workstreams and reporting lines mapped; each contractor is paired with an owner for the Q3 push.",
			},
		},
	},
	{
		id:          "req_seed_crm",
		title:       "Migrate CRM to new vendor",
		description: "Move off the legacy CRM to a new vendor with zero data loss and minimal downtime.",
		priority:    "urgent",
		status:      "in_progress",
		requestType: "infra",
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
		id:          "req_seed_offshore",
		title:       "Stand up an offshore data center",
		description: "Host EU customer data in a new low-cost region to cut infrastructure spend.",
		priority:    "high",
		status:      "rejected",
		requestType: "infra",
		details: map[string]any{
			"system":      "Customer data warehouse",
			"environment": "Production",
			"est_cost":    4000,
		},
		progress:  45,
		requester: "coo@acme.test",
		ageDays:   4,
		etaDays:   0,
		profile: graphProfile{
			completed: []string{"intake", "planning", "it_assessment", "legal_review"},
			outcomes: map[string]string{
				"legal_review": "reject",
			},
			flags: map[string][]string{
				"it_assessment": {"warning: Region lacks our standard encryption-at-rest tier."},
				"legal_review": {
					"critical: Hosting EU customer data outside the EU violates GDPR data residency (Legal Policy).",
				},
			},
			summaries: map[string]string{
				"it_assessment": "The target region is ~30% cheaper, but it does not offer our standard encryption-at-rest tier and would need a bespoke security review. Technically possible, with caveats.",
				"legal_review":  "This would host EU customer personal data outside the EU. That breaches GDPR data-residency requirements in our Legal Policy and exposes the company to regulatory penalties. There is no lawful basis or transfer mechanism on file, so this cannot proceed as scoped.",
			},
		},
	},
	{
		id:          "req_seed_policy",
		title:       "Update remote-work policy",
		description: "Refresh the remote-work policy to cover hybrid schedules and home-office stipends.",
		priority:    "low",
		status:      "submitted",
		requestType: "policy_change",
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
	log.Printf("  agents       %d", counts.agents)
	log.Printf("  policies     %d", counts.policies)
	log.Printf("  machines     %d", len(machines))
	log.Printf("  incidents    %d", counts.incidents)
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
	// Department directory (agents + policies). The canonical content lives in
	// internal/orgdir and is shared with the org-create handler; here we link it
	// to the demo teams by name.
	teamByName := make(map[string]string, len(teams))
	for _, t := range teams {
		teamByName[t.name] = t.id
	}
	nAgents, err := seedAgents(ctx, pool, teamByName)
	if err != nil {
		return seedCounts{}, fmt.Errorf("agents: %w", err)
	}
	nPolicies, err := seedPolicies(ctx, pool, teamByName)
	if err != nil {
		return seedCounts{}, fmt.Errorf("policies: %w", err)
	}
	counts, err := seedRequests(ctx, pool, userIDs)
	if err != nil {
		return seedCounts{}, fmt.Errorf("requests: %w", err)
	}
	if err := seedMachines(ctx, pool, userIDs); err != nil {
		return seedCounts{}, fmt.Errorf("machines: %w", err)
	}
	incidentCount, err := seedIncidents(ctx, pool, userIDs)
	if err != nil {
		return seedCounts{}, fmt.Errorf("incidents: %w", err)
	}
	counts.incidents = incidentCount
	auditCount, err := seedAuditEvents(ctx, pool, userIDs)
	if err != nil {
		return seedCounts{}, fmt.Errorf("audit events: %w", err)
	}
	counts.auditEvents = auditCount
	counts.agents = nAgents
	counts.policies = nPolicies
	return counts, nil
}

// reset removes any prior demo data. The org delete cascades to its members,
// teams, requests, and workflow graph; users are removed afterwards so the
// requests' requester FK is already gone.
func reset(ctx context.Context, pool *pgxpool.Pool) error {
	if _, err := pool.Exec(ctx, `DELETE FROM incidents WHERE org_id = (SELECT id FROM organizations WHERE slug = $1)`, demoOrgSlug); err != nil {
		return fmt.Errorf("delete incidents: %w", err)
	}
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
		// The Maintenance team lead also needs the technician role so
		// they can see assigned machines.
		if t.name == "Maintenance" {
			if _, err := pool.Exec(ctx,
				`INSERT INTO team_members (team_id, user_id, role) VALUES ($1, $2, 'technician') ON CONFLICT (team_id, user_id) DO UPDATE SET role = 'technician'`,
				t.id, userIDs[t.leadEmail],
			); err != nil {
				return fmt.Errorf("insert technician %s: %w", t.leadEmail, err)
			}
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
	incidents   int
	auditEvents int
	agents      int
	policies    int
}

func seedRequests(ctx context.Context, pool *pgxpool.Pool, userIDs map[string]int64) (seedCounts, error) {
	plan := agentclient.DefaultPlan()
	now := time.Now()

	var counts seedCounts
	for _, r := range requests {
		nodes, edges := buildGraph(r, plan, now)
		tasks := buildTasks(nodes)
		deps := buildDeps(r, plan, nodes)
		flags := buildFlags(r, nodes)
		checks := buildChecks(r, nodes)
		if err := insertRequestGraph(ctx, pool, r, userIDs[r.requester], nodes, edges, tasks, deps, flags, checks, now); err != nil {
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
			n.DecisionOutcome = "approve"
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
			n.DecisionOutcome = "block"
			st := started
			n.StartedAt = &st
		}
		// An explicit profile outcome wins (e.g. a flag or a rejection on a
		// completed node) so the demo shows the full range of decisions.
		if o := r.profile.outcomes[pn.Key]; o != "" {
			n.DecisionOutcome = o
		}
		// The agent's reasoning, shown in the node detail panel.
		if s := r.profile.summaries[pn.Key]; s != "" {
			n.DecisionSummary = s
		} else if n.Status == "completed" {
			n.DecisionSummary = pn.Name + " reviewed the request and recorded its assessment."
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

// buildFlags turns each request's profile flags ("severity: message") into
// node_flags rows mapped to the right node, so the demo's node panels show real
// risks without the LLM.
func buildFlags(r requestSpec, nodes []repo.WorkflowNode) []repo.NodeFlag {
	if len(r.profile.flags) == 0 {
		return nil
	}
	keyToNode := make(map[string]repo.WorkflowNode, len(nodes))
	for _, n := range nodes {
		keyToNode[n.Key] = n
	}
	var out []repo.NodeFlag
	for key, msgs := range r.profile.flags {
		n, ok := keyToNode[key]
		if !ok {
			continue
		}
		for i, raw := range msgs {
			severity, message := "info", raw
			if before, after, found := strings.Cut(raw, ":"); found {
				severity = strings.TrimSpace(before)
				message = strings.TrimSpace(after)
			}
			if severity != "info" && severity != "warning" && severity != "critical" {
				severity = "info"
			}
			out = append(out, repo.NodeFlag{
				ID:        "nf_" + shortID(),
				RequestID: r.id,
				NodeID:    n.ID,
				Severity:  severity,
				Message:   message,
				Ordinal:   i,
			})
		}
	}
	return out
}

// buildChecks evaluates each department's policy rules against the request's
// details and produces node_checks for the matching nodes, so the demo shows
// exact pass/fail checks offline — the same evaluation the engine runs live.
func buildChecks(r requestSpec, nodes []repo.WorkflowNode) []repo.NodeCheck {
	if len(r.details) == 0 {
		return nil
	}
	// department (lowercased) → policy from orgdir.
	polByDept := make(map[string]orgdir.PolicySpec, len(orgdir.Policies))
	for _, p := range orgdir.Policies {
		polByDept[strings.ToLower(p.Department)] = p
	}
	var out []repo.NodeCheck
	for _, n := range nodes {
		if n.Status != "completed" && n.Status != "awaiting_review" {
			continue
		}
		p, ok := polByDept[strings.ToLower(n.Department)]
		if !ok || len(p.Rules) == 0 {
			continue
		}
		for i, c := range policyrules.Evaluate(p.Title, p.Rules, r.details) {
			out = append(out, repo.NodeCheck{
				ID: "nck_" + shortID(), RequestID: r.id, NodeID: n.ID,
				Label: c.Label, Status: c.Status, Detail: c.Detail, PolicyTitle: c.PolicyTitle, Ordinal: i,
			})
		}
	}
	return out
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
	flags []repo.NodeFlag,
	checks []repo.NodeCheck,
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

	requestType := r.requestType
	if requestType == "" {
		requestType = "general"
	}
	detailsJSON := []byte("{}")
	if len(r.details) > 0 {
		if b, err := json.Marshal(r.details); err == nil {
			detailsJSON = b
		}
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO requests
			(id, org_id, title, description, requester_user_id, request_type, details, priority, status, progress, estimated_completion, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
	`, r.id, demoOrgID, r.title, r.description, requesterID, requestType, detailsJSON, r.priority, r.status, r.progress, eta, createdAt); err != nil {
		return fmt.Errorf("insert request: %w", err)
	}

	batch := &pgx.Batch{}
	for _, n := range nodes {
		outcome := n.DecisionOutcome
		if outcome == "" {
			outcome = "pending"
		}
		batch.Queue(`
			INSERT INTO workflow_nodes
				(id, request_id, key, name, agent_type, department, status, description, progress_percent, status_text, decision_outcome, decision_summary, started_at, completed_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
		`, n.ID, n.RequestID, n.Key, n.Name, n.AgentType, n.Department, n.Status, n.Description, n.ProgressPercent, n.StatusText, outcome, n.DecisionSummary, n.StartedAt, n.CompletedAt)
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
	// node_flags — the risks agents raised, reference workflow_nodes above.
	for _, f := range flags {
		batch.Queue(`
			INSERT INTO node_flags (id, request_id, node_id, severity, message, ordinal)
			VALUES ($1, $2, $3, $4, $5, $6)
		`, f.ID, f.RequestID, f.NodeID, f.Severity, f.Message, f.Ordinal)
	}
	// node_checks — the exact policy-rule results, reference workflow_nodes above.
	for _, ch := range checks {
		batch.Queue(`
			INSERT INTO node_checks (id, request_id, node_id, label, status, detail, policy_title, ordinal)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		`, ch.ID, ch.RequestID, ch.NodeID, ch.Label, ch.Status, ch.Detail, ch.PolicyTitle, ch.Ordinal)
	}

	br := tx.SendBatch(ctx, batch)
	for range len(nodes) + len(edges) + len(tasks) + len(deps) + len(flags) + len(checks) {
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

// machineSpec is one machine in the demo seed.
type machineSpec struct {
	id            string
	name          string
	machineType   string
	location      string
	serialNumber  string
	status        string
	assignedEmail string
}

var machines = []machineSpec{
	{"mach_seed_cnc", "CNC Mill #4", "CNC Mill", "Building A, Floor 1", "CNC-2024-004", "down", "tech.lead@acme.test"},
	{"mach_seed_press", "Press Line B", "Hydraulic Press", "Building A, Floor 2", "PRS-2023-012", "operational", "tech.lead@acme.test"},
	{"mach_seed_laser", "Laser Cutter #1", "Laser Cutter", "Building B, Workshop", "LCS-2024-007", "operational", ""},
	{"mach_seed_server", "Server Rack A", "Server Rack", "Data Center, Rack A03", "SRV-001", "operational", ""},
	{"mach_seed_hvac", "HVAC Unit 3", "HVAC System", "Roof, East Wing", "HVAC-2023-003", "degraded", ""},
}

func seedMachines(ctx context.Context, pool *pgxpool.Pool, userIDs map[string]int64) error {
	for _, m := range machines {
		var assignedID *int64
		if m.assignedEmail != "" {
			if id, ok := userIDs[m.assignedEmail]; ok {
				assignedID = &id
			}
		}
		if _, err := pool.Exec(ctx, `
			INSERT INTO machines (id, org_id, assigned_user_id, name, machine_type, location, serial_number, status)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
			ON CONFLICT (id) DO NOTHING
		`, m.id, demoOrgID, assignedID, m.name, m.machineType, m.location, m.serialNumber, m.status); err != nil {
			return fmt.Errorf("insert machine %s: %w", m.name, err)
		}
	}
	return nil
}

// seedAuditEvents creates believable audit events for each seeded request.
// Completed nodes get node.started + node.completed; in_progress nodes get
// node.started; completed requests get request.completed.
// ────────────────────────────────────────────────────────────────────────────
// Incidents
// ────────────────────────────────────────────────────────────────────────────

type incidentSpec struct {
	id          string
	machineID   string
	title       string
	description string
	severity    string
	status      string
	reporter    string // email
}

var demoIncidents = []incidentSpec{
	{
		id:          "inc_seed_spindle",
		machineID:   "mach_seed_cnc",
		title:       "Spindle bearing failure",
		description: "CNC Mill #4 spindle bearing overheated and seized during production run. Machine shut down automatically.",
		severity:    "critical",
		status:      "open",
		reporter:    "tech.lead@acme.test",
	},
	{
		id:          "inc_seed_coolant",
		machineID:   "mach_seed_hvac",
		title:       "Coolant leak detected",
		description: "HVAC Unit 3 showing gradual coolant pressure loss. Compressor cycling frequently.",
		severity:    "high",
		status:      "in_progress",
		reporter:    "tech.lead@acme.test",
	},
}

func seedIncidents(ctx context.Context, pool *pgxpool.Pool, userIDs map[string]int64) (int, error) {
	for _, inc := range demoIncidents {
		reporterID, ok := userIDs[inc.reporter]
		if !ok {
			continue
		}
		// Get the machine's org_id.
		var orgID string
		if err := pool.QueryRow(ctx, `SELECT org_id FROM machines WHERE id = $1`, inc.machineID).Scan(&orgID); err != nil {
			return 0, fmt.Errorf("get machine org for %s: %w", inc.machineID, err)
		}
		if _, err := pool.Exec(ctx, `
			INSERT INTO incidents (id, machine_id, org_id, reported_by, title, description, severity, status, created_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, now() - interval '2 hours')
			ON CONFLICT (id) DO NOTHING
		`, inc.id, inc.machineID, orgID, reporterID, inc.title, inc.description, inc.severity, inc.status); err != nil {
			return 0, fmt.Errorf("insert incident %s: %w", inc.id, err)
		}
		// Create agent_task so the incident appears in the technician's work inbox.
		taskID := "enpanne:machine:" + inc.id
		if _, err := pool.Exec(ctx, `
			INSERT INTO agent_tasks (id, title, status, incident_id)
			VALUES ($1, $2, 'pending', $3)
			ON CONFLICT (id) DO NOTHING
		`, taskID, inc.title, inc.id); err != nil {
			return 0, fmt.Errorf("insert agent_task for incident %s: %w", inc.id, err)
		}
	}
	return len(demoIncidents), nil
}

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

			completedAt := submitted.Add(4 * time.Hour)

			// Flags an agent raised, so the risk is traceable and visible at the gate.
			for _, f := range r.profile.flags[n.Key] {
				if _, err := pool.Exec(ctx, `
					INSERT INTO audit_events (id, request_id, node_id, actor, action, reason, created_at)
					VALUES ($1, $2, $3, $4, $5, $6, $7)
				`, "aev_"+shortID(), r.id, &n.ID, n.Name, "node.flagged", f, completedAt); err != nil {
					return 0, fmt.Errorf("insert audit node.flagged for %s: %w", n.ID, err)
				}
				count++
			}

			if n.Status == "completed" {
				// A rejecting department records agent.rejected instead of completing clean.
				if r.profile.outcomes[n.Key] == "reject" {
					if _, err := pool.Exec(ctx, `
						INSERT INTO audit_events (id, request_id, node_id, actor, action, reason, created_at)
						VALUES ($1, $2, $3, $4, $5, $6, $7)
					`, "aev_"+shortID(), r.id, &n.ID, n.Name, "agent.rejected",
						n.Name+" rejected the request", completedAt); err != nil {
						return 0, fmt.Errorf("insert audit agent.rejected for %s: %w", n.ID, err)
					}
					count++
					continue
				}
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
		if r.status == "completed" || r.status == "in_progress" || r.status == "rejected" {
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

// seedAgents creates one agent per department that has a seeded demo team.
// Cross-cutting departments without a demo team (Planning, Executive) are
// skipped. Returns the number inserted.
func seedAgents(ctx context.Context, pool *pgxpool.Pool, teamByName map[string]string) (int, error) {
	count := 0
	for _, a := range orgdir.Agents {
		teamID, ok := teamByName[a.Department]
		if !ok {
			continue
		}
		if _, err := pool.Exec(ctx, `
			INSERT INTO agents (id, org_id, team_id, agent_type, name, capabilities)
			VALUES ($1, $2, $3, $4, $5, $6)
		`, "agent_seed_"+a.AgentType, demoOrgID, teamID, a.AgentType, a.Name, a.Capabilities); err != nil {
			return count, fmt.Errorf("insert agent %s: %w", a.AgentType, err)
		}
		count++
	}
	return count, nil
}

// seedPolicies creates the starter policies for each department that has a
// seeded demo team. Returns the number inserted.
func seedPolicies(ctx context.Context, pool *pgxpool.Pool, teamByName map[string]string) (int, error) {
	count := 0
	for _, p := range orgdir.Policies {
		teamID, ok := teamByName[p.Department]
		if !ok {
			continue
		}
		rulesJSON := []byte("[]")
		if len(p.Rules) > 0 {
			if b, mErr := json.Marshal(p.Rules); mErr == nil {
				rulesJSON = b
			}
		}
		if _, err := pool.Exec(ctx, `
			INSERT INTO department_policies (id, org_id, team_id, title, body, rules)
			VALUES ($1, $2, $3, $4, $5, $6)
		`, "pol_seed_"+strings.ToLower(p.Department), demoOrgID, teamID, p.Title, p.Body, rulesJSON); err != nil {
			return count, fmt.Errorf("insert policy %s: %w", p.Title, err)
		}
		count++
	}
	return count, nil
}
