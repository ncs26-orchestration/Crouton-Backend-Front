# AI Organization OS

An AI-powered multi-agent workflow system that processes business requests
through specialized department agents — Finance, Legal, IT, HR, Operations —
with full traceability, cross-agent collaboration, and a live visual canvas.

Submit a request like "Open a new office in Berlin" and watch it flow through
Intake → Planning → parallel Department Reviews → Executive Approval →
Implementation → Report, with every agent decision visible and clickable.

## What This Is

A hackathon MVP for the NCS challenge: prove that multiple AI agents can
collaborate on a single business request, with every decision traceable and
every handoff visible. The system is not a mockup — agents run real logic,
the graph reflects live state, and every status change is logged.

**One sentence:** AI agents organized as a company — with teams, roles, tasks,
dependencies, and an audit trail — processing a business request end to end.

## Documentation

| File | Purpose |
|---|---|
| [README.md](README.md) | This file. Setup, stack, architecture, commands. |
| [FEATURES.md](FEATURES.md) | The 10 vertical features (F1–F10) that define the MVP. |
| [VISION.md](VISION.md) | Product vision, demo narrative, what the jury sees. |
| [SYSTEM.md](SYSTEM.md) | System definition: domain objects, services, data flow. |
| [DESIGN.md](DESIGN.md) | Visual design system (Stripe-inspired) and UI specifications. |
| [PHASES.md](PHASES.md) | Implementation phases, what's built, what's next. |
| [plan.md](plan.md) | Development method, build order, verification. |
| `.agents/` | Agent alignment files — context for any AI working on this repo. |

## Architecture

```text
┌─────────────────────────────────────────────────────────────┐
│  REACT WEB APP                                              │
│  Shell rail, workflow canvas (React Flow), request panel,   │
│  agent roster, node detail panel, audit trail view          │
│        │                                                    │
│        │  /api/*                                            │
│        ▼                                                    │
│  GO API (Echo)                                              │
│  Auth (JWT), Orgs, Teams, Members, Requests,                │
│  Workflow Engine, Agent Orchestration, Audit Log             │
│        │                                                    │
│        ├── Python Agent Service (FastAPI)                    │
│        │   Department agent logic, LLM-powered decisions    │
│        │                                                    │
│        ├── PostgreSQL 18 + pgvector                         │
│        │   Orgs, teams, users, requests, workflow state,    │
│        │   agent tasks, audit events                        │
│        │                                                    │
│        └── Redis 8                                          │
│            Cache, queues, real-time state                   │
└─────────────────────────────────────────────────────────────┘
```

## What's Built

### Foundation (complete)
- **Authentication**: JWT-based login/register
- **Organizations**: Create/manage orgs with slug, branding
- **Teams**: Department teams within orgs (Finance, Legal, IT, HR, Operations)
- **Members**: Role-based membership (admin/executor/employee) + team roles (lead/member)
- **Projects**: Org-scoped project management
- **Navigation**: Shell rail with Home, Inbox, Workflows, Organization, Agents, Settings

### What's Next (see FEATURES.md)

**Core workflow:**
- F1: Request Intake & Workflow Plan
- F2: Workflow Graph Engine
- F3: Department Agents (Finance/Legal/IT)
- F4: Cross-Agent Dependencies & Handoffs
- F5: Executive Approval
- F6: Audit Trail
- F7: Execution Stage (HR/Operations)
- F8: Workflow Canvas UI
- F9: Agent Roster Panel
- F10: Final Report

**Sidebar tab views:**
- F11: Home — dashboard, recent requests, stats
- F12: My Work — personal inbox, assigned tasks
- F13: Requests — request list, create/filter
- F14: Agents — full roster with details
- F15: Reports — audit trail, generated reports
- F16: Integrations — connected systems

## Stack

### Web — `apps/web`
| | |
|---|---|
| Language | TypeScript 5.7 |
| UI | React 19.2 |
| Build | Vite 8 with `@vitejs/plugin-react` |
| Data fetching | TanStack Query v5 |
| Canvas | React Flow (workflow graph) |
| Dev server | `vite --host` on `:5173` with HMR |

