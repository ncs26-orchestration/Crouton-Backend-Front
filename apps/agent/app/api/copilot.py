"""FastAPI endpoints for the Copilot sidebar.

Three endpoints:
  * POST /copilot/ask      — grounded Q&A over the workflow
  * POST /copilot/clarify  — propose JSON-Patch fixes for one
                             low-confidence element
  * POST /copilot/apply    — apply a patch to the IR in-place and
                             return the result. The Go API is the
                             preferred caller; it re-validates the
                             output against the IR schema + cross-refs
                             before writing to its own DB.

The `apply` endpoint doesn't touch the LLM — it's a pure
JSON-Patch transform. We keep it here (not in Go) so the full
"chat → patch → IR" pipeline lives in one place and can be swapped
for a constrained-decoding LLM later without moving endpoints.
"""

from __future__ import annotations

from typing import Any

import jsonpatch
from fastapi import APIRouter
from pydantic import BaseModel, Field

from app.nodes.copilot import copilot_ask, copilot_clarify

router = APIRouter(prefix="/copilot", tags=["copilot"])


class AskRequest(BaseModel):
    ir: dict[str, Any]
    is_registry: dict[str, Any] = Field(default_factory=dict)
    question: str = Field(min_length=1, max_length=2000)


class AskResponse(BaseModel):
    answer: str
    evidence: list[dict[str, Any]] = Field(default_factory=list)
    error: str | None = None


@router.post("/ask", response_model=AskResponse)
async def ask(req: AskRequest) -> AskResponse:
    result = await copilot_ask(req.ir, req.is_registry, req.question)
    return AskResponse(
        answer=result.get("answer", ""),
        evidence=result.get("evidence", []),
        error=result.get("error"),
    )


class ClarifyRequest(BaseModel):
    ir: dict[str, Any]
    is_registry: dict[str, Any] = Field(default_factory=dict)
    kind: str  # "task" | "actor" | "gateway" | "condition"
    element_id: str
    current: dict[str, Any] | None = None
    evidence: str | None = None
    confidence: float | None = None


class Suggestion(BaseModel):
    label: str
    rationale: str | None = None
    patch: list[dict[str, Any]] = Field(default_factory=list)


class ClarifyResponse(BaseModel):
    suggestions: list[Suggestion] = Field(default_factory=list)
    error: str | None = None


@router.post("/clarify", response_model=ClarifyResponse)
async def clarify(req: ClarifyRequest) -> ClarifyResponse:
    result = await copilot_clarify(
        req.ir,
        req.is_registry,
        req.kind,
        req.element_id,
        req.current,
        req.evidence,
        req.confidence,
    )
    raw_suggestions = result.get("suggestions") or []
    # Pydantic handles the per-item shape coercion; we just need to
    # feed well-formed dicts.
    suggestions = [
        Suggestion(
            label=s.get("label", ""),
            rationale=s.get("rationale"),
            patch=s.get("patch", []) or [],
        )
        for s in raw_suggestions
        if isinstance(s, dict)
    ]
    return ClarifyResponse(suggestions=suggestions, error=result.get("error"))


class ApplyRequest(BaseModel):
    ir: dict[str, Any]
    patch: list[dict[str, Any]]


class ApplyResponse(BaseModel):
    ir: dict[str, Any] | None = None
    error: str | None = None
    # Set to True when we silently rewrote the incoming patch because
    # the model picked the wrong operator (e.g. `replace` on a missing
    # path → rewrote to `add`). The UI can surface this as a softer
    # toast than an outright failure.
    normalized: bool = False


def _path_exists(doc: Any, path: str) -> bool:
    """True when the JSON-Pointer path resolves to a value in doc.

    We use python-jsonpatch's Pointer.resolve which raises
    JsonPointerException on missing segments.
    """

    if path in ("", "/"):
        return True
    try:
        jsonpatch.JsonPointer(path).resolve(doc)
        return True
    except jsonpatch.JsonPointerException:
        return False


def _normalize_patch(doc: dict[str, Any], patch: list[dict[str, Any]]) -> tuple[list[dict[str, Any]], bool]:
    """Rewrite the model's most common mistakes into valid ops.

    Rules (applied in order, each a conservative pass):
      1. `replace` on a missing path → `add` (same semantics for a
         field that's about to be set the first time).
      2. `replace` with `value: null` → `remove` (matches our prompt
         contract: null means "clear this field").
      3. `remove` on a missing path → dropped silently (idempotent
         removes are safe).
      4. Any op with an unknown `op` name → dropped with a warning.

    Returns the rewritten patch + a bool indicating whether we
    changed anything. We re-check path existence against the
    *already-rewritten* document as we go so a sequence of changes
    sees the correct state.
    """

    out: list[dict[str, Any]] = []
    changed = False
    # Working copy we mutate as each op is "committed" so the
    # existence check stays accurate for subsequent ops.
    working = jsonpatch.apply_patch(doc, [], in_place=False)

    for op in patch:
        kind = op.get("op")
        path = op.get("path", "")

        if kind not in {"add", "remove", "replace", "move", "copy", "test"}:
            changed = True
            continue

        if kind == "replace":
            if "value" in op and op["value"] is None:
                op = {"op": "remove", "path": path}
                kind = "remove"
                changed = True
            elif not _path_exists(working, path):
                op = {**op, "op": "add"}
                kind = "add"
                changed = True

        if kind == "remove" and not _path_exists(working, path):
            # Already removed — idempotent drop.
            changed = True
            continue

        # Commit this op against the working doc so later ops see
        # the correct state.
        try:
            working = jsonpatch.apply_patch(working, [op], in_place=False)
        except jsonpatch.JsonPatchException:
            # Give up on this op silently; the final apply_patch
            # below will raise with a descriptive error anyway.
            changed = True
            continue

        out.append(op)

    return out, changed


@router.post("/apply", response_model=ApplyResponse)
async def apply(req: ApplyRequest) -> ApplyResponse:
    """Apply a JSON-Patch to the IR and return the result.

    The Go API calls this then re-runs schema + cross-ref validation
    on the returned IR before committing. We keep transformation and
    validation separated because we want the patch step to be a pure
    JSON operation the frontend can mirror for optimistic updates.
    """

    if not req.patch:
        # Empty patch — return input unchanged; useful for "accept as
        # written" flows the UI may use.
        return ApplyResponse(ir=req.ir)

    # Normalize first — handles the model's most common mistakes
    # (replace-on-missing, replace-with-null). The repaired patch is
    # what we actually apply.
    normalized_patch, normalized = _normalize_patch(req.ir, req.patch)

    try:
        patched = jsonpatch.apply_patch(req.ir, normalized_patch, in_place=False)
    except jsonpatch.JsonPatchException as exc:
        return ApplyResponse(error=f"patch failed: {exc}")
    except Exception as exc:  # noqa: BLE001
        return ApplyResponse(error=f"patch failed: {exc}")

    if not isinstance(patched, dict):
        return ApplyResponse(error="patch produced non-object")
    return ApplyResponse(ir=patched, normalized=normalized)
