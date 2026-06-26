import { useEffect, useMemo, useRef, useState } from "react";
import {
  Background,
  BackgroundVariant,
  Controls,
  MiniMap,
  ReactFlow,
  applyNodeChanges,
  useReactFlow,
  type Edge,
  type NodeChange,
} from "@xyflow/react";

import { TaskNode } from "./nodes/TaskNode";
import { GatewayNode } from "./nodes/GatewayNode";
import { EventNode } from "./nodes/EventNode";
import { ConditionalEdge, type ConditionalEdgeData } from "./edges/ConditionalEdge";
import { irToFlow, type FlowNode } from "../lib/ir-to-flow";
import { TIER_COLOR } from "../lib/confidence";
import type { ISRegistry, Workflow } from "../lib/types";

const nodeTypes = {
  task: TaskNode,
  gateway: GatewayNode,
  event: EventNode,
};

const edgeTypes = {
  conditional: ConditionalEdge,
};

interface Props {
  workflow: Workflow | null;
  isRegistry?: ISRegistry;
  onTaskSelect?: (taskId: string | null) => void;
  selectedTaskId?: string | null;
  onEditFlowExpression?: (flowId: string, expression: string) => void;
  onClearFlowExpression?: (flowId: string) => void;
}

export function IRCanvas({
  workflow,
  isRegistry,
  onTaskSelect,
  selectedTaskId,
  onEditFlowExpression,
  onClearFlowExpression,
}: Props) {
  const seed = useMemo(() => {
    if (!workflow) return { nodes: [] as FlowNode[], edges: [] as Edge<ConditionalEdgeData>[] };
    return irToFlow(workflow, isRegistry, {
      onEditExpression: (id, expr) => onEditFlowExpression?.(id, expr),
      onClearExpression: (id) => onClearFlowExpression?.(id),
    });
  }, [workflow, isRegistry, onEditFlowExpression, onClearFlowExpression]);

  const [nodes, setNodes] = useState<FlowNode[]>(seed.nodes);
  const [edges, setEdges] = useState<Edge<ConditionalEdgeData>[]>(seed.edges);
  const { fitView } = useReactFlow();

  // Track which node ids are new/changed since the last seed so we
  // can briefly highlight them — gives the user a visible "the
  // canvas just changed because of your patch" cue without them
  // having to re-scan the whole graph.
  const prevNodesRef = useRef<Map<string, FlowNode>>(new Map());
  const [recentlyChanged, setRecentlyChanged] = useState<Set<string>>(new Set());

  useEffect(() => {
    const prev = prevNodesRef.current;
    const next = new Set<string>();
    for (const n of seed.nodes) {
      const was = prev.get(n.id);
      if (!was) {
        next.add(n.id);
        continue;
      }
      // Shallow compare what matters visually: task name + bindingState +
      // bindingLabel + confidence. If any of those changed, pulse it.
      if (n.type === "task" && was.type === "task") {
        const a = n.data;
        const b = was.data;
        if (
          a.bindingState !== b.bindingState ||
          a.bindingLabel !== b.bindingLabel ||
          a.task.name !== b.task.name ||
          a.confidence !== b.confidence
        ) {
          next.add(n.id);
        }
      }
    }
    setNodes(seed.nodes);
    setEdges(seed.edges);
    const newMap = new Map<string, FlowNode>();
    for (const n of seed.nodes) newMap.set(n.id, n);
    prevNodesRef.current = newMap;

    if (next.size > 0) {
      setRecentlyChanged(next);
      const t = window.setTimeout(() => setRecentlyChanged(new Set()), 1600);
      return () => window.clearTimeout(t);
    }
  }, [seed]);

  // Smart zoom — when the user clicks a task, gently frame it +
  // its first-hop neighbors so the surrounding context stays
  // visible but the selection gets center stage.
  useEffect(() => {
    if (!selectedTaskId || !workflow) return;
    const neighborIds = new Set<string>([selectedTaskId]);
    for (const f of workflow.flows) {
      if (f.from === selectedTaskId) neighborIds.add(f.to);
      if (f.to === selectedTaskId) neighborIds.add(f.from);
    }
    const frame = seed.nodes.filter((n) => neighborIds.has(n.id));
    if (frame.length > 0) {
      // A small debounce so rapid arrow-navigation doesn't fight.
      const t = window.setTimeout(() => {
        fitView({ nodes: frame, padding: 0.35, duration: 420 });
      }, 30);
      return () => window.clearTimeout(t);
    }
  }, [selectedTaskId, workflow, seed.nodes, fitView]);

  const onNodesChange = (changes: NodeChange[]) => {
    setNodes((prev) => applyNodeChanges(changes, prev) as FlowNode[]);
  };

  if (!workflow) {
    return (
      <div className="w-full h-full flex items-center justify-center text-[var(--color-fg-subtle)] canvas-dots">
        <div className="max-w-md text-center px-6 anim-fade-in">
          <div className="mx-auto mb-4 size-12 rounded-2xl bg-[var(--color-accent-bg)] flex items-center justify-center">
            <svg
              width="22"
              height="22"
              viewBox="0 0 24 24"
              fill="none"
              stroke="currentColor"
              strokeWidth="1.75"
              strokeLinecap="round"
              strokeLinejoin="round"
              className="text-[var(--color-brand)]"
            >
              <rect x="3" y="10" width="6" height="6" rx="1.5" />
              <rect x="15" y="6" width="6" height="6" rx="1.5" />
              <rect x="15" y="14" width="6" height="6" rx="1.5" />
              <path d="M9 13h3M12 13v-4h3M12 13v4h3" />
            </svg>
          </div>
          <div
            className="text-[11px] tracking-widest uppercase text-[var(--color-fg-subtle)] mb-2"
            style={{ fontWeight: 400 }}
          >
            empty canvas
          </div>
          <p className="text-sm text-[var(--color-fg-muted)] leading-relaxed" style={{ fontWeight: 300 }}>
            Describe a process in the composer below. Crouton will extract actors, tasks, decisions
            and bind them to your{" "}
            <span className="text-[var(--color-fg)]" style={{ fontWeight: 400 }}>
              information system
            </span>
            .
          </p>
        </div>
      </div>
    );
  }

  const selectedNodes = nodes.map((n) => ({
    ...n,
    selected: n.id === selectedTaskId,
    // Tack a class name on recently-changed nodes. CSS animates a
    // brief pulse ring — no layout jitter, no layout recompute.
    className: recentlyChanged.has(n.id) ? "node-pulse" : undefined,
  }));

  return (
    <ReactFlow
      nodes={selectedNodes}
      edges={edges}
      nodeTypes={nodeTypes}
      edgeTypes={edgeTypes}
      onNodesChange={onNodesChange}
      fitView
      fitViewOptions={{ padding: 0.25 }}
      minZoom={0.3}
      maxZoom={1.5}
      proOptions={{ hideAttribution: true }}
      nodesDraggable
      nodesConnectable={false}
      elementsSelectable
      panOnDrag
      panOnScroll={false}
      zoomOnScroll
      onNodeClick={(_, node) => {
        if (!onTaskSelect) return;
        if (node.type === "task") onTaskSelect(node.id);
        else onTaskSelect(null);
      }}
      onPaneClick={() => onTaskSelect?.(null)}
    >
      <Background
        variant={BackgroundVariant.Dots}
        gap={22}
        size={1}
        className="!bg-[var(--color-bg)]"
      />
      <Controls position="bottom-right" showInteractive={false} />
      {/* Minimap — only useful once there are enough nodes to
          warrant it. Explicit size prevents React Flow's default
          (~200×150) from bleeding past the canvas. Nodes color-
          coded by confidence tier. */}
      {nodes.length > 5 && (
        <MiniMap
          position="bottom-left"
          pannable
          zoomable
          ariaLabel="workflow minimap"
          nodeStrokeWidth={2}
          nodeBorderRadius={3}
          nodeColor={(n) => {
            const data = (n.data ?? {}) as Record<string, unknown>;
            const tier = data.confidenceTier as keyof typeof TIER_COLOR | undefined;
            switch (tier) {
              case "high":
                return "#10b981";
              case "medium":
                return "#f59e0b";
              case "low":
                return "#f43f5e";
              default:
                return "#a1a1aa"; // zinc-400 — same tone light/dark
            }
          }}
          maskColor="rgba(24,24,27,0.18)"
          style={{
            width: 160,
            height: 96,
            backgroundColor: "var(--color-surface)",
            border: "1px solid var(--color-border)",
            borderRadius: 6,
            boxShadow: "var(--shadow-stripe-ambient)",
            margin: 12,
          }}
        />
      )}
    </ReactFlow>
  );
}
