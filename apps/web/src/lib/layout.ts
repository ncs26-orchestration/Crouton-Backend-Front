// dagre auto-layout. We keep node sizes fixed per node kind so the
// graph stays compact and predictable, and we always lay out
// left-to-right — workflow reading direction.

import dagre from "dagre";
import type { Edge, Node } from "@xyflow/react";

export type LayoutNodeData = {
  width: number;
  height: number;
};

export function layoutGraph<N extends Node<LayoutNodeData>>(
  nodes: N[],
  edges: Edge[],
): N[] {
  const g = new dagre.graphlib.Graph();
  g.setDefaultEdgeLabel(() => ({}));
  // Wider ranksep gives edge labels (condition expressions) clear
  // breathing room between nodes so the chip doesn't get crossed by
  // the line. nodesep controls siblings, not edge length.
  g.setGraph({ rankdir: "LR", nodesep: 48, ranksep: 110, marginx: 40, marginy: 40 });

  for (const n of nodes) {
    g.setNode(n.id, { width: n.data.width, height: n.data.height });
  }
  for (const e of edges) {
    g.setEdge(e.source, e.target);
  }
  dagre.layout(g);

  return nodes.map((n) => {
    const p = g.node(n.id);
    return {
      ...n,
      // dagre returns centre coords; React Flow expects top-left.
      position: { x: p.x - n.data.width / 2, y: p.y - n.data.height / 2 },
    };
  });
}
