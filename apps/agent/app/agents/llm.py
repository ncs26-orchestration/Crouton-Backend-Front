"""Shared LLM access for the intake and department agents.

DeepSeek, Groq, and OpenAI are all OpenAI-compatible, so we use the OpenAI SDK
pointed at the configured base URL, ask for a JSON object, and let the callers
validate it into the typed ``Plan`` / ``Decision`` models. Returns ``None`` when
no provider is configured (or on any error) so callers fall back to their
deterministic path — the system always works offline.

Provider priority: DeepSeek > Groq > OpenAI.
"""

from __future__ import annotations

import logging

from openai import AsyncOpenAI

from app.settings import settings

logger = logging.getLogger(__name__)


def _client_and_model() -> tuple[AsyncOpenAI, str] | None:
    """Pick an OpenAI-compatible provider from settings, or ``None``.

    Every client gets an explicit timeout so a stuck provider fails fast into
    the next fallback, well inside the orchestrator's per-node budget.
    """
    timeout = settings.llm_timeout_seconds
    if settings.deepseek_api_key:
        return (
            AsyncOpenAI(
                api_key=settings.deepseek_api_key,
                base_url=settings.deepseek_base_url,
                timeout=timeout,
            ),
            settings.deepseek_model,
        )
    if settings.groq_api_key:
        return (
            AsyncOpenAI(
                api_key=settings.groq_api_key,
                base_url=settings.groq_base_url,
                timeout=timeout,
            ),
            settings.groq_model,
        )
    if settings.openai_api_key:
        return (
            AsyncOpenAI(api_key=settings.openai_api_key, timeout=timeout),
            settings.openai_model,
        )
    # Fall back to local Ollama. Uses a generous timeout (60s) because the
    # model may need to load into memory on first request. If Ollama is not
    # running, the connection will fail fast (<1s) instead.
    return (
        AsyncOpenAI(
            api_key="ollama",
            base_url=settings.ollama_base_url.rstrip("/") + "/v1",
            timeout=60.0,
        ),
        settings.ollama_model,
    )


def llm_available() -> bool:
    """True when an LLM provider is reachable (remote or local Ollama)."""
    return _client_and_model() is not None


async def complete_json(system: str, user: str, *, max_tokens: int = 2048) -> str | None:
    """Return the model's JSON string response, or ``None`` on any failure.

    Uses JSON-object response format and a low temperature for stable, parseable
    output. Never raises — a failure means the caller uses its deterministic
    fallback. A configured-but-failing provider logs at ERROR so a
    misconfiguration is visible rather than silently masked by the fallback.
    """
    picked = _client_and_model()
    if picked is None:
        # No provider configured — expected offline path, not an error.
        logger.debug("No LLM provider configured; caller will use deterministic path")
        return None
    client, model = picked
    try:
        resp = await client.chat.completions.create(
            model=model,
            messages=[
                {"role": "system", "content": system},
                {"role": "user", "content": user},
            ],
            response_format={"type": "json_object"},
            temperature=0.2,
            max_tokens=max_tokens,
        )
        return resp.choices[0].message.content
    except Exception as exc:  # noqa: BLE001 — any failure falls back to deterministic
        # Provider IS configured but the call failed (bad key/model, network,
        # timeout). Surface it loudly — the operator expects real reasoning.
        logger.error(
            "LLM call failed (model=%s); falling back to deterministic. "
            "Check the provider key/model/connectivity. error=%s",
            model,
            exc,
        )
        return None
