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
from app.agents.models import OUTCOMES, Decision, DependencyDecl, Flag, TaskItem

logger = logging.getLogger(__name__)


def _decision(
    summary: str,
    status_text: str,
    tasks: list[str],
    flags: list[Flag] | None = None,
    blocked_on: DependencyDecl | None = None,
    outcome: str = "approve",
    reasoning: str | None = None,
    key_factors: list[str] | None = None,
) -> Decision:
    flags = flags or []
    # Offline playbooks rarely pass reasoning/key_factors; synthesize a believable
    # version from what they do pass so the approver's "how it decided" brief is
    # never empty on the deterministic path.
    if reasoning is None:
        reasoning = f"{summary} Reached '{outcome.replace('_', ' ')}' after standard checks."
    if key_factors is None:
        key_factors = [f.message for f in flags] or [summary]
    return Decision(
        summary=summary,
        reasoning=reasoning,
        key_factors=key_factors,
        outcome=outcome,
        flags=flags,
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
        outcome="block",
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


# Role guidance per agent_type — the lens each department reasons through, with
# the concrete criteria and thresholds it should weigh so decisions vary by
# request instead of reading like boilerplate.
_ROLE_GUIDANCE = {
    "intake": (
        "the Intake coordinator. Classify the request and identify which departments "
        "it actually involves."
    ),
    "planning": ("the Planning lead. Outline the cross-department plan and sequence the reviews."),
    "finance": (
        "the Finance department. Weigh budget feasibility, total cost, and ROI against the "
        "approved budget. If the cost depends on another department's estimate (e.g. IT "
        "infrastructure), block on it. Flag spend that exceeds the quarterly budget; reject "
        "only a clear, policy-defined overspend with no funding path."
    ),
    "legal": (
        "the Legal department. Check regulatory compliance, contracts, data protection, and "
        "jurisdiction. Flag legal risk that needs tracking; reject a request that would "
        "violate a hard compliance or regulatory requirement."
    ),
    "it": (
        "the IT department. Assess technical feasibility, security, data handling, and "
        "systems integration. Give a concrete cost/effort estimate downstream departments "
        "can rely on. Flag security or scalability risk."
    ),
    "hr": (
        "the HR department. Plan staffing, hiring, headcount, and onboarding. Flag requests "
        "that exceed approved headcount or compress hiring timelines unrealistically."
    ),
    "ops": (
        "the Operations department. Plan logistics, facilities, vendors, and the operational "
        "timeline. Flag capacity or supply constraints."
    ),
    "approval": "the Executive office. Compile the department decisions and flags for approval.",
    "implementation": "the Implementation team. Execute the approved plan across departments.",
    "report": "the Reporting function. Summarize the decisions, flags, and outcomes.",
}

_DEPT_SYSTEM = """You are {role}

Review THIS specific request and decide. Respond ONLY with a JSON object:
{{
  "summary": "1-2 sentence assessment grounded in the actual request",
  "reasoning": "2-4 sentences on HOW you reached the outcome from details, policies, upstream",
  "key_factors": ["concrete facts that drove it: amounts, policy names, dates, risks"],
  "outcome": "approve | approve_with_conditions | flag | reject | block",
  "flags": [{{"severity": "info|warning|critical", "message": "specific risk or note"}}],
  "tasks": [{{"title": "concrete action you took", "status": "completed"}}],
  "status_text": "one plain-language sentence for the UI",
  "blocked_on": null
}}

How to choose "outcome":
- "approve": no concerns from your department.
- "approve_with_conditions": fine to proceed, but list the conditions as flags.
- "flag": a real risk the executive should see before approving (does not stop the request).
- "reject": the request violates a hard rule your department owns. Only reject on a clear,
  policy-grounded violation, and name the policy in a critical flag.
- "block": you cannot finish until another department gives you something first. Set
  "blocked_on": {{"on_department": "<Department>", "reason": "<what you need and why>"}} and
  pick this outcome only when that department has not reported yet.

{policies}{upstream_guidance}Be specific to the request (amounts, locations, people).
Do not give generic boilerplate. When a flag or rejection is driven by a policy, quote the
policy in the flag message. Produce 3-5 tasks describing what you actually checked.
Output JSON only."""


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
    # Keep outcome and blocked_on consistent: an off-contract outcome falls back
    # to "approve"; a block without a target is downgraded to a flag; a declared
    # dependency forces the block outcome so the engine acts on it (F5).
    decision.outcome = decision.outcome.lower().strip()
    if decision.outcome not in OUTCOMES:
        decision.outcome = "approve"
    if decision.outcome == "block" and decision.blocked_on is None:
        decision.outcome = "flag"
    if decision.blocked_on is not None:
        decision.outcome = "block"
    return decision


def _summarize_upstream(upstream_context: list[dict[str, Any]]) -> str:
    """Compact, bounded view of upstream decisions for the prompt.

    Carries each upstream department's outcome and flags (not just its status
    line) so a department can actually reason over what others found — e.g.
    Finance using IT's cost flag. Trimmed to the last few so the context stays
    small without slicing a serialized JSON string mid-token.
    """
    compact = [
        {
            "department": item.get("department"),
            "outcome": item.get("outcome") or "approve",
            "summary": str(item.get("summary") or item.get("status_text") or "")[:300],
            "flags": [
                f"{f.get('severity', 'info')}: {f.get('message', '')}"
                for f in (item.get("flags") or [])
            ][:4],
        }
        for item in upstream_context[-8:]
    ]
    return json.dumps(compact, ensure_ascii=False)


def _policies_block(org_context: dict[str, Any] | None) -> str:
    """Render the department's policies for the prompt, or an empty string.

    The engine fills ``org_context["policies"]`` with the policies that apply to
    this department (title + body). The agent must check the request against them
    and cite the policy by title in any flag or rejection.
    """
    if not org_context:
        return ""
    policies = org_context.get("policies") or []
    if not policies:
        return ""
    lines = [
        f"- {p.get('title', 'Policy')}: {str(p.get('body', '')).strip()}"
        for p in policies
        if isinstance(p, dict)
    ]
    if not lines:
        return ""
    header = "Your department's policies. Check the request against each and cite by title:\n"
    return header + "\n".join(lines) + "\n\n"


def _details_block(details: dict[str, Any] | None) -> str:
    """Render the request's structured fields so an agent can cite real numbers."""
    if not details:
        return ""
    lines = [f"  {k}: {v}" for k, v in details.items() if v not in (None, "")]
    if not lines:
        return ""
    return "\n\nStructured details (use these exact facts):\n" + "\n".join(lines)


async def run_department(
    agent_type: str,
    title: str,
    description: str,
    priority: str,
    details: dict[str, Any] | None = None,
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
        policies = _policies_block(org_context)
        upstream_guidance = (
            "Use the upstream department decisions below — reference their concrete "
            "findings (costs, headcount, risks) in your reasoning.\n"
            if upstream_context
            else ""
        )
        system = _DEPT_SYSTEM.format(
            role=role,
            policies=policies,
            upstream_guidance=upstream_guidance,
        )
        user = (
            f"Request title: {title}\nDescription: {description}\nPriority: {priority}"
            + _details_block(details)
        )
        if upstream_context:
            user += "\n\nUpstream department decisions so far:\n" + _summarize_upstream(
                upstream_context
            )
        raw = await complete_json(system, user)
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


_ANSWER_SYSTEM = """You are {role}

A human verifier is reviewing your assessment of a request and asked a question.
Answer it directly and concretely in 1-3 sentences, grounded in the request and
your own prior assessment. Respond ONLY with JSON: {{"reply": "your answer"}}"""

_REVISE_SYSTEM = """You are {role}

A human verifier reviewed your assessment and asked you to change it. Reconsider
and produce a REVISED decision that takes their feedback into account. Respond
ONLY with a JSON object:
{{
  "reply": "1-2 sentences telling the verifier what you changed and why",
  "summary": "your revised 1-2 sentence assessment",
  "reasoning": "2-4 sentences on how you reached the revised outcome given the feedback",
  "key_factors": ["the concrete facts that drove the revised decision"],
  "outcome": "approve | approve_with_conditions | flag | reject | block",
  "flags": [{{"severity": "info|warning|critical", "message": "specific risk or note"}}],
  "tasks": [{{"title": "concrete action you took", "status": "completed"}}],
  "status_text": "one plain-language sentence for the UI",
  "blocked_on": null
}}
{policies}Be specific (amounts, locations, systems, people). Output JSON only."""


def _prior_block(prior: dict[str, Any] | None) -> str:
    if not prior:
        return ""
    out = "\n\nYour prior assessment:\n"
    out += f"  outcome: {prior.get('outcome', 'approve')}\n"
    out += f"  summary: {prior.get('summary') or prior.get('status_text') or ''}\n"
    return out


async def run_converse(
    agent_type: str,
    title: str,
    description: str,
    priority: str,
    mode: str,
    feedback: str,
    prior: dict[str, Any] | None = None,
    details: dict[str, Any] | None = None,
    upstream_context: list[dict[str, Any]] | None = None,
    org_context: dict[str, Any] | None = None,
) -> tuple[str, Decision | None]:
    """Continue the verifier↔agent conversation on a node.

    mode "answer": reply to a question; the decision is unchanged (returns None).
    mode "revise": reconsider given the feedback and return a revised Decision.
    Offline (no LLM) it acknowledges without changing the decision.
    """
    role = _ROLE_GUIDANCE.get(agent_type, f"the {agent_type} function.")
    if not llm_available():
        if mode == "revise":
            return (
                "I've noted your feedback. Connect the LLM to have me revise the assessment.",
                None,
            )
        return ("Thanks — noted. My assessment stands for now.", None)

    base = (
        f"Request title: {title}\nDescription: {description}\nPriority: {priority}"
        + _details_block(details)
        + _prior_block(prior)
        + f"\n\nVerifier said: {feedback}"
    )
    if mode == "revise":
        system = _REVISE_SYSTEM.format(role=role, policies=_policies_block(org_context))
        raw = await complete_json(system, base)
        decision = _parse_decision(raw)
        reply = "I've reconsidered."
        try:
            import json as _json

            parsed = _json.loads(raw) if raw else {}
            if isinstance(parsed, dict) and parsed.get("reply"):
                reply = str(parsed["reply"])
            elif decision is not None:
                reply = "I've updated my assessment. " + decision.summary
        except Exception:  # noqa: BLE001
            if decision is not None:
                reply = "I've updated my assessment. " + decision.summary
        return (reply, decision)

    # answer
    system = _ANSWER_SYSTEM.format(role=role)
    raw = await complete_json(system, base)
    reply = "I don't have more to add right now."
    try:
        import json as _json

        parsed = _json.loads(raw) if raw else {}
        if isinstance(parsed, dict) and parsed.get("reply"):
            reply = str(parsed["reply"])
    except Exception:  # noqa: BLE001
        pass
    return (reply, None)
