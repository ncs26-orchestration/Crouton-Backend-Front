"""Diagnostic endpoints for machine fault diagnosis."""

from __future__ import annotations

from typing import Any

from fastapi import APIRouter
from pydantic import BaseModel, Field

from app.agents.diagnostic import Diagnosis, run_diagnostic

router = APIRouter(prefix="/diagnostic", tags=["diagnostic"])


class DiagnoseRequest(BaseModel):
    incident_title: str
    incident_description: str = ""
    severity: str = "medium"
    machine_name: str
    machine_type: str
    manual_text: str = ""
    telemetry: dict[str, Any] = Field(default_factory=dict)


@router.post("/diagnose")
async def diagnose(body: DiagnoseRequest) -> Diagnosis:
    """Generate a step-by-step diagnostic plan for a machine incident."""
    return await run_diagnostic(
        incident_title=body.incident_title,
        incident_description=body.incident_description,
        severity=body.severity,
        machine_name=body.machine_name,
        machine_type=body.machine_type,
        manual_text=body.manual_text,
        telemetry=body.telemetry,
    )
