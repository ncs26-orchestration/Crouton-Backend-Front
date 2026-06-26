"""Agent endpoints called by the Go orchestration engine."""

from __future__ import annotations

from typing import Any

from fastapi import APIRouter
from pydantic import BaseModel, Field

from app.agents.intake import run_intake

router = APIRouter(prefix="/agents", tags=["agents"])


class IntakeRequestBody(BaseModel):
    title: str
    description: str = ""
    priority: str = "medium"


class IntakeRequest(BaseModel):
    request: IntakeRequestBody
    org_context: dict[str, Any] = Field(default_factory=dict)


class PlanNodeResponse(BaseModel):
    key: str
    name: str
    agent_type: str
    department: str


class PlanEdgeResponse(BaseModel):
    from_: str = Field(alias="from", serialization_alias="from")
    to: str
    type: str = "sequence"

    model_config = {"populate_by_name": True}


class PlanResponse(BaseModel):
    nodes: list[PlanNodeResponse]
    edges: list[PlanEdgeResponse]


@router.post("/intake")
async def intake(body: IntakeRequest) -> PlanResponse:
    """Plan a department workflow for a business request."""
    plan = await run_intake(
        title=body.request.title,
        description=body.request.description,
        priority=body.request.priority,
        org_context=body.org_context,
    )
    return PlanResponse(
        nodes=[
            PlanNodeResponse(
                key=n.key,
                name=n.name,
                agent_type=n.agent_type,
                department=n.department,
            )
            for n in plan.nodes
        ],
        edges=[
            PlanEdgeResponse(**{"from": e.from_, "to": e.to, "type": e.type})
            for e in plan.edges
        ],
    )
