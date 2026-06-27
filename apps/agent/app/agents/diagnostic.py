"""Machine diagnostic agent: generates step-by-step repair checkpoints.

Takes incident info, machine info, and extracted manual text, then produces
ordered diagnostic steps grounded in the actual machine manual. When no LLM
provider key is configured the agent returns deterministic, severity-based
steps so the whole flow runs offline.
"""

from __future__ import annotations

import logging
from typing import Any

from pydantic import BaseModel, Field

from app.agents.llm import complete_json, llm_available

logger = logging.getLogger(__name__)


# ---------------------------------------------------------------------------
# Output models
# ---------------------------------------------------------------------------


class DiagnosticStep(BaseModel):
    """A single step in a machine diagnostic procedure."""

    title: str = Field(description="Short title for this step")
    description: str = Field(description="Detailed instruction for the technician")
    action_type: str = Field(
        description="One of: check, measure, replace, restart, calibrate, inspect, clean, test"
    )
    expected_outcome: str | None = Field(
        default=None, description="What the technician should observe if this step succeeds"
    )
    warning: str | None = Field(
        default=None, description="Safety warning associated with this step"
    )


class Diagnosis(BaseModel):
    """The diagnostic agent's output: a structured repair plan."""

    summary: str = Field(description="One-sentence summary of the diagnosis")
    root_cause: str | None = Field(default=None, description="Probable root cause, if identifiable")
    steps: list[DiagnosticStep] = Field(
        default_factory=list, description="Ordered diagnostic/repair steps"
    )


# ---------------------------------------------------------------------------
# LLM prompt
# ---------------------------------------------------------------------------

_DIAG_SYSTEM = """You are a senior industrial maintenance engineer.

Given an incident report, machine information, telemetry readings, and the \
machine's service manual (excerpted below), produce a JSON diagnostic plan.

=== MACHINE MANUAL EXCERPT ===
{manual_excerpt}
=== END MANUAL ===

If the manual is relevant to the incident, use it to ground your steps. \
If the manual content is empty, generic, or unrelated, ignore it and rely on \
your general industrial maintenance knowledge instead.

Respond ONLY with a JSON object:
{{
  "summary": "1-sentence diagnosis grounded in the manual or general reasoning",
  "root_cause": "probable root cause or null if uncertain",
  "steps": [
    {{
      "title": "short step title",
      "description": "detailed technician instruction",
      "action_type": "check|measure|replace|restart|calibrate|inspect|clean|test",
      "expected_outcome": "what success looks like, or null",
      "warning": "safety warning if applicable, or null"
    }}
  ]
}}

Rules:
- Generate 4-8 steps ordered from safest/simplest to most invasive.
- Start with inspection/measurement, end with test/restart.
- Reference specific machine type, telemetry values, and manual sections if available.
- Include safety warnings for heat, pressure, electrical, or chemical hazards.
- Output JSON only."""


_ALLOWED_ACTION_TYPES = {
    "check",
    "measure",
    "replace",
    "restart",
    "calibrate",
    "inspect",
    "clean",
    "test",
}


def _parse_diagnosis(raw: str | None) -> Diagnosis | None:
    """Validate an LLM JSON diagnosis, or return None to fall back."""
    if not raw:
        return None
    try:
        diagnosis = Diagnosis.model_validate_json(raw)
    except Exception:  # noqa: BLE001
        return None
    if not diagnosis.summary or not diagnosis.steps:
        return None
    for step in diagnosis.steps:
        if step.action_type not in _ALLOWED_ACTION_TYPES:
            step.action_type = "check"
    return diagnosis


def _format_telemetry(telemetry: dict[str, Any]) -> str:
    if not telemetry:
        return "No telemetry available."
    lines = [f"  {k}: {v}" for k, v in telemetry.items()]
    return "\n".join(lines)


# ---------------------------------------------------------------------------
# Deterministic fallback — realistic, severity-based procedures
# ---------------------------------------------------------------------------


