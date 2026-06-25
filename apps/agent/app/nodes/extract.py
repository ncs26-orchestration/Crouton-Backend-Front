"""Extraction node — turns free-form process description + IS Registry
into a Workflow IR.

Default provider in v0.1 is Google Gemini (Flash tier) with structured
JSON output: fast, cheap, strong at schema-following, Google's
small-model story. Anthropic (Claude Haiku) is retained as a fallback
via AGENT_EXTRACTOR_PROVIDER=anthropic.

The model is always asked for a single JSON object. The Go side
re-validates it with the canonical JSON Schema + cross-reference
check; retries / gap-aware refinement live in later slices.
"""

from __future__ import annotations

import json
from pathlib import Path
from typing import Any, TypedDict

from app.settings import settings

# Schema lives alongside the agent so the container has no runtime
# dependency on the monorepo layout. Kept byte-identical to
# packages/ir/schema.json at commit time.
_SCHEMA_PATH = Path(__file__).resolve().parent.parent / "data" / "workflow_ir.schema.json"


def load_ir_schema() -> dict[str, Any]:
    with _SCHEMA_PATH.open("r", encoding="utf-8") as fh:
        return json.load(fh)


class ExtractionState(TypedDict, total=False):
    text: str
    is_registry: dict[str, Any]
    ir: dict[str, Any] | None
    # Optional clarifying questions. Emitted by the extractor for any
    # element it produces with confidence < 0.8. Empty or missing
    # means "no clarification needed, answer is confident".
    questions: list[dict[str, Any]] | None
    error: str | None


# --- prompting --------------------------------------------------------