### API — `apps/api`
| | |
|---|---|
| Language | Go 1.25 |
| HTTP | Echo v4.15 |
| Auth | JWT (bcrypt passwords, 7-day tokens) |
| Postgres | pgx v5.7 + pgxpool |
| Redis | go-redis v9.14 |
| Config | caarlos0/env v11 |
| Migrations | golang-migrate |
| Hot reload | air |

### Agent — `apps/agent`
| | |
|---|---|
| Language | Python 3.13 |
| HTTP | FastAPI 0.136 |
| Agent framework | LangGraph 1.1 |
| LLM dispatch | Ollama (default), Gemini, Anthropic |

### Data
| | |
|---|---|
| Postgres | pgvector/pgvector:pg18 |
| Redis | redis:8-alpine |

### Monorepo
| | |
|---|---|
| Package manager | pnpm 10.33 (workspaces) |
| Task runner | Turborepo 2.9 |
| Orchestration | Docker Compose v2 |

## Layout

```
apps/
  web/       React 19 + Vite + TanStack Query + React Flow
  api/       Go 1.25 + Echo + pgx + JWT auth
  agent/     FastAPI + LangGraph (department agent logic)
packages/
  tsconfig/  shared TS presets
infra/
  postgres/  init.sql (pgvector, pg_trgm)
  redis/     redis.conf
```

## Prerequisites

**Docker** and **Docker Compose** — everything else lives in containers.

Optional (for running outside Docker):
- Node 24 + pnpm 10
- Go 1.25+
- Python 3.13 + uv

## Setup

```bash
# 1. Copy env and set keys
cp .env.example .env
$EDITOR .env           # set JWT_SECRET (required), ANTHROPIC_API_KEY (optional)

# 2. Start everything
make up

# 3. Apply migrations (required on first run)
export $(cat .env | grep -v '^#' | xargs) && make migrate-up
```

## Endpoints (dev)

| Service | URL |
|---------|-----|
| Web | http://localhost:5173 |
| Go API | http://localhost:8080 |
| Agent | http://localhost:8000 |
| Postgres | localhost:5432 (app/app/app) |
| Redis | localhost:6379 |

The web app proxies `/api/*` → Go API and `/agent/*` → Agent.

## Commands

```bash
make up             # start everything (dev, hot reload)
make down           # stop, keep volumes
make logs           # tail all logs
make ps             # list services
make psql           # open psql
make redis-cli      # open redis-cli
make migrate-up     # apply pending migrations
make migrate-new name=add_workflow_nodes  # scaffold migration
make clean          # wipe everything
```

## API Routes

### Auth
```
POST   /auth/register     → Create account
POST   /auth/login        → Get JWT token
GET    /users/lookup       → Find user by email
```

### Organizations
```
POST   /orgs                          → Create org
GET    /orgs                          → List user's orgs
GET    /orgs/:orgId                   → Get org
DELETE /orgs/:orgId                   → Delete org (admin)
```

### Teams
```
POST   /orgs/:orgId/teams             → Create team
GET    /orgs/:orgId/teams             → List teams
GET    /orgs/:orgId/teams/:teamId     → Get team + members
PATCH  /orgs/:orgId/teams/:teamId     → Update team
DELETE /orgs/:orgId/teams/:teamId     → Delete team
```

### Members
```
POST   /orgs/:orgId/members                        → Add org member
GET    /orgs/:orgId/members                        → List org members
PATCH  /orgs/:orgId/members/:userId                → Update role
DELETE /orgs/:orgId/members/:userId                → Remove member
POST   /orgs/:orgId/teams/:teamId/members          → Add to team
DELETE /orgs/:orgId/teams/:teamId/members/:userId  → Remove from team
```

### Projects (org-scoped)
```
POST   /orgs/:orgId/projects          → Create project
GET    /orgs/:orgId/projects          → List projects
```

## Database Schema

```sql
-- Foundation (implemented)
organizations (id, name, slug, accent_color, created_at)
org_members   (org_id, user_id, role, joined_at)
teams         (id, org_id, name, description, color, created_at)
team_members  (team_id, user_id, role, joined_at)
users         (id, email, name, password_hash, created_at)
projects      (id, org_id, name, created_at)

-- Workflow engine (next: F1, F2)
-- requests, workflow_nodes, workflow_edges, agent_tasks, audit_events
```
