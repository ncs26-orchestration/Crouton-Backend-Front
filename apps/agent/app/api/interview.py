"""FastAPI endpoint for onboarding-interview turns.

The Go API is the orchestrator. For each user turn in an interview-
kind chat it loads the chat context + the prior overview snapshot
from `projects.overview_json` and posts here. The response shape is
deliberately parallel to /extract: `{overview, questions[], error}`.
"""

from __future__ import annotations

from typing import Any, cast

from fastapi import APIRouter, HTTPException
from pydantic import BaseModel, Field

from app.nodes.interview import InterviewState, run_interview

router = APIRouter(prefix="/interview", tags=["interview"])


class InterviewRequest(BaseModel):
    text: str = Field(default="", max_length=32_000)
    chat_context: str = Field(default="")
    prior_overview: dict[str, Any] | None = None


class InterviewResponse(BaseModel):
    overview: dict[str, Any] | None
    questions: list[dict[str, Any]] = Field(default_factory=list)
    error: str | None = None


@router.post("", response_model=InterviewResponse)
async def interview(req: InterviewRequest) -> InterviewResponse:
    state: InterviewState = {
        "text": req.text,
        "chat_context": req.chat_context,
        "prior_overview": req.prior_overview,
    }
    raw = await run_interview(state)
    result = cast(InterviewState, raw)
    overview = result.get("overview")
    err = result.get("error")
    questions = result.get("questions") or []
    if overview is None and err is None:
        raise HTTPException(status_code=500, detail="interview produced no overview and no error")
    return InterviewResponse(overview=overview, questions=questions, error=err)
