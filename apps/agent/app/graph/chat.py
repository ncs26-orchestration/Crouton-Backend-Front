from typing import Annotated, TypedDict

from langchain_core.messages import BaseMessage
from langgraph.checkpoint.postgres.aio import AsyncPostgresSaver
from langgraph.graph import END, START, StateGraph
from langgraph.graph.message import add_messages
from langgraph.graph.state import CompiledStateGraph

from app.nodes.llm import call_model


class ChatState(TypedDict):
    messages: Annotated[list[BaseMessage], add_messages]


def build_chat_graph(checkpointer: AsyncPostgresSaver) -> CompiledStateGraph:
    graph: StateGraph = StateGraph(ChatState)
    graph.add_node("model", call_model)
    graph.add_edge(START, "model")
    graph.add_edge("model", END)
    return graph.compile(checkpointer=checkpointer)
