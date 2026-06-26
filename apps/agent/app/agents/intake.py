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


def _default_plan() -> Plan:
    """Deterministic fallback plan with ~10 stages and parallel branches."""
    return Plan(
        nodes=[
            PlanNode(key="intake", name="Intake & Classification", agent_type="intake", department="Planning"),
            PlanNode(key="planning", name="Strategic Planning", agent_type="planning", department="Planning"),
            PlanNode(key="finance_review", name="Finance Review", agent_type="finance", department="Finance"),
            PlanNode(key="legal_review", name="Legal Review", agent_type="legal", department="Legal"),
            PlanNode(key="it_assessment", name="IT Assessment", agent_type="it", department="IT"),
            PlanNode(key="hr_planning", name="HR Planning", agent_type="hr", department="HR"),
            PlanNode(key="ops_planning", name="Operations Planning", agent_type="ops", department="Operations"),
            PlanNode(
                key="exec_approval", name="Executive Approval", agent_type="approval", department="Executive"
            ),
            PlanNode(
                key="implementation",
                name="Implementation",
                agent_type="implementation",
                department="Operations",
            ),
            PlanNode(key="report", name="Review & Report", agent_type="report", department="Planning"),
        ],
        edges=[
            PlanEdge(**{"from": "intake", "to": "planning", "type": "sequence"}),
            PlanEdge(**{"from": "planning", "to": "finance_review", "type": "sequence"}),
            PlanEdge(**{"from": "planning", "to": "legal_review", "type": "sequence"}),
            PlanEdge(**{"from": "planning", "to": "it_assessment", "type": "sequence"}),
            PlanEdge(**{"from": "finance_review", "to": "exec_approval", "type": "sequence"}),
            PlanEdge(**{"from": "legal_review", "to": "exec_approval", "type": "sequence"}),
            PlanEdge(**{"from": "it_assessment", "to": "exec_approval", "type": "sequence"}),
            PlanEdge(**{"from": "exec_approval", "to": "hr_planning", "type": "sequence"}),
            PlanEdge(**{"from": "exec_approval", "to": "ops_planning", "type": "sequence"}),
            PlanEdge(**{"from": "hr_planning", "to": "implementation", "type": "sequence"}),
            PlanEdge(**{"from": "ops_planning", "to": "implementation", "type": "sequence"}),
            PlanEdge(**{"from": "implementation", "to": "report", "type": "sequence"}),
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

    # When an LLM key IS available, we still return the default plan for now.
    # A future feature (F3/AG-6) will wire up the actual Pydantic AI agent
    # with structured output and tool calling.
    logger.info("Returning default plan (LLM intake agent not yet wired)")
    return _default_plan()
