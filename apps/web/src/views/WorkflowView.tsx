import { useMemo, useCallback, useEffect, useRef } from "react";
import { useQuery } from "@tanstack/react-query";
import {
  ReactFlow,
  Background,
  Controls,
  Panel,
  useNodesState,
  useEdgesState,
  useReactFlow,
  type NodeTypes,
  type Node,
  type Edge,
} from "@xyflow/react";
import "@xyflow/react/dist/style.css";
import {
  AlertCircle,
  ArrowLeft,
  Bot,
  CheckCircle2,
  Clock,
  Loader2,
  Maximize2,
  RotateCcw,
  ShieldAlert,
} from "lucide-react";

import { api } from "../lib/api";
import { requestToFlow } from "../lib/request-to-flow";
import {
  loadNodePositions,
  saveNodePositions,
  clearNodePositions,
} from "../lib/workflow-layout";
import {
  decisionOutcomeBadgeClass,
  decisionOutcomeLabel,
  flagSeverityDot,
  flagSeverityText,
  isNotableOutcome,
  nodeStatusColorClass,
  prettyLabel,
  requestStatusTextClass,
} from "../lib/request-format";
import { detailLabel } from "../lib/request-templates";
import { useRequestStream } from "../lib/sse";
import { DepartmentNode } from "../components/DepartmentNode";
import type { AuditEvent, WorkflowNodeData } from "../lib/types";

const nodeTypes: NodeTypes = {
  department: DepartmentNode,
};

interface Props {
  requestId: string | null;
  selectedNodeId: string | null;
  onSelectNode: (nodeId: string | null) => void;
  onBack: () => void;
}

export function WorkflowView({ requestId, selectedNodeId, onSelectNode, onBack }: Props) {
  if (!requestId) {
    return (
      <div className="flex-1 flex flex-col items-center justify-center gap-3 text-center px-8">
        <h2
          className="text-lg font-medium text-[var(--color-fg)]"
          style={{ fontFeatureSettings: '"ss01"' }}
        >
          No request selected
        </h2>
        <p className="text-sm text-[var(--color-fg-muted)] max-w-[45ch]">
          Open a request from the Requests tab to see its workflow graph.
        </p>
        <button
          onClick={onBack}
          className="flex items-center gap-1.5 text-sm text-[var(--color-brand)] hover:underline mt-2"
        >
          <ArrowLeft size={14} />
          Go to Requests
        </button>
      </div>
    );
  }

  return <WorkflowCanvas requestId={requestId} selectedNodeId={selectedNodeId} onSelectNode={onSelectNode} onBack={onBack} />;
}

