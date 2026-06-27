"""Unit tests for the LLM wiring: provider gate + strict output validation.

Pure and deterministic — no network. They prove that malformed or off-catalog
model output is rejected (so callers fall back) and that valid output parses."""

import json

from app.agents import llm
from app.agents.department import _parse_decision
from app.agents.intake import _additional_catalog, _parse_plan


def test_llm_unavailable_without_keys(monkeypatch) -> None:
    monkeypatch.setattr(llm.settings, "deepseek_api_key", None)
    monkeypatch.setattr(llm.settings, "groq_api_key", None)
    monkeypatch.setattr(llm.settings, "openai_api_key", None)
    assert llm.llm_available() is False


def test_llm_available_with_deepseek_key(monkeypatch) -> None:
    monkeypatch.setattr(llm.settings, "deepseek_api_key", "sk-test")
    assert llm.llm_available() is True


def test_parse_plan_rejects_garbage() -> None:
    assert _parse_plan(None) is None
    assert _parse_plan("not json") is None
    assert _parse_plan('{"nodes": [], "edges": []}') is None  # empty / no required keys


def test_parse_plan_rejects_off_catalog_keys() -> None:
    bad = json.dumps(
        {
            "nodes": [{"key": "made_up", "name": "X", "agent_type": "x", "department": "X"}],
            "edges": [{"from": "made_up", "to": "made_up", "type": "sequence"}],
        }
    )
    assert _parse_plan(bad) is None


def test_additional_catalog_allows_custom_departments() -> None:
    org = {
        "additional_departments": [
            {"key": "marketing_review", "agent_type": "marketing", "department": "Marketing"},
        ]
    }
    text, keys = _additional_catalog(org)
    assert "marketing_review" in keys
    assert "Marketing" in text

    def node(key: str, agent_type: str, dept: str) -> dict:
        return {"key": key, "name": key, "agent_type": agent_type, "department": dept}

    # A plan that uses the custom key is accepted only when that key is allowed.
    plan = json.dumps(
        {
            "nodes": [
                node("intake", "intake", "Planning"),
                node("marketing_review", "marketing", "Marketing"),
                node("exec_approval", "approval", "Executive"),
                node("report", "report", "Planning"),
            ],
            "edges": [
                {"from": "intake", "to": "marketing_review", "type": "sequence"},
                {"from": "marketing_review", "to": "exec_approval", "type": "sequence"},
                {"from": "exec_approval", "to": "report", "type": "sequence"},
            ],
        }
    )
    assert _parse_plan(plan, keys) is not None
    # Without the custom key allowed, the same plan is rejected.
    assert _parse_plan(plan) is None


def test_additional_catalog_empty_without_org_context() -> None:
    text, keys = _additional_catalog(None)
    assert text == "" and keys == set()


def test_parse_plan_accepts_valid_catalog_plan() -> None:
    good = json.dumps(
        {
            "nodes": [
                {
                    "key": "intake",
                    "name": "Intake",
                    "agent_type": "intake",
                    "department": "Planning",
                },
                {
                    "key": "exec_approval",
                    "name": "Approval",
                    "agent_type": "approval",
                    "department": "Executive",
                },
                {
                    "key": "report",
                    "name": "Report",
                    "agent_type": "report",
                    "department": "Planning",
                },
            ],
            "edges": [
                {"from": "intake", "to": "exec_approval", "type": "sequence"},
                {"from": "exec_approval", "to": "report", "type": "sequence"},
            ],
        }
    )
    plan = _parse_plan(good)
    assert plan is not None
    assert {n.key for n in plan.nodes} == {"intake", "exec_approval", "report"}


def test_parse_decision_rejects_garbage() -> None:
    assert _parse_decision(None) is None
    assert _parse_decision("{") is None
    assert _parse_decision('{"flags": []}') is None  # missing summary/status_text/tasks


def test_parse_decision_keeps_declared_block() -> None:
    raw = json.dumps(
        {
            "summary": "Budget is feasible.",
            "outcome": "approve",
            "flags": [{"severity": "info", "message": "Within budget."}],
            "tasks": [{"title": "Assess budget", "status": "completed"}],
            "status_text": "Finance review complete.",
            "blocked_on": {"on_department": "IT", "reason": "need assessment"},
        }
    )
    d = _parse_decision(raw)
    assert d is not None
    assert d.summary == "Budget is feasible."
    # An agent may now declare a cross-department block (F5); a declared
    # dependency is kept and forces the block outcome so the engine acts on it.
    assert d.blocked_on is not None
    assert d.blocked_on.on_department == "IT"
    assert d.outcome == "block"


