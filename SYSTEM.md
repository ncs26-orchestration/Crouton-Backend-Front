# AI Organization OS — System Definition

This document defines the system precisely. No pitch, no narrative.

## 0. Related Documents

| Document | Role |
|---|---|
| `README.md` | Entry point: architecture, stack, setup, API routes. |
| `FEATURES.md` | The 10 MVP features (F1–F10) as vertical slices. |
| `VISION.md` | Product vision, demo narrative, value proposition. |
| `DESIGN.md` | Visual design system (Stripe-inspired) and UI specs. |
| `PHASES.md` | Implementation phases and current status. |
| `plan.md` | Development method, build order, verification. |
| `.agents/` | AI agent alignment files for contributors. |

## 1. What This System Is

A **multi-agent workflow system** that processes a business request through
specialized AI agents organized into department teams. The system owns:

- **Request intake and workflow planning** — an Intake Agent reads a request
  and decides which stages are needed.
- **Workflow state management** — a graph engine tracks nodes (stages) and
  edges (dependencies) with per-node status.
- **Agent orchestration** — department agents (Finance, Legal, IT, HR,
  Operations, Executive) run tasks within their assigned nodes.
- **Cross-agent dependencies** — agents can declare they need data from
  another agent; the system models and resolves these dependencies.
- **Audit trail** — every status change, decision, and handoff is logged.
- **Live visualization** — a canvas UI renders the graph in real time.

## 2. Domain Objects

| Object | What It Is | Scope |
|---|---|---|
| **Organization** | A workspace. Has teams, members, projects. | Root |
| **Team** | A department group within an org (Finance Team, Legal Team, etc.). | Per org |
| **Member** | A user belonging to an org with a role (admin/executor/employee) and optionally to teams (lead/member). | Per org |
| **User** | An authenticated account (email, password, JWT). | Global |
| **Request** | A business need submitted for processing. Has: title, description, requester, priority, status, progress. | Per org |
| **Workflow** | The graph of stages (nodes + edges) created for a request. One workflow per request. | Per request |
| **Workflow Node** | A stage in the workflow. Has: name, type, owning agent, status (Pending/In Progress/Completed/Blocked), tasks, timestamps, description. | Per workflow |
| **Workflow Edge** | A directed connection between nodes. Type: sequential (solid) or parallel-merge (dashed). | Per workflow |
| **Agent** | A specialized AI actor assigned to a workflow node. Has: name, department, avatar, status. Each agent type has domain-specific decision logic. | Per node |
| **Agent Task** | A sub-task within a node's task list. Has: title, status, timestamp. Agents progress through their tasks using domain logic. | Per node |
| **Dependency** | A declared need from one agent for another's output. Models cross-agent collaboration. When the blocking agent completes, the dependent agent is unblocked. | Per workflow |
| **Audit Event** | An append-only log entry. Has: timestamp, actor (agent name), action, target node, reason. Powers the Activity and Audit Trail views. | Per workflow |
| **Project** | An org-scoped container for requests and workflows. | Per org |

## 3. Agent Types

