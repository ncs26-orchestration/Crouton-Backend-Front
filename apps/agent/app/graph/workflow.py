"""Workflow extraction graph.

v0.1 is intentionally a single-node graph — just the LLM call. Future
versions (Slice 3 in the plan) layer:

  normalize -> extract_actors / extract_tasks / extract_rules
             -> assemble_ir -> resolve_bindings -> detect_gaps

The graph is the right abstraction now (not "just call the node")
because it is checkpointable and because the layering above slots in
without rewriting the endpoint.
"""

from __future__ import annotations

from langgraph.graph import END, START, StateGraph
from langgraph.graph.state import CompiledStateGraph

from app.nodes.extract import ExtractionState, extract_ir


def build_workflow_graph() -> CompiledStateGraph:
    graph: StateGraph = StateGraph(ExtractionState)
    graph.add_node("extract_ir", extract_ir)
    graph.add_edge(START, "extract_ir")
    graph.add_edge("extract_ir", END)
    return graph.compile()