_INSTRUCTIONS = """\
You are the extractor for AUP — a workflow authoring tool that sits
above real workflow engines. Turn the business-process description
below into a single Workflow IR that MATCHES THE JSON SCHEMA EXACTLY
and RESPECTS THE INFORMATION SYSTEM below.

Hard rules (violating any of these is a bug):

0. REFINEMENT VS NEW WORKFLOW. If the PROCESS DESCRIPTION section
   below contains a "CURRENT WORKFLOW" block with an existing IR
   JSON, the user is iterating on that workflow. Your default is to
   PRESERVE IT: keep every actor, task, gateway, event, flow, and
   binding AS-IS, and apply only the minimal change the user's
   latest message asks for. Only produce a completely new workflow
   if the user explicitly says things like "start over", "scrap
   this", "new workflow", "redo from scratch". A vague change
   request ("rename X", "add a notification step", "remove the last
   task", "change the approver") is a refinement — do not drop
   unrelated nodes. When the user asks a META question ("why did
   you remove X?", "what does this do?"), return the CURRENT
   WORKFLOW unchanged.

1. Output ONE JSON object with this shape:
     {{
       "ir":        {{ /* the Workflow IR matching the schema below */ }},
       "questions": [ /* optional; see rule 10 */ ]
     }}
   No prose, no markdown fences, no ```json.
2. `version` MUST be "0.1". `metadata.name` MUST be present.
3. Every `task.actor_ref` must match an `actors[].id` you emit.
3b. `actor_ref` is only for USER tasks. SERVICE tasks describe system
    calls and MUST NOT set `actor_ref` at all — the subject is the
    system, not a person. For a service task, use `binding.system_ref`
    + `binding.capability` only. Never put a system id into
    `actor_ref`. Script tasks also have no `actor_ref`.
4. Every `task.form_ref` must match a `forms[].id` you emit.
5. Every `flows[].from` / `flows[].to` must reference a task, gateway,
   or event id you emit.
6. Include exactly one start event and at least one end event, and
   connect them via flows so every task is reachable.
6b. DECISIONS MUST BECOME GATEWAYS, not parallel end-states:
    - Any phrase describing multiple outcomes ("trois issues possibles",
      "deux cas", "si X alors Y sinon Z", "si le dossier est conforme /
      non conforme", "si le recours est fondé / non fondé", "en cas de",
      "either / or") MUST emit an `exclusive` gateway in `gateways[]`
      and connect each outcome as a separate outgoing flow whose
      `condition.expression` captures the branch predicate.
    - Fan-out to tasks happening in parallel (e.g. "simultanément",
      "en parallèle", two actors each doing their own step) MUST use a
      `parallel` gateway.
    - NEVER encode a branch by creating multiple end events. End events
      represent process termination, not decision branches.
7. For bindings:
     - `assignee_user_id` MUST be one of the IS users listed below.
     - `candidate_group_id` MUST be one of the IS groups listed below.
     - `system_ref` MUST be one of the declared systems below.
     - `capability` MUST be a capability the referenced system declares.
   If the prompt mentions a role like "le chef comptable" or "the
   manager", pick the closest IS group or user. If nothing matches,
   leave the corresponding binding field empty — DO NOT invent an id.
8. Conditions go on sequenceFlows under `condition.expression` using
   JUEL syntax (e.g. "${{amount >= 50000}}"). Set `condition.language`
   to "juel".

10. CLARIFYING QUESTIONS. Your response may include a top-level
    `questions` array alongside the IR. Add one question entry for
    every element you emit with `confidence < 0.8`, asking the user
    for the specific information that would raise that confidence
    to ≥ 0.9. Omit `questions` entirely (or return `[]`) if every
    element you emit is confident.

    Shape of each entry:
      {{
        "id":     "q_<short-stable-slug>",     // for UI keying, e.g. "q_task_approve_actor"
        "ir_ref": "/tasks/2/binding/system_ref", // JSON-Pointer into the IR you just emitted
        "text":   "Which system archives the note — OpenBee or SharePoint?"
      }}

    Question-writing rules (important — these are shown to a human):
      - PHRASE AS A QUESTION the user can answer with a short reply,
        not as a TODO ("Who is the approver?" — not "actor unclear").
      - BE SPECIFIC about what you need ("Is the monthly limit 5 days
        or 10 days?" — not "clarify the duration").
      - Prefer yes/no or picker-style wording. 2–3 options beat an
        open-ended question because the user can reply with one word.
      - One question per ambiguous element; don't bundle two
        independent uncertainties into the same question.
      - Keep to ≤ 120 characters. Questions longer than that mean
        you haven't found the real ambiguity yet.

    When the user answers a question in the next message, you should
    re-emit the updated element with higher confidence and REMOVE
    that entry from `questions` (unless a new ambiguity surfaces).

9. CONFIDENCE + EVIDENCE — emit these on EVERY actor, task, gateway,
   binding, and flow.condition you produce. They are optional fields
   but the system uses them to drive clarification, so skipping them
   is wasteful.

   For each element add two fields:
     "confidence": <number 0.0..1.0>
     "evidence":   "<short quoted span from the process description>"

   Calibration:
     0.90+  — the element is directly named in the text
              (e.g. "le comptable" -> actor "comptable")
     0.70-0.90  — strongly implied but not named verbatim
                  (e.g. the text says "Archiver la note" and the
                   IS has OpenBee with document.archive capability)
     0.50-0.70  — reasonable inference from context
     <0.50  — a guess; PREFER leaving the uncertain field empty
              over emitting something confident-looking.

   Put the `evidence` string close to the verbatim phrase that
   convinced you — this is shown in the UI as a traceback, so "amount
   exceeds 50000 DZD" is better than paraphrasing "threshold check".
   Keep it under 120 characters.

   For `binding.confidence`, reflect only the binding decision: how
   sure are you that this verb/object maps to this system.capability?
   For `task.confidence`, reflect the existence of the task itself
   in the source text (a separate signal from whether the binding is
   right).

--- INFORMATION SYSTEM (authoritative list of allowed ids) ---
{is_registry_block}

--- JSON SCHEMA (structural contract — your output must validate against this) ---
{schema_block}

--- PROCESS DESCRIPTION ---
{text}

Return the JSON object {{"ir": ..., "questions": ...}} now. Nothing else.
"""


