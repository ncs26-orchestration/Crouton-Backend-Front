# Pablo Development Plan And Method

This file explains how the system was developed, how AI assistance was used,
and how the technical architecture was validated.

## 1. Development Method

Pablo was built with an incremental, end-to-end method:

1. Define the product boundary:
   Pablo is an authoring system, not a workflow runtime.
2. Define a canonical Workflow IR:
   one JSON model for actors, tasks, gateways, events, flows, forms,
   confidence, and evidence.
3. Build the conversation loop:
   each user turn updates the IR and can return clarifying questions.
4. Build the visual feedback loop:
   the UI renders the current IR immediately on a canvas.
5. Build compilers:
   the same IR compiles to Camunda BPMN and Elsa WorkflowDefinition JSON.
6. Build deploy targets:
   each project can register Camunda/Elsa connectors and deploy the latest IR.
7. Validate on real engines:
   every compiler and deploy path is tested against local Docker Camunda and
   Elsa containers.
8. Document the system:
   README, SYSTEM, DESIGN, VISION, PHASES, and diagrams explain the result at
   different levels.

## 2. AI Skills And Workflows Used

The project used AI-assisted development as a set of focused skills rather
than one generic prompt. The important skill-style workflows were:

| Skill / workflow | How it was used |
|---|---|
| Codebase exploration | Read existing React, Go, Python, Docker, and schema files before changing behavior. |
| System design synthesis | Turn product requirements into layered architecture: conversation, IR, compiler, adapter, runtime. |
| Frontend implementation | Fix chat scrolling, connector controls, deploy UX, status toasts, and API error visibility. |
| Backend/API implementation | Add deploy diagnostics, connector auth handling, compile/deploy dispatch fixes. |
| Compiler engineering | Make Camunda BPMN valid for unresolved service tasks; make Elsa JSON use installed activity descriptors. |
| Runtime diagnostics | Use Docker Compose, API logs, health checks, and direct HTTP probes to debug real engine behavior. |
| Documentation synthesis | Align README, SYSTEM, DESIGN, VISION, PHASES, and diagrams into a coherent explanation. |
| Verification | Run focused Go tests, web builds, and real deploy smoke tests after implementation. |

This is compatible with Claude/Codex-style skills: each task is scoped,
grounded in repository files, implemented, tested, then documented.

## 3. Why This Method Fits The Problem

The challenge is not only to draw a workflow. The system must turn vague
business text into a workflow that is:

- understandable by the operator;
- portable across engines;
- executable in real Camunda/Elsa deployments;
- traceable back to the conversation and documents;
- safe to refine through clarifying questions.

That requires a full vertical slice for every feature. For example, "Deploy
to Elsa" was not considered complete until:

- the IR compiled to valid Elsa JSON;
- Elsa accepted the deploy endpoint;
- Studio opened the workflow;
- activities rendered with installed Elsa descriptors;
- auth worked in Docker Compose;
- the UI deep-linked to the right Studio route.

## 4. Verification Checklist

Before committing a feature, use the smallest useful verification set:

```bash
pnpm --filter web build
cd apps/api && go test ./internal/handler ./internal/compiler/bpmn ./internal/engine/elsa3
```

For runtime features:

```bash
docker compose ps
curl http://localhost:8080/readyz
curl http://localhost:8180/engine-rest/engine
curl http://localhost:8280
```

For deploy features, verify that the workflow is visible in the engine UI:

- Camunda Cockpit: `http://localhost:8180/camunda/app/cockpit`
- Elsa Studio: `http://localhost:8280`

## 5. Documentation Structure

| Document | Role |
|---|---|
| `README.md` | Main entry point for setup, architecture, and stack. |
| `SYSTEM.md` | Formal technical definition of the system and ownership boundaries. |
| `DESIGN.md` | Solution design system: architecture, UI, visual language, and runtime behavior. |
| `VISION.md` | Product rationale, demo story, value proposition. |
| `PHASES.md` | Feature phases and user-story tracking. |
| `docs/diagrams/README.md` | Markdown-rendered architecture, compilation, data model, and flow diagrams. |
| `docs/diagrams/*.html` | Full-page HTML diagram views. |

## 6. Rendered Diagrams

The diagrams are part of the development method: they are used to explain the
architecture, then checked against the running implementation.

![Pablo architecture](docs/diagrams/01.webp)

![Pablo data model](docs/diagrams/04.webp)

## 7. Current Known Limits

- Document reading handles text PDFs and text-like files, but not robust OCR
  for scanned PDFs yet.
- Workflow identity binding is still conversational; a later version can add
  stronger organization/identity registry mapping.
- Elsa rendering uses activities available in the stock dev image. If a
  client installs HumanTask packages, the adapter can map user tasks to those
  richer descriptors.
- Versioning and diff UX exists in the product direction, but should continue
  to receive test coverage as it grows.
