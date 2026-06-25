from typing import cast
from uuid import uuid4

from fastapi import APIRouter, HTTPException
from langchain_core.messages import BaseMessage, HumanMessage
from langchain_core.runnables import RunnableConfig
from pydantic import BaseModel, Field

from app.deps import container
from app.graph import build_chat_graph
from app.graph.chat import ChatState

router = APIRouter(prefix="/chat", tags=["chat"])


class ChatRequest(BaseModel):
    message: str = Field(min_length=1, max_length=32_000)
    thread_id: str | None = None


class ChatResponse(BaseModel):
    thread_id: str
    reply: str


def _reply_text(content: str | list) -> str:
    if isinstance(content, str):
        return content
    chunks: list[str] = []
    for block in content:
        if isinstance(block, str):
            chunks.append(block)
        elif isinstance(block, dict) and block.get("type") == "text":
            chunks.append(str(block.get("text", "")))
    return "".join(chunks)


@router.post("", response_model=ChatResponse)
async def chat(req: ChatRequest) -> ChatResponse:
    if container.checkpointer is None:
        raise HTTPException(status_code=503, detail="checkpointer not ready")

    graph = build_chat_graph(container.checkpointer)
    thread_id: str = req.thread_id or str(uuid4())
    config: RunnableConfig = {"configurable": {"thread_id": thread_id}}

    raw = await graph.ainvoke(
        {"messages": [HumanMessage(content=req.message)]},
        config=config,
    )
    result = cast(ChatState, raw)
    messages: list[BaseMessage] = result["messages"]
    if not messages:
        raise HTTPException(status_code=500, detail="graph returned no messages")
    return ChatResponse(thread_id=thread_id, reply=_reply_text(messages[-1].content))
