# PRD — Frontend (React Web)

Owner: Frontend team · Stack: React 19, Vite, React Flow, TanStack Query, Tailwind 4 · Skills: `.agents/skills/frontend/`

## Context

Pivot the web app from the legacy authoring UI to the **AI Organization OS** dashboard in `../mvp.png`:
a left nav, a user-submitted request, and a live clickable workflow graph of department stages with a
Request Overview panel and a Node Detail panel. Match `../DESIGN.md` (Stripe-inspired: navy `#061b31`,
purple `#533afd`). Consume the REST + SSE contract in `PRD-BACKEND.md`; the agent layer is invisible
to the UI. Build against the contract — don't wait for the engine.

Read `i-impeccable`, `emil-design-eng`, `design-taste-frontend`, `i-react-flow-architect`,
`i-react-flow-node-ts`, `i-tailwind-design-system`, `i-frontend-api-integration-patterns`; run
`react-doctor` before each commit. Build order = feature order below.

Status colors (from `MVP-SPEC.md`): completed `#15be53`, in_progress `#533afd`, pending slate, blocked `#ea2261`.

---

## FE-1 — Shell nav + routing refactor
Goal: the left rail and section model match the mockup. Skills: `i-react-patterns`.
Steps:
1. In `components/ShellRail.tsx` set `ShellSection = home | my-work | requests | workflows | agents | reports | integrations | policies | teams` with the mockup's icons + Teams group.
2. In `App.tsx` change location state to `{ section, requestId, nodeId }`; remove the `ProjectTree`/chat branch from the request flow (keep auth + `OrgProvider`).
3. Route each section to its view (placeholder components for now).
Acceptance: every nav item renders its (stub) view; deep state (`requestId`/`nodeId`) persists across reload.

## FE-2 — API client, types, SSE client
Goal: one typed data layer. Skills: `i-frontend-api-integration-patterns`, `i-react-state-management`.
Steps:
1. Add request/node/edge/agent/audit/policy types to `lib/types.ts` matching the contract.
2. Add methods to `lib/api.ts`: createRequest, listRequests, getRequest, getNode, approve, listAgents, listAudit, listPolicies.
3. `lib/sse.ts`: a hook `useRequestStream(requestId)` opening `EventSource('/requests/:id/events?token=<jwt>')`, patching the TanStack Query cache per `node_status|request_status|task|audit`, with reconnect + interval-poll fallback and cleanup on unmount.
Acceptance: `getRequest` populates a query; `useRequestStream` updates it on mock SSE; unmount closes the stream.

## FE-3 — Requests tab + New Request
Goal: create and list requests. Skills: `i-impeccable`.
Steps:
1. `views/RequestsView.tsx`: table (title, requester, priority, status, progress) with status/priority filters.
2. New Request modal (title, description, priority) → `createRequest` → on success navigate to Workflows with the new `requestId`.
3. Empty + loading + error states.
Acceptance: submitting "Open a new office in Berlin" (High) creates a request and lands on its Workflows canvas.

## FE-4 — Workflows canvas: 3-panel shell
Goal: the centerpiece layout. Skills: `i-react-flow-architect`.
Steps:
1. `views/WorkflowView.tsx`: three columns — left Request Overview, center canvas, right Node Detail.
2. Left panel from `getRequest`: title, request ID, requester, priority, status, progress bar, ETA, participating agents with live status dots.
3. Right panel from `getNode(selectedNodeId)`: Overview / Tasks / Activity tabs (description, agent, progress %, task list, latest `status_text`, audit events). Empty when no node selected.
Acceptance: opening a request shows all three panels; clicking a node fills the right panel.

## FE-5 — Department node + auto-layout
Goal: render the graph. Skills: `i-react-flow-node-ts`.
Steps:
1. `components/DepartmentNode.tsx`: status-colored card (agent icon, name, status badge, task count, spinner when in_progress), selectable.
2. `lib/request-to-flow.ts`: map nodes/edges → React Flow nodes/edges; auto-layout from stage order/edges (dagre or a ranked layout); parallel branches side by side, merge points.
3. Register the node type with the existing `ReactFlowProvider`; reuse edge components + `index.css` tokens; add a legend + fit/zoom controls.
Acceptance: the Berlin graph renders ~9–10 stages with parallel Finance/Legal/IT and HR/Ops branches; node colors reflect status.

## FE-6 — Live SSE wiring
Goal: the canvas updates itself. Skills: `i-frontend-api-integration-patterns`.
Steps:
1. Mount `useRequestStream(requestId)` in `WorkflowView`; node/edge/panel re-render from cache patches.
2. Animate status transitions (pending→in_progress→completed/blocked); show the "Waiting for [agent]" line on blocked nodes from `status_text`.
3. Reflect live changes in the left panel progress + participating-agent dots.
Acceptance: with the engine running, IT Assessment completing and Finance unblocking is visible on the canvas without a manual refresh.

## FE-7 — My Work + approval
Goal: the human-in-the-loop surface. Skills: `i-impeccable`.
Steps:
1. `views/MyWorkView.tsx` (replace `InboxView`): pending approvals, blocked items ("Waiting for [agent]"), recently completed; each links into the canvas node.
2. Approval action: Approve/Reject with a required justification textarea → `approve` → optimistic update.
Acceptance: when a request reaches the Executive gate it appears here; approving with justification resumes the workflow (visible on the canvas); rejecting stops it.

## FE-8 — Home dashboard
Goal: the opening/closing shot. Skills: `high-end-visual-design`.
Steps:
1. `views/HomeView.tsx`: active/recent requests, quick stats (total, completion rate, avg time), agent-activity summary, recent audit feed, "New Request".
Acceptance: with `DEMO_SEED` data, Home shows non-empty stats and the completed sample request.

## FE-9 — Agents roster
Goal: F14. Skills: `i-impeccable`.
Steps:
1. `views/AgentsView.tsx`: roster grouped by team with avatar, department, live status, capabilities (`listAgents`); click → detail with recent actions.
Acceptance: all seeded agents show, with status that goes "busy" while they own an in_progress node.

## FE-10 — Reports
Goal: F15. Skills: `i-impeccable`.
Steps:
1. `views/ReportsView.tsx`: audit-trail browser with filters (request/agent/date/action), completed-request report cards, and metrics.
Acceptance: the audit trail of a completed run is browsable with timestamps and reasons.

## FE-11 — Policies + Integrations
Goal: the two lighter tabs. Skills: `i-tailwind-design-system`.
Steps:
1. `views/PoliciesView.tsx`: read-only browser of `listPolicies` (the policies agents actually consult).
2. `views/IntegrationsView.tsx`: display-only connected-systems cards (per the vision non-goal).
Acceptance: Policies shows the seeded department policies; Integrations renders the static cards cleanly.

## FE-12 — Design pass
Goal: match the mockup. Skills: `emil-design-eng`, `i-impeccable`, `i-web-design-guidelines`.
Steps:
1. Audit spacing/typography/shadows/colors against `../DESIGN.md` and `../mvp.png`.
2. Polish transitions, empty states, focus rings; run `react-doctor` and fix findings.
Acceptance: side-by-side with `../mvp.png` the layout, colors, and node states match; `react-doctor` clean.

---

## Definition of done (frontend)
- The full golden path works against a running backend (Home → Requests → submit → live Workflows canvas → My Work approval → Reports).
- Canvas updates live via SSE; the Finance "Waiting for IT" → unblock is visible without refresh.
- `pnpm --filter web build` passes; no console errors; `react-doctor` clean.