def _valid_plan_json() -> str:
    return json.dumps(
        {
            "nodes": [
                {
                    "key": "intake",
                    "name": "Intake",
                    "agent_type": "intake",
                    "department": "Planning",
                },
                {
                    "key": "exec_approval",
                    "name": "Approval",
                    "agent_type": "approval",
                    "department": "Executive",
                },
                {
                    "key": "report",
                    "name": "Report",
                    "agent_type": "report",
                    "department": "Planning",
                },
            ],
            "edges": [
                {"from": "intake", "to": "exec_approval", "type": "sequence"},
                {"from": "exec_approval", "to": "report", "type": "sequence"},
            ],
        }
    )


def test_parse_plan_rejects_unreachable_report() -> None:
    # report exists but nothing connects to it -> orchestrator would stall.
    bad = json.dumps(
        {
            "nodes": [
                {"key": "intake", "name": "I", "agent_type": "intake", "department": "Planning"},
                {
                    "key": "exec_approval",
                    "name": "A",
                    "agent_type": "approval",
                    "department": "Executive",
                },
                {"key": "report", "name": "R", "agent_type": "report", "department": "Planning"},
            ],
            "edges": [{"from": "intake", "to": "exec_approval", "type": "sequence"}],
        }
    )
    assert _parse_plan(bad) is None


def test_parse_plan_rejects_edge_to_missing_node() -> None:
    bad = json.dumps(
        {
            "nodes": [
                {"key": "intake", "name": "I", "agent_type": "intake", "department": "Planning"},
                {
                    "key": "exec_approval",
                    "name": "A",
                    "agent_type": "approval",
                    "department": "Executive",
                },
                {"key": "report", "name": "R", "agent_type": "report", "department": "Planning"},
            ],
            "edges": [
                {"from": "intake", "to": "report", "type": "sequence"},
                {"from": "intake", "to": "planning", "type": "sequence"},  # planning not a node
            ],
        }
    )
    assert _parse_plan(bad) is None


def test_parse_decision_normalizes_severity() -> None:
    raw = json.dumps(
        {
            "summary": "ok",
            "flags": [
                {"severity": "high", "message": "x"},
                {"severity": "weird", "message": "y"},
            ],
            "tasks": [{"title": "t", "status": "completed"}],
            "status_text": "done",
        }
    )
    d = _parse_decision(raw)
    assert d is not None
    assert [f.severity for f in d.flags] == ["critical", "info"]


async def test_run_intake_uses_llm_then_validates(monkeypatch) -> None:
    from app.agents import intake

    monkeypatch.setattr(intake, "llm_available", lambda: True)

    async def fake_complete(system: str, user: str, **kw: object) -> str:
        return _valid_plan_json()

    monkeypatch.setattr(intake, "complete_json", fake_complete)
    plan = await intake.run_intake("t", "d", "high")
    assert {n.key for n in plan.nodes} == {"intake", "exec_approval", "report"}


async def test_run_intake_falls_back_on_bad_output(monkeypatch) -> None:
    from app.agents import intake

    monkeypatch.setattr(intake, "llm_available", lambda: True)

    async def fake_complete(system: str, user: str, **kw: object) -> str:
        return "not json"

    monkeypatch.setattr(intake, "complete_json", fake_complete)
    plan = await intake.run_intake("t", "d", "high")
    # the deterministic default plan has the full 10-stage catalog
    assert len(plan.nodes) == 10


async def test_run_department_uses_llm_then_falls_back(monkeypatch) -> None:
    from app.agents import department

    monkeypatch.setattr(department, "llm_available", lambda: True)

    async def good(system: str, user: str, **kw: object) -> str:
        return json.dumps(
            {
                "summary": "specific finance assessment",
                "flags": [],
                "tasks": [{"title": "assess", "status": "completed"}],
                "status_text": "done",
            }
        )

    monkeypatch.setattr(department, "complete_json", good)
    d = await department.run_department("finance", "Berlin office", "500k", "high")
    assert d.summary == "specific finance assessment"

    async def bad(system: str, user: str, **kw: object) -> str:
        return "{"

    monkeypatch.setattr(department, "complete_json", bad)
    d2 = await department.run_department("finance", "Berlin office", "500k", "high")
    # falls back to the deterministic finance playbook
    assert "budget" in d2.summary.lower() or "financial" in d2.summary.lower()
