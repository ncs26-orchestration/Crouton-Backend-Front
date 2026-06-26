"""Onboarding-interview node — turns interview turns + attachments
into an Organisation Overview.

Same one-call-per-turn pattern as `extract.py`: the small local model
emits a single envelope `{overview, questions, error}` so the canvas
(here: the overview panel) updates every turn AND clarifying questions
flow into the chat thread without a second round-trip.

Distinct from the workflow extractor in two ways:
  1. Output shape — Organisation Overview, not a Workflow IR.
  2. Question style — strictly OBJECTIVE (factual yes/no / picker /
     count / name) — never subjective ("how do you feel about X?").

Provider dispatch reuses the same Ollama-first / Gemini / Anthropic
fallback chain as the extractor; the local Ollama call dominates the
latency budget so we want the same think-disabled, JSON-format
shortcuts.
"""

from __future__ import annotations

import json
from typing import Any, TypedDict

from app.settings import settings


class InterviewState(TypedDict, total=False):
    text: str
    chat_context: str
    prior_overview: dict[str, Any] | None
    overview: dict[str, Any] | None
    questions: list[dict[str, Any]] | None
    error: str | None


# --- prompt ----------------------------------------------------------

_INSTRUCTIONS = """\
You are AUP's onboarding interviewer. Your job is to build a factual
**Organisation Overview** for a client company through a structured
chat. The integrator (operator) is talking to you on behalf of a
client; their messages, attached documents, and prior answers are
provided below.

You are NOT a workflow designer. Do NOT produce workflows. Build the
overview only.

Output ONE JSON object, with this shape — no prose, no markdown:

  {{
    "overview": {{
      "name":         "<company name>",
      "description":  "<1-2 sentence factual description>",
      "size":         "<headcount band, e.g. 50-200 employees>",
      "sectors":      ["healthcare", "fintech", ...],
      "regions":      ["Algeria", "France", ...],
      "key_systems":  [
        {{ "name": "Odoo 17", "category": "ERP",
           "evidence": "mentioned in turn 3: 'we use Odoo for invoicing'" }}
      ],
      "key_processes": [
        {{ "name": "Expense approval",
           "stakeholders": ["finance", "managers"],
           "evidence": "..." }}
      ],
      "stakeholders": [
        {{ "role": "Finance Director", "name": "Karim",
           "evidence": "..." }}
      ],
      "compliance":   ["GDPR", "ISO27001"],
      "languages":    ["French", "Arabic"]
    }},
    "questions": [
      {{ "id": "q_size", "field": "size",
         "text": "How many employees does the organisation have? (10-50, 50-200, 200-1000, 1000+?)"
      }}
    ]
  }}

Hard rules:

0. PRESERVE PRIOR OVERVIEW. If a "PRIOR OVERVIEW" block is provided
   below, treat it as authoritative facts already established. Your
   default is to KEEP every field as-is and only update what the
   user's latest message clearly changes or adds. Never silently drop
   a field that was previously filled.

1. OBJECTIVE QUESTIONS ONLY. Every question must be answerable with
   a fact — a name, a number, a yes/no, or a pick from a small list.
   Forbidden patterns:
     - "How do you feel about ...?"
     - "What do you think is best for ...?"
     - "Why does X matter to you?"
   Allowed patterns:
     - "How many employees? (10-50, 50-200, 200-1000, 1000+?)"
     - "Do you use Active Directory or LDAP for authentication?"
     - "Which document management system handles signed contracts —
        OpenBee, SharePoint, Google Drive, or another?"
     - "Is the finance team based in the same office as engineering?"
   Phrase questions in ≤ 120 characters. Prefer pickers (3-5 options)
   over open-ended.

2. ONE QUESTION PER UNFILLED OR LOW-CONFIDENCE FIELD. If a field in
   the overview is empty / null / "unknown", emit one question for
   it. If a field is filled but you have low confidence (≤ 0.7) in
   it, emit a yes/no confirmation question instead.

3. NEVER FABRICATE. If the user hasn't told you the company name,
   leave `overview.name` as null and ASK for it. Do not infer it from
   filenames, email addresses in attachments, or surrounding context
   unless it's verbatim stated.

4. EVIDENCE TRACEABILITY. Every entry in `key_systems`,
   `key_processes`, `stakeholders` MUST carry an `evidence` field
   quoting (≤ 120 chars) the message or attachment span that
   established it. This is the audit trail.

5. SUMMARIZE BUT DO NOT EDITORIALIZE. `description` is one or two
   factual sentences ("XYZ is a 200-employee logistics company
   operating in Algeria and Tunisia"). It is NOT a value
   proposition or marketing copy.

6. EMPTY VS UNKNOWN. Use empty arrays [] for "no data yet, may have
   none". Use null for "not yet asked". Keep the distinction —
   it changes which questions you ask.

7. ASK AT MOST 3 QUESTIONS PER TURN. Pick the most foundational
   unfilled fields first (name → size → sectors → key_systems →
   stakeholders → key_processes → compliance). The integrator can
   only answer so many in one reply.

--- PRIOR CHAT CONTEXT ---
{chat_context}

--- LATEST INTEGRATOR MESSAGE ---
{text}

Return the JSON object {{"overview": ..., "questions": ...}} now. Nothing else.
"""


