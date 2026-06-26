# Project Context — AI Organization OS

## What Is This Project?

A hackathon MVP: an AI-powered multi-agent workflow system. Multiple AI
agents (Finance, Legal, IT, HR, Operations) collaborate to process a
business request through a visible, traceable workflow graph.

**The demo:** Submit "Open a new office in Berlin" → watch it flow through
Intake → Planning → parallel Reviews → Approval → Implementation → Report,
with every agent decision clickable on a live canvas.

## Hackathon Theme

Modern organizations operate through complex, multi-step workflows involving
multiple departments (HR, Finance, Legal, Operations). This challenge asks
participants to build a **multi-agent system that simulates how real companies
coordinate work internally** — not a single chatbot, not a simple automation
pipeline, but a network of specialized AI agents that collaborate, validate,
and make decisions like employees in a real organization.

### Judging Criteria (prioritize these in every feature)

1. **Realistic workflow** — the workflow must represent how a real organization
   actually processes requests, with realistic stages and department involvement
2. **Clear separation of responsibilities** — each agent owns a distinct
   department role; no agent does everything
3. **Meaningful decision-making** — agents must validate, analyze, flag risks,
   approve/reject with reasoning — not just pass data through
4. **Genuine collaboration and dependencies** — agents depend on each other's
   output, wait for each other, share findings — this is the core differentiator
5. **Traceability** — every decision must be logged and viewable: who decided
   what, when, and why — the audit trail is the proof of organizational intelligence

### What judges will look for specifically
- Multiple specialized agents (not one agent wearing hats)
- At least three stages of decision-making
- Structured handoffs between stages
- Realistic processes: request creation → validation → approval → execution → reporting
- How decisions were made throughout the workflow (the audit trail)

## Tech Stack

- **Frontend:** React 19 + Vite + TypeScript + React Flow + TanStack Query
- **Backend:** Go 1.25 + Echo + pgx + JWT auth
- **Agents:** Python 3.13 + FastAPI + LangGraph
- **Data:** PostgreSQL 18 (pgvector) + Redis 8
- **Monorepo:** pnpm + Turborepo + Docker Compose

## What's Already Built (Phase 0 — Foundation)

Everything needed to support the multi-agent features:

### Backend (`apps/api`)
- **Auth:** JWT login/register (`internal/handler/auth.go`, `internal/auth/jwt.go`)
- **Middleware:** JWT validation (`internal/middleware/auth.go`)
- **Organizations:** Full CRUD (`internal/handler/orgs.go`)
- **Teams:** Create/list/get/update/delete teams within orgs
- **Members:** Org members (admin/executor/employee) + team members (lead/member)
- **Projects:** Org-scoped project management
- **Config:** Environment-based (`internal/config/config.go`)
- **Routes:** All registered in `internal/http/server.go`

### Frontend (`apps/web`)
- **Auth flow:** Login → Register → Org Setup (3-step wizard)
- **Shell:** Navigation rail (ShellRail) with all sections
- **Org management:** Teams tab, Members tab, full CRUD UI
- **Contexts:** AuthContext (JWT/user), OrgContext (active org)
- **API client:** TypeScript client for all endpoints (`lib/api.ts`)
- **Views:** LoginView, RegisterView, OrgSetupView, OrgView, AgentsView (empty), InboxView (empty)

### Database
```sql
users          (id, email, name, password_hash, created_at)
organizations  (id, name, slug, accent_color, created_at)
org_members    (org_id, user_id, role, joined_at)
teams          (id, org_id, name, description, color, created_at)
team_members   (team_id, user_id, role, joined_at)
projects       (id, org_id, name, created_at)
```

### Legacy (from previous "Pablo" version)
The project previously had workflow authoring features (chat → extract →
compile → deploy to Camunda/Elsa). Some of this code still exists in:
- `apps/api/internal/handler/projects.go` — chat/message handlers
- `apps/agent/app/nodes/extract.py` — LLM extraction
- `apps/web/src/views/ChatView.tsx` — chat + canvas view

This code can be adapted or replaced for the new multi-agent system.

## What Needs to Be Built (Features F1–F16)

See `../FEATURES.md` for the full spec. Summary:

**Core workflow (F1–F10):**

| Feature | What | Priority |
|---------|------|----------|
| F1 | Request Intake — submit request, Intake Agent generates workflow plan | HIGH |
| F2 | Workflow Graph Engine — nodes + edges + state machine | HIGH |
| F3 | Department Agents — Finance, Legal, IT with real task lists | HIGH |
| F4 | Cross-Agent Dependencies — Finance waits for IT | HIGH |
| F5 | Executive Approval — convergence gate | MEDIUM |
| F6 | Audit Trail — append-only event log | HIGH (start early) |
| F7 | Execution Stage — HR, Operations, Implementation | MEDIUM |
| F8 | Workflow Canvas UI — React Flow, clickable, live | HIGH |
| F9 | Agent Roster Panel — agent list with status | MEDIUM |
| F10 | Final Report — auto-generated summary | MEDIUM |

**Sidebar tab views (F11–F16):**

| Feature | Tab | What | Priority |
|---------|-----|------|----------|
| F11 | Home | Dashboard, recent requests, stats, activity feed | MEDIUM |
| F12 | My Work | Personal inbox, assigned tasks, blocked items | MEDIUM |
| F13 | Requests | Request list, create new, filter, navigate | HIGH |
| F14 | Agents | Full agent roster with details and capabilities | MEDIUM |
| F15 | Reports | Audit trail browser, completed reports, metrics | MEDIUM |
| F16 | Integrations | Connected systems agents use (display-only for MVP) | LOW |

### Build order:
```
F1 + F2 + F6 first (backbone) + F13 (Requests list)
  → F3 + F4 (agents + dependencies) + F14 (Agents page)
    → F5 + F7 (approval + execution) + F12 (My Work)
      → F8 + F9 + F10 (canvas + roster + report)
        → F11 + F15 + F16 (dashboard + reports + integrations)

F8/F9: start with mock data immediately.
F16: display-only cards, build last.
All sidebar tabs must show content, even if mock data.
```

## Key Decisions

1. **Agents run server-side.** The Go API orchestrates agent execution.
   The Python service handles LLM calls for agent decision-making.

2. **The graph is the source of truth.** All node status, task progress,
   and dependencies are stored in PostgreSQL. The UI reads from this.

3. **Audit trail from day one.** Every state change writes an audit event.
   This is a core feature, not an afterthought.

4. **Mock-first UI.** The canvas (F8) should be built with hardcoded data
   matching the screenshot first, then wired to real APIs.

5. **Deterministic fallback.** If LLM is slow, agents can use rule-based
   logic with LLM only for status message generation.

6. **One request, fully alive.** The MVP shows one request flowing through
   the entire pipeline. No request history, no multi-workflow support.

## Design System

The UI follows DESIGN.md — a Stripe-inspired visual system:
- **Font:** sohne-var with `"ss01"`, weight 300 for headings
- **Colors:** Navy headings (#061b31), purple accents (#533afd), slate body (#64748d)
- **Shadows:** Blue-tinted (`rgba(50,50,93,0.25)`)
- **Radius:** 4px–8px (conservative, no pills)
- **Spacing:** 8px base unit

See `../DESIGN.md` for full specifications.

## Running the Project

```bash
cp .env.example .env       # set JWT_SECRET
make up                    # start all services
make migrate-up            # apply migrations
# Web: http://localhost:5173
# API: http://localhost:8080
```
