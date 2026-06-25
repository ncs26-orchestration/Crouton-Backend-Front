# Pablo — System definition

This document defines Pablo as a system. No pitch, no narrative.

## 0. Related documents

This file is the formal system definition. The rest of the documentation is
split by audience:

| Document | Role |
|---|---|
| `README.md` | Main entry point: architecture summary, stack, setup. |
| `DESIGN.md` | Technical and UI design system for the solution. |
| `VISION.md` | Product story, value proposition, and demo narrative. |
| `PHASES.md` | User stories, phases, and implementation status. |
| `plan.md` | Development method, AI skills/workflows, validation plan. |
| `docs/diagrams/README.md` | Markdown-rendered diagrams. |
| `docs/diagrams/index.html` | HTML diagram hub for architecture, data, compilation, and flow. |

## 1. What Pablo is

An **operator tool** that turns a conversation about a business process —
text, documents, questions, answers — into an executable workflow
deployed on a real engine (Camunda 7 or Elsa 3 in v0.1).

- **Input:** natural-language prompts, PDF/TXT attachments, clarifying
  answers, refinements — all exchanged in a persistent chat thread.
- **Output:** a compiled artifact (BPMN 2.0 XML or Elsa 3 JSON)
  pushed to a project-registered deploy target and published there.

One operator organization runs Pablo and designs workflows for many
client companies.

## 2. What Pablo owns, what it delegates

A workflow's life has three layers. Pablo owns two of them.

```
┌─────────────────────────────────────────────────────────────┐
│  SPECIFICATION LAYER  — what the process is                 │  ← Pablo
│  Workflow IR, chat context, attachments, versions, stages   │
├─────────────────────────────────────────────────────────────┤
│  EXECUTION LAYER      — tokens moving through a state graph │  ← engine
│  instances, timers, persistence of running state            │
├─────────────────────────────────────────────────────────────┤
│  INTEGRATION LAYER    — actually calling real systems       │  ← engine
│  task inboxes, connector modules, retries                   │
└─────────────────────────────────────────────────────────────┘
```

Pablo is the **authoring** system. The engine is the **runtime**. Unlike
earlier drafts of this system, **Pablo does not maintain a separate
Information System Registry** — the only grounding signal the
extractor has is the chat itself (prior messages + attached documents +
the current IR). This matches how an operator actually works: the
client tells them about their systems in conversation, sometimes drops
an architecture doc, iterates by chatting.

### What Pablo keeps

| Thing Pablo owns | Why |
|---|---|
| **Workflow IR + version history** | Portability. One IR → two engine dialects. Edits always create new versions; approved snapshots are immutable. |
| **Chat thread (messages + attachments)** | The grounding substrate. Extraction, refinement, and clarification all re-read the thread. |
| **Extractor prompt contract** | The small LM emits `{ir, questions}` in one call; we validate the IR and render the questions inline. |
| **Engine adapters + compiler registry** | Same IR → BPMN for Camunda, WorkflowDefinition for Elsa. New engines plug in by implementing `engine.Adapter`. |
| **Deploy-target catalog (per project)** | Client-specific engine endpoints, credentials, auth modes — scoped per project, not tenant-wide. |

### What Pablo delegates

| Delegated to | What |
|---|---|
| Engine (Camunda 7 / Elsa 3) | Token execution, timers, instance persistence, task inboxes, BPMN/DMN evaluation |
| The operator's conversation | Identities ("the manager", "the CFO"), systems ("OpenBee", "Odoo") — extracted as free strings; the engine is the source of truth for identity resolution at runtime |
| Engine-side connector runtimes | Actually calling external systems — Camunda topics / Elsa HTTP activities / etc. |

## 3. Domain objects