def _render_is_block(is_registry: dict[str, Any]) -> str:
    users = is_registry.get("users") or []
    groups = is_registry.get("groups") or []
    systems = is_registry.get("systems") or []
    forms = is_registry.get("deployed_forms") or []

    def bullet(label: str, rows: list[str]) -> str:
        if not rows:
            return f"  {label}: (none)"
        joined = "\n    - " + "\n    - ".join(rows)
        return f"  {label}:{joined}"

    user_rows = [
        f"{u['id']}  ({u.get('name', u['id'])}"
        + (f", groups={u.get('group_ids')}" if u.get("group_ids") else "")
        + ")"
        for u in users
    ]
    group_rows = [f"{g['id']}  ({g.get('name', g['id'])})" for g in groups]
    system_rows = [
        f"{s['id']}  kind={s.get('kind')}  capabilities={s.get('capabilities', [])}"
        for s in systems
    ]
    form_rows = [f"{f['form_key']}" for f in forms]

    parts = [
        bullet("USERS (use as assignee_user_id)", user_rows),
        bullet("GROUPS (use as candidate_group_id)", group_rows),
        bullet("SYSTEMS (use as system_ref + pick a declared capability)", system_rows),
        bullet("DEPLOYED FORMS (use as binding.form_key)", form_rows),
    ]
    return "\n".join(parts)


def _strip_code_fences(s: str) -> str:
    """Some models still emit ```json ... ``` despite being told not to.
    Strip the fences safely — no-op if none are present.
    """
    t = s.strip()
    if t.startswith("```"):
        # Drop the opening fence line.
        nl = t.find("\n")
        t = t[nl + 1 :] if nl != -1 else t[3:]
        # Drop trailing fence if any.
        if t.rstrip().endswith("```"):
            t = t.rstrip()[:-3]
    return t.strip()


# --- providers --------------------------------------------------------


async def _generate_gemini(prompt: str) -> str:
    """Call Gemini in JSON mode. Returns raw response text."""
    from google import genai
    from google.genai import types

    client = genai.Client(api_key=settings.google_api_key)
    resp = await client.aio.models.generate_content(
        model=settings.extractor_model,
        contents=prompt,
        config=types.GenerateContentConfig(
            response_mime_type="application/json",
            temperature=0.1,
            max_output_tokens=8192,
        ),
    )
    # The new SDK exposes .text which concatenates all parts.
    return (resp.text or "").strip()


async def _generate_ollama(prompt: str, schema: dict[str, Any]) -> str:
    """Call a local Ollama model with JSON-mode output.

    We use `format: "json"` (loose JSON) rather than the full schema.
    Passing a large schema as `format` triggers schema-constrained
    decoding in Ollama, which on consumer hardware with a 9B model
    can take minutes per request — the model walks the schema
    token-by-token. The Go side validates against the same schema
    afterwards, so there's no safety gain from also constraining here.

    Context window is sized for our prompt + schema-in-the-prompt
    (still printed verbatim as instructions). 8192 is comfortable and
    keeps first-token latency reasonable.
    """
    _ = schema  # kept in the signature for parity with other providers
    import httpx

    url = settings.ollama_base_url.rstrip("/") + "/api/chat"
    payload: dict[str, Any] = {
        "model": settings.extractor_model,
        "messages": [{"role": "user", "content": prompt}],
        "stream": False,
        # Qwen3-family models default to "thinking mode" where they
        # produce a long chain-of-thought in a separate `thinking`
        # field before emitting the answer. For our JSON extraction
        # task that doubles latency and sometimes exhausts the token
        # budget before the real content is produced — disable it.
        "think": False,
        "options": {
            "temperature": 0.1,
            "num_ctx": 8192,
        },
        "format": "json",
    }
    async with httpx.AsyncClient(timeout=180.0) as client:
        resp = await client.post(url, json=payload)
        resp.raise_for_status()
        data = resp.json()
    # Ollama's chat response shape: { message: { role, content }, ... }
    msg = data.get("message") or {}
    return (msg.get("content") or "").strip()


async def _generate_anthropic(prompt: str, schema: dict[str, Any]) -> str:
    """Fallback: Claude Haiku tool-use. Returns JSON text of the
    tool-use input.
    """
    from anthropic import AsyncAnthropic

    client = AsyncAnthropic(api_key=settings.anthropic_api_key)
    resp = await client.messages.create(
        model=settings.extractor_model,
        max_tokens=4096,
        messages=[{"role": "user", "content": prompt}],
        tools=[
            {
                "name": "emit_workflow_ir",
                "description": "Emit a Workflow IR for the described process.",
                "input_schema": schema,
            }
        ],
        tool_choice={"type": "tool", "name": "emit_workflow_ir"},
    )
    for block in resp.content:
        if (
            getattr(block, "type", None) == "tool_use"
            and getattr(block, "name", "") == "emit_workflow_ir"
        ):
            payload = getattr(block, "input", None)
            if isinstance(payload, dict):
                return json.dumps(payload)
    return ""


