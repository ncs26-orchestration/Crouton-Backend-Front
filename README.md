# Pablo

Pablo is an operator tool that turns an unstructured conversation about a
business process into a versioned, executable workflow that can be compiled
and deployed to real workflow engines. The same canonical Workflow IR can be
rendered in the Pablo canvas, compiled to Camunda 7 BPMN, or compiled to Elsa 3
WorkflowDefinition JSON.

The core idea is simple:

```text
Conversation + documents
        -> AI extraction + clarifying questions
        -> canonical Workflow IR
        -> validation + lowering
        -> engine adapter
        -> Camunda Cockpit / Elsa Studio
```

## Documentation Map

Use these files together:

| File | Purpose |
|---|---|
| [README.md](README.md) | Entry point: architecture summary, stack, setup, commands. |
| [SYSTEM.md](SYSTEM.md) | Precise system definition: ownership boundaries, domain objects, pipeline, deploy contract. |
| [DESIGN.md](DESIGN.md) | Solution design system: technical architecture, UI principles, visual system. |
| [VISION.md](VISION.md) | Product vision and demo narrative: why the system exists and what it proves. |
| [PHASES.md](PHASES.md) | Implementation phases, user stories, status, and non-functional requirements. |
| [plan.md](plan.md) | Development method: planning, AI skills/workflows used, verification strategy. |
| [DeepWiki](https://deepwiki.com/Noussour/aup) | Generated documentation index for repository navigation and implementation context. |
| [docs/diagrams/README.md](docs/diagrams/README.md) | Markdown-rendered diagrams for GitHub/readers. |
| [docs/diagrams/index.html](docs/diagrams/index.html) | HTML diagram hub for architecture, compilation layers, data model, and flowcharts. |

Important diagrams:

- [Architecture](docs/diagrams/architecture.html)
- [Compilation layers](docs/diagrams/compilation-layers.html)
- [Data model](docs/diagrams/data-model.html)
- [Flowchart](docs/diagrams/flowchart.html)

Rendered in Markdown:

![Pablo architecture](docs/diagrams/01.webp)
![Pablo compilation pipeline](docs/diagrams/03.webp)
![Pablo compact pipeline](docs/diagrams/06.png)

## System Design

Pablo is split into four technical planes:

| Plane | Responsibility | Main modules |
|---|---|---|
| Authoring plane | Projects, chats, attachment upload, canvas preview, operator controls. | `apps/web` |
| Orchestration plane | CRUD, workflow versions, extraction orchestration, compile/deploy dispatch. | `apps/api` |
| AI plane | Extract process structure from conversation and documents; ask clarifying questions. | `apps/agent` |
| Runtime plane | Execute or host the final workflow artifact. Pablo does not own runtime execution. | Camunda 7, Elsa 3 |

The most important architectural decision is that Pablo owns the
specification layer, not the runtime layer. It stores chat context,
attachments, Workflow IR versions, deploy targets, and approval state. The
workflow engines own token execution, task inboxes, timers, incidents, and
runtime history.

## Architecture Overview

```text
apps/web
  React 19 + Vite UI
  project tree, chat thread, composer, canvas, settings, deploy controls
        |
        | /api/*
        v
apps/api
  Go + Echo
  projects, chats, messages, attachments, workflow versions
  validates IR, lowers IR, compiles through engine adapters
        |
        | /extract, /attachments/extract-text
        v
apps/agent
  FastAPI + LangGraph-ready Python service
  LLM provider dispatch: Groq, Gemini, Anthropic, Ollama
        |
        v
Postgres + Redis
  durable state, workflow versions, deploy targets, cache/queues
        |
        v
Camunda 7 / Elsa 3
  real engine deployments and visual runtime dashboards
```

For the full rendered diagram set, see [docs/diagrams/README.md](docs/diagrams/README.md).

## Core Workflow

1. The operator creates a project for a client organization.
2. The operator starts a workflow chat and describes a process, optionally
   attaching a PDF/TXT/MD/CSV document.
3. The agent extracts a canonical Workflow IR and returns clarifying
   questions for ambiguous elements.
4. The UI renders the best current workflow immediately, while low-confidence
   items remain visible in the chat loop.
5. The operator answers questions until the workflow is ready.
6. Pablo compiles the same IR to the selected target:
   - Camunda 7: BPMN 2.0 XML with BPMN DI layout.
   - Elsa 3: WorkflowDefinition JSON using activities available in the
     stock Elsa Server + Studio image.
7. Pablo deploys the artifact to a project-scoped connector and deep-links to
   Camunda Cockpit or Elsa Studio.

## Engine-Adapter Model

Engines are plugins, not dependencies. The Go API registers adapters behind
one interface:

```go
type Adapter interface {
    Kind() string
    Name() string
    Capabilities() Capabilities
    Compile(exe *ir.ExecutableIR) ([]byte, string, []ir.Diagnostic, error)
    Discover(ctx context.Context, endpoint, user, secret string) (*Projection, error)
    Deploy(ctx context.Context, endpoint, user, secret, name string, artifact []byte) (DeploymentResult, error)
}
```

Today:

- `camunda7` compiles to deployable BPMN 2.0 XML and opens Cockpit.
- `elsa3` compiles to deployable Elsa WorkflowDefinition JSON and opens
  Studio.

The same IR can therefore target different client engines without changing
the conversation or authoring workflow.

## Development Method

The implementation followed an AI-assisted engineering workflow documented in
[plan.md](plan.md):

- Plan the system as layers: conversation, IR, compiler, adapter, runtime.
- Use AI coding skills for codebase exploration, frontend implementation,
  backend/API implementation, compiler work, Docker/runtime diagnostics, and
  documentation synthesis.
- Validate each feature end-to-end against real containers, not only mocks:
  extraction, compile, Camunda deploy, Elsa deploy, Studio/Cockpit visibility.
- Keep diagrams and docs aligned with the implementation so the architecture
  can be explained from source files and running behavior.

## Stack

Polyglot monorepo: React 19 web, Go (Echo) API, FastAPI + LangGraph agent, all
backed by PostgreSQL 18 (with pgvector) and Redis 8.

## Stack

### Web — `apps/web`
| | |
|---|---|
| Language | TypeScript 5.7 |
| UI | [React 19.2](https://react.dev/) |
| Build tool | [Vite 8](https://vite.dev/) with `@vitejs/plugin-react` |
| Data fetching | [TanStack Query v5](https://tanstack.com/query/latest) + devtools |
| Runtime image | `nginx:alpine` (proxies `/api`, `/agent`) |
| Dev server | `vite --host` on `:5173` with HMR |

### API — `apps/api`
| | |
|---|---|
| Language | Go 1.25 |
| HTTP framework | [Echo v4.15](https://echo.labstack.com/) |
| Postgres driver | [pgx v5.7](https://github.com/jackc/pgx) + `pgxpool` |
| Redis client | [go-redis v9.14](https://github.com/redis/go-redis) |
| Config | [caarlos0/env v11](https://github.com/caarlos0/env) |
| Migrations | [golang-migrate](https://github.com/golang-migrate/migrate) (bundled in dev image) |
| Hot reload | [air](https://github.com/air-verse/air) |
| Runtime image | `gcr.io/distroless/static-debian12:nonroot` |
| Layout | Hexagonal-lite: `cmd/`, `internal/{config,http,handler,service,repo,domain}` |

### Agent — `apps/agent`
| | |
|---|---|
| Language | Python 3.13 (strict typing: `mypy --strict`, `ruff ANN`) |
| Package manager | [uv](https://docs.astral.sh/uv/) |
| HTTP framework | [FastAPI 0.136](https://fastapi.tiangolo.com/) |
| Agent framework | [LangGraph 1.1](https://langchain-ai.github.io/langgraph/) |
| State persistence | `langgraph-checkpoint-postgres` (`AsyncPostgresSaver`) |
| Postgres driver | `psycopg[binary,pool]` 3.2 |
| Vector client | [`pgvector`](https://pypi.org/project/pgvector/) Python adapter |
| Redis client | `redis` (async) 5.2 |
| LLM SDKs | `anthropic`, `openai` |
| Settings | `pydantic-settings` 2.7 |

### Data
| | |
|---|---|
| Postgres | `pgvector/pgvector:pg18` — Postgres 18 with [pgvector](https://github.com/pgvector/pgvector) and `pg_trgm` enabled |
| Redis | `redis:8-alpine` with AOF persistence, `allkeys-lru` eviction |

### Monorepo tooling
| | |
|---|---|
| Package manager | [pnpm 10.33](https://pnpm.io/) (workspaces) |
| Task runner | [Turborepo 2.9](https://turborepo.dev/) — drives Go and Python scripts too, via each app's `package.json` |
| Shared TS config | `packages/tsconfig` (`base.json`, `react.json`) |
| Orchestration | Docker Compose v2 — one `compose.yaml` (prod shape) + auto-merged `compose.override.yaml` (dev) |
| CI | GitHub Actions, one job per app (`web`, `api`, `agent`) |

### Layout

```
apps/
  web/     React 19 + Vite + TanStack Query
  api/     Go 1.25 + Echo + pgx + go-redis
  agent/   FastAPI + LangGraph (Postgres checkpointer)
packages/
  tsconfig/  shared TS presets
infra/
  postgres/  init.sql (pgvector, pg_trgm)
  redis/     redis.conf
```

## Prerequisites

You only need **Docker** and **Docker Compose** to run the stack. Everything
else (Go, Node, Python, uv) lives inside containers.

Optional, for running individual apps outside Docker:
- Node 24 + pnpm 10
- Go 1.25+
- Python 3.13 + [uv](https://docs.astral.sh/uv/)

## First-time setup

```bash
# 1. copy env and fill in LLM keys (use UPPERCASE keys!)
cp .env.example .env
$EDITOR .env           # set ANTHROPIC_API_KEY (and/or OPENAI_API_KEY)
```

**Important:** The `.env` file must use **UPPERCASE** keys (e.g., `DATABASE_URL`, not `database_url`).
Docker Compose reads uppercase environment variables.

## Run everything (dev mode, hot reload)

```bash
# Start all services (first time builds images)
make up

# Apply database migrations (REQUIRED on first run)
export $(cat .env | grep -v '^#' | xargs) && make migrate-up
```

The first `make up` may take a few minutes to build images. Subsequent runs are faster.

That single command:

- Starts Postgres 18 + pgvector, Redis 8, Go API, FastAPI agent, and the Vite
  dev server — all on a shared Docker network.
- Waits for Postgres and Redis to become healthy before starting the apps.
- Bind-mounts your source into each container so edits hot-reload:
  - Web → Vite HMR
  - API → `air` rebuilds on `.go` change
  - Agent → `uvicorn --reload`

### Endpoints (dev)

| Service   | URL                         |
|-----------|-----------------------------|
| Web       | http://localhost:5173       |
| Go API    | http://localhost:8080       |
| Agent     | http://localhost:8000       |
| Postgres  | `localhost:5432` (user/pass `app`/`app`, db `app`) |
| Redis     | `localhost:6379`            |

The web app proxies `/api/*` → Go API and `/agent/*` → agent automatically.

### Quick smoke tests

```bash
curl http://localhost:8080/readyz
# {"status":"ok","db":"up","redis":"up"}

curl -X POST http://localhost:8000/chat \
  -H 'content-type: application/json' \
  -d '{"message":"hello"}'
# {"thread_id":"...","reply":"..."}
```

## Common commands

```bash
make up           # start everything (dev)
make down         # stop, keep volumes
make logs         # tail all service logs
make ps           # list services
make psql         # open psql in the postgres container
make redis-cli    # open redis-cli in the redis container
make migrate-up   # apply pending migrations (REQUIRED on first run)
make migrate-new name=add_orders   # scaffold a new migration pair
make clean        # wipe containers, volumes, node_modules, build artifacts
```

## Running a single app

```bash
docker compose up web             # just the web (brings up its deps)
docker compose up api             # just the api
docker compose logs -f agent      # tail agent logs
docker compose exec api sh        # shell into api container
```

Outside Docker (requires local toolchains):

```bash
pnpm --filter web dev             # vite dev server
cd apps/api && go run ./cmd/server
cd apps/agent && uv run uvicorn app.main:app --reload
```

## Database migrations

Migrations live in `apps/api/migrations/` (used by [dbmate](https://github.com/amacneil/dbmate)).

```bash
make migrate-up        # apply pending migrations (run after make up)
make migrate-down      # rollback the last migration
make migrate-status    # show which migrations have been applied
make migrate-new name=create_orders   # create a new migration
```

## Production build

`compose.override.yaml` is dev-only. CI and prod use only `compose.yaml`:

```bash
docker compose -f compose.yaml up --build -d
```

This produces:
- Web → `nginx:alpine` serving a static Vite build, with `/api` and `/agent`
  proxied to the respective services.
- API → distroless Go binary.
- Agent → slim Python 3.13 image with a baked `.venv`.

## Layout notes

- **Each app has its own `Dockerfile`** with a `runtime` stage (prod) and a
  `dev` stage (hot reload).
- **Build context is the repo root** so any Dockerfile can `COPY` shared files
  (e.g. `packages/tsconfig`). Root `.dockerignore` keeps images lean.
- **Turborepo** orchestrates tasks across all three apps via each app's
  `package.json` scripts — even for Go and Python.
- **LangGraph state** is persisted to Postgres via `AsyncPostgresSaver`; pass
  a `thread_id` to `/chat` to resume a conversation.