| Object | What it is | Scope |
|---|---|---|
| **Project** | One client company. | Root |
| **Chat** | One workflow-design session inside a project. Has a persistent message thread + attachments + a pointer to its latest workflow version. | Per project |
| **Message** | One entry in a chat thread. `role ∈ user/assistant/system`. Body is a JSON envelope: `{text, attachment_ids?, questions?, workflow_version_id?, error?}`. | Per chat |
| **Attachment** | A PDF/TXT (voice/image stubs exist) the user dropped into the composer. Normalized to `text_content` — the binary is not kept. | Per chat |
| **Workflow Version** | One snapshot of the IR. Stage: `drafting` (schema issues or extractor errors), `ready` (schema-clean), or `approved` (operator explicitly sanctioned). New turns always create a new version. | Per chat |
| **Deploy Target** | A registered engine endpoint a project can push to. `kind ∈ camunda7 / elsa3`, with auth mode (`none / apikey / bearer / basic / credentials`). | Per project |
| **Workflow IR** | Canonical, engine-agnostic JSON of a process — actors, tasks, gateways, events, flows, forms. Every element carries a `confidence` (0.0–1.0) + an `evidence` span quoting the source text. | Per workflow version |

## 4. Services (concrete, maps 1:1 to the repo)

| Service | Role | Stack |
|---|---|---|
| `apps/web` | UI. Project tree, chat view with canvas + composer + attachment tray, Compile/Approve/Deploy controls, deploy-target CRUD. | React 19 + Vite + React Flow + framer-motion + Tailwind v4 |
| `apps/api` | HTTP API. CRUD (projects, chats, messages, attachments, deploy targets), extraction orchestration, compile + deploy dispatch, validator + lowering, adapter registry. | Go + Echo + pgx |
| `apps/agent` | LLM frontend. One `/extract` endpoint that wraps the extractor prompt + provider dispatch (Ollama default, Gemini/Anthropic fallbacks); one `/attachments/extract-text` for PDF/TXT normalization; Copilot Ask/Clarify endpoints. | Python 3.13 + FastAPI + LangGraph |
| `postgres` (pgvector) | Authoritative store: projects, chats, messages, attachments, workflow_versions, deploy_targets. | pgvector/pgvector:pg18 |
| `redis` | Queues, caches. | redis:8-alpine |
| `camunda7` | Dev-mode Camunda 7 Run distribution (`camunda/camunda-bpm-platform:run-latest`) for round-trip testing. | Upstream image |
| `elsa3` | Dev-mode Elsa Server 3 + Studio (`elsaworkflows/elsa-server-and-studio-v3`). Admin login `admin/password`. | Upstream image |

## 4.1 Technical architecture

![Pablo architecture](docs/diagrams/01.webp)

```text
Browser
  |
  | Vite/nginx proxy: /api, /agent
  v
apps/web  ------------------------------+
React canvas + chat + settings          |
                                         |
                                         v
apps/api -------------------------- Postgres
Go Echo API                         projects, chats, messages,
IR validation/lowering              attachments, workflow versions,
compile/deploy registry             deploy targets
  |
  +------ apps/agent
  |       FastAPI extraction, document text normalization,
  |       provider dispatch: Groq/Gemini/Anthropic/Ollama
  |
  +------ Camunda 7
  |       BPMN deployment + Cockpit visibility
  |
  +------ Elsa 3
          WorkflowDefinition deployment + Studio visibility
```

The API is the control plane. It never directly executes a workflow instance.
It validates and compiles specifications, then delegates runtime behavior to
the configured engine.

Additional rendered diagrams are available in [docs/diagrams/README.md](docs/diagrams/README.md).

## 5. The Conversation → Workflow pipeline

Each user message is one turn. The extractor, invoked once per turn,
returns a single envelope:

```json
{ "ir":       { /* Workflow IR with confidence + evidence per element */ },
  "questions": [ { "id", "ir_ref", "text" } ] }
```

This lets the UI render two interleaved signals in the same assistant
bubble without a second LLM roundtrip (critical on a local 3–9B model
where each call costs tens of seconds):

