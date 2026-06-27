"""Intake planner: turns a business request into a department workflow graph.

Uses an LLM when a provider key is available; falls back to a deterministic
default plan when no key is set so the system always works offline.
"""

from __future__ import annotations

import logging
from typing import Any

from app.agents.llm import complete_json, llm_available
from app.agents.models import Plan, PlanEdge, PlanNode

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


# The fixed department catalog the intake agent must plan from. The
# orchestrator only understands these stage keys, so any LLM plan that strays
# outside them (or drops the required ends) is rejected for the default plan.
_ALLOWED_KEYS = {
    "intake",
    "planning",
    "finance_review",
    "legal_review",
    "it_assessment",
    "hr_planning",
    "ops_planning",
    "exec_approval",
    "implementation",
    "report",
}
_REQUIRED_KEYS = {"intake", "exec_approval", "report"}

_INTAKE_SYSTEM = """You are the intake planner for an AI Organization OS. Turn a \
business request into a department workflow graph.

Return ONLY a JSON object of this shape:
{
  "nodes": [{"key": str, "name": str, "agent_type": str, "department": str}],
  "edges": [{"from": str, "to": str, "type": "sequence"}]
}

Choose node "key" values ONLY from this fixed set (omit stages the request does \
not need, but keep the flow sensible):
  intake (agent_type intake, dept Planning)
  planning (agent_type planning, dept Planning)
  finance_review (agent_type finance, dept Finance)
  legal_review (agent_type legal, dept Legal)
  it_assessment (agent_type it, dept IT)
  hr_planning (agent_type hr, dept HR)
  ops_planning (agent_type ops, dept Operations)
  exec_approval (agent_type approval, dept Executive)
  implementation (agent_type implementation, dept Operations)
  report (agent_type report, dept Planning)

Pick the departments the request actually needs — do not include every stage by \
reflex. Guidance:
  finance_review — any spend, budget, pricing, or funding implication.
  legal_review — contracts, regulation, compliance, data/privacy, hiring abroad.
  it_assessment — software, infrastructure, security, data, or systems work.
  hr_planning — hiring, headcount, staffing, or onboarding.
  ops_planning — facilities, logistics, vendors, or physical operations.
A small software tweak may need only it_assessment; a hire needs hr_planning and \
legal_review; a pure policy change may need only legal_review. Include planning and \
implementation when the work is cross-department or needs execution.

Rules: always include intake, exec_approval, and report. The department reviews you \
choose run in parallel after planning and all feed exec_approval. After approval, \
any post-approval stages (hr_planning, ops_planning, implementation) run, then \
report. Connect every node with edges so the graph flows from intake to report. \
Output JSON only, no prose."""


def _parse_plan(raw: str | None) -> Plan | None:
    """Validate an LLM JSON plan against the fixed catalog, or return None."""
    if not raw:
        return None
    try:
        plan = Plan.model_validate_json(raw)
    except Exception:  # noqa: BLE001 — malformed output falls back to default
        return None
    keys = {n.key for n in plan.nodes}
    if not keys or not keys.issubset(_ALLOWED_KEYS):
        return None
    if not _REQUIRED_KEYS.issubset(keys):
        return None
    if not plan.edges:
        return None
    # Light well-formedness check: every edge must reference real nodes, and
    # `report` must be reachable from `intake` — otherwise the orchestrator
    # would just stall on a disconnected graph. Fall back to the default plan.
    adjacency: dict[str, list[str]] = {k: [] for k in keys}
    for edge in plan.edges:
        if edge.from_ not in keys or edge.to not in keys:
            return None
        adjacency[edge.from_].append(edge.to)
    if not _reaches("intake", "report", adjacency):
        return None
    return plan


def _reaches(start: str, goal: str, adjacency: dict[str, list[str]]) -> bool:
    """True if `goal` is reachable from `start` in the directed graph."""
    seen: set[str] = set()
    stack = [start]
    while stack:
        node = stack.pop()
        if node == goal:
            return True
        if node in seen:
            continue
        seen.add(node)
        stack.extend(adjacency.get(node, []))
    return False


async def run_intake(
    title: str,
    description: str,
    priority: str,
    org_context: dict[str, Any] | None = None,
) -> Plan:
    """Plan a workflow for the given business request.

    Uses the configured LLM to reason about which departments the request needs;
    falls back to the deterministic default plan when no provider is configured
    or the model output fails validation.
    """
    if llm_available():
        raw = await complete_json(
            _INTAKE_SYSTEM,
            f"Request title: {title}\nDescription: {description}\nPriority: {priority}",
        )
        plan = _parse_plan(raw)
        if plan is not None:
            logger.info("Intake plan from LLM (%d nodes)", len(plan.nodes))
            return plan
        logger.info("LLM intake plan unusable, using default plan")

    return _default_plan()