function WorkflowCanvas({
  requestId,
  selectedNodeId,
  onSelectNode,
  onBack,
}: {
  requestId: string;
  selectedNodeId: string | null;
  onSelectNode: (nodeId: string | null) => void;
  onBack: () => void;
}) {
  const { data, isLoading, error } = useQuery({
    queryKey: ["request", requestId],
    queryFn: () => api.getRequest(requestId),
    refetchInterval: (query) =>
      query.state.data?.request.status === "in_progress" ? 4000 : false,
  });

  // Live updates patch this query's cache entry directly. Open the stream only
  // once the base graph has loaded, so patchCache has an entry to update.
  useRequestStream(requestId, !!data);

  const [nodes, setNodes, onNodesChange] = useNodesState<Node>([]);
  const [edges, setEdges] = useEdgesState<Edge>([]);
  const { fitView } = useReactFlow();
  const didFit = useRef(false);
  const prevStatus = useRef<Record<string, string>>({});

  // Rebuild nodes whenever the request data changes (initial load + live SSE
  // patches), but keep each node where the user dragged it (session positions,
  // then persisted positions, then the auto-layout). Nodes whose status just
  // flipped get a one-shot pulse so live progress is visible.
  useEffect(() => {
    if (!data) return;
    const layout = requestToFlow(data.nodes, data.edges);
    const persisted = loadNodePositions(requestId);
    const changed = new Set<string>();
    for (const n of data.nodes) {
      const prev = prevStatus.current[n.id];
      if (prev && prev !== n.status) changed.add(n.id);
      prevStatus.current[n.id] = n.status;
    }
    setNodes((curr) => {
      const currPos = new Map(curr.map((n) => [n.id, n.position]));
      return layout.nodes.map((n) => ({
        ...n,
        position: currPos.get(n.id) ?? persisted[n.id] ?? n.position,
        selected: n.id === selectedNodeId,
        className: changed.has(n.id) ? "node-pulse" : undefined,
      }));
    });
    setEdges(layout.edges);
  }, [data, selectedNodeId, requestId, setNodes, setEdges]);

  // Fit the view once, after the first nodes have landed. Not on every update,
  // so the user's panned/dragged view is left alone.
  useEffect(() => {
    if (!didFit.current && nodes.length > 0) {
      didFit.current = true;
      requestAnimationFrame(() => fitView({ padding: 0.16, maxZoom: 1.15, duration: 300 }));
    }
  }, [nodes.length, fitView]);

  const selectedNode = useMemo(() => {
    if (!selectedNodeId || !data) return null;
    return data.nodes.find((n) => n.id === selectedNodeId) ?? null;
  }, [selectedNodeId, data]);

  const onNodeClick = useCallback(
    (_event: React.MouseEvent, node: { id: string }) => {
      onSelectNode(node.id === selectedNodeId ? null : node.id);
    },
    [onSelectNode, selectedNodeId],
  );

  const onNodeDragStop = useCallback(() => {
    setNodes((curr) => {
      const positions: Record<string, { x: number; y: number }> = {};
      for (const n of curr) positions[n.id] = { x: n.position.x, y: n.position.y };
      saveNodePositions(requestId, positions);
      return curr;
    });
  }, [requestId, setNodes]);

  const resetLayout = useCallback(() => {
    if (!data) return;
    clearNodePositions(requestId);
    const layout = requestToFlow(data.nodes, data.edges);
    setNodes(layout.nodes.map((n) => ({ ...n, selected: n.id === selectedNodeId })));
    requestAnimationFrame(() => fitView({ padding: 0.16, maxZoom: 1.15, duration: 300 }));
  }, [requestId, data, selectedNodeId, setNodes, fitView]);

  if (isLoading) {
    return (
      <div className="flex-1 flex items-center justify-center">
        <div className="size-6 rounded-full border-2 border-[var(--color-brand)] border-t-transparent animate-spin" />
      </div>
    );
  }

  if (error || !data) {
    return (
      <div className="flex-1 flex items-center justify-center gap-2 text-sm text-[var(--color-danger)]">
        <AlertCircle size={16} />
        Failed to load request
      </div>
    );
  }

  const req = data.request;

  return (
    <div className="flex-1 flex overflow-hidden">
      {/* Left panel — Request Overview */}
      <div className="w-64 shrink-0 border-r border-[var(--color-border)] flex flex-col overflow-auto bg-[var(--color-surface)]">
        <div className="px-4 py-3 border-b border-[var(--color-border)]">
          <button
            onClick={onBack}
            className="flex items-center gap-1 text-xs text-[var(--color-fg-muted)] hover:text-[var(--color-fg)] mb-2 transition-colors"
          >
            <ArrowLeft size={12} />
            All Requests
          </button>
          <h2
            className="text-sm font-medium text-[var(--color-fg)] leading-tight"
            style={{ fontFeatureSettings: '"ss01"' }}
          >
            {req.title}
          </h2>
          <p className="text-[10px] text-[var(--color-fg-subtle)] mt-0.5 font-mono">{req.id}</p>
        </div>

        <div className="px-4 py-3 flex flex-col gap-2.5 text-xs">
          <InfoRow
            label="Requester"
            value={req.requester_role ? `${req.requester_name} (${prettyLabel(req.requester_role)})` : req.requester_name}
          />
          {req.request_type && req.request_type !== "general" && (
            <InfoRow label="Type" value={prettyLabel(req.request_type)} />
          )}
          {req.details &&
            Object.entries(req.details).map(([k, v]) => (
              <InfoRow key={k} label={detailLabel(k)} value={String(v)} />
            ))}
          <InfoRow label="Priority" value={prettyLabel(req.priority)} />
          <InfoRow label="Status">
            <span className={`font-medium ${requestStatusTextClass(req.status)}`}>
              {prettyLabel(req.status)}
            </span>
          </InfoRow>

          {/* Progress bar */}
          <div>
            <div className="flex justify-between mb-1">
              <span className="text-[var(--color-fg-muted)]">Progress</span>
              <span className="text-[var(--color-fg)] font-medium">{req.progress}%</span>
            </div>
            <div className="h-1.5 rounded-full bg-[var(--color-surface-3)] overflow-hidden">
              <div
                className="h-full rounded-full bg-[var(--color-brand)] transition-all duration-500"
                style={{ width: `${req.progress}%` }}
              />
            </div>
          </div>
        </div>

        {/* Participating agents */}
        <div className="px-4 py-3 border-t border-[var(--color-border)]">
          <h3 className="text-[10px] uppercase tracking-wide text-[var(--color-fg-muted)] mb-2">
            Participating Agents
          </h3>
          <div className="flex flex-col gap-1.5">
            {data.nodes.map((n) => (
              <button
                key={n.id}
                onClick={() => onSelectNode(n.id)}
                className={`flex items-center gap-2 px-2 py-1 rounded text-left transition-colors ${
                  selectedNodeId === n.id
                    ? "bg-[var(--color-accent-bg)]"
                    : "hover:bg-[var(--color-surface-2)]"
                }`}
              >
                <span className={`size-1.5 rounded-full shrink-0 ${nodeStatusColorClass(n.status)}`} />
                <span className="text-xs text-[var(--color-fg)] truncate">{n.name}</span>
              </button>
            ))}
          </div>
        </div>
      </div>

      {/* Center — Canvas */}
      <div className="flex-1 relative bg-[var(--color-surface-2)]">
        <ReactFlow
          nodes={nodes}
          edges={edges}
          nodeTypes={nodeTypes}
          onNodesChange={onNodesChange}
          onNodeClick={onNodeClick}
          onNodeDragStop={onNodeDragStop}
          proOptions={{ hideAttribution: true }}
          minZoom={0.3}
          maxZoom={2}
          nodesDraggable
          nodesConnectable={false}
          elementsSelectable
          panOnScroll
          selectionOnDrag
        >
          <Background gap={16} size={1} color="var(--color-border)" />
          <Controls
            showInteractive={false}
            className="!bg-[var(--color-surface)] !border-[var(--color-border)] !rounded-md !shadow-stripe"
          />
          <Panel position="top-right" className="flex gap-1.5">
            <button
              onClick={() => fitView({ padding: 0.16, maxZoom: 1.15, duration: 300 })}
              title="Fit to view"
              className="flex items-center gap-1.5 rounded-md border border-[var(--color-border)] bg-[var(--color-surface)] px-2.5 py-1.5 text-xs font-medium text-[var(--color-fg-muted)] shadow-stripe-ambient transition-colors hover:text-[var(--color-fg)]"
            >
              <Maximize2 size={13} /> Fit
            </button>
            <button
              onClick={resetLayout}
              title="Reset node layout"
              className="flex items-center gap-1.5 rounded-md border border-[var(--color-border)] bg-[var(--color-surface)] px-2.5 py-1.5 text-xs font-medium text-[var(--color-fg-muted)] shadow-stripe-ambient transition-colors hover:text-[var(--color-fg)]"
            >
              <RotateCcw size={13} /> Reset layout
            </button>
          </Panel>
        </ReactFlow>

        {/* Legend */}
        <div className="absolute bottom-4 left-4 flex items-center gap-3 bg-[var(--color-surface)] border border-[var(--color-border)] rounded-md px-3 py-1.5 text-[10px] text-[var(--color-fg-muted)]"
          style={{ boxShadow: "0 2px 5px rgba(50,50,93,0.1), 0 1px 2px rgba(0,0,0,0.08)" }}
        >
          <LegendItem color="var(--color-fg-subtle)" label="Pending" />
          <LegendItem color="var(--color-brand)" label="In Progress" />
          <LegendItem color="var(--color-success)" label="Completed" />
          <LegendItem color="var(--color-danger)" label="Blocked" />
        </div>
      </div>

      {/* Right panel — Node Detail */}
      <div className="w-72 shrink-0 border-l border-[var(--color-border)] flex flex-col overflow-auto bg-[var(--color-surface)]">
        {selectedNode ? (
          <NodeDetail requestId={requestId} node={selectedNode} />
        ) : (
          <div className="flex-1 flex flex-col items-center justify-center gap-2 text-center px-6">
            <div className="size-8 rounded-lg bg-[var(--color-surface-2)] flex items-center justify-center">
              <Bot size={16} className="text-[var(--color-fg-subtle)]" />
            </div>
            <p className="text-xs text-[var(--color-fg-muted)]">
              Click a node on the canvas to see its details
            </p>
          </div>
        )}
      </div>
    </div>
  );
}

