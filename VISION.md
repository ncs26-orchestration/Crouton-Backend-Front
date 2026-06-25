# Pablo — Vision

> **Pablo turns a conversation about a business process — text,
> transcripts, procedure PDFs — into a live workflow that runs on the
> client's existing engine, using small local models and no vendor
> lock-in.**

Three words the jury should remember: **Conversational → Portable → Executable**.

## Documentation and method

The product is documented from several angles:

- `README.md` explains the stack, architecture, setup, and commands.
- `SYSTEM.md` defines the exact system boundaries and runtime ownership.
- `DESIGN.md` explains the technical design system and UI language.
- `PHASES.md` tracks user stories and implementation phases.
- `plan.md` explains the development method, AI skills/workflows used, and
  verification strategy.
- `docs/diagrams/README.md` renders the diagrams directly in Markdown.
- `docs/diagrams/index.html` points to architecture, data model, compilation
  layer, and flow diagrams as full HTML pages.

The development process used AI assistance as focused engineering skills:
system design, codebase exploration, frontend implementation, backend/API
implementation, compiler work, runtime diagnostics, and documentation
synthesis. Each skill was checked against repository code and real Docker
services, not only static reasoning.

![Pablo user flow](docs/diagrams/02.webp)

---

## 1. The problem

Business processes exist long before they are formalized. They live in
conversations, working documents, meeting notes, operational instructions,
sketches, and non-homogeneous procedures. Translating them into an executable
workflow is slow, iterative, and leaks information at every step between
business and IT.

The challenge from Innovia ESI / Algiers'UP / ETIC:

> *Comment passer d'un besoin métier non structuré à un workflow suffisamment
> clair, cohérent et exploitable pour soutenir la digitalisation d'un
> processus ?*

Two strong constraints from the jury:

1. **Engine-agnostic.** We must not depend on any specific workflow system,
   but the workflow we produce must be functional inside the engine the client
   already uses (Elsa, Camunda, Flowable, n8n, …).
2. **Sobriety.** Prefer small, light, deployable models over a big-LLM stack.
   Cost, performance, and sovereignty matter.

## 2. Product identity

Pablo is an **operator tool for workflow design**. A single operator
organization uses Pablo to author workflows for many client companies.
One top-level workspace per client ("Project"); inside, one persistent
**chat thread** per workflow-design effort.

In that thread the operator:

- describes the process in natural language (French, Arabic, English);
- drops procedure PDFs, meeting transcripts, or text attachments;
- answers a handful of targeted clarifying questions the assistant
  raises for low-confidence elements;
- watches the workflow diagram redraw live on every turn;
- clicks **Approve** when the draft is clean, **Deploy** to push it to
  the client's Camunda or Elsa instance.

The engine is a **plugin**, not a dependency. The chat thread is the
source of truth for *what the workflow is*; the engine is the runtime.

## 3. Core architecture

```
  ┌───────────────────────────────────────────────────────────┐
  │  CONVERSATION (multimodal, unstructured)                   │
  │  text turns · PDF / TXT attachments · answers to questions │
  └───────────────────────────────────────────────────────────┘
                            │
                            ▼
  ┌───────────────────────────────────────────────────────────┐
  │  ONE-CALL EXTRACTOR  (local small model by default)        │
  │   qwen2.5:3b via Ollama — runs fully offline               │
  │   JSON-mode output, schema validated server-side            │
  │   Emits {ir, questions[]} in a single envelope              │
  └───────────────────────────────────────────────────────────┘
                            │
                            ▼
  ┌───────────────────────────────────────────────────────────┐
  │  WORKFLOW IR  (canonical JSON, versioned per turn)         │
  │  actors, tasks, gateways, events, flows, forms             │
  │  confidence + evidence on every element                    │
  │  drafting → ready → approved                               │
  └───────────────────────────────────────────────────────────┘
                            │
          ┌─────────────────┴──────────────────┐
          ▼                                    ▼
      Lower + compile                    Lower + compile
          │                                    │
          ▼                                    ▼
     BPMN 2.0 XML                       Elsa 3 WorkflowDef JSON
  (Camunda 7 / etc.)                    (`{model, publish}`)
          │                                    │
          ▼                                    ▼
   Camunda /engine-rest              Elsa /elsa/api/workflow-definitions
```

