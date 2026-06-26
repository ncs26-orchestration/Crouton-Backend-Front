# AI Organization OS — Implementation Phases

## Phase 0: Foundation ✅ COMPLETE

Everything needed before the 10 MVP features can be built.

| Component | Status | What's Built |
|-----------|--------|-------------|
| Auth (JWT) | ✅ | Login, register, token validation, bcrypt passwords |
| Users | ✅ | User table with email, name, password_hash |
| Organizations | ✅ | Org CRUD, slug, accent color |
| Org Members | ✅ | Add/remove/update members, role-based (admin/executor/employee) |
| Teams | ✅ | Team CRUD within orgs, description, color |
| Team Members | ✅ | Add/remove users to teams, lead/member roles |
| Projects (org-scoped) | ✅ | Projects belong to orgs via org_id FK |
| Web Auth Flow | ✅ | Login → Register → Org Setup (3-step wizard) |
| Shell Navigation | ✅ | ShellRail with Home, Inbox, Workflows, Org, Agents, Settings |
| Org Management UI | ✅ | Teams tab, Members tab, add/remove, role assignment |
| API Client | ✅ | TypeScript client for all auth/org/team/member endpoints |
| Auth Context | ✅ | React context for JWT token + user state |
| Org Context | ✅ | React context for active organization |
| DB Migrations | ✅ | organizations, org_members, teams, team_members, projects (org-scoped) |

**Key files:**
- `apps/api/internal/handler/auth.go` — login/register
- `apps/api/internal/handler/orgs.go` — org/team/member CRUD
- `apps/api/internal/auth/jwt.go` — JWT logic
- `apps/api/internal/middleware/auth.go` — auth middleware
- `apps/web/src/contexts/AuthContext.tsx` — auth state
- `apps/web/src/contexts/OrgContext.tsx` — org state
- `apps/web/src/views/LoginView.tsx` — login
- `apps/web/src/views/RegisterView.tsx` — register
- `apps/web/src/views/OrgSetupView.tsx` — org onboarding

---

## Phase 1: Request Intake + Workflow Graph ⏳ NEXT

> **Features:** F1 (Request Intake), F2 (Workflow Graph Engine), F6 (Audit Trail start)

### F1 — Request Intake & Workflow Plan

| Task | Status |
|------|--------|
| Create `requests` table migration | ⏳ |
| Request CRUD endpoints (create, get, list) | ⏳ |
| Intake Agent logic — reads request, generates workflow plan | ⏳ |
| Request Overview UI panel (title, progress, ETA, status) | ⏳ |
| Submit Request form/modal | ⏳ |

### F2 — Workflow Graph Engine

| Task | Status |
|------|--------|
| Create `workflow_nodes` + `workflow_edges` tables | ⏳ |
| Node state machine (Pending → In Progress → Completed / Blocked) | ⏳ |
| Graph traversal — unblock downstream nodes when dependencies clear | ⏳ |
| API endpoints: get workflow for request, update node status | ⏳ |
| Parallel branch support (multiple nodes start when parent completes) | ⏳ |
| Merge point logic (wait for all incoming branches) | ⏳ |

### F6 — Audit Trail (start early)

| Task | Status |
|------|--------|
| Create `audit_events` table (append-only) | ⏳ |
| Log audit events on every state change from day one | ⏳ |
| API endpoint: list audit events for a request/node | ⏳ |

---

## Phase 2: Department Agents + Dependencies ⏳

> **Features:** F3 (Department Agents), F4 (Cross-Agent Dependencies)

### F3 — Department Agents

| Task | Status |
|------|--------|
| Create `agent_tasks` table | ⏳ |
| Finance Reviewer agent (budget check, impact analysis, ROI) | ⏳ |
| Legal Reviewer agent (compliance, regulatory, contracts) | ⏳ |
| IT Manager agent (technical feasibility, infra, security) | ⏳ |
| Each agent writes plain-language status updates | ⏳ |
| Agent task progression logic (move through task list) | ⏳ |

### F4 — Cross-Agent Dependencies

| Task | Status |
|------|--------|
| Create `agent_dependencies` table | ⏳ |
| Dependency declaration API (agent X needs agent Y) | ⏳ |
| Auto-unblock when blocking agent completes | ⏳ |
| Node status → Blocked when dependency declared | ⏳ |
| At least one visible dependency: Finance ← IT | ⏳ |

---

## Phase 3: Approval + Execution ⏳

> **Features:** F5 (Executive Approval), F7 (Execution Stage)

### F5 — Executive Approval

| Task | Status |
|------|--------|
| Approval gate logic — only activates when all upstream complete | ⏳ |
| Executive Approver agent with decision logic | ⏳ |
| Written approve/reject justification | ⏳ |
| On reject: workflow halted with reason | ⏳ |

### F7 — Execution Stage

| Task | Status |
|------|--------|
| HR Manager agent (staffing, hiring plan, policies) | ⏳ |
| Operations Manager agent (logistics, facilities) | ⏳ |
| Implementation node (completes when sub-tasks done) | ⏳ |
| Both must complete before Implementation can start | ⏳ |

---

## Phase 4: UI — Canvas + Panels + Report ⏳

> **Features:** F8 (Workflow Canvas), F9 (Agent Roster), F10 (Final Report)

### F8 — Workflow Canvas UI (Workflows tab)

