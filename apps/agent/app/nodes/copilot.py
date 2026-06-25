"""Copilot — the Ask / Clarify LLM backend for the workflow sidebar.

Both modes share the same LLM surface (Gemini 2.5 Flash Lite via
`google.genai`) and prompt-building utilities, with two distinct
tasks:

  * **Ask**  — grounded retrieval over the current IR, IS Registry
    and the extractor's original source text. Returns a streamed
    natural-language answer plus an `evidence` list the UI renders
    as citation chips.

  * **Clarify** — given one low-confidence element (task, binding,
    gateway, condition), propose 2–4 concrete fixes expressed as
    JSON-Patch operations (RFC 6902). The UI renders each patch as
    a quick-reply chip; clicking one round-trips to `/copilot/apply`
    which validates and persists.

Both return plain JSON — no streaming in v0.1. Streaming is a
Round 4.5 polish, wrapping the existing handler in a generator.
"""

from __future__ import annotations

import json
from pathlib import Path
from typing import Any

from app.settings import settings

_SCHEMA_PATH = Path(__file__).resolve().parent.parent / "data" / "workflow_ir.schema.json"


def _load_schema() -> dict[str, Any]:
    with open(_SCHEMA_PATH, "r", encoding="utf-8") as f:
        return json.load(f)


def _render_is_block(is_registry: dict[str, Any] | None) -> str:
    """Compact IS summary, identical shape to the one the extractor
    uses — so the Copilot references the same identifier vocabulary
    the IR was built from."""

    if not is_registry:
        return "  (no IS registry provided)"
    users = is_registry.get("users") or []
    groups = is_registry.get("groups") or []
    systems = is_registry.get("systems") or []
    parts: list[str] = []
    if users:
        parts.append("  USERS:")
        for u in users:
            parts.append(f"    - {u['id']} ({u.get('name', u['id'])})")
    if groups:
        parts.append("  GROUPS:")
        for g in groups:
            parts.append(f"    - {g['id']} ({g.get('name', g['id'])})")
    if systems:
        parts.append("  SYSTEMS:")
        for s in systems:
            parts.append(
                f"    - {s['id']} kind={s.get('kind')} capabilities={s.get('capabilities', [])}"
            )
    return "\n".join(parts) or "  (empty)"


async def _provider_json(prompt: str) -> str:
    """Route to the configured provider in JSON mode. Returns raw text.

    Copilot doesn't enforce a schema like the extractor does (the
    response shape is small and tolerant), so we use Ollama's loose
    "json" format which is a bit faster on small local models.
    """

    provider = (settings.extractor_provider or "ollama").lower()

    if provider == "ollama":
        import httpx

        url = settings.ollama_base_url.rstrip("/") + "/api/chat"
        payload = {
            "model": settings.extractor_model,
            "messages": [{"role": "user", "content": prompt}],
            "stream": False,
            # qwen3 thinking mode burns tokens on chain-of-thought; disable.
            "think": False,
            "options": {"temperature": 0.2, "num_ctx": 8192},
            "format": "json",
        }
        async with httpx.AsyncClient(timeout=120.0) as client:
            resp = await client.post(url, json=payload)
            resp.raise_for_status()
            data = resp.json()
        msg = data.get("message") or {}
        return (msg.get("content") or "").strip()

    if provider == "gemini":
        from google import genai
        from google.genai import types

        client = genai.Client(api_key=settings.google_api_key)
        resp = await client.aio.models.generate_content(
            model=settings.extractor_model,
            contents=prompt,
            config=types.GenerateContentConfig(
                response_mime_type="application/json",
                temperature=0.2,
                max_output_tokens=2048,
            ),
        )
        return (resp.text or "").strip()

    if provider == "anthropic":
        from anthropic import AsyncAnthropic

        client = AsyncAnthropic(api_key=settings.anthropic_api_key)
        resp = await client.messages.create(
            model=settings.extractor_model,
            max_tokens=2048,
            messages=[{"role": "user", "content": prompt}],
        )
        for block in resp.content:
            if getattr(block, "type", None) == "text":
                return (getattr(block, "text", "") or "").strip()
        return ""

    if provider == "groq":
        import httpx

        url = "https://api.groq.com/openai/v1/chat/completions"
        payload = {
            "model": settings.extractor_model,
            "messages": [{"role": "user", "content": prompt}],
            "temperature": 0.2,
            "max_tokens": 2048,
            "response_format": {"type": "json_object"},
        }
        headers = {
            "Authorization": f"Bearer {settings.groq_api_key}",
            "Content-Type": "application/json",
        }
        async with httpx.AsyncClient(timeout=120.0) as client:
            resp = await client.post(url, json=payload, headers=headers)
            resp.raise_for_status()
            data = resp.json()
            return (data.get("choices", [{}])[0].get("message", {}).get("content") or "").strip()

    raise RuntimeError(f"unknown extractor_provider {provider!r}")