function NodeDetail({ requestId, node }: { requestId: string; node: WorkflowNodeData }) {
  const config = {
    pending: { icon: Clock, color: "text-[var(--color-fg-subtle)]" },
    in_progress: { icon: Loader2, color: "text-[var(--color-brand)]" },
    completed: { icon: CheckCircle2, color: "text-[var(--color-success)]" },
    blocked: { icon: ShieldAlert, color: "text-[var(--color-danger)]" },
  }[node.status] ?? { icon: Clock, color: "text-[var(--color-fg-subtle)]" };

  const StatusIcon = config.icon;

  // Tasks load lazily per node and keep polling while the node is mid-flight.
  // status is part of the key so the panel refetches once the node flips to
  // completed (tasks are written at that moment) instead of holding the empty
  // in-progress result.
	const tasksQuery = useQuery({
		queryKey: ["node", requestId, node.id, node.status],
		queryFn: () => api.getNode(requestId, node.id),
		refetchInterval: node.status === "in_progress" ? 1500 : false,
	});
	const tasks = tasksQuery.data?.tasks ?? [];
	const activity = tasksQuery.data?.activity ?? [];
	// The per-node fetch carries the reasoning + flags the graph list omits, so
	// prefer it once loaded and fall back to the graph node before then.
	const n = tasksQuery.data?.node ?? node;
	const flags = n.flags ?? [];

	return (
		<div className="flex flex-col">
      <div className="px-4 py-3 border-b border-[var(--color-border)]">
        <div className="flex items-center gap-1.5 mb-1">
          <Bot size={12} className="text-[var(--color-fg-muted)]" />
          <span className="text-[10px] uppercase tracking-wide text-[var(--color-fg-muted)]">
            {node.department}
          </span>
        </div>
        <h3
          className="text-sm font-medium text-[var(--color-fg)]"
          style={{ fontFeatureSettings: '"ss01"' }}
        >
          {node.name}
        </h3>
      </div>

      <div className="px-4 py-3 flex flex-col gap-2.5 text-xs">
        <InfoRow label="Status">
          <span className={`flex items-center gap-1 font-medium ${config.color}`}>
            <StatusIcon size={12} className={node.status === "in_progress" ? "animate-spin" : ""} />
            {prettyLabel(node.status)}
          </span>
        </InfoRow>
        {isNotableOutcome(n.decision_outcome) && n.decision_outcome && (
          <InfoRow label="Decision">
            <span
              className={`rounded px-1.5 py-0.5 text-[10px] font-medium ${decisionOutcomeBadgeClass(n.decision_outcome)}`}
            >
              {decisionOutcomeLabel(n.decision_outcome)}
            </span>
          </InfoRow>
        )}
        <InfoRow label="Agent Type" value={node.agent_type} />
        <InfoRow label="Progress" value={`${node.progress_percent}%`} />
        {node.started_at && <InfoRow label="Started" value={new Date(node.started_at).toLocaleString()} />}
        {node.completed_at && <InfoRow label="Completed" value={new Date(node.completed_at).toLocaleString()} />}
      </div>

      {node.status === "blocked" && node.blocked_by && (
        <div className="px-4 py-3 border-t border-[var(--color-border)]">
          <h4 className="text-[10px] uppercase tracking-wide text-[var(--color-danger)] mb-1.5 flex items-center gap-1">
            <ShieldAlert size={11} />
            Blocked
          </h4>
          <p className="text-xs text-[var(--color-fg)] leading-relaxed">
            {node.blocked_by.reason}
          </p>
        </div>
      )}

      {/* The agent's reasoning — the "why" behind the decision. */}
      {(n.decision_summary || (n.status_text && node.status !== "blocked")) && (
        <div className="px-4 py-3 border-t border-[var(--color-border)]">
          <h4 className="text-[10px] uppercase tracking-wide text-[var(--color-fg-muted)] mb-1.5">
            {n.decision_summary ? "Assessment" : "Latest status"}
          </h4>
          <p className="text-xs text-[var(--color-fg)] leading-relaxed">
            {n.decision_summary || n.status_text}
          </p>
        </div>
      )}

      {flags.length > 0 && (
        <div className="px-4 py-3 border-t border-[var(--color-border)]">
          <h4 className="text-[10px] uppercase tracking-wide text-[var(--color-fg-muted)] mb-2">
            {n.decision_outcome === "approve_with_conditions" ? "Conditions & flags" : "Flags"}
          </h4>
          <ul className="flex flex-col gap-2">
            {flags.map((f, i) => (
              <li key={i} className="flex items-start gap-2 text-xs">
                <span className={`mt-1 size-1.5 rounded-full shrink-0 ${flagSeverityDot(f.severity)}`} />
                <span className="leading-snug">
                  <span className={`uppercase text-[9px] font-semibold tracking-wide mr-1.5 ${flagSeverityText(f.severity)}`}>
                    {f.severity}
                  </span>
                  <span className="text-[var(--color-fg)]">{f.message}</span>
                </span>
              </li>
            ))}
          </ul>
        </div>
      )}

      {tasks.length > 0 && (
        <div className="px-4 py-3 border-t border-[var(--color-border)]">
          <h4 className="text-[10px] uppercase tracking-wide text-[var(--color-fg-muted)] mb-2">
            Tasks
          </h4>
          <ul className="flex flex-col gap-1.5">
            {tasks.map((t) => (
              <li key={t.id} className="flex items-start gap-1.5 text-xs">
                {t.status === "completed" ? (
                  <CheckCircle2 size={13} className="text-[var(--color-success)] shrink-0 mt-0.5" />
                ) : t.status === "in_progress" ? (
                  <Loader2 size={13} className="text-[var(--color-brand)] animate-spin shrink-0 mt-0.5" />
                ) : (
                  <Clock size={13} className="text-[var(--color-fg-subtle)] shrink-0 mt-0.5" />
                )}
                <span className="text-[var(--color-fg)] leading-snug">{t.title}</span>
              </li>
            ))}
          </ul>
        </div>
      )}

      {activity.length > 0 && (
        <div className="px-4 py-3 border-t border-[var(--color-border)]">
          <h4 className="text-[10px] uppercase tracking-wide text-[var(--color-fg-muted)] mb-2">
            Activity
          </h4>
          <div className="flex flex-col gap-2 max-h-48 overflow-y-auto">
            {activity.map((a: AuditEvent) => (
              <div key={a.id} className="text-[11px] leading-snug">
                <div className="flex items-center gap-1.5">
                  <span className="font-medium text-[var(--color-fg)]">{a.actor}</span>
                  <ActionBadge action={a.action} />
                </div>
                {a.reason && (
                  <p className="text-[var(--color-fg-muted)] mt-0.5">{a.reason}</p>
                )}
                <span className="text-[10px] text-[var(--color-fg-subtle)]">
                  {new Date(a.created_at).toLocaleString()}
                </span>
              </div>
            ))}
          </div>
        </div>
      )}

      {node.description && (
        <div className="px-4 py-3 border-t border-[var(--color-border)]">
          <h4 className="text-[10px] uppercase tracking-wide text-[var(--color-fg-muted)] mb-1.5">
            Description
          </h4>
          <p className="text-xs text-[var(--color-fg-muted)] leading-relaxed">
            {node.description}
          </p>
        </div>
      )}
    </div>
  );
}

