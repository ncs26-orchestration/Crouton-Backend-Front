"""Agent endpoint + department-decision tests — deterministic, no network.

Covers the F3 offline path: every department agent returns a typed ``Decision``
with real tasks and a plain-language status, and ``/agents/run`` routes by
``agent_type``. With no LLM key set these are fully deterministic.
"""

from fastapi.testclient import TestClient

from app.agents.department import run_department
from app.agents.models import Decision
from app.main import app

DEPARTMENTS = [
    "intake",
    "planning",
    "finance",
    "legal",
    "it",
    "hr",
    "ops",
    "approval",
    "implementation",
    "report",
]


IT_UPSTREAM = [
    {"key": "it_assessment", "department": "IT", "summary": "Technically feasible."},
]


async def test_every_department_completes_with_tasks() -> None:
    for agent_type in DEPARTMENTS:
        upstream = IT_UPSTREAM if agent_type == "finance" else None
        decision = await run_department(
            agent_type=agent_type,
            title="Open a new office in Berlin",
            description="Expand into the EU market",
            priority="high",
            upstream_context=upstream,
        )
        assert isinstance(decision, Decision)
        assert decision.summary
        assert decision.status_text
        assert len(decision.tasks) >= 1
        assert all(t.status == "completed" for t in decision.tasks)
        # F3 completes every node; the blocked_on case is F5.
        if agent_type != "finance":
            assert decision.blocked_on is None
        # Finance needs IT output to complete; otherwise it declares blocked_on (F5).
        if agent_type == "finance":
            assert decision.blocked_on is None, (
                "finance should complete with IT upstream; got blocked_on"
            )


async def test_finance_flags_budget() -> None:
    decision = await run_department(
        "finance",
        "Procure laptops",
        "",
        "medium",
        upstream_context=IT_UPSTREAM,
    )
    assert any("budget" in f.message.lower() for f in decision.flags)


async def test_playbook_matches_go_fallback() -> None:
    # Pins the Python playbook to the Go fallback (apps/api internal/agentclient
    # DefaultDecision). The two are duplicated across runtimes on purpose; these
    # literals and the matching Go test make drift fail loudly on both sides.
    finance = await run_department("finance", "x", "", "high", upstream_context=IT_UPSTREAM)
    assert finance.status_text == "Finance review complete — the request is financially viable."
    assert [t.title for t in finance.tasks] == [
        "Assess budget feasibility",
        "Estimate the financial impact",
        "Project the return on investment",
        "Confirm funding availability",
    ]
    legal = await run_department("legal", "x", "", "high")
    assert legal.status_text == "Legal review complete — no blocking issues, one item to track."
    assert [t.title for t in legal.tasks] == [
        "Review regulatory compliance",
        "Check contract requirements",
        "Flag legal risks",
    ]


async def test_unknown_agent_type_still_completes() -> None:
    decision = await run_department("mystery", "Do a thing", "", "low")
    assert decision.tasks
    assert decision.blocked_on is None


async def test_decisions_carry_an_outcome() -> None:
    # Every fallback decision exposes the new outcome field; the happy path
    # approves, and Finance-without-IT blocks.
    approve = await run_department("it", "Ship a feature", "", "low")
    assert approve.outcome == "approve"
    blocked = await run_department("finance", "Open a Berlin office", "", "high")
    assert blocked.outcome == "block"
    assert blocked.blocked_on is not None


def test_parse_decision_normalizes_outcome_and_block() -> None:
    from app.agents.department import _parse_decision

    # Off-contract outcome falls back to approve.
    d = _parse_decision(
        '{"summary":"ok","outcome":"yolo","tasks":[{"title":"t"}],"status_text":"done"}'
    )
    assert d is not None and d.outcome == "approve"
    # A declared dependency forces the block outcome so the engine acts on it.
    d2 = _parse_decision(
        '{"summary":"need IT","outcome":"approve","tasks":[{"title":"t"}],'
        '"status_text":"waiting","blocked_on":{"on_department":"IT","reason":"cost"}}'
    )
    assert d2 is not None and d2.outcome == "block"
    # A reject survives and keeps its flag.
    d3 = _parse_decision(
        '{"summary":"violates policy","outcome":"reject","tasks":[{"title":"t"}],'
        '"status_text":"rejected","flags":[{"severity":"critical","message":"Policy X"}]}'
    )
    assert d3 is not None and d3.outcome == "reject"


def test_run_endpoint_routes_by_agent_type() -> None:
    client = TestClient(app)
    resp = client.post(
        "/agents/run",
        json={
            "agent_type": "finance",
            "request": {
                "title": "Open a new office in Berlin",
                "description": "",
                "priority": "high",
            },
            "upstream_context": [
                {"key": "it_assessment", "department": "IT", "summary": "Technically feasible."},
            ],
            "org_context": {},
        },
    )
    assert resp.status_code == 200
    body = resp.json()
    assert set(["summary", "flags", "tasks", "status_text", "blocked_on"]).issubset(body.keys())
    assert body["blocked_on"] is None
    assert len(body["tasks"]) >= 1
    assert body["tasks"][0]["status"] == "completed"


async def test_finance_blocked_on_when_no_it_upstream() -> None:
    decision = await run_department("finance", "Open a new office in Berlin", "", "high")
    assert decision.blocked_on is not None
    assert decision.blocked_on.on_department == "IT"
    assert "IT security assessment" in decision.blocked_on.reason


def test_intake_endpoint_still_returns_plan() -> None:
    client = TestClient(app)
    resp = client.post(
        "/agents/intake",
        json={"request": {"title": "x", "description": "", "priority": "high"}, "org_context": {}},
    )
    assert resp.status_code == 200
    body = resp.json()
    assert len(body["nodes"]) >= 9
    assert "from" in body["edges"][0]
