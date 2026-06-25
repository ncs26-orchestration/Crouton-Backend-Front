"""FastAPI endpoint for text -> IR extraction.

The Go API is the intended orchestrator: it reads the tenant's IS
Registry from Postgres and forwards it here together with the text.
Callers can also hit this endpoint directly for testing.
"""

from __future__ import annotations

from typing import Any, cast

from fastapi import APIRouter, HTTPException
from pydantic import BaseModel, Field

from app.graph.workflow import build_workflow_graph
from app.nodes.extract import ExtractionState

router = APIRouter(prefix="/extract", tags=["extract"])


class ExtractRequest(BaseModel):
    text: str = Field(min_length=1, max_length=32_000)
    # is_registry is legacy — kept for backwards-compat with older
    # clients. The operator-tool repositioning grounds the extractor
    # in `chat_context` instead: prior messages + attachment text
    # flow through as one big context block.
    is_registry: dict[str, Any] = Field(default_factory=dict)
    chat_context: str = Field(default="")


class ExtractResponse(BaseModel):
    ir: dict[str, Any] | None
    # Clarifying questions produced for low-confidence elements, in
    # the order the extractor emitted them. Empty list means no
    # ambiguity remains; the UI may enable the Approve action.
    questions: list[dict[str, Any]] = Field(default_factory=list)
    error: str | None = None


@router.post("", response_model=ExtractResponse)
async def extract(req: ExtractRequest) -> ExtractResponse:
    graph = build_workflow_graph()
    # Merge chat_context in front of the user's text so the prompt's
    # "--- PROCESS DESCRIPTION ---" section carries both the latest
    # message and any prior conversation / attachments. Keeping them
    # in one field means the extractor node doesn't need to learn
    # about the new shape.
    composed_text = req.text
    if req.chat_context.strip():
        composed_text = (
            "--- PRIOR CHAT CONTEXT ---\n"
            + req.chat_context.strip()
            + "\n\n--- NEW USER MESSAGE ---\n"
            + req.text
        )
    raw = await graph.ainvoke(
        ExtractionState(
            text=composed_text,
            is_registry=req.is_registry,
            ir=None,
            error=None,
        ),
    )
    result = cast(ExtractionState, raw)
    ir = result.get("ir")
    err = result.get("error")
    questions = result.get("questions") or []
    if ir is None and err is None:
        raise HTTPException(status_code=500, detail="graph produced no IR and no error")
    return ExtractResponse(ir=ir, questions=questions, error=err)