def _provider_precheck() -> str | None:
    """Return an error string when the configured provider is missing
    its required credential, or None if ready. Ollama has no
    credential — we let the network call surface connectivity issues
    rather than trying to probe the daemon here.
    """
    p = (settings.extractor_provider or "ollama").lower()
    if p == "ollama":
        return None
    if p == "gemini" and not settings.google_api_key:
        return "GOOGLE_API_KEY not set"
    if p == "anthropic" and not settings.anthropic_api_key:
        return "ANTHROPIC_API_KEY not set"
    if p == "groq" and not settings.groq_api_key:
        return "GROQ_API_KEY not set"
    return None


def _strip_code_fences(s: str) -> str:
    t = s.strip()
    if t.startswith("```"):
        nl = t.find("\n")
        t = t[nl + 1 :] if nl != -1 else t[3:]
        if t.rstrip().endswith("```"):
            t = t.rstrip()[:-3]
    return t.strip()


# --- Ask mode ---------------------------------------------------------


_ASK_INSTRUCTIONS = """\
You are AUP's Copilot. Answer the user's question about the workflow
they just generated. The WORKFLOW and INFORMATION SYSTEM sections
below are the only facts you are allowed to cite — do not invent
users, systems, or tasks that aren't listed.

Return ONE JSON object with this shape:

  {{
    "answer": "<natural language response, 1–3 sentences>",
    "evidence": [
      {{"ir_ref": "/tasks/archive", "quote": "archiver la note dans OpenBee"}},
      ...
    ]
  }}

Rules:
- `evidence` entries must point at real IR elements via JSON-Pointer
  (/tasks/<id>, /actors/<id>, /flows/<id>, /gateways/<id>, etc.).
- When the workflow element carries an `evidence` field from
  extraction, prefer quoting it verbatim in your `quote`.
- If you genuinely cannot answer from the data, set answer to an
  honest "I don't know" and return an empty evidence array — never
  fabricate.

--- WORKFLOW (ProcessIR) ---
{ir_block}

--- INFORMATION SYSTEM ---
{is_block}

--- USER QUESTION ---
{question}

Return the JSON object now. Nothing else.
"""


async def copilot_ask(
    ir: dict[str, Any],
    is_registry: dict[str, Any] | None,
    question: str,
) -> dict[str, Any]:
    """Run the Ask prompt. Returns a dict with `answer` and `evidence`."""

    err = _provider_precheck()
    if err:
        return {"answer": "", "error": err}
    if not question.strip():
        return {"answer": "", "error": "empty question"}

    prompt = _ASK_INSTRUCTIONS.format(
        ir_block=json.dumps(ir, indent=2, ensure_ascii=False),
        is_block=_render_is_block(is_registry),
        question=question.strip(),
    )
    try:
        raw = await _provider_json(prompt)
    except Exception as exc:  # noqa: BLE001 — surface as error to UI
        return {"answer": "", "error": f"{settings.extractor_provider} call failed: {exc}"}

    try:
        data = json.loads(_strip_code_fences(raw))
    except json.JSONDecodeError as exc:
        return {"answer": "", "error": f"{settings.extractor_provider} returned non-JSON: {exc}"}
    if not isinstance(data, dict):
        return {"answer": "", "error": f"{settings.extractor_provider} returned non-object"}
    # Normalize shape so the UI always sees these keys.
    data.setdefault("answer", "")
    data.setdefault("evidence", [])
    return data


