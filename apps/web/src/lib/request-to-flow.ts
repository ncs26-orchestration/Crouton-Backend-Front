/**
 * request-to-flow.ts — Pure function mapping a request graph (nodes + edges)
 * into React Flow nodes and edges with an auto-layout.
 *
 * Uses a simple ranked layout algorithm: nodes are placed in columns based
 * on their topological depth. Parallel branches sit side by side in the
 * same rank. No external layout library needed for this graph size (~10 nodes).
 */

import type { Node, Edge } from "@xyflow/react";
import type { WorkflowNodeData, WorkflowEdgeData, NodeAssignment } from "./types";
import { nodeStatusToken } from "./request-format";

export const NODE_WIDTH = 210;
export const NODE_HEIGHT = 76;
// Tighter horizontal gap keeps the (inherently wide) pipeline compact so
// fitView can show it at a larger, more readable zoom. A roomier vertical gap
// gives the parallel branches clear separation.
export const X_GAP = 56;
export const Y_GAP = 48;

export interface FlowResult {
  nodes: Node[];
  edges: Edge[];
}

export function requestToFlow(
  workflowNodes: WorkflowNodeData[],
  workflowEdges: WorkflowEdgeData[],
  assignments: NodeAssignment[] = [],
): FlowResult {
  if (workflowNodes.length === 0) {
    return { nodes: [], edges: [] };
  }

  // Group assignee names per node so the card can show their avatars.
  const assigneesByNode = new Map<string, string[]>();
  for (const a of assignments) {
    const list = assigneesByNode.get(a.node_id) ?? [];
    list.push(a.user_name || a.user_email);
    assigneesByNode.set(a.node_id, list);
  }

  // Build adjacency and in-degree for topological ranking.
  const children = new Map<string, string[]>();
  const inDegree = new Map<string, number>();

  for (const n of workflowNodes) {
    children.set(n.id, []);
    inDegree.set(n.id, 0);
  }
  for (const e of workflowEdges) {
    children.get(e.source_node_id)?.push(e.target_node_id);
    inDegree.set(e.target_node_id, (inDegree.get(e.target_node_id) ?? 0) + 1);
  }

  // Topological sort into ranks (BFS by level).
  const ranks: string[][] = [];
  let queue = workflowNodes.filter((n) => (inDegree.get(n.id) ?? 0) === 0).map((n) => n.id);

  while (queue.length > 0) {
    ranks.push([...queue]);
    const next: string[] = [];
    for (const id of queue) {
      for (const child of children.get(id) ?? []) {
        const deg = (inDegree.get(child) ?? 1) - 1;
        inDegree.set(child, deg);
        if (deg === 0) next.push(child);
      }
    }
    queue = next;
  }

  // Cycle guard: nodes inside a cycle never reach in-degree 0, so the BFS
  // above never ranks them. Drop them into one trailing rank instead of
  // letting them all stack at the origin. The default plan is a DAG, so
  // this only triggers on a malformed plan.
  const ranked = new Set(ranks.flat());
  const unranked = workflowNodes.filter((n) => !ranked.has(n.id)).map((n) => n.id);
  if (unranked.length > 0) {
    ranks.push(unranked);
  }

  // Position nodes. Each rank is a column; nodes in the same rank
  // are stacked vertically, centered around the middle.
  const nodeById = new Map(workflowNodes.map((n) => [n.id, n]));
  const positions = new Map<string, { x: number; y: number }>();

  const maxRankSize = Math.max(...ranks.map((r) => r.length));
  const totalHeight = maxRankSize * (NODE_HEIGHT + Y_GAP) - Y_GAP;

  for (let col = 0; col < ranks.length; col++) {
    const rank = ranks[col]!;
    const rankHeight = rank.length * (NODE_HEIGHT + Y_GAP) - Y_GAP;
    const yOffset = (totalHeight - rankHeight) / 2;

    for (let row = 0; row < rank.length; row++) {
      positions.set(rank[row]!, {
        x: col * (NODE_WIDTH + X_GAP),
        y: yOffset + row * (NODE_HEIGHT + Y_GAP),
      });
    }
  }

  const flowNodes: Node[] = workflowNodes.map((n) => {
    const pos = positions.get(n.id) ?? { x: 0, y: 0 };
    return {
      id: n.id,
      type: "department",
      position: pos,
      data: { ...n, assignees: assigneesByNode.get(n.id) ?? [] },
      style: { width: NODE_WIDTH, height: NODE_HEIGHT },
    };
  });

  const flowEdges: Edge[] = workflowEdges.map((e) => {
    const sourceNode = nodeById.get(e.source_node_id);
    const color = sourceNode ? nodeStatusToken(sourceNode.status) : "var(--color-fg-subtle)";
    return {
      id: e.id,
      source: e.source_node_id,
      target: e.target_node_id,
      type: "smoothstep",
      animated: sourceNode?.status === "in_progress",
      style: { stroke: color, strokeWidth: 2 },
    };
  });

  return { nodes: flowNodes, edges: flowEdges };
}