def _strip_code_fences(s: str) -> str:
    t = s.strip()
    if t.startswith("```"):
        nl = t.find("\n")
        t = t[nl + 1 :] if nl != -1 else t[3:]
        if t.rstrip().endswith("```"):
            t = t.rstrip()[:-3]
    return t.strip()


# --- providers -------------------------------------------------------


async def _generate_ollama(prompt: str) -> str:
    import httpx

    url = settings.ollama_base_url.rstrip("/") + "/api/chat"
    payload: dict[str, Any] = {
        "model": settings.extractor_model,
        "messages": [{"role": "user", "content": prompt}],
        "stream": False,
        "think": False,  # qwen3 thinking-mode squashes the answer; off
        "options": {"temperature": 0.1, "num_ctx": 8192},
        "format": "json",
    }
    async with httpx.AsyncClient(timeout=180.0) as client:
        resp = await client.post(url, json=payload)
        resp.raise_for_status()
        data = resp.json()
    msg = data.get("message") or {}
    return (msg.get("content") or "").strip()


async def _generate_gemini(prompt: str) -> str:
    from google import genai
    from google.genai import types

    client = genai.Client(api_key=settings.google_api_key)
    resp = await client.aio.models.generate_content(
        model=settings.extractor_model,
        contents=prompt,
        config=types.GenerateContentConfig(
            response_mime_type="application/json",
            temperature=0.1,
            max_output_tokens=4096,
        ),
    )
    return (resp.text or "").strip()


# --- node -----------------------------------------------------------


async def run_interview(state: InterviewState) -> InterviewState:
    text = (state.get("text") or "").strip()
    chat_context = state.get("chat_context") or ""
    prior_overview = state.get("prior_overview")

    if not text and not prior_overview:
        return {"overview": None, "questions": [], "error": "empty input text"}

    # The prior overview rides INSIDE the chat_context block so the
    # node interface stays a single string. Caller (Go side) is free
    # to assemble it however it wants; we expect the operator-tool
    # path to prepend a "PRIOR OVERVIEW: <json>" line.
    composed_context = chat_context
    if prior_overview:
        composed_context = (
            "PRIOR OVERVIEW (preserve unless the user clearly changes a field):\n"
            + json.dumps(prior_overview, ensure_ascii=False, indent=2)
            + "\n\n"
            + composed_context
        )

    prompt = _INSTRUCTIONS.format(
        chat_context=composed_context.strip() or "(none yet — this is the first turn)",
        text=text or "(no new message — refresh the overview from prior context)",
    )

    provider = (settings.extractor_provider or "ollama").lower()
    try:
        if provider == "ollama":
            raw = await _generate_ollama(prompt)
        elif provider == "gemini":
            if not settings.google_api_key:
                return {"overview": None, "questions": [], "error": "GOOGLE_API_KEY not set"}
            raw = await _generate_gemini(prompt)
        else:
            return {
                "overview": None,
                "questions": [],
                "error": f"unsupported interview provider {provider!r}",
            }
    except Exception as exc:  # noqa: BLE001
        return {"overview": None, "questions": [], "error": f"{provider} call failed: {exc}"}

    if not raw:
        return {"overview": None, "questions": [], "error": f"{provider} returned empty output"}

    try:
        data = json.loads(_strip_code_fences(raw))
    except json.JSONDecodeError as exc:
        return {"overview": None, "questions": [], "error": f"{provider} returned non-JSON: {exc}"}
    if not isinstance(data, dict):
        return {
            "overview": None,
            "questions": [],
            "error": f"{provider} returned a {type(data).__name__}, want object",
        }

    overview = data.get("overview")
    if not isinstance(overview, dict):
        # Models occasionally collapse the wrapper; if the top-level
        # object looks like an overview shape, accept it.
        if "name" in data or "size" in data or "sectors" in data:
            overview = data
        else:
            return {
                "overview": None,
                "questions": [],
                "error": f"{provider} returned no 'overview' field",
            }

    raw_qs = data.get("questions") or []
    questions: list[dict[str, Any]] = []
    if isinstance(raw_qs, list):
        for i, q in enumerate(raw_qs):
            if not isinstance(q, dict):
                continue
            text_val = str(q.get("text") or "").strip()
            if not text_val:
                continue
            questions.append(
                {
                    "id": str(q.get("id") or f"q_{i}"),
                    "field": str(q.get("field") or "") or None,
                    "text": text_val,
                }
            )

    return {"overview": overview, "questions": questions, "error": None}
