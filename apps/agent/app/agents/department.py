"""Department agents: one stateless decision per workflow node.

Mirrors the intake planner's approach (``app/agents/intake.py``): when no LLM
provider key is configured the agents return a deterministic, department-specific
``Decision`` so the whole flow runs offline. Real Pydantic AI reasoning wires in
later behind the same ``run_department`` seam.

The cross-dependency case (Finance waiting on IT via ``raise_dependency``) is F5;
here every department completes with real tasks and a plain-language status.
"""

from __future__ import annotations

import json
import logging
from typing import Any

from app.agents.llm import complete_json, llm_available
from app.agents.models import Decision, DependencyDecl, Flag, TaskItem

logger = logging.getLogger(__name__)


def _decision(
    summary: str,
    status_text: str,
    tasks: list[str],
    flags: list[Flag] | None = None,
    blocked_on: DependencyDecl | None = None,
) -> Decision:
    return Decision(
        summary=summary,
        flags=flags or [],
        tasks=[TaskItem(title=t, status="completed") for t in tasks],
        status_text=status_text,
        blocked_on=blocked_on,
    )


# Deterministic department playbooks keyed by agent_type. Each produces a
# believable, role-specific decision with real tasks and a plain-language
# status line for the UI.
def _intake(title: str, **kwargs: Any) -> Decision:
    return _decision(
        summary=f"Classified '{title}' and identified the departments involved.",
        status_text="Intake complete — routed to planning.",
        tasks=[
            "Classify the request type",
            "Identify involved departments",
            "Set initial priority",
        ],
    )


def _planning(title: str, **kwargs: Any) -> Decision:
    return _decision(
        summary="Outlined the cross-department plan and review sequencing.",
        status_text="Planning complete — department reviews can begin.",
        tasks=["Draft the execution plan", "Sequence department reviews", "Estimate the timeline"],
    )


def _finance(
    title: str,
    upstream_context: list[dict[str, Any]] | None = None,
    **kwargs: Any,
) -> Decision:
    """Finance review. If IT assessment is not yet available, declare a
    cross-department dependency (F5) so the engine marks this node blocked
    until IT completes."""
    if upstream_context:
        for item in upstream_context:
            if isinstance(item, dict) and item.get("key", "").startswith("it_"):
                return _decision(
                    summary="Assessed budget feasibility with IT's input.",
                    status_text="Finance review complete — the request is financially viable.",
                    tasks=[
                        "Assess budget feasibility",
                        "Estimate the financial impact",
                        "Project the return on investment",
                        "Confirm funding availability",
                    ],
                    flags=[
                        Flag(
                            severity="info",
                            message="Spend is within the approved quarterly budget.",
                        ),
                    ],
                )
    return _decision(
        summary="Financial impact analysis in progress. Waiting for data from IT assessment.",
        status_text="Finance review is blocked — waiting for IT assessment.",
        tasks=[
            "Assess budget feasibility",
            "Estimate the financial impact",
            "Project the return on investment",
            "Confirm funding availability",
        ],
        blocked_on=DependencyDecl(
            on_department="IT",
            reason=(
                "Need the IT security assessment and infrastructure cost"
                " estimate before the budget can be finalized."
            ),
        ),
    )


def _legal(title: str, **kwargs: Any) -> Decision:
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


def _it(title: str, **kwargs: Any) -> Decision:
    return _decision(
        summary="Evaluated technical feasibility, security, and systems integration.",
        status_text="IT assessment complete — technically feasible with standard provisioning.",
        tasks=[
            "Assess technical feasibility",
            "Review security requirements",
            "Plan systems integration",
        ],
    )


def _hr(title: str, **kwargs: Any) -> Decision:
    return _decision(
        summary="Planned staffing and hiring needs.",
        status_text="HR planning complete — staffing plan ready.",
        tasks=["Plan staffing needs", "Outline the hiring timeline", "Identify onboarding steps"],
    )


def _ops(title: str, **kwargs: Any) -> Decision:
    return _decision(
        summary="Planned logistics, facilities, and the operational timeline.",
        status_text="Operations planning complete — execution plan ready.",
        tasks=["Plan logistics", "Arrange facilities", "Set the operational timeline"],
    )


def _approval(title: str, **kwargs: Any) -> Decision:
    return _decision(
        summary="Compiled department decisions and flags for executive review.",
        status_text="Ready for executive approval.",
        tasks=[
            "Compile department decisions",
            "Summarize flags and risks",
            "Prepare the approval packet",
        ],
    )


def _implementation(title: str, **kwargs: Any) -> Decision:
    return _decision(
        summary="Executed the approved plan across departments.",
        status_text="Implementation complete.",
        tasks=["Kick off execution", "Coordinate the departments", "Track delivery milestones"],
    )