# --- Clarify mode -----------------------------------------------------


_CLARIFY_INSTRUCTIONS = """\
You are AUP's Copilot. The user flagged an element in the workflow
as low-confidence. Propose 2–4 alternative interpretations as
JSON-Patch operations (RFC 6902) that would resolve the ambiguity.

Each suggestion is one JSON-Patch array, applied atomically. Keep
the scope tight — one property or one sibling object, not a whole
rewrite.

Return ONE JSON object with this shape:

  {{
    "suggestions": [
      {{
        "label": "short chip text (1–5 words)",
        "rationale": "why this might be the right reading",
        "patch": [
          {{"op": "replace", "path": "/tasks/0/binding/system_ref", "value": "openbee"}}
        ]
      }},
      ...
    ]
  }}

Rules:
- JSON-Patch `path` must be an array index or dict key path that
  exists in the IR you were given. Use indexes, not ids
  (e.g. "/tasks/2/binding/capability", not "/tasks/archive/...").
- `op` is "replace", "add", or "remove".
- Operator choice (strict — getting this wrong is the #1 cause of
  failed patches):
    * Use `replace` ONLY when the target path currently has a
      non-null value. If the field is missing or you're unsure,
      use `add` instead (which also overwrites).
    * To CLEAR a field, use `remove` and omit `value`. Never emit
      `replace` with `value: null`.
    * To CREATE a new sibling (a new binding on a task that has
      none, a new flow), use `add`.
- `value` must respect the target field's schema (binding ids must
  come from the IS; conditions must be valid juel expressions).
- Ground every suggestion in the available IS — if the IS lacks the
  needed entity, emit a suggestion whose `rationale` says "needs a
  new system declaration" and set `patch` to an empty array.

--- FULL WORKFLOW IR ---
{ir_block}

--- INFORMATION SYSTEM ---
{is_block}

--- AMBIGUOUS ELEMENT ---
kind:        {kind}
id:          {element_id}
current:     {current_json}
evidence:    {evidence!r}
confidence:  {confidence}

Return the JSON object with suggestions now.
"""


async def copilot_clarify(
    ir: dict[str, Any],
    is_registry: dict[str, Any] | None,
    kind: str,
    element_id: str,
    current: dict[str, Any] | None,
    evidence: str | None,
    confidence: float | None,
) -> dict[str, Any]:
    err = _provider_precheck()
    if err:
        return {"suggestions": [], "error": err}

    prompt = _CLARIFY_INSTRUCTIONS.format(
        ir_block=json.dumps(ir, indent=2, ensure_ascii=False),
        is_block=_render_is_block(is_registry),
        kind=kind,
        element_id=element_id,
        current_json=json.dumps(current or {}, ensure_ascii=False),
        evidence=evidence or "",
        confidence=confidence if confidence is not None else "unknown",
    )
    try:
        raw = await _provider_json(prompt)
    except Exception as exc:  # noqa: BLE001
        return {"suggestions": [], "error": f"{settings.extractor_provider} call failed: {exc}"}

    try:
        data = json.loads(_strip_code_fences(raw))
    except json.JSONDecodeError as exc:
        return {
            "suggestions": [],
            "error": f"{settings.extractor_provider} returned non-JSON: {exc}",
        }
    if not isinstance(data, dict):
        return {"suggestions": [], "error": f"{settings.extractor_provider} returned non-object"}
    data.setdefault("suggestions", [])
    return data