- **modify** — `questions` empty → the canvas updates, assistant posts
  a summary ("Extracted 5 tasks, 1 gateway.")
- **clarify** — `questions` non-empty → the canvas still updates with
  the best current draft, assistant posts a lead-in + numbered
  questions ("I've drafted 3 tasks, but I need 2 clarifications:").

The user answers the questions in their next message; the extractor
sees the full Q&A in the chat-context block and re-emits the IR with
higher confidence. The loop terminates when every element crosses the
0.8 confidence threshold — this is a client-side predicate derived
from `collectLowConfidence(workflow)`, not a server flag.

```
DRAFTING ───▶ READY ──(operator clicks Approve)──▶ APPROVED
   ▲            │                                    │
   │            │  new user message that modifies the IR
   └────────────┴────────────────────────────────────┘
                  new workflow_version; older 'approved' row stays in history
```

`drafting` = schema-invalid or extractor error. `ready` = schema-clean.
`approved` = operator explicit. Compile + Deploy read the latest
version regardless of approval, but the Approve button is gated on
`ready` + zero low-confidence items.

## 6. Extraction (agent side)

One prompt, one call, one response. No multi-node LangGraph today.

Key rules embedded in the prompt (`apps/agent/app/nodes/extract.py`):

1. Output `{"ir": ..., "questions": ...}`. No prose, no markdown fences.
2. Preserve the current IR on refinement prompts unless the user
   explicitly says "start over". Meta questions ("why did you remove X?")
   return the IR unchanged.
3. Decisions with multiple outcomes ("si A alors X, sinon Y") become
   exclusive gateways, not parallel end events. Parallel activities
   ("simultanément") become parallel gateways.
4. Every actor, task, gateway, binding, and flow-condition carries a
   `confidence` (0.0–1.0) + an `evidence` span from the source text.
5. For every element with `confidence < 0.8`, emit one clarifying
   question targeting it. Questions are single-sentence, picker-style
   when possible, ≤ 120 characters.

Provider is switchable via `AGENT_EXTRACTOR_PROVIDER`:

| Provider | Model | Latency (M-series Mac) | Notes |
|---|---|---|---|
| `ollama` (default) | `qwen2.5:3b` | 50–70 s | No quota, no cost, runs fully offline. |
| `gemini` | `gemini-2.5-flash-lite` | 3–8 s | Needs `GOOGLE_API_KEY`. Free tier has daily quota. |
| `anthropic` | configurable | 3–8 s | Needs `ANTHROPIC_API_KEY`. |

Ollama calls use `format: "json"` (loose) — schema-constrained
decoding with the full IR schema pathologically slows small models.
The Go validator still enforces the schema on the other side.

## 7. Compilation

A compiler is a pure deterministic function `ExecutableIR → artifact_bytes`.
Two compilers in v0.1, each registered as an `engine.Adapter`:

| Adapter | `Kind` | Artifact | Deploy? |
|---|---|---|---|
| `camunda7` | `camunda7` | BPMN 2.0 XML + BPMNDI diagram layout | Yes (POST `/engine-rest/deployment/create`) |
| `elsa3` | `elsa3` | Elsa WorkflowDefinition JSON | Yes (POST `/elsa/api/workflow-definitions` with `{model, publish}` wrapper) |

Before compilation, the IR goes through a `Lower()` pass:

- **Condition normalization** — `${{amount >= 50000}}` / `amount >= 50000` etc. all canonicalize.
- **Default-branch synthesis** — an exclusive gateway with no explicit
  default gets one (+ a synthesized end event) so the engine never
  token-stalls.
- **Actor resolution** — tasks inherit their lane's actor when unbound.
- **Confidence propagation** — a task's effective confidence is the
  min of `task.confidence` and `binding.confidence`.

Every compile is also scanned for **decision tables** — exclusive
gateways whose branches share a pivot variable collapse into a
DMN-style table, surfaced in the compile response for UI review.

