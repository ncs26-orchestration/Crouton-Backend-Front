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


async def test_every_department_completes_with_tasks() -> None:
    for agent_type in DEPARTMENTS:
        decision = await run_department(
            agent_type=agent_type,
            title="Open a new office in Berlin",
            description="Expand into the EU market",
            priority="high",
        )
        assert isinstance(decision, Decision)
        assert decision.summary
        assert decision.status_text
        assert len(decision.tasks) >= 1
        assert all(t.status == "completed" for t in decision.tasks)
        # F3 completes every node; the blocked_on case is F5.
        assert decision.blocked_on is None


async def test_finance_flags_budget() -> None:
    decision = await run_department("finance", "Procure laptops", "", "medium")
    assert any("budget" in f.message.lower() for f in decision.flags)


async def test_playbook_matches_go_fallback() -> None:
    # Pins the Python playbook to the Go fallback (apps/api internal/agentclient
    # DefaultDecision). The two are duplicated across runtimes on purpose; these
    # literals and the matching Go test make drift fail loudly on both sides.
    finance = await run_department("finance", "x", "", "high")
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
            "upstream_context": [],
            "org_context": {},
        },
    )
    assert resp.status_code == 200
    body = resp.json()
    assert set(["summary", "flags", "tasks", "status_text", "blocked_on"]).issubset(body.keys())
    assert body["blocked_on"] is None
    assert len(body["tasks"]) >= 1
    assert body["tasks"][0]["status"] == "completed"


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