def _fallback_critical(machine_name: str, machine_type: str) -> Diagnosis:
    return Diagnosis(
        summary=f"Critical failure on {machine_name} ({machine_type}) requires immediate intervention.",
        root_cause=None,
        steps=[
            DiagnosticStep(
                title="Emergency power isolation",
                description=f"Engage the lockout/tagout procedure for {machine_name}. Disconnect main power and verify zero-energy state.",
                action_type="check",
                expected_outcome="All energy sources confirmed isolated; LOTO tag applied.",
                warning="Do not proceed without completing lockout/tagout. Risk of electrocution or mechanical injury.",
            ),
            DiagnosticStep(
                title="Visual damage inspection",
                description="Inspect the machine exterior, control panel, and surrounding area for visible damage, leaks, smoke residue, or deformation.",
                action_type="inspect",
                expected_outcome="Damage area identified and photographed for the incident report.",
                warning="Wear appropriate PPE including heat-resistant gloves and safety glasses.",
            ),
            DiagnosticStep(
                title="Measure critical parameters",
                description="Using a calibrated multimeter and thermal camera, measure insulation resistance, coil temperatures, and bearing housing temperatures.",
                action_type="measure",
                expected_outcome="Readings within manufacturer spec or deviation clearly documented.",
                warning="Surfaces may be hot. Allow cooldown or use thermal-rated gloves.",
            ),
            DiagnosticStep(
                title="Inspect internal components",
                description="Open access panels and inspect drive belts, couplings, seals, and wiring harnesses for wear, breakage, or contamination.",
                action_type="inspect",
                expected_outcome="Faulty component identified or internal condition documented.",
                warning=None,
            ),
            DiagnosticStep(
                title="Clean debris and contamination",
                description="Remove any debris, dust buildup, or leaked fluids from the machine interior and cooling vents.",
                action_type="clean",
                expected_outcome="All foreign material removed; airflow paths clear.",
                warning="Use only approved solvents. Ensure adequate ventilation.",
            ),
            DiagnosticStep(
                title="Replace failed components",
                description="Replace any components identified as damaged or out-of-spec. Use OEM-equivalent parts and torque fasteners to specification.",
                action_type="replace",
                expected_outcome="New components installed and torqued to spec.",
                warning=None,
            ),
            DiagnosticStep(
                title="Calibrate sensors and safety interlocks",
                description="Recalibrate temperature, pressure, and vibration sensors. Verify all safety interlocks and emergency stops engage correctly.",
                action_type="calibrate",
                expected_outcome="All sensors reading within tolerance; safety interlocks tested and functional.",
                warning=None,
            ),
            DiagnosticStep(
                title="Controlled restart and verification",
                description="Remove LOTO, restore power, and perform a controlled startup. Monitor telemetry for the first 15 minutes of operation.",
                action_type="restart",
                expected_outcome="Machine operating within normal parameters; no fault codes or abnormal vibration.",
                warning="Stand clear of moving parts during initial startup. Keep emergency stop accessible.",
            ),
        ],
    )


def _fallback_high(machine_name: str, machine_type: str) -> Diagnosis:
    return Diagnosis(
        summary=f"Significant issue detected on {machine_name} ({machine_type}); prompt repair recommended.",
        root_cause=None,
        steps=[
            DiagnosticStep(
                title="Safety check and preparation",
                description=f"Ensure {machine_name} is in a safe state. Engage standby mode and verify no active loads before servicing.",
                action_type="check",
                expected_outcome="Machine in safe standby; no active processes running.",
                warning="Confirm zero active loads before opening any access panels.",
            ),
            DiagnosticStep(
                title="Inspect wear components",
                description="Check belts, bearings, filters, and seals for wear, misalignment, or degradation.",
                action_type="inspect",
                expected_outcome="Worn components identified and tagged for replacement.",
                warning=None,
            ),
            DiagnosticStep(
                title="Measure operating parameters",
                description="Record current draw, vibration levels, operating temperature, and fluid pressures. Compare against baseline values.",
                action_type="measure",
                expected_outcome="Deviations from baseline quantified and logged.",
                warning=None,
            ),
            DiagnosticStep(
                title="Clean and lubricate",
                description="Clean intake filters, cooling fins, and lubricate bearings and slide surfaces per the maintenance schedule.",
                action_type="clean",
                expected_outcome="Filters clean; lubrication points serviced.",
                warning=None,
            ),
            DiagnosticStep(
                title="Replace degraded parts",
                description="Swap out any components that exceed wear limits. Update the maintenance log with part numbers and installation date.",
                action_type="replace",
                expected_outcome="All replaced parts within spec; maintenance log updated.",
                warning=None,
            ),
            DiagnosticStep(
                title="Functional test",
                description="Return the machine to operating mode and run a standard production cycle. Verify output quality and telemetry stability.",
                action_type="test",
                expected_outcome="Production cycle completes normally; telemetry stable within tolerance.",
                warning=None,
            ),
        ],
    )


