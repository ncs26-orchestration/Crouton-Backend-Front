"""Unit tests for the LLM wiring: provider gate + strict output validation.

Pure and deterministic — no network. They prove that malformed or off-catalog
model output is rejected (so callers fall back) and that valid output parses."""

import json

from app.agents import llm
from app.agents.department import _parse_decision
from app.agents.intake import _parse_plan


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


def test_parse_decision_accepts_valid_and_clears_blocked_on() -> None:
    raw = json.dumps(
        {
            "summary": "Budget is feasible.",
            "flags": [{"severity": "info", "message": "Within budget."}],
            "tasks": [{"title": "Assess budget", "status": "completed"}],
            "status_text": "Finance review complete.",
            "blocked_on": {"on_department": "IT", "reason": "need assessment"},
        }
    )
    d = _parse_decision(raw)
    assert d is not None
    assert d.summary == "Budget is feasible."
    # F5 not built yet — blocked_on must be cleared so the engine never stalls.
    assert d.blocked_on is None