## 4. How "small models punch above their weight"

Two design choices turn a 3B local model into a usable extractor:

### 4.1 The conversation is the grounding source

The extractor's prompt carries the full chat context: every prior
message (bounded to the most recent N), every attachment's extracted
text, and — crucially — the **current IR**. Refinement prompts
("rename the last task", "who approves over $5k?") modify what the
operator is actually looking at, instead of re-extracting from scratch.

### 4.2 Modify vs. clarify in one call

On a local 3–9B model, a second cold call per turn is unacceptable —
60–100 s round-trips. So one call emits both:

- the **updated IR** (canvas stays live every turn),
- plus an optional **`questions[]`** — one question per element whose
  confidence is below 0.8, phrased as a specific, answerable,
  preferably picker-style question.

The UI renders questions inline in the assistant's bubble; the user
answers by sending the next normal message; the loop terminates when
no low-confidence elements remain. The "fully resolved" predicate is
derived client-side from `collectLowConfidence(workflow)` so canvas
and Approve button stay in lockstep.

## 5. The three artifacts everything rotates around

```
┌──────────────┐   ┌──────────────┐   ┌──────────────┐
│  Workflow IR │   │ Chat Context │   │  Deploy      │
│              │   │              │   │  Targets     │
│ actors,      │◀──│ messages +   │──▶│              │
│ tasks,       │   │ attachments +│   │ per-project  │
│ gateways,    │   │ current IR + │   │ engine       │
│ flows,       │   │ prior Q&A    │   │ endpoints    │
│ confidence   │   │              │   │ (Camunda,    │
│ + evidence   │   │              │   │  Elsa)       │
└──────┬───────┘   └──────────────┘   └──────────────┘
       │
       ▼
 ┌─────────────────────────────────────────────┐
 │  Compilers (plugins, not dependencies)      │
 │  → BPMN 2.0 XML  (Camunda 7 today)          │
 │  → Elsa 3 JSON   (deployed + published)      │
 └─────────────────────────────────────────────┘
```

## 6. Graceful degradation — workflows run from day one

The extractor makes a best-effort IR even when the prompt is vague.
Every task emits a `confidence` + `evidence` span; low-confidence
elements become **questions in the next assistant bubble**, not
silent hallucinations. The canvas shows them tinted so the operator
knows what to confirm.

When a task's binding is uncertain, it still compiles — as a user
task the client can complete manually, or as a service task the
engine will route through its native connector configuration. The
workflow runs on day one; integrations tighten over iterations.

---

## 7. End-to-end user flow

### Three acts

**Act I — Open the project** *(seconds)*
The operator opens Pablo, finds or creates the client's project in the
sidebar, starts a new chat.

**Act II — Describe the process** *(minutes)*
They type a paragraph, maybe drop a procedure PDF. The canvas draws
an initial workflow in ~60 seconds. A few targeted questions appear
inline ("Who validates amounts above $5000?"). They answer by typing
the next message.

**Act III — Approve & deploy** *(seconds)*
Once the canvas is clean, they click **Approve** → status flips to
APPROVED. Click **Deploy → Elsa** → the artifact is pushed to the
client's Elsa Server and published. The toast carries a clickable
link straight to the definition's page in Elsa Studio.

### Demo script — what the jury watches in 3 minutes

1. **[0:00]** Empty Pablo → click *New project* → name it after the
   client → new chat auto-opens.
2. **[0:20]** Drop a one-page procedure PDF in the composer. Type:
   *"Turn this into an approval workflow"*. Hit send.
3. **[1:20]** Canvas redraws: 5 tasks, 1 exclusive gateway. Assistant
   bubble asks **two** clarifying questions about ambiguous branches.
