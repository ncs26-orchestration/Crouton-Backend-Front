from typing import TYPE_CHECKING, TypedDict

from langchain_core.messages import AIMessage, BaseMessage, HumanMessage

from app.settings import settings

if TYPE_CHECKING:
    from app.graph.chat import ChatState


class AnthropicMessage(TypedDict):
    role: str
    content: str


def _content_as_text(content: str | list) -> str:
    if isinstance(content, str):
        return content
    parts: list[str] = []
    for block in content:
        if isinstance(block, str):
            parts.append(block)
        elif isinstance(block, dict) and block.get("type") == "text":
            parts.append(str(block.get("text", "")))
    return "".join(parts)


def _to_anthropic(messages: list[BaseMessage]) -> list[AnthropicMessage]:
    out: list[AnthropicMessage] = []
    for m in messages:
        text = _content_as_text(m.content)
        if isinstance(m, HumanMessage):
            out.append(AnthropicMessage(role="user", content=text))
        elif isinstance(m, AIMessage):
            out.append(AnthropicMessage(role="assistant", content=text))
    return out


def _msg_role(msg: BaseMessage) -> str:
    if isinstance(msg, HumanMessage):
        return "user"
    return "model"


async def call_model(state: "ChatState") -> dict[str, list[BaseMessage]]:
    provider = settings.extractor_provider

    if provider == "gemini":
        if not settings.google_api_key:
            return {"messages": [AIMessage(content="(no GOOGLE_API_KEY set — stub reply)")]}
        from google import genai
        from google.genai import types

        client = genai.Client(api_key=settings.google_api_key)
        contents = [
            {"role": _msg_role(msg), "parts": [{"text": _content_as_text(msg.content)}]}
            for msg in state["messages"]
        ]
        resp = await client.aio.models.generate_content(
            model=settings.extractor_model,
            contents=contents,  # type: ignore[arg-type]
            config=types.GenerateContentConfig(
                temperature=0.2,
                max_output_tokens=1024,
            ),
        )
        text = (resp.text or "").strip()
        return {"messages": [AIMessage(content=text)]}

    if provider == "ollama":
        import httpx

        url = settings.ollama_base_url.rstrip("/") + "/api/chat"
        payload = {
            "model": settings.extractor_model,
            "messages": [
                {
                    "role": "user" if isinstance(m, HumanMessage) else "assistant",
                    "content": _content_as_text(m.content),
                }
                for m in state["messages"]
            ],
            "stream": False,
            "think": False,
            "options": {"temperature": 0.2, "num_ctx": 8192},
        }
        try:
            async with httpx.AsyncClient(timeout=120.0) as client:
                resp = await client.post(url, json=payload)
                resp.raise_for_status()
                data = resp.json()
                text = data.get("message", {}).get("content", "")
                return {"messages": [AIMessage(content=text)]}
        except httpx.ConnectError:
            return {
                "messages": [
                    AIMessage(
                        content=(
                            "(Ollama not running - set GROQ_API_KEY or"
                            " GOOGLE_API_KEY or ANTHROPIC_API_KEY in .env)"
                        )
                    )
                ]
            }

    if provider == "anthropic":
        if not settings.anthropic_api_key:
            return {"messages": [AIMessage(content="(no ANTHROPIC_API_KEY set — stub reply)")]}
        from anthropic import AsyncAnthropic

        client = AsyncAnthropic(api_key=settings.anthropic_api_key)
        resp = await client.messages.create(
            model=settings.default_model,
            max_tokens=1024,
            messages=[dict(m) for m in _to_anthropic(state["messages"])],
        )
        first = resp.content[0] if resp.content else None
        text = first.text if first is not None and first.type == "text" else ""
        return {"messages": [AIMessage(content=text)]}

    if provider == "groq":
        if not settings.groq_api_key:
            return {"messages": [AIMessage(content="(no GROQ_API_KEY set — stub reply)")]}
        import httpx

        url = "https://api.groq.com/openai/v1/chat/completions"
        payload = {
            "model": settings.extractor_model,
            "messages": [
                {
                    "role": "user" if isinstance(m, HumanMessage) else "assistant",
                    "content": _content_as_text(m.content),
                }
                for m in state["messages"]
            ],
            "temperature": 0.2,
            "max_tokens": 1024,
        }
        headers = {
            "Authorization": f"Bearer {settings.groq_api_key}",
            "Content-Type": "application/json",
        }
        async with httpx.AsyncClient(timeout=120.0) as client:
            resp = await client.post(url, json=payload, headers=headers)
            resp.raise_for_status()
            data = resp.json()
            text = data.get("choices", [{}])[0].get("message", {}).get("content", "")
            return {"messages": [AIMessage(content=text)]}

    return {"messages": [AIMessage(content=f"(unknown provider: {provider})")]}