function ActionBadge({ action }: { action: string }) {
	const colors: Record<string, string> = {
		"node.started": "bg-blue-100 text-blue-700",
		"node.completed": "bg-green-100 text-green-700",
		"request.completed": "bg-green-100 text-green-700",
		"agent.fallback": "bg-yellow-100 text-yellow-700",
		"node.blocked": "bg-red-100 text-red-700",
		"node.unblocked": "bg-teal-100 text-teal-700",
		"approval.granted": "bg-purple-100 text-purple-700",
		"approval.rejected": "bg-red-100 text-red-700",
	};
	const cls = colors[action] ?? "bg-gray-100 text-gray-600";
	return (
		<span className={`text-[10px] px-1.5 py-0.5 rounded font-medium ${cls}`}>
			{action.replace(/\./g, " ")}
		</span>
	);
}

function InfoRow({ label, value, children }: { label: string; value?: string; children?: React.ReactNode }) {
  return (
    <div className="flex justify-between items-center">
      <span className="text-[var(--color-fg-muted)]">{label}</span>
      {children ?? <span className="text-[var(--color-fg)] font-medium">{value}</span>}
    </div>
  );
}

function LegendItem({ color, label }: { color: string; label: string }) {
  return (
    <span className="flex items-center gap-1">
      <span className="size-2 rounded-full" style={{ backgroundColor: color }} />
      {label}
    </span>
  );
}
