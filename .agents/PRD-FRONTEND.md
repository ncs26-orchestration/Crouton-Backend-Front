# PRD — Frontend (React Web)

Owner: Frontend team · Stack: React 19, Vite, React Flow, TanStack Query, Tailwind 4 · Skills: `.agents/skills/frontend/`

## Context

We pivot the web app from the AUP workflow *authoring* UI to the **AI Organization OS** dashboard in
`../mvp.png`: a left nav, a request submitted by a user, and a live, clickable workflow graph of
department stages with a Request Overview panel and a Node Detail panel. Match `../DESIGN.md`
(Stripe-inspired: sohne-var/Inter, navy `#061b31`, purple `#533afd`).

Frontend consumes the backend contract in `PRD-BACKEND.md` (REST + SSE). The agent layer is invisible
to the UI — it only talks to the Go API. Build against the contract; don't wait for the engine.

Read `i-impeccable`, `emil-design-eng`, `design-taste-frontend`, `i-react-flow-architect`,
`i-react-flow-node-ts`, `i-tailwind-design-system`, `i-frontend-api-integration-patterns`, and run
`react-doctor` before committing.

## Scope / ownership

- Expand the app shell and navigation; replace the AUP project/chat location machine.
- Build the six functional tabs + two display tabs.
- The 3-panel Workflows canvas (the centerpiece) with live SSE updates.
- Status-colored department nodes on React Flow.
- Out of scope: the dormant AUP `ChatView`/`IRCanvas` stay in the repo, unlinked from the new nav.

## Shell (`apps/web/src/App.tsx` + `components/ShellRail.tsx`)

- `ShellSection` becomes: `home | my-work | requests | workflows | agents | reports | integrations | policies | teams`.
- Location state becomes `{ section, requestId, nodeId }`; drop `ProjectTree` from the request flow.
- Keep auth/org-setup flow as-is (`AuthProvider`, `OrgProvider`).

## Views (`apps/web/src/views/`)

- **HomeView** (F11): active/recent requests, agent-activity summary, quick stats, recent audit feed, "New Request".
- **MyWorkView** (F12): pending approvals with **Approve/Reject + justification** (calls `POST /requests/:id/approve`),
  blocked items showing "Waiting for [agent]", links into the canvas. Replaces the empty `InboxView`.
- **RequestsView** (F13): list + filters + a New Request modal (title/description/priority → `POST /orgs/:id/requests`).
- **WorkflowView** (F8/F9): the centerpiece — see below.
- **AgentsView** (F14): roster grouped by team with live status (`GET /orgs/:id/agents`). Reuse pieces of `OrgView`.
- **ReportsView** (F15): audit-trail browser + completed-request reports + metrics (`GET /orgs/:id/audit`).
- **PoliciesView**: read-only browser of seeded `department_policies` (`GET /orgs/:id/policies`) — wired to real data, the agents consult these.
- **IntegrationsView**: display-only connected-systems cards (per the vision non-goal).
- **Teams**: keep existing `OrgView` (teams/members management).

## Workflows canvas — 3-panel layout (the mockup)

- **Left** Request Overview: title, request ID, requester, priority, status, progress bar, ETA, participating agents with live status dots.
- **Center** React Flow graph (`GET /requests/:id`): nodes auto-laid-out from stage order/edges.
- **Right** Node Detail: Overview / Tasks / Activity tabs (description, agent, progress %, task list, latest status_text, audit events) from `GET /requests/:id/nodes/:nodeId`.

New components (`apps/web/src/components/`): `DepartmentNode.tsx` (status-colored card: agent icon,
name, status badge, task count, spinner when in_progress) + `request-to-flow.ts` mapper + auto-layout
(dagre or a ranked layout). Reuse the existing `ReactFlowProvider`, edge components, and `index.css`
design tokens. Follow `i-react-flow-node-ts` for node typing/store integration.

Status colors (from `MVP-SPEC.md`): completed `#15be53`, in_progress `#533afd`, pending slate, blocked `#ea2261`.

## Live updates (`apps/web/src/lib/`)

- An `EventSource` client to `GET /requests/:id/events?token=<jwt>` that patches/invalidates the
  TanStack Query cache for the open request on each `node_status | request_status | task | audit` event.
- Interval-polling fallback if the stream drops. Handle reconnection and cancellation per
  `i-frontend-api-integration-patterns`.
- Add the new endpoints to `apps/web/src/lib/api.ts` and types to `apps/web/src/lib/types.ts`.

## Acceptance criteria

- Golden path works against a running backend: Home (seeded stats) → Requests → New Request "Open a
  new office in Berlin" (High) → auto-open Workflows → canvas renders and updates **live via SSE** →
  click Finance Review → right panel shows "Waiting for data from IT assessment" → IT completes →
  Finance unblocks on the canvas → My Work shows the pending Executive Approval → approve with
  justification → execution runs → Reports shows the completed report + audit trail.
- Node colors and the 3-panel layout match `../mvp.png` and `../DESIGN.md`.
- `pnpm --filter web build` passes; `react-doctor` scan is clean; no console errors.