async def _generate_groq(prompt: str) -> str:
    """Call Groq API for JSON output."""
    import httpx

    url = "https://api.groq.com/openai/v1/chat/completions"
    payload = {
        "model": settings.extractor_model,
        "messages": [{"role": "user", "content": prompt}],
        "temperature": 0.1,
        "max_tokens": 4096,
        "response_format": {"type": "json_object"},
    }
    headers = {
        "Authorization": f"Bearer {settings.groq_api_key}",
        "Content-Type": "application/json",
    }
    async with httpx.AsyncClient(timeout=180.0) as client:
        resp = await client.post(url, json=payload, headers=headers)
        resp.raise_for_status()
        data = resp.json()
        return (data.get("choices", [{}])[0].get("message", {}).get("content") or "").strip()


# --- node -------------------------------------------------------------


async def extract_ir(state: ExtractionState) -> ExtractionState:
    text = state.get("text", "").strip()
    is_registry = state.get("is_registry") or {}

    if not text:
        return {"ir": None, "error": "empty input text"}

    provider = (settings.extractor_provider or "gemini").lower()
    schema = load_ir_schema()
    prompt = _INSTRUCTIONS.format(
        is_registry_block=_render_is_block(is_registry),
        schema_block=json.dumps(schema, indent=2),
        text=text,
    )

    try:
        if provider == "ollama":
            raw = await _generate_ollama(prompt, schema)
        elif provider == "gemini":
            if not settings.google_api_key:
                return {"ir": None, "error": "GOOGLE_API_KEY not set"}
            raw = await _generate_gemini(prompt)
        elif provider == "anthropic":
            if not settings.anthropic_api_key:
                return {"ir": None, "error": "ANTHROPIC_API_KEY not set"}
            raw = await _generate_anthropic(prompt, schema)
        elif provider == "groq":
            if not settings.groq_api_key:
                return {"ir": None, "error": "GROQ_API_KEY not set"}
            raw = await _generate_groq(prompt)
        else:
            return {"ir": None, "error": f"unknown extractor_provider {provider!r}"}
    except Exception as exc:  # noqa: BLE001 — surfaced to the client as a diagnostic
        return {"ir": None, "error": f"{provider} call failed: {exc}"}

    if not raw:
        return {"ir": None, "error": f"{provider} returned empty output"}

    try:
        data = json.loads(_strip_code_fences(raw))
    except json.JSONDecodeError as exc:
        return {"ir": None, "error": f"{provider} returned non-JSON: {exc}"}

    if not isinstance(data, dict):
        return {"ir": None, "error": f"{provider} returned a {type(data).__name__}, want object"}

    # New envelope: {"ir": {...}, "questions": [...]}.
    # Older prompt variants emitted the IR at the top level with no
    # wrapper — accept that shape too so a stale cached response or a
    # model that slips back to the old habit still works. Detection is
    # the presence of "version" + "metadata" at the top level (both
    # are required IR fields).
    if "ir" in data and isinstance(data.get("ir"), dict):
        ir_obj = data["ir"]
        raw_qs = data.get("questions") or []
    elif "version" in data and "metadata" in data:
        ir_obj = data
        raw_qs = []
    else:
        return {
            "ir": None,
            "error": f"{provider} returned unexpected shape: missing 'ir' or top-level IR",
        }

    # Normalize questions: drop anything not a dict, coerce fields to
    # strings, enforce a usable id (fill from position if missing).
    questions: list[dict[str, Any]] = []
    if isinstance(raw_qs, list):
        for i, q in enumerate(raw_qs):
            if not isinstance(q, dict):
                continue
            text = str(q.get("text") or "").strip()
            if not text:
                continue
            questions.append(
                {
                    "id": str(q.get("id") or f"q_{i}"),
                    "ir_ref": str(q.get("ir_ref") or "") or None,
                    "text": text,
                }
            )

    return {"ir": ir_obj, "questions": questions, "error": None}
