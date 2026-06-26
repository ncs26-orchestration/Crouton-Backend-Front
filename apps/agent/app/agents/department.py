"""Department agents: one stateless decision per workflow node.

Mirrors the intake planner's approach (``app/agents/intake.py``): when no LLM
provider key is configured the agents return a deterministic, department-specific
``Decision`` so the whole flow runs offline. Real Pydantic AI reasoning wires in
later behind the same ``run_department`` seam.

The cross-dependency case (Finance waiting on IT via ``raise_dependency``) is F5;
here every department completes with real tasks and a plain-language status.
"""

from __future__ import annotations

import logging
from typing import Any

from app.agents.models import Decision, Flag, TaskItem
from app.settings import settings

logger = logging.getLogger(__name__)


def _decision(
    summary: str, status_text: str, tasks: list[str], flags: list[Flag] | None = None
) -> Decision:
    return Decision(
        summary=summary,
        flags=flags or [],
        tasks=[TaskItem(title=t, status="completed") for t in tasks],
        status_text=status_text,
        blocked_on=None,
    )


# Deterministic department playbooks keyed by agent_type. Each produces a
# believable, role-specific decision with real tasks and a plain-language
# status line for the UI.
def _intake(title: str) -> Decision:
    return _decision(
        summary=f"Classified '{title}' and identified the departments involved.",
        status_text="Intake complete — routed to planning.",
        tasks=[
            "Classify the request type",
            "Identify involved departments",
            "Set initial priority",
        ],
    )


def _planning(title: str) -> Decision:
    return _decision(
        summary="Outlined the cross-department plan and review sequencing.",
        status_text="Planning complete — department reviews can begin.",
        tasks=["Draft the execution plan", "Sequence department reviews", "Estimate the timeline"],
    )


def _finance(title: str) -> Decision:
    return _decision(
        summary="Assessed budget feasibility, financial impact, and ROI.",
        status_text="Finance review complete — the request is financially viable.",
        tasks=[
            "Assess budget feasibility",
            "Estimate the financial impact",
            "Project the return on investment",
            "Confirm funding availability",
        ],
        flags=[Flag(severity="info", message="Spend is within the approved quarterly budget.")],
    )


def _legal(title: str) -> Decision:
    return _decision(
        summary="Checked regulatory compliance and contractual requirements.",
        status_text="Legal review complete — no blocking issues, one item to track.",
        tasks=["Review regulatory compliance", "Check contract requirements", "Flag legal risks"],
        flags=[
            Flag(
                severity="warning",
                message="A registered local entity may be required before hiring.",
            )
        ],
    )


def _it(title: str) -> Decision:
    return _decision(
        summary="Evaluated technical feasibility, security, and systems integration.",
        status_text="IT assessment complete — technically feasible with standard provisioning.",
        tasks=[
            "Assess technical feasibility",
            "Review security requirements",
            "Plan systems integration",
        ],
    )


def _hr(title: str) -> Decision:
    return _decision(
        summary="Planned staffing and hiring needs.",
        status_text="HR planning complete — staffing plan ready.",
        tasks=["Plan staffing needs", "Outline the hiring timeline", "Identify onboarding steps"],
    )


def _ops(title: str) -> Decision:
    return _decision(
        summary="Planned logistics, facilities, and the operational timeline.",
        status_text="Operations planning complete — execution plan ready.",
        tasks=["Plan logistics", "Arrange facilities", "Set the operational timeline"],
    )


def _approval(title: str) -> Decision:
    return _decision(
        summary="Compiled department decisions and flags for executive review.",
        status_text="Ready for executive approval.",
        tasks=[
            "Compile department decisions",
            "Summarize flags and risks",
            "Prepare the approval packet",
        ],
    )


def _implementation(title: str) -> Decision:
    return _decision(
        summary="Executed the approved plan across departments.",
        status_text="Implementation complete.",
        tasks=["Kick off execution", "Coordinate the departments", "Track delivery milestones"],
    )


def _report(title: str) -> Decision:
    return _decision(
        summary="Produced the final report of decisions, flags, and outcomes.",
        status_text="Final report generated.",
        tasks=["Summarize approvals", "Compile flags", "Record time taken"],
    )


_PLAYBOOK = {
    "intake": _intake,
    "planning": _planning,
    "finance": _finance,
    "legal": _legal,
    "it": _it,
    "hr": _hr,
    "ops": _ops,
    "approval": _approval,
    "implementation": _implementation,
    "report": _report,
}


def _has_any_llm_key() -> bool:
    """Check if any LLM provider key is configured."""
    return any(
        [
            settings.anthropic_api_key,
            settings.openai_api_key,
            settings.google_api_key,
            settings.groq_api_key,
        ]
    )


async def run_department(
    agent_type: str,
    title: str,
    description: str,
    priority: str,
    upstream_context: list[dict[str, Any]] | None = None,
    org_context: dict[str, Any] | None = None,
) -> Decision:
    """Produce a department decision for one workflow node.

    Returns a deterministic, department-specific decision when no LLM key is
    available (the offline path the F3 done-check exercises).
    """
    if not _has_any_llm_key():
        logger.info("No LLM key configured, returning deterministic decision for %s", agent_type)

    playbook = _PLAYBOOK.get(agent_type)
    if playbook is None:
        # Unknown stage — still complete with a generic, honest decision.
        return _decision(
            summary=f"Reviewed the request for the {agent_type} stage.",
            status_text=f"{agent_type.replace('_', ' ').title()} stage complete.",
            tasks=["Review the request", "Record the outcome"],
        )
    return playbook(title)
