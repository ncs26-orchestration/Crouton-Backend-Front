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
    """Pick an OpenAI-compatible provider from settings, or ``None``."""
    if settings.deepseek_api_key:
        return (
            AsyncOpenAI(
                api_key=settings.deepseek_api_key,
                base_url=settings.deepseek_base_url,
            ),
            settings.deepseek_model,
        )
    if settings.groq_api_key:
        return (
            AsyncOpenAI(
                api_key=settings.groq_api_key,
                base_url="https://api.groq.com/openai/v1",
            ),
            "llama-3.3-70b-versatile",
        )
    if settings.openai_api_key:
        return (AsyncOpenAI(api_key=settings.openai_api_key), "gpt-4o-mini")
    return None


def llm_available() -> bool:
    """True when an OpenAI-compatible provider is configured."""
    return _client_and_model() is not None


async def complete_json(system: str, user: str, *, max_tokens: int = 2048) -> str | None:
    """Return the model's JSON string response, or ``None`` on any failure.

    Uses JSON-object response format and a low temperature for stable, parseable
    output. Never raises — a failure means the caller uses its deterministic
    fallback.
    """
    picked = _client_and_model()
    if picked is None:
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
        logger.warning("LLM call failed, falling back to deterministic: %s", exc)
        return None