def _fallback_medium(machine_name: str, machine_type: str) -> Diagnosis:
    return Diagnosis(
        summary=f"Moderate issue on {machine_name} ({machine_type}); scheduled maintenance recommended.",
        root_cause=None,
        steps=[
            DiagnosticStep(
                title="Operational status check",
                description=f"Review {machine_name} control panel for active fault codes, warnings, or parameter deviations.",
                action_type="check",
                expected_outcome="Active fault codes recorded; operating mode confirmed.",
                warning=None,
            ),
            DiagnosticStep(
                title="Inspect suspect subsystem",
                description="Based on the reported symptoms, inspect the most likely subsystem (mechanical drive, hydraulic circuit, or electrical controls).",
                action_type="inspect",
                expected_outcome="Subsystem condition assessed; any anomalies documented.",
                warning=None,
            ),
            DiagnosticStep(
                title="Measure and compare baselines",
                description="Take vibration, temperature, and pressure readings and compare against the machine's baseline profile.",
                action_type="measure",
                expected_outcome="Readings logged; deviations from baseline identified.",
                warning=None,
            ),
            DiagnosticStep(
                title="Clean or adjust",
                description="Clean filters, adjust belt tension, or recalibrate sensors as indicated by the inspection findings.",
                action_type="calibrate",
                expected_outcome="Adjustments made; parameters closer to baseline.",
                warning=None,
            ),
            DiagnosticStep(
                title="Verification test run",
                description="Run the machine under normal load for one cycle and confirm the reported issue is resolved.",
                action_type="test",
                expected_outcome="Issue no longer present; machine performs within spec.",
                warning=None,
            ),
        ],
    )


def _fallback_low(machine_name: str, machine_type: str) -> Diagnosis:
    return Diagnosis(
        summary=f"Minor issue reported on {machine_name} ({machine_type}); routine check advised.",
        root_cause=None,
        steps=[
            DiagnosticStep(
                title="Visual inspection",
                description=f"Perform a walk-around inspection of {machine_name}. Check for unusual noise, vibration, or visible wear.",
                action_type="inspect",
                expected_outcome="No significant issues observed, or minor finding documented.",
                warning=None,
            ),
            DiagnosticStep(
                title="Clean and verify",
                description="Clean external surfaces, air intakes, and sensor lenses. Verify indicator lights and display readings are normal.",
                action_type="clean",
                expected_outcome="Machine exterior clean; all indicators normal.",
                warning=None,
            ),
            DiagnosticStep(
                title="Operational test",
                description="Run the machine through a short test cycle and confirm normal operation.",
                action_type="test",
                expected_outcome="Test cycle completes without fault; output within tolerance.",
                warning=None,
            ),
        ],
    )


_SEVERITY_FALLBACK = {
    "critical": _fallback_critical,
    "high": _fallback_high,
    "medium": _fallback_medium,
    "low": _fallback_low,
}


# ---------------------------------------------------------------------------
# Public entry point
# ---------------------------------------------------------------------------


async def run_diagnostic(
    incident_title: str,
    incident_description: str,
    severity: str,
    machine_name: str,
    machine_type: str,
    manual_text: str = "",
    telemetry: dict[str, Any] | None = None,
) -> Diagnosis:
    """Produce a machine diagnostic plan.

    Uses the configured LLM with the machine manual as context for grounded,
    manual-specific reasoning; falls back to deterministic severity-based steps
    when no provider is configured or the model output fails validation.
    """
    telemetry = telemetry or {}

    if llm_available():
        manual_excerpt = manual_text[:4000] if manual_text else "No manual provided."
        system = _DIAG_SYSTEM.format(manual_excerpt=manual_excerpt)

        user_parts = [
            f"Incident: {incident_title}",
            f"Description: {incident_description}",
            f"Severity: {severity}",
            f"Machine: {machine_name} (type: {machine_type})",
            f"Telemetry:\n{_format_telemetry(telemetry)}",
        ]
        user = "\n".join(user_parts)

        raw = await complete_json(system, user)
        diagnosis = _parse_diagnosis(raw)
        if diagnosis is not None:
            logger.info("Diagnosis from LLM for %s", machine_name)
            return diagnosis
        logger.info("LLM diagnosis unusable for %s, using deterministic fallback", machine_name)

    fallback = _SEVERITY_FALLBACK.get(severity.lower(), _fallback_medium)
    return fallback(machine_name, machine_type)