| Agent | Department | Role in Workflow |
|---|---|---|
| **Intake Agent** | — | Reads the request, generates the workflow plan (which stages, what order, what's parallel). |
| **Planning Analyst** | Planning | Breaks down the request into actionable analysis areas. |
| **Finance Reviewer** | Finance | Budget feasibility, financial impact analysis, ROI projection. |
| **Legal Reviewer** | Legal | Compliance check, regulatory review, contract assessment. |
| **IT Manager** | IT | Technical feasibility, infrastructure assessment, security review. |
| **HR Manager** | HR | Staffing requirements, hiring plan, policy compliance. |
| **Operations Manager** | Operations | Logistics, facilities, operational planning. |
| **Executive Approver** | Executive | Convergence gate — reviews all upstream results, approves or rejects with justification. |

Each agent has:
- A fixed task list relevant to its domain.
- Decision logic that moves through tasks and reaches an end state.
- The ability to declare dependencies on other agents.
- Status updates written in plain language (explainability).

## 4. Workflow State Machine

### Node Status
```
Pending ──→ In Progress ──→ Completed
                │
                ├──→ Blocked (dependency declared)
                │       │
                │       └──→ In Progress (dependency resolved)
                │
                └──→ Completed
```

### Request Status
```
Submitted ──→ In Progress ──→ Completed
                                  │
                                  └──→ Report Generated
```

### Workflow Execution Rules
1. A node becomes **In Progress** when all its incoming sequential
   dependencies are Completed.
2. Parallel branches start simultaneously when their shared parent completes.
3. A merge node (after parallel branches) only activates when **all**
   incoming branches are Completed or Resolved.
4. A node becomes **Blocked** when its agent declares a dependency on
   another agent that hasn't completed yet.
5. A Blocked node automatically transitions to In Progress when the
   blocking agent completes.
6. The Executive Approval node only activates after all review and
   planning branches complete.

## 5. Services

| Service | Role | Stack |
|---|---|---|
| `apps/web` | UI. Shell rail, workflow canvas (React Flow), request overview panel, node detail panel (tabs: Overview, Tasks, Activity), agent roster, audit trail view. | React 19 + Vite + React Flow + TanStack Query |
| `apps/api` | HTTP API. Auth (JWT), org/team/member CRUD, request management, workflow graph engine (nodes, edges, state transitions), agent task management, audit event logging, dependency resolution. | Go 1.25 + Echo + pgx |
| `apps/agent` | Agent logic. Department-specific decision making, LLM-powered reasoning for intake planning and agent status updates. | Python 3.13 + FastAPI + LangGraph |
| `postgres` | Persistent store. Orgs, teams, users, requests, workflow nodes/edges, agent tasks, audit events, dependencies. | pgvector/pgvector:pg18 |
| `redis` | Cache, queues, real-time state propagation. | redis:8-alpine |

## 6. Data Flow

### Request Submission
```
User submits request (title, description, priority)
  → API creates Request record (status: Submitted)
  → API calls Agent service: Intake Agent
  → Intake Agent returns workflow plan (nodes + edges + agent assignments)
  → API creates Workflow with nodes and edges
  → API logs audit event: "Request submitted, workflow created"
  → UI renders canvas with all nodes
```

### Agent Execution
```
Workflow Engine checks: which nodes have all dependencies met?
  → For each ready node: status → In Progress
  → Agent service runs the node's agent logic
  → Agent progresses through its task list
  → Agent writes status updates (→ audit events)
  → If agent needs data from another: declare Dependency (→ node Blocked)
  → When agent completes: status → Completed
  → Engine checks: does this unblock any downstream nodes?
  → If yes: unblock them (Blocked → In Progress)
  → Log all transitions as audit events
```

### Approval Gate
```
All review branches Completed
  → Executive Approval node: Pending → In Progress
  → Executive Approver reviews upstream results
  → Decision: Approve (with justification) or Reject (with reason)
  → If Approve: downstream nodes unblocked
  → If Reject: workflow halted, reason logged
  → Audit event with full decision text
```

### Report Generation
```
Implementation node Completed
  → Review & Report node: In Progress
  → Auto-generate summary from audit trail:
    - What was requested
    - Who approved
    - What was flagged
    - What was executed
    - Total time
  → Report attached to workflow
  → Request status → Completed
```

## 7. UI Structure (all sidebar tabs)

### Shell Rail (persistent left sidebar)
```
[Logo] AI Organization OS

Home           → F11: Dashboard, stats, recent activity
My Work        → F12: Personal inbox, assigned tasks
Requests       → F13: Request list, create, filter
Workflows      → F8:  Workflow canvas (main view)
Agents         → F14: Full agent roster with details
Reports        → F15: Audit trail, generated reports
Integrations   → F16: Connected systems & data sources

─── TEAMS ───
Finance Team   → filter by department
IT Team
HR Team
Operations Team
```

### Workflows Tab Layout (the centerpiece)
```
┌──────────┬──────────────────────────────────┬──────────────┐
│ SHELL    │ TOP BAR                          │              │
│ RAIL     │ Workflow title, ID, priority,    │              │
│          │ status badge, share              │              │
│          ├──────────┬───────────────────────┤ NODE DETAIL  │
│          │ REQUEST  │                       │ PANEL        │
│          │ OVERVIEW │  WORKFLOW CANVAS      │              │
│          │          │                       │ Overview tab │
│          │ Title    │  Nodes + Edges        │  Description │
│          │ Progress │  Color-coded status   │  Agent       │
│          │ Steps    │  Click to select      │  Progress %  │
│          │ ETA      │  Zoom / Fit controls  │  Task list   │
│          │          │  Legend               │  Latest upd  │
│          ├──────────┤                       │              │
│          │ AGENTS   │                       │ Activity tab │
│          │ ROSTER   │                       │  Event log   │
│          │          │                       │              │
│          │ Agent 1  │                       │              │
│          │ Agent 2  │                       │              │
│          │ ...      │                       │              │
└──────────┴──────────┴───────────────────────┴──────────────┘
```

## 8. Database Schema (target)

```sql
-- Foundation (IMPLEMENTED)
users          (id UUID PK, email, name, password_hash, created_at)
organizations  (id UUID PK, name, slug UNIQUE, accent_color, created_at)
org_members    (org_id FK, user_id FK, role, joined_at, PK(org_id, user_id))
teams          (id UUID PK, org_id FK, name, description, color, created_at)
team_members   (team_id FK, user_id FK, role, joined_at, PK(team_id, user_id))
projects       (id UUID PK, org_id FK, name, created_at)

-- Workflow engine (TO BUILD: F1, F2)
requests       (id UUID PK, org_id FK, project_id FK, title, description,
                requester_name, priority, status, progress_current,
                progress_total, estimated_completion, created_at, updated_at)

workflow_nodes (id UUID PK, request_id FK, name, node_type, agent_type,
                agent_name, status, description, position_x, position_y,
                sort_order, created_at, updated_at)

workflow_edges (id UUID PK, request_id FK, source_node_id FK, target_node_id FK,
                edge_type, created_at)
                -- edge_type: 'sequential' | 'parallel_merge'

-- Agent tasks (TO BUILD: F3)
agent_tasks    (id UUID PK, node_id FK, title, status, completed_at,
                sort_order, created_at)

-- Dependencies (TO BUILD: F4)
agent_dependencies (id UUID PK, request_id FK, dependent_node_id FK,
                    blocking_node_id FK, reason, resolved, resolved_at,
                    created_at)

-- Audit trail (TO BUILD: F6)
audit_events   (id UUID PK, request_id FK, node_id FK NULL, actor,
                action, reason, created_at)
                -- append-only, never updated or deleted
```

## 9. Critical Files

### Backend
| File | What It Does |
|---|---|
| `apps/api/cmd/server/main.go` | Server entry point, DB/Redis init |
| `apps/api/internal/config/config.go` | Environment config (DATABASE_URL, JWT_SECRET, etc.) |
| `apps/api/internal/http/server.go` | Route registration |
| `apps/api/internal/auth/jwt.go` | JWT generation and validation |
| `apps/api/internal/middleware/auth.go` | Auth middleware |
| `apps/api/internal/handler/auth.go` | Login/register handlers |
| `apps/api/internal/handler/orgs.go` | Org/team/member CRUD |
| `apps/api/internal/handler/projects.go` | Project + chat handlers |
| `apps/api/migrations/` | SQL migrations |

### Frontend
| File | What It Does |
|---|---|
| `apps/web/src/App.tsx` | Root routing, auth flow |
| `apps/web/src/components/ShellRail.tsx` | Left navigation rail |
| `apps/web/src/contexts/AuthContext.tsx` | JWT token + user state |
| `apps/web/src/contexts/OrgContext.tsx` | Active org tracking |
| `apps/web/src/views/LoginView.tsx` | Login page |
| `apps/web/src/views/RegisterView.tsx` | Registration page |
| `apps/web/src/views/OrgSetupView.tsx` | 3-step org onboarding |
| `apps/web/src/lib/api.ts` | API client (auth, orgs, teams, members) |
| `apps/web/src/lib/auth.ts` | Token persistence (localStorage) |

### Agent
| File | What It Does |
|---|---|
| `apps/agent/app/nodes/extract.py` | LLM extraction (existing, to be adapted) |
