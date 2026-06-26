import { describe, expect, it } from "vitest";

import { requestToFlow, NODE_WIDTH, X_GAP } from "./request-to-flow";
import { nodeStatusToken } from "./request-format";
import type { NodeStatus, WorkflowEdgeData, WorkflowNodeData } from "./types";

function node(id: string, status: NodeStatus = "pending"): WorkflowNodeData {
  return {
    id,
    key: id,
    name: id,
    agent_type: id,
    department: "Dept",
    status,
    description: "",
    progress_percent: 0,
    status_text: "",
    started_at: null,
    completed_at: null,
  };
}

function edge(id: string, from: string, to: string): WorkflowEdgeData {
  return { id, source_node_id: from, target_node_id: to, edge_type: "sequence" };
}

describe("requestToFlow", () => {
  it("returns empty for empty input", () => {
    expect(requestToFlow([], [])).toEqual({ nodes: [], edges: [] });
  });

  it("ranks nodes into columns by depth", () => {
    // a -> b -> c
    const { nodes } = requestToFlow(
      [node("a"), node("b"), node("c")],
      [edge("e1", "a", "b"), edge("e2", "b", "c")],
    );
    const posOf = (id: string) => nodes.find((n) => n.id === id)!.position;
    expect(posOf("a").x).toBe(0);
    expect(posOf("b").x).toBe(NODE_WIDTH + X_GAP);
    expect(posOf("c").x).toBe(2 * (NODE_WIDTH + X_GAP));
  });

  it("places parallel branches in the same column at different rows", () => {
    // a -> b, a -> c  (b and c are parallel, same rank)
    const { nodes } = requestToFlow(
      [node("a"), node("b"), node("c")],
      [edge("e1", "a", "b"), edge("e2", "a", "c")],
    );
    const b = nodes.find((n) => n.id === "b")!.position;
    const c = nodes.find((n) => n.id === "c")!.position;
    expect(b.x).toBe(c.x); // same column
    expect(b.y).not.toBe(c.y); // different rows
  });

  it("colors an edge by its source node status", () => {
    const { edges } = requestToFlow(
      [node("a", "completed"), node("b")],
      [edge("e1", "a", "b")],
    );
    expect((edges[0]!.style as { stroke: string }).stroke).toBe(nodeStatusToken("completed"));
    const { edges: e2 } = requestToFlow(
      [node("a", "in_progress"), node("b")],
      [edge("e1", "a", "b")],
    );
    expect(e2[0]!.animated).toBe(true);
  });

  it("does not stack cyclic nodes at the origin", () => {
    // x <-> y is a cycle; neither reaches in-degree 0
    const { nodes } = requestToFlow(
      [node("x"), node("y")],
      [edge("e1", "x", "y"), edge("e2", "y", "x")],
    );
    const x = nodes.find((n) => n.id === "x")!.position;
    const y = nodes.find((n) => n.id === "y")!.position;
    // Both placed, and not on top of each other at {0,0}.
    expect(x).not.toEqual(y);
    expect(nodes.every((n) => n.position.y >= 0)).toBe(true);
  });
});
