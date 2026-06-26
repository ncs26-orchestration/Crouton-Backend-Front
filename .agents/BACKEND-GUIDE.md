# Backend Guide — Go API

## Architecture

The Go API (`apps/api`) is the control plane. It uses Echo v4 with a
hexagonal-lite layout:

```
apps/api/
  cmd/server/main.go          → entry point, DB/Redis init
  internal/
    config/config.go           → env-based config (DATABASE_URL, JWT_SECRET, etc.)
    auth/jwt.go                → JWT generation + validation
    middleware/auth.go         → Echo middleware for JWT auth
    handler/
      auth.go                  → login, register handlers
      orgs.go                  → org, team, member CRUD handlers
      projects.go              → project + chat handlers (legacy, to adapt)
    http/server.go             → route registration
    repo/                      → database queries (if separated)
    domain/                    → domain types (if separated)
  migrations/                  → SQL migration files
```

## Patterns

### Handler Pattern
Handlers are methods on a struct that holds DB pool and config:

```go
type Handler struct {
    DB     *pgxpool.Pool
    Config *config.Config
}

func (h *Handler) CreateRequest(c echo.Context) error {
    // 1. Parse + validate input
    // 2. Get user from context (set by auth middleware)
    // 3. Query database
    // 4. Return JSON response
    return c.JSON(http.StatusCreated, result)
}
```

### Auth Middleware
The JWT middleware extracts the user from the token and sets it in
Echo's context. Handlers access it via:

```go
userID := c.Get("user_id").(string)
```

### Route Registration
Routes are registered in `internal/http/server.go`. Pattern:

```go
// Public routes (no auth)
e.POST("/auth/login", h.Login)
e.POST("/auth/register", h.Register)

// Protected routes (require JWT)
api := e.Group("", authMiddleware)
api.POST("/orgs", h.CreateOrg)
api.GET("/orgs", h.ListOrgs)
```

### Database Queries
Use pgx directly with raw SQL. No ORM. Pattern:

```go
row := h.DB.QueryRow(ctx,
    `INSERT INTO requests (id, org_id, title, description, priority, status)
     VALUES (gen_random_uuid(), $1, $2, $3, $4, 'submitted')
     RETURNING id, created_at`,
    orgID, req.Title, req.Description, req.Priority,
)
var id string
var createdAt time.Time
err := row.Scan(&id, &createdAt)
```

### Migrations
SQL files in `apps/api/migrations/`. Naming: `YYYYMMDDHHMMSS_name.sql`.
Run with `make migrate-up`.

## What to Build Next

### F1 — Requests Table + Endpoints

Migration:
```sql
CREATE TABLE requests (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id UUID NOT NULL REFERENCES organizations(id),
    project_id UUID REFERENCES projects(id),
    title TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    requester_name TEXT NOT NULL DEFAULT '',
    priority TEXT NOT NULL DEFAULT 'medium',
    status TEXT NOT NULL DEFAULT 'submitted',
    progress_current INT NOT NULL DEFAULT 0,
    progress_total INT NOT NULL DEFAULT 0,
    estimated_completion TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

Endpoints:
```
POST   /orgs/:orgId/requests          → create request + trigger intake
GET    /orgs/:orgId/requests          → list requests
GET    /orgs/:orgId/requests/:id      → get request with workflow
```

### F2 — Workflow Nodes + Edges

Migration:
```sql
CREATE TABLE workflow_nodes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    request_id UUID NOT NULL REFERENCES requests(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    node_type TEXT NOT NULL DEFAULT 'stage',
    agent_type TEXT NOT NULL DEFAULT '',
    agent_name TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'pending',
    description TEXT NOT NULL DEFAULT '',
    latest_update TEXT NOT NULL DEFAULT '',
    progress_current INT NOT NULL DEFAULT 0,
    progress_total INT NOT NULL DEFAULT 0,
    position_x REAL NOT NULL DEFAULT 0,
    position_y REAL NOT NULL DEFAULT 0,
    sort_order INT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE workflow_edges (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    request_id UUID NOT NULL REFERENCES requests(id) ON DELETE CASCADE,
    source_node_id UUID NOT NULL REFERENCES workflow_nodes(id) ON DELETE CASCADE,
    target_node_id UUID NOT NULL REFERENCES workflow_nodes(id) ON DELETE CASCADE,
    edge_type TEXT NOT NULL DEFAULT 'sequential',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

### F3 — Agent Tasks

```sql
CREATE TABLE agent_tasks (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    node_id UUID NOT NULL REFERENCES workflow_nodes(id) ON DELETE CASCADE,
    title TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending',
    completed_at TIMESTAMPTZ,
    sort_order INT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

### F4 — Dependencies

```sql
CREATE TABLE agent_dependencies (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    request_id UUID NOT NULL REFERENCES requests(id) ON DELETE CASCADE,
    dependent_node_id UUID NOT NULL REFERENCES workflow_nodes(id),
    blocking_node_id UUID NOT NULL REFERENCES workflow_nodes(id),
    reason TEXT NOT NULL DEFAULT '',
    resolved BOOLEAN NOT NULL DEFAULT false,
    resolved_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

### F6 — Audit Events

```sql
CREATE TABLE audit_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    request_id UUID NOT NULL REFERENCES requests(id) ON DELETE CASCADE,
    node_id UUID REFERENCES workflow_nodes(id),
    actor TEXT NOT NULL,
    action TEXT NOT NULL,
    reason TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
-- Append-only: never UPDATE or DELETE
CREATE INDEX idx_audit_events_request ON audit_events(request_id, created_at);
CREATE INDEX idx_audit_events_node ON audit_events(node_id, created_at);
```

## Workflow Engine Logic

The core engine logic belongs in the Go API (not the Python agent).
The Python agent handles LLM calls for agent decision-making.

### State Transition Rules
```go
// When a node completes:
func (h *Handler) completeNode(ctx context.Context, nodeID string) error {
    // 1. Set node status = "completed"
    // 2. Log audit event
    // 3. Find all edges where source = this node
    // 4. For each target node:
    //    a. Check if ALL incoming edges have completed sources
    //    b. If yes: transition target to "in_progress"
    //    c. If the target was "blocked" and its dependency resolved: unblock
    // 5. Update request progress (progress_current++)
}
```

### Dependency Resolution
```go
// When a blocking node completes, resolve all its dependencies:
func (h *Handler) resolveDependencies(ctx context.Context, nodeID string) error {
    // 1. Find all dependencies where blocking_node_id = nodeID
    // 2. Set resolved = true, resolved_at = now()
    // 3. For each dependent node: if status = "blocked", set to "in_progress"
    // 4. Log audit events
}
```

## Conventions

- **UUIDs** for all primary keys (gen_random_uuid())
- **Timestamps** as TIMESTAMPTZ, default now()
- **Status values** as TEXT (not enums) for flexibility: "pending", "in_progress", "completed", "blocked"
- **Soft conventions** over hard constraints — the app enforces business rules, not the DB
- **JSON responses** use camelCase keys (Echo default)
- **Error responses** as `{"error": "message"}`
