# PRD — Agent AI (Python, Pydantic AI)

Owner: Agent team · Stack: Python 3.13, FastAPI, **Pydantic AI**, uv · Skills: `.agents/skills/agent/`

## Context

The department agents are the heart of the product and the answer to "meaningful decision-making" and
"genuine collaboration". The raw `httpx` + `json.loads` + ```` ``` ````-stripping provider layer is
**retired** for the request flow. We rebuild on **Pydantic AI**: typed agents, structured outputs,
tool-calling, dependency injection, automatic validation + retries, multi-model support.

Go orchestrates (`PRD-BACKEND.md`); Python reasons. Each `/agents/run` is one stateless department
invocation. The signature behavior (Finance waiting on IT) is **emergent**: the agent calls a
`raise_dependency` tool, and Go gates/unblocks. Read `i-pydantic-ai`, `i-pydantic-models-py`,
`i-agent-tool-builder`, `i-multi-agent-patterns`, `i-agent-evaluation` before building.

Build order = feature order below.

## Contract with Go (must match `PRD-BACKEND.md`)
```
POST /agents/intake  {request:{title,description,priority}, org_context}              -> Plan
POST /agents/run     {agent_type, request, upstream_context:[...], org_context}       -> Decision
```

---

## AG-1 — Dependency + package scaffold
Goal: Pydantic AI installed and a clean package. Skills: `i-pydantic-ai`.
Steps:
1. Add `pydantic-ai-slim` (with the provider extras matching `settings.py`: anthropic/openai/google/groq) to `apps/agent/pyproject.toml`; `uv lock && uv sync`.
2. Create `apps/agent/app/agents/` with `__init__.py`.
3. Remove `app.agents.*` from the mypy ignore list in `pyproject.toml` (new code is strict-typed).
Acceptance: `uv run python -c "import pydantic_ai"` works; `uv run mypy app/agents` runs.

## AG-2 — Output models
Goal: the typed contracts the agents emit. Skills: `i-pydantic-models-py`.
Steps:
1. `agents/models.py`: `PlanNode {key, name, agent_type, department}`, `PlanEdge {from_, to, type}`, `Plan {nodes, edges}`.
2. `Flag {severity, message}`, `TaskItem {title, status}`, `DependencyDecl {on_department, reason}`.
3. `Decision {summary, flags, tasks, status_text, blocked_on: DependencyDecl | None}`.
4. Add field docstrings/descriptions — Pydantic AI feeds these to the model.
Acceptance: models import; `Decision.model_json_schema()` shows `blocked_on` as nullable.

## AG-3 — Injected context (deps)
Goal: tools read injected data, not HTTP callbacks. Skills: `i-pydantic-ai` (dependency injection).
Steps:
1. `agents/deps.py`: a dataclass `AgentDeps {org_context, upstream_context, declared: list[DependencyDecl]}`.
2. `org_context` carries the IS registry snapshot + department policies; `upstream_context` carries completed upstream node summaries.
3. `declared` is the mutable slot the `raise_dependency` tool appends to.
Acceptance: a unit test constructs `AgentDeps` and reads policy/registry fields.

## AG-4 — Tools (incl. `raise_dependency`)
Goal: agents act, and declare dependencies. Skills: `i-agent-tool-builder`.
Steps:
1. `agents/tools.py`: `read_is_registry(ctx)`, `get_department_policy(ctx, department)` reading from `ctx.deps.org_context`.
2. Domain calcs: `assess_budget(ctx, amount)` (Finance), `compliance_lookup(ctx, topic)` (Legal), and analogous read-only helpers for IT/HR/Ops.
3. `raise_dependency(ctx, on_department, reason)`: append a `DependencyDecl` to `ctx.deps.declared` and return a confirmation string.
4. Register tools on the relevant agents only (Finance/Legal get their domain tools; all get `raise_dependency`).
Acceptance: calling `raise_dependency` in a `TestModel` run populates `deps.declared`.

## AG-5 — Model selection + offline fallback
Goal: multi-model, and it runs with no key. Skills: `i-pydantic-ai`.
Steps:
1. `agents/model.py`: `select_model()` reusing `app/settings.py` priority (Groq > Gemini > Anthropic > Ollama).
2. When no key is set, return a Pydantic AI `FunctionModel` that emits deterministic canned `Plan`/`Decision` (including a Finance `blocked_on` case) so the system runs offline.
Acceptance: with all keys unset, `select_model()` returns the `FunctionModel` and agents produce valid typed output.

## AG-6 — Intake planner agent
Goal: request → department workflow. Skills: `i-multi-agent-patterns`.
Steps:
1. `agents/intake.py`: `intake_agent = Agent(model, deps_type=AgentDeps, output_type=Plan, system_prompt=...)`.
2. Constrain to the fixed catalog: `intake, planning, finance_review, legal_review, it_assessment, hr_planning, ops_planning, exec_approval, implementation, report`, with sensible parallel branches.
3. On validation failure after retries, return a deterministic default template.
Acceptance: `intake_agent.run("Open a new office in Berlin")` returns a `Plan` with the expected nodes + parallel review edges; invalid model output still yields the template.

## AG-7 — Department agent factory
Goal: one tool-using agent per role. Skills: `i-ai-agents-architect`, `i-multi-agent-architect`.
Steps:
1. `agents/department.py`: `build_agent(agent_type)` returning an `Agent(..., output_type=Decision)` with a role-specific system prompt + bound tools.
2. Roles: Finance (budget/ROI), Legal (compliance/contract), IT (feasibility/security), HR (staffing), Ops (logistics).
3. Prompts require plain-language `status_text` (e.g. *"Financial impact analysis in progress. Waiting for data from IT assessment."*) and instruct calling `raise_dependency` when an upstream department's output is missing.
4. After a run, surface `ctx.deps.declared[0]` onto `Decision.blocked_on` if present.
Acceptance: the Finance agent, given context lacking IT output, returns `blocked_on={on_department:"IT", reason:...}`; given IT's summary in `upstream_context`, returns a completed `Decision`.

## AG-8 — FastAPI surface
Goal: the two endpoints Go calls. Skills: `i-api-design-principles`.
Steps:
1. `app/api/agents.py`: `POST /agents/intake` builds `AgentDeps`, runs `intake_agent`, returns the `Plan`.
2. `POST /agents/run`: select the agent by `agent_type`, build deps from the request, run, return the `Decision` (with `blocked_on` from declared deps).
3. Register the router in `app/main.py` alongside the existing routers.
Acceptance: both endpoints return contract-valid JSON for the Berlin request; `agent_type` routing works for all five departments.

## AG-9 — Tests
Goal: deterministic, no-network coverage. Skills: `i-agent-evaluation`.
Steps:
1. `apps/agent/tests/test_intake.py`: `TestModel`/`FunctionModel` → assert `Plan` shape and catalog keys.
2. `apps/agent/tests/test_department.py`: assert each role returns a `Decision`; assert the Finance block→unblock transition; assert `raise_dependency` was called.
3. `apps/agent/tests/test_endpoints.py`: FastAPI `TestClient` over both routes.
Acceptance: `uv run pytest` green; `ruff` + `mypy` clean for `app/agents/` and `app/api/agents.py`.

---

## Definition of done (agent)
- Both endpoints return typed, validated output for the Berlin request.
- The Finance→IT dependency is produced by the agent's `raise_dependency` tool, not hardcoded.
- With no API key, the whole surface still works via `FunctionModel`.
- No raw `json.loads`/fence-stripping in the new request-flow code.