## 8. Deploy

Route: `POST /chats/:id/deploy` with `{target_id}`.

1. Load the chat's latest IR. Refuse if it doesn't exist.
2. Load the named deploy target. Refuse if it belongs to a different
   project from the chat (cross-client isolation).
3. Lower + compile using the adapter for `target.kind`.
4. Call `adapter.Deploy(endpoint, authUser, authSecret, name, artifact)`
   which returns a `DeploymentResult` (engine-side id, process key).
5. Construct a browser-openable **Studio URL** when known (Elsa Studio
   lives at the compose-published port; Camunda Cockpit URL is the
   obvious follow-up) and include it in the response.

The web deploy toast uses the returned `studio_url` to surface an
"Open in Elsa Studio →" link that deep-links into the definition's
detail page.

Deploy targets are project-scoped. This matters because one Pablo operator may
serve multiple client organizations; a target registered for one project must
not be usable by another project.

### Elsa 3 deploy specifics

- Endpoint: `POST /elsa/api/workflow-definitions`.
- Envelope: **`{ "model": <WorkflowDefinition>, "publish": true }`** —
  the compiler emits a bare definition so it's also drag-drop
  importable into Elsa Studio; the transport wrapper is added at
  deploy time.
- Auth modes: `none | apikey | bearer | basic | credentials`.
  `credentials` does login-on-demand against `/elsa/api/identity/login`
  (default admin is `admin/password`) and caches the JWT on the client.

### Camunda 7 deploy specifics

- Endpoint: `POST /engine-rest/deployment/create` (multipart).
- Auth: HTTP Basic with the target's user + secret.
- Response feeds Cockpit + Tasklist URL derivation so the UI can
  link into the engine's own runtime views.
- Unresolved service tasks are compiled as BPMN manual tasks instead of bare
  service tasks, because Camunda rejects service tasks without an
  implementation (`camunda:type`, class, delegate expression, or expression).

## 9. Development method and AI assistance

Pablo was implemented with an AI-assisted engineering process, documented in
`plan.md`. The important development workflows were:

- codebase exploration before edits;
- frontend UX implementation and visual verification;
- backend/API implementation;
- compiler and adapter work;
- Docker Compose runtime diagnostics;
- direct engine smoke tests against Camunda and Elsa;
- documentation synthesis across README, SYSTEM, DESIGN, VISION, PHASES, and
  diagrams.

The AI assistance was used as scoped engineering skills, not as an unchecked
generator. Every runtime-critical change was verified through tests and/or
real container endpoints.

## 10. Non-goals

- Pablo does **not** host a workflow engine. It compiles and deploys to
  existing ones.
- Pablo does **not** replace task inboxes. Users work in the engine's
  native UI.
- Pablo does **not** maintain an IS Registry, manage client identities,
  or mirror the client's business data. All grounding comes from the
  chat and all execution lives on the engine.
- Pablo does **not** require a hosted LLM. Local Ollama is the default;
  Gemini / Anthropic are opt-in fallbacks.

## 10. Critical files

Grounding signals for anyone extending the system:

- `packages/ir/schema.json` — the IR shape. Stable across engine targets.
- `apps/api/internal/compiler/lower.go` — the lowering pass.
- `apps/api/internal/engine/adapter.go` — the adapter contract.
- `apps/api/internal/handler/projects.go` — `AppendMessage` (the per-turn
  dispatch) + `ApproveWorkflow`.
- `apps/api/internal/handler/deploy_targets.go` — `DeployChat` (the
  compile+push glue).
- `apps/agent/app/nodes/extract.py` — the prompt + provider dispatch
  + the `{ir, questions}` splitter.
- `apps/web/src/views/ChatView.tsx` — the thread, composer, canvas,
  TopBar (Compile/Approve/Deploy).
- `apps/web/src/lib/confidence.ts` — `collectLowConfidence` — the
  single source of truth for "is the workflow fully resolved".