def _report(title: str, **kwargs: Any) -> Decision:
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


# Role guidance per agent_type — gives the LLM the lens to reason through.
_ROLE_GUIDANCE = {
    "intake": "the Intake coordinator. Classify the request and identify the departments involved.",
    "planning": "the Planning lead. Outline the cross-department plan and sequence the reviews.",
    "finance": "the Finance department. Assess budget feasibility, financial impact, and ROI.",
    "legal": "the Legal department. Check regulatory compliance and contracts; flag legal risks.",
    "it": "the IT department. Evaluate technical feasibility, security, and systems integration.",
    "hr": "the HR department. Plan staffing, hiring, and onboarding needs.",
    "ops": "the Operations department. Plan logistics, facilities, and the operational timeline.",
    "approval": "the Executive office. Compile the department decisions and flags for approval.",
    "implementation": "the Implementation team. Execute the approved plan across departments.",
    "report": "the Reporting function. Summarize the decisions, flags, and outcomes.",
}

_DEPT_SYSTEM = """You are {role}

Review THIS specific request and respond ONLY with a JSON object:
{{
  "summary": "1-2 sentence assessment grounded in the actual request",
  "flags": [{{"severity": "info|warning|critical", "message": "specific risk or note"}}],
  "tasks": [{{"title": "concrete action you took", "status": "completed"}}],
  "status_text": "one plain-language sentence for the UI"
}}

Be specific to the request (amounts, locations, systems, people) — do not give \
generic boilerplate. Produce 3-5 tasks. Set blocked_on to null. Output JSON only."""


_ALLOWED_SEVERITY = {"info", "warning", "critical"}
# Map common off-contract severities the model might emit onto our scale.
_SEVERITY_ALIASES = {"high": "critical", "medium": "warning", "low": "info", "error": "critical"}


def _parse_decision(raw: str | None) -> Decision | None:
    """Validate an LLM JSON decision, or return None to fall back."""
    if not raw:
        return None
    try:
        decision = Decision.model_validate_json(raw)
    except Exception:  # noqa: BLE001 — malformed output falls back to deterministic
        return None
    if not decision.summary or not decision.status_text or not decision.tasks:
        return None
    # Normalize severities to the promised info|warning|critical scale rather
    # than letting a free-form value (e.g. "high") through.
    for flag in decision.flags:
        sev = flag.severity.lower().strip()
        flag.severity = sev if sev in _ALLOWED_SEVERITY else _SEVERITY_ALIASES.get(sev, "info")
    # The LLM prompt asks for blocked_on: null, so clear any stray value the
    # model emits. Cross-department blocking (F5) is driven by the deterministic
    # playbook below, not the LLM path.
    decision.blocked_on = None
    return decision


def _summarize_upstream(upstream_context: list[dict[str, Any]]) -> str:
    """Compact, bounded view of upstream decisions for the prompt.

    Trims the list (and each item to its key fields) so the context stays small
    without slicing a serialized JSON string mid-token.
    """
    compact = [
        {
            "node": item.get("node_key") or item.get("key"),
            "department": item.get("department"),
            "summary": str(item.get("summary") or item.get("status_text") or "")[:300],
        }
        for item in upstream_context[-8:]
    ]
    return json.dumps(compact, ensure_ascii=False)


async def run_department(
    agent_type: str,
    title: str,
    description: str,
    priority: str,
    upstream_context: list[dict[str, Any]] | None = None,
    org_context: dict[str, Any] | None = None,
) -> Decision:
    """Produce a department decision for one workflow node.

    Uses the configured LLM for real, request-specific reasoning; falls back to
    the deterministic, department-specific playbook when no provider is
    configured or the model output fails validation (the offline path the F3
    done-check exercises).
    """
    if llm_available():
        role = _ROLE_GUIDANCE.get(agent_type, f"the {agent_type} function.")
        user = f"Request title: {title}\nDescription: {description}\nPriority: {priority}"
        if upstream_context:
            user += "\n\nUpstream department decisions so far:\n" + _summarize_upstream(
                upstream_context
            )
        raw = await complete_json(_DEPT_SYSTEM.format(role=role), user)
        decision = _parse_decision(raw)
        if decision is not None:
            logger.info("Decision from LLM for %s", agent_type)
            return decision
        logger.info("LLM decision unusable for %s, using deterministic playbook", agent_type)

    playbook = _PLAYBOOK.get(agent_type)
    if playbook is None:
        # Unknown stage — still complete with a generic, honest decision.
        return _decision(
            summary=f"Reviewed the request for the {agent_type} stage.",
            status_text=f"{agent_type.replace('_', ' ').title()} stage complete.",
            tasks=["Review the request", "Record the outcome"],
        )
    return playbook(title, upstream_context=upstream_context)