4. **[1:40]** Answer the questions in one reply. Canvas updates.
   Status chip flips `DRAFTING → READY`.
5. **[2:00]** Click **Approve** → chip flips `READY → APPROVED`.
6. **[2:10]** Click **Deploy → Elsa** → toast "Deployed to Elsa 3"
   with **Open in Elsa Studio →** link. Click it → jury sees the
   definition live in Studio, published, version 1.
7. **[2:40]** Switch target: Deploy → Camunda 7. Same IR, BPMN 2.0
   XML this time, live in Cockpit.
8. **[3:00]** Kill the network. Ollama on `htop`. Point out: the whole
   conversation ran on a 3B local model. No OpenAI. No Anthropic. No
   data left the box.

---

## 8. Winning value propositions

### 1. "The engine is a plugin, not a dependency."
Same IR → Camunda 7 BPMN, Elsa 3 WorkflowDefinition. Clients keep
their investment; Pablo sits above the market. New engines plug in by
implementing one Go interface (`engine.Adapter`).

### 2. "We don't generate workflow diagrams. We generate live workflows."
Approve → Deploy → the engine is running your process, reachable in
Cockpit / Studio, by the end of the demo.

### 3. "Sober by design — local by default."
Default provider is Ollama with `qwen2.5:3b`. No cloud LLM required.
Gemini and Anthropic are opt-in fallbacks, not dependencies. Matches
the brief's *modèles légers* requirement and the sovereignty concern
for DZ banking/admin.

### 4. "Conversational authoring, not forms."
No templates, no drop-downs, no IS registry onboarding. The operator
types what they'd say to a colleague; the small model extracts
structure; low-confidence elements become specific inline questions
the operator can answer in one reply.

### 5. "Canvas and chat stay locked."
Every turn updates the IR; the canvas redraws immediately. Low-confidence
elements are tinted; the Approve button gates on the same
`collectLowConfidence()` predicate. What the operator sees is what
they'll ship.

### 6. "Multimodal in — PDF today, voice + image designed in."
Drop a procedure PDF — extracted text is appended to the chat context
and the extractor reads it alongside your prompt. Voice and image
normalization are scaffolded in the agent and UI; only the ASR/OCR
backends are stubbed.

### 7. "Approval creates a durable snapshot — edits don't destroy history."
Approve flips the current workflow version's stage. Subsequent turns
create new versions at ready/drafting; the approved row stays
immutable in `workflow_versions` for audit.

### 8. "Elsa deploy is real, not hand-wavy."
Round-trip verified: compile → wrap in `{model, publish}` envelope →
POST to `/elsa/api/workflow-definitions` → definition appears in
Elsa Studio with version 1, `isPublished: true`. Toast shows a click-
through link. Same story for Camunda 7.

### 9. "Standards-first, not reinvention."
BPMN 2.0 (Camunda / Flowable / Bonita consume it natively), Elsa 3's
documented API shape, pypdf for document extraction, JSON Schema for
IR validation. No proprietary formats, no bespoke runtime.

### 10. "Algerian context — French-language processes, first-class."
The extractor prompt is explicit about French idioms
("si … alors … sinon", "en cas de", "deux issues possibles"). The
demo prompt is French. No translation layer required.

---

## 9. Non-goals

Pablo is deliberately **not**:

- A hosted workflow engine.
- A task-inbox replacement.
- A client-identity system or IS Registry.
- A dependency on hosted LLMs.
- A round-trip modeler for BPMN files edited in the engine (drift
  detection is a future concern, not a v0.1 promise).

## 10. The single sentence that wins the jury

> « Pablo transforme une conversation sur un processus métier — quelques
> phrases, un PDF, des questions-réponses — en un workflow **portable,
> approuvable et exécutable**, qui tourne sur le moteur de votre
> entreprise (Camunda ou Elsa), parle à vos systèmes via le moteur
> lui-même, et ne dépend d'aucun grand modèle ni d'aucun fournisseur. »
