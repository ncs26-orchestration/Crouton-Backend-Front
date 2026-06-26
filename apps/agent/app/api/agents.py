"""Agent endpoints called by the Go orchestration engine."""

from __future__ import annotations

from typing import Any

from fastapi import APIRouter
from pydantic import BaseModel, Field

from app.agents.department import run_department
from app.agents.intake import run_intake
from app.agents.models import Decision, Plan

router = APIRouter(prefix="/agents", tags=["agents"])


class IntakeRequestBody(BaseModel):
    title: str
    description: str = ""
    priority: str = "medium"


class IntakeRequest(BaseModel):
    request: IntakeRequestBody
    org_context: dict[str, Any] = Field(default_factory=dict)


class RunRequest(BaseModel):
    agent_type: str
    request: IntakeRequestBody
    upstream_context: list[dict[str, Any]] = Field(default_factory=list)
    org_context: dict[str, Any] = Field(default_factory=dict)


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
        upstream_context=body.upstream_context,
        org_context=body.org_context,
    )
