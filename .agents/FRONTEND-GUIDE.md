# Frontend Guide — React Web App

## Architecture

The web app (`apps/web`) is a React 19 + Vite + TypeScript SPA.

```
apps/web/src/
  App.tsx                    → root routing + auth flow
  main.tsx                   → entry point
  index.css                  → global styles
  components/
    ShellRail.tsx             → left navigation rail
    OnboardingWizard.tsx      → org onboarding (legacy)
    ProjectTree.tsx           → project sidebar (legacy)
  contexts/
    AuthContext.tsx            → JWT token + user state
    OrgContext.tsx             → active organization state
  views/
    LoginView.tsx              → email/password login
    RegisterView.tsx           → registration form
    OrgSetupView.tsx           → 3-step org wizard
    OrgView.tsx                → org/team/member management
    AgentsView.tsx             → agent list (placeholder)
    InboxView.tsx              → inbox (placeholder)
    ProjectsHomeView.tsx       → project list
    SettingsView.tsx           → settings (placeholder)
  lib/
    api.ts                     → API client (fetch-based)
    auth.ts                    → token localStorage helpers
```

## Patterns

### API Client (`lib/api.ts`)
All API calls go through the `api` object. Pattern:

```typescript
async function createRequest(orgId: string, data: CreateRequestPayload) {
  const res = await fetch(`/api/orgs/${orgId}/requests`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      'Authorization': `Bearer ${getToken()}`,
    },
    body: JSON.stringify(data),
  });
  if (!res.ok) throw new Error(await res.text());
  return res.json();
}
```

### Data Fetching (TanStack Query)
Use TanStack Query for all data fetching:

```typescript
const { data: nodes } = useQuery({
  queryKey: ['workflow-nodes', requestId],
  queryFn: () => api.getWorkflowNodes(requestId),
  refetchInterval: 3000, // poll for live updates
});
```

### Auth Flow
The app flow is:
1. No token → LoginView
2. Token but no org → OrgSetupView
3. Token + org → Authenticated shell (ShellRail + views)

### Context Usage
```typescript
const { user, token } = useAuth();
const { activeOrg } = useOrg();
```

## Design System (DESIGN.md)

Every component MUST follow the Stripe-inspired design system.

### Typography
```css
font-family: 'sohne-var', 'SF Pro Display', system-ui, sans-serif;
font-feature-settings: "ss01";
```

| Role | Size | Weight | Color |
|------|------|--------|-------|
| Page heading | 32px | 300 | #061b31 |
| Section heading | 22px | 300 | #061b31 |
| Body | 16px | 300–400 | #64748d |
| Label | 14px | 400 | #273951 |
| Caption | 12px | 400 | #64748d |
| Button | 16px | 400 | white on #533afd |

### Colors
```
Primary purple:    #533afd (CTAs, links, active states)
Deep navy:         #061b31 (headings — NEVER use #000000)
Slate:             #64748d (body text)
Dark slate:        #273951 (labels)
Border:            #e5edf5 (cards, dividers)
Background:        #ffffff (page)
Dark section:      #1c1e54 (sidebar)
Success green:     #15be53 (completed status)
Ruby:              #ea2261 (blocked/error — decorative only)
```

### Shadows
Always blue-tinted, never plain gray:
```css
/* Standard card */
box-shadow: rgba(50,50,93,0.25) 0px 30px 45px -30px,
            rgba(0,0,0,0.1) 0px 18px 36px -18px;

/* Ambient */
box-shadow: rgba(23,23,23,0.08) 0px 15px 35px 0px;
```

### Border Radius
- Buttons, inputs, badges: `4px`
- Cards: `5px–6px`
- Featured elements: `8px`
- **NEVER** use pill shapes (12px+)

### Buttons
```css
/* Primary */
background: #533afd; color: #fff; padding: 8px 16px; border-radius: 4px;

/* Ghost */
background: transparent; color: #533afd; border: 1px solid #b9b9f9;
border-radius: 4px; padding: 8px 16px;
```

### Cards
```css
background: #fff;
border: 1px solid #e5edf5;
border-radius: 6px;
box-shadow: rgba(50,50,93,0.25) 0px 30px 45px -30px,
            rgba(0,0,0,0.1) 0px 18px 36px -18px;
```

## What to Build Next

### F8 — Workflow Canvas (highest priority UI)

Use **React Flow** (`@xyflow/react`) for the graph canvas.

```typescript
import { ReactFlow, Background, Controls, MiniMap } from '@xyflow/react';

function WorkflowCanvas({ nodes, edges, onNodeClick }) {
  return (
    <ReactFlow
      nodes={nodes}
      edges={edges}
      onNodeClick={(_, node) => onNodeClick(node)}
      fitView
    >
      <Background />
      <Controls />
    </ReactFlow>
  );
}
```

**Custom node component:**
Each workflow node renders as a custom React Flow node with:
- Colored left border by status
- Agent avatar/icon
- Node name
- Status badge
- Progress indicator (if in progress)

**Start with mock data.** Hardcode the exact graph from the screenshot:
- 10 nodes with positions
- Sequential and parallel edges
- Status values matching a mid-workflow state

Wire to real API once F1/F2 are built.

### Node Detail Panel

Opens when a node is clicked on the canvas. Slides in from the right.

Tabs:
- **Overview:** Description, agent, progress bar, task list, latest update
- **Tasks:** Expanded task list (F3 data)
- **Activity:** Audit events for this node (F6 data)

### Request Overview Panel

Left side, shows:
- Request title, ID, requester, date
- Priority badge, status badge
- Progress bar (X of Y steps)
- Estimated completion
- Current status text

### Agent Roster Panel

Below Request Overview, shows:
- List of all agents with avatar, name, status dot
- Grouped by team (optional for MVP)
- "View All Agents" link

## File Organization

New files to create:
```
views/
  HomeView.tsx                 → F11: Home dashboard
  MyWorkView.tsx               → F12: Personal inbox
  RequestsView.tsx             → F13: Requests list
  WorkflowView.tsx             → F8: Workflow canvas page
  AgentsView.tsx               → F14: Agents roster (update existing)
  ReportsView.tsx              → F15: Reports + audit trail
  IntegrationsView.tsx         → F16: Connected systems
components/
  workflow/
    WorkflowCanvas.tsx         → React Flow canvas
    WorkflowNode.tsx           → custom node component
    NodeDetailPanel.tsx        → right-side detail panel
    RequestOverviewPanel.tsx   → left-side request card
    AgentRosterPanel.tsx       → agent list
  shared/
    StatusBadge.tsx            → colored status badge
    ProgressBar.tsx            → progress indicator
    AuditEventList.tsx         → chronological event list
    StatCard.tsx               → dashboard stat card
    RequestCard.tsx            → request list item
    AgentCard.tsx              → agent list card
    IntegrationCard.tsx        → integration card
```

## Status Badge Component

```typescript
const STATUS_COLORS = {
  completed:   { bg: 'rgba(21,190,83,0.2)',  text: '#108c3d', border: 'rgba(21,190,83,0.4)' },
  in_progress: { bg: 'rgba(83,58,253,0.1)',  text: '#533afd', border: 'rgba(83,58,253,0.3)' },
  pending:     { bg: 'rgba(100,116,141,0.1)', text: '#64748d', border: 'rgba(100,116,141,0.3)' },
  blocked:     { bg: 'rgba(234,34,97,0.1)',  text: '#ea2261', border: 'rgba(234,34,97,0.3)' },
};
```
