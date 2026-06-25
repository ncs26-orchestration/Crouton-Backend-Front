// Translate a Workflow IR into React Flow nodes + edges, resolving
// binding state against the IS Registry so each node knows which
// color border to render.

import type { Edge, Node } from "@xyflow/react";

import { layoutGraph, type LayoutNodeData } from "./layout";
import type { ConditionalEdgeData } from "../components/edges/ConditionalEdge";
import {
  evidenceOf,
  taskConfidence,
  tierOf,
  type ConfidenceTier,
} from "./confidence";
import type {
  BindingState,
  ISRegistry,
  ISSystem,
  Task,
  Workflow,
} from "./types";

export type TaskNodeData = LayoutNodeData & {
  kind: "task";
  task: Task;
  bindingState: BindingState;
  bindingLabel: string;
  assigneeLabel: string;
  // Confidence surface — nodes color their confidence bar and dot
  // from these two values. Computed here so TaskNode stays dumb.
  confidence: number | undefined;
  confidenceTier: ConfidenceTier;
  evidence: string | undefined;
};
export type GatewayNodeData = LayoutNodeData & {
  kind: "gateway";
  gatewayType: "exclusive" | "parallel";
  label: string;
};
export type EventNodeData = LayoutNodeData & {
  kind: "event";
  eventType: "start" | "end";
};

export type FlowNode =
  | Node<TaskNodeData, "task">
  | Node<GatewayNodeData, "gateway">
  | Node<EventNodeData, "event">;

const TASK_SIZE = { width: 248, height: 94 };
const GATEWAY_SIZE = { width: 56, height: 56 };
const EVENT_SIZE = { width: 44, height: 44 };

// Whether the IS Registry is the authoritative grounding source
// for bindings. In the operator-tool repositioning the extractor
// grounds in chat context, not a declared registry, so an absent
// or empty IS means "no authority here" — unresolved bindings are
// NEUTRAL, not errors. We only flag mismatches when the user has
// actually declared systems/users/groups to validate against.
function isValidatingAgainstRegistry(is: ISRegistry | undefined): boolean {
  if (!is) return false;
  const hasUsers = (is.users?.length ?? 0) > 0;
  const hasGroups = (is.groups?.length ?? 0) > 0;
  const hasSystems = (is.systems?.length ?? 0) > 0;
  return hasUsers || hasGroups || hasSystems;
}

function resolveBindingState(
  task: Task,
  is: ISRegistry | undefined,
): { state: BindingState; label: string } {
  if (!task.binding) return { state: "idle", label: "" };
  const b = task.binding;
  const validating = isValidatingAgainstRegistry(is);

  if (task.type === "user") {
    if (b.assignee_user_id) {
      const u = is?.users.find((x) => x.id === b.assignee_user_id);
      if (u) return { state: "ok", label: `→ ${u.name}` };
      // Registry-less mode: show the id as-is (extractor-inferred,
      // treated as informational rather than an error).
      return validating
        ? { state: "error", label: `user ${b.assignee_user_id}?` }
        : { state: "ok", label: `→ ${b.assignee_user_id}` };
    }
    if (b.candidate_group_id) {
      const g = is?.groups.find((x) => x.id === b.candidate_group_id);
      if (g) return { state: "ok", label: `→ ${g.name}` };
      return validating
        ? { state: "error", label: `group ${b.candidate_group_id}?` }
        : { state: "ok", label: `→ ${b.candidate_group_id}` };
    }
    return { state: "idle", label: "" };
  }

  if (task.type === "service") {
    if (!b.system_ref || !b.capability) return { state: "idle", label: "" };
    const sys: ISSystem | undefined = is?.systems.find((x) => x.id === b.system_ref);
    if (sys) {
      if (!sys.capabilities.includes(b.capability)) {
        // Explicit declaration + missing capability IS a real error
        // regardless of mode — the user said "this system exists"
        // and then referenced a verb it doesn't advertise.
        return { state: "error", label: `${sys.id} ∤ ${b.capability}` };
      }
      return { state: "ok", label: `→ ${sys.id} · ${b.capability}` };
    }
    return validating
      ? { state: "error", label: `${b.system_ref}?` }
      : { state: "ok", label: `→ ${b.system_ref} · ${b.capability}` };
  }

  return { state: "idle", label: "" };
}

function resolveAssignee(task: Task): string {
  if (!task.actor_ref) return "";
  return task.actor_ref;
}

export interface EdgeHandlers {
  onEditExpression: (flowId: string, expression: string) => void;
  onClearExpression: (flowId: string) => void;
}

export function irToFlow(
  wf: Workflow,
  is: ISRegistry | undefined,
  handlers: EdgeHandlers,
): { nodes: FlowNode[]; edges: Edge<ConditionalEdgeData>[] } {
  const nodes: FlowNode[] = [];

  for (const ev of wf.events) {
    nodes.push({
      id: ev.id,
      type: "event",
      position: { x: 0, y: 0 },
      draggable: true,
      data: { kind: "event", eventType: ev.type, ...EVENT_SIZE },
    });
  }
  for (const g of wf.gateways ?? []) {
    nodes.push({
      id: g.id,
      type: "gateway",
      position: { x: 0, y: 0 },
      draggable: true,
      data: {
        kind: "gateway",
        gatewayType: g.type,
        label: g.type === "exclusive" ? "X" : "+",
        ...GATEWAY_SIZE,
      },
    });
  }
  for (const t of wf.tasks) {
    const br = resolveBindingState(t, is);
    const conf = taskConfidence(t);
    nodes.push({
      id: t.id,
      type: "task",
      position: { x: 0, y: 0 },
      draggable: true,
      data: {
        kind: "task",
        task: t,
        bindingState: br.state,
        bindingLabel: br.label,
        assigneeLabel: resolveAssignee(t),
        confidence: conf,
        confidenceTier: tierOf(conf),
        evidence: evidenceOf(t) ?? evidenceOf(t.binding),
        ...TASK_SIZE,
      },
    });
  }

  const edges: Edge<ConditionalEdgeData>[] = wf.flows.map((f) => ({
    id: f.id,
    source: f.from,
    target: f.to,
    type: "conditional",
    data: {
      expression: f.condition?.expression,
      language: f.condition?.language ?? "juel",
      onEditExpression: handlers.onEditExpression,
      onClearExpression: handlers.onClearExpression,
    },
    markerEnd: {
      type: "arrowclosed",
      width: 16,
      height: 16,
      color: "var(--color-fg-subtle)",
    } as unknown as Edge["markerEnd"],
  }));

  const laidOut = layoutGraph(nodes, edges);
  return { nodes: laidOut, edges };
}