| Task | Status |
|------|--------|
| React Flow canvas rendering nodes + edges from API | ⏳ |
| Node color-coding by status (green/blue/yellow/red) | ⏳ |
| Click node → open right-side detail panel | ⏳ |
| Detail panel tabs: Overview, Tasks, Activity | ⏳ |
| Overview tab: description, agent, progress %, task list, latest update | ⏳ |
| Activity tab: audit events for this node | ⏳ |
| Top bar: workflow title, request ID, priority badge, status badge | ⏳ |
| Left Request Overview card (progress bar, steps, ETA) | ⏳ |
| Legend (Completed/In Progress/Pending/Blocked) | ⏳ |
| Zoom/Fit controls | ⏳ |
| Stripe-inspired design per DESIGN.md | ⏳ |

### F9 — Agent Roster Panel

| Task | Status |
|------|--------|
| Participating agents list with live status badges | ⏳ |
| Agent avatars and department grouping | ⏳ |
| Teams section in sidebar matches org teams | ⏳ |

### F10 — Final Report

| Task | Status |
|------|--------|
| Auto-generate summary when Implementation completes | ⏳ |
| Report content: request, approvals, flags, execution, time | ⏳ |
| Attach report to Review & Report node | ⏳ |

---

## Phase 5: Sidebar Tab Views ⏳

> **Features:** F11 (Home), F12 (My Work), F13 (Requests), F14 (Agents), F15 (Reports), F16 (Integrations)

### F11 — Home Dashboard

| Task | Status |
|------|--------|
| Recent/active requests list with status badges | ⏳ |
| Agent activity summary (active count, tasks completed) | ⏳ |
| Quick stats cards (requests, completion rate, avg time) | ⏳ |
| "New Request" quick action button | ⏳ |
| Recent audit events feed (last 5–10) | ⏳ |

### F12 — My Work (Personal Inbox)

| Task | Status |
|------|--------|
| Tasks assigned to current user/department | ⏳ |
| Pending approval items | ⏳ |
| Blocked items with "Waiting for [agent]" indicator | ⏳ |
| Recently completed items | ⏳ |
| Each item links to workflow node | ⏳ |

### F13 — Requests List

| Task | Status |
|------|--------|
| All requests table with title, requester, priority, status, progress | ⏳ |
| Status and priority filter controls | ⏳ |
| "New Request" button with submission form | ⏳ |
| Click request → navigate to workflow canvas | ⏳ |

### F14 — Agents Management Page

| Task | Status |
|------|--------|
| Full agent roster with name, department, avatar, status | ⏳ |
| Agent capability descriptions | ⏳ |
| Agent activity stats (tasks completed, current workload) | ⏳ |
| Group by team (Finance, Legal, IT, HR, Operations) | ⏳ |
| Click agent → detail view with recent actions | ⏳ |

### F15 — Reports Page

| Task | Status |
|------|--------|
| Completed workflow reports list | ⏳ |
| Full audit trail browser with filters | ⏳ |
| Filter by request, agent, date, action type | ⏳ |
| Key metrics: requests processed, avg time, most active agents | ⏳ |

### F16 — Integrations Page

| Task | Status |
|------|--------|
| Integration cards: name, icon, status, which agents use it | ⏳ |
| Financial systems (SAP, QuickBooks) | ⏳ |
| Legal databases (LexisNexis, compliance) | ⏳ |
| IT infrastructure (AWS, Azure, CMDB) | ⏳ |
| HR systems (Workday, BambooHR) | ⏳ |
| Communication (Slack, Email) | ⏳ |

---

## Definition of Done (MVP)

**Core workflow (must have):**
- [ ] One request can be submitted and generates a real workflow plan (F1)
- [ ] The graph shows correct branching/merging and live status per node (F2)
- [ ] At least 2 agents run in true parallel with independent task lists (F3)
- [ ] At least one real cross-agent dependency is visible and resolves (F4)
- [ ] Approval only unlocks after dependencies clear, with written decision (F5)
- [ ] Every state change is logged and viewable node by node (F6)
- [ ] Execution stage runs post-approval and completes (F7)
- [ ] Full graph is visible, clickable, matches live backend state (F8)
- [ ] Agent roster reflects real-time status (F9)
- [ ] Final report generated automatically at the end (F10)

**All sidebar tabs functional:**
- [ ] Home shows dashboard with recent activity (F11)
- [ ] My Work shows personal task inbox (F12)
- [ ] Requests shows list with create/filter/navigate (F13)
- [ ] Agents shows full roster with details (F14)
- [ ] Reports shows audit trail and completed reports (F15)
- [ ] Integrations shows connected systems (F16)

---

## Build Priority

```
Phase 0 ✅ (Foundation — auth, orgs, teams, members)
    │
    ▼
Phase 1 ⏳ (F1 + F2 + F6 — the backbone)
    │        F13 (Requests list — thin, reads F1)
    ▼
Phase 2 ⏳ (F3 + F4 — agents + dependencies)
    │        F14 (Agents page — reads F3)
    ▼
Phase 3 ⏳ (F5 + F7 — approval + execution)
    │        F12 (My Work — reads F3/F5)
    ▼
Phase 4 ⏳ (F8 + F9 + F10 — canvas + roster + report)
    │
    ▼
Phase 5 ⏳ (F11 + F15 + F16 — dashboard + reports + integrations)

F8/F9: start with mock data immediately.
F16 (Integrations): display-only cards, build last.
```
