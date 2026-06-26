"""Intake planner: turns a business request into a department workflow graph.

Uses an LLM when a provider key is available; falls back to a deterministic
default plan when no key is set so the system always works offline.
"""

from __future__ import annotations

import logging
from typing import Any

from app.agents.models import Plan, PlanEdge, PlanNode
from app.settings import settings

logger = logging.getLogger(__name__)


def _node(key: str, name: str, agent_type: str, department: str) -> PlanNode:
    return PlanNode(key=key, name=name, agent_type=agent_type, department=department)


def _edge(src: str, tgt: str) -> PlanEdge:
    return PlanEdge(**{"from": src, "to": tgt, "type": "sequence"})


def _default_plan() -> Plan:
    """Deterministic fallback plan with ~10 stages and parallel branches."""
    return Plan(
        nodes=[
            _node("intake", "Intake & Classification", "intake", "Planning"),
            _node("planning", "Strategic Planning", "planning", "Planning"),
            _node("finance_review", "Finance Review", "finance", "Finance"),
            _node("legal_review", "Legal Review", "legal", "Legal"),
            _node("it_assessment", "IT Assessment", "it", "IT"),
            _node("hr_planning", "HR Planning", "hr", "HR"),
            _node("ops_planning", "Operations Planning", "ops", "Operations"),
            _node("exec_approval", "Executive Approval", "approval", "Executive"),
            _node("implementation", "Implementation", "implementation", "Operations"),
            _node("report", "Review & Report", "report", "Planning"),
        ],
        edges=[
            _edge("intake", "planning"),
            _edge("planning", "finance_review"),
            _edge("planning", "legal_review"),
            _edge("planning", "it_assessment"),
            _edge("finance_review", "exec_approval"),
            _edge("legal_review", "exec_approval"),
            _edge("it_assessment", "exec_approval"),
            _edge("exec_approval", "hr_planning"),
            _edge("exec_approval", "ops_planning"),
            _edge("hr_planning", "implementation"),
            _edge("ops_planning", "implementation"),
            _edge("implementation", "report"),
        ],
    )


def _has_any_llm_key() -> bool:
    """Check if any LLM provider key is configured."""
    return any([
        settings.anthropic_api_key,
        settings.openai_api_key,
        settings.google_api_key,
        settings.groq_api_key,
    ])


async def run_intake(
    title: str,
    description: str,
    priority: str,
    org_context: dict[str, Any] | None = None,
) -> Plan:
    """Plan a workflow for the given business request.

    Returns a deterministic default plan when no LLM key is available.
    """
    if not _has_any_llm_key():
        logger.info("No LLM key configured, returning default plan")
        return _default_plan()

    # When an LLM key IS available, we still return the default plan
    # for now. F3/AG-6 will wire the actual Pydantic AI agent.
    logger.info("Returning default plan (LLM intake agent not yet wired)")
    return _default_plan()
