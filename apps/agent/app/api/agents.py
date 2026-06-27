"""Agent endpoints called by the Go orchestration engine."""

from __future__ import annotations

from typing import Any

from fastapi import APIRouter
from pydantic import BaseModel, Field

from app.agents.department import run_converse, run_department
from app.agents.intake import run_intake
from app.agents.models import Decision, Plan

router = APIRouter(prefix="/agents", tags=["agents"])


class IntakeRequestBody(BaseModel):
    title: str
    description: str = ""
    priority: str = "medium"
    details: dict[str, Any] = Field(default_factory=dict)


class IntakeRequest(BaseModel):
    request: IntakeRequestBody
    org_context: dict[str, Any] = Field(default_factory=dict)


class RunRequest(BaseModel):
    agent_type: str
    request: IntakeRequestBody
    upstream_context: list[dict[str, Any]] = Field(default_factory=list)
    org_context: dict[str, Any] = Field(default_factory=dict)


class ConverseRequest(BaseModel):
    agent_type: str
    request: IntakeRequestBody
    mode: str = "answer"  # answer | revise
    feedback: str = ""
    prior_decision: dict[str, Any] = Field(default_factory=dict)
    upstream_context: list[dict[str, Any]] = Field(default_factory=list)
    org_context: dict[str, Any] = Field(default_factory=dict)


class ConverseResponse(BaseModel):
    reply: str
    decision: Decision | None = None


@router.post("/intake")
async def intake(body: IntakeRequest) -> Plan:
    """Plan a department workflow for a business request.

    Returns the typed ``Plan`` directly; FastAPI serializes it by alias so
    ``PlanEdge.from_`` is emitted as ``from`` on the wire.
    """
    return await run_intake(
        title=body.request.title,
        description=body.request.description,
        priority=body.request.priority,
        details=body.request.details,
        org_context=body.org_context,
    )


@router.post("/run")
async def run(body: RunRequest) -> Decision:
    """Run one department agent for a workflow node and return its decision."""
    return await run_department(
        agent_type=body.agent_type,
        title=body.request.title,
        description=body.request.description,
        priority=body.request.priority,
        details=body.request.details,
        upstream_context=body.upstream_context,
        org_context=body.org_context,
    )


@router.post("/converse")
async def converse(body: ConverseRequest) -> ConverseResponse:
    """Continue the verifier↔agent conversation on a node: answer a question, or
    revise the decision given feedback."""
    reply, decision = await run_converse(
        agent_type=body.agent_type,
        title=body.request.title,
        description=body.request.description,
        priority=body.request.priority,
        mode=body.mode,
        feedback=body.feedback,
        prior=body.prior_decision,
        details=body.request.details,
        upstream_context=body.upstream_context,
        org_context=body.org_context,
    )
    return ConverseResponse(reply=reply, decision=decision)
