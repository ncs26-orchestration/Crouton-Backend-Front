# PRD — Agent AI (Python, Pydantic AI)

Owner: Agent team · Stack: Python 3.13, FastAPI, **Pydantic AI**, uv · Skills: `.agents/skills/agent/`

## Context

The department agents are the heart of the product and the answer to the "meaningful decision-making"
and "genuine collaboration" judging criteria. Today the Python service does raw `httpx` POSTs to each
provider with hand-rolled `json.loads` and ```` ``` ```` stripping — that is being **retired** for the
request flow. We rebuild the agents on **Pydantic AI**: typed agents with structured outputs,
tool-calling, dependency injection, automatic validation + retries, and multi-model support.

Go orchestrates (see `PRD-BACKEND.md`); Python reasons. Each `/agents/run` call is a single,
stateless department-agent invocation. The signature behavior — Finance waiting on IT — is
**emergent**: an agent decides it needs another department's output and calls a `raise_dependency`
tool; Go reads that and gates/unblocks.

Read `i-pydantic-ai`, `i-pydantic-models-py`, `i-agent-tool-builder`, `i-ai-agents-architect`,
`i-multi-agent-patterns`, and `i-agent-evaluation` before building.

## Scope / ownership

- Add `pydantic-ai-slim` (with the provider extras already configured) to `apps/agent/pyproject.toml`.
- A new `apps/agent/app/agents/` package: output models, deps, tools, model selection, intake agent,
  department agent factory.
- Two FastAPI endpoints (`apps/agent/app/api/agents.py`, registered in `app/main.py`).
- Unit tests using Pydantic AI `TestModel`/`FunctionModel` (no network).
- Out of scope: the existing `extract`/`copilot`/`interview` routers stay but are unused by this flow.

## Deliverables

### `agents/models.py` — typed outputs (`i-pydantic-models-py`)
- `Plan { nodes: list[PlanNode], edges: list[PlanEdge] }`, `PlanNode {key, name, agent_type, department}`, `PlanEdge {from, to, type}`
- `Decision { summary, flags: list[Flag], tasks: list[TaskItem], status_text, blocked_on: DependencyDecl | None }`
- `DependencyDecl {on_department, reason}`, `Flag {severity, message}`, `TaskItem {title, status}`
These are the agents' `output_type` — validation + retries are automatic, no manual JSON parsing.

### `agents/deps.py` — injected context
A dataclass passed as Pydantic AI `deps`: `org_context` (IS registry snapshot, department policies),
`upstream_context` (completed upstream node summaries), and a mutable `declared_dependency` slot the
tool writes into. Tools read injected data only — no HTTP back into Go.

### `agents/tools.py` — typed tools (`i-agent-tool-builder`)
- `read_is_registry()` — systems/users/groups available to the org
- `get_department_policy(department)` — returns the seeded policy text the agent must respect
- domain calcs: `assess_budget(amount)` (Finance), `compliance_lookup(topic)` (Legal), etc.
- `raise_dependency(on_department, reason)` — records a `DependencyDecl` into deps; the agent then
  returns with `Decision.blocked_on` set. **This is how cross-dependencies are declared.**

### `agents/intake.py` — the Intake planner
A `Agent[Deps, Plan]` that turns the request into a department workflow chosen from the **fixed
catalog** (intake, planning, finance_review, legal_review, it_assessment, hr_planning, ops_planning,
exec_approval, implementation, report) with sensible parallelism. Deterministic default template if
validation fails after retries.

### `agents/department.py` — department agent factory
Builds one `Agent[Deps, Decision]` per role with a role-specific system prompt and the bound tools
(Finance: budget/ROI; Legal: compliance/contract; IT: feasibility/security; HR: staffing; Ops:
logistics). Status text must read like a human handoff, e.g. *"Financial impact analysis in progress.
Waiting for data from IT assessment."*

### `agents/model.py` — model selection + fallback
Reuse the provider auto-selection in `app/settings.py` (Groq > Gemini > Anthropic > Ollama). When no
key is set, return a Pydantic AI `FunctionModel` producing deterministic canned `Decision`/`Plan`
output so the whole system runs offline.

### `app/api/agents.py` — FastAPI surface
```
POST /agents/intake  {request, org_context}                          -> Plan
POST /agents/run     {agent_type, request, upstream_context, org_context} -> Decision
```
Thin handlers: build deps, run the matching agent, return the typed result as JSON.

## Acceptance criteria

- `uv run` boots; `POST /agents/intake` returns a valid `Plan` for "Open a new office in Berlin".
- `POST /agents/run` for the Finance agent, given a context lacking IT's output, returns a `Decision`
  with `blocked_on = {on_department: "IT", reason: "..."}` produced via the `raise_dependency` tool.
- Re-running the Finance agent with IT's summary in `upstream_context` returns a completed `Decision`
  (no `blocked_on`).
- `apps/agent/tests/`: each agent runs against `TestModel`/`FunctionModel` asserting the typed shape
  and tool calls — fast, deterministic, no network (`i-agent-evaluation`).
- With no API key, both endpoints still return valid typed output via `FunctionModel`.
- `ruff` + `mypy` clean for the new `app/agents/` package (drop it from the mypy ignore list in `pyproject.toml`).
