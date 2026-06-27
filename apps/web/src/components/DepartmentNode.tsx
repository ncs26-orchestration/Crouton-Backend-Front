import { memo } from "react";
import { Handle, Position, type NodeProps } from "@xyflow/react";
import { CheckCircle2, Clock, Loader2, ShieldAlert, UserCheck } from "lucide-react";

import type { NodeStatus, WorkflowNodeData } from "../lib/types";
import { decisionOutcomeBadgeClass, decisionOutcomeLabel, isNotableOutcome } from "../lib/request-format";
import { Avatar } from "./Avatar";

// Stable per-department accent so each agent reads as a distinct identity on the
// canvas. Known departments get a fixed hue; anything else is hashed to one.
const DEPT_COLORS: Record<string, string> = {
  planning: "#6366f1",
  finance: "#0ea5e9",
  legal: "#8b5cf6",
  it: "#14b8a6",
  hr: "#ec4899",
  operations: "#f59e0b",
  executive: "#64748b",
};
const DEPT_FALLBACK = ["#6366f1", "#0ea5e9", "#14b8a6", "#f59e0b", "#ec4899", "#8b5cf6", "#ef4444"];

function departmentColor(department: string): string {
  const key = department.trim().toLowerCase();
  if (DEPT_COLORS[key]) return DEPT_COLORS[key];
  let hash = 0;
  for (let i = 0; i < key.length; i++) hash = (hash * 31 + key.charCodeAt(i)) | 0;
  return DEPT_FALLBACK[Math.abs(hash) % DEPT_FALLBACK.length]!;
}

function departmentInitials(department: string): string {
  const words = department.trim().split(/\s+/).filter(Boolean);
  if (words.length === 0) return "?";
  if (words.length === 1) return words[0]!.slice(0, 2).toUpperCase();
  return (words[0]![0]! + words[1]![0]!).toUpperCase();
}

const STATUS_CONFIG: Record<NodeStatus, { bg: string; border: string; icon: typeof Clock; label: string }> = {
  pending: {
    bg: "bg-[var(--color-surface-2)]",
    border: "border-[var(--color-border)]",
    icon: Clock,
    label: "Pending",
  },
  in_progress: {
    bg: "bg-[var(--color-brand)]/10",
    border: "border-[var(--color-brand)]",
    icon: Loader2,
    label: "In Progress",
  },
  awaiting_review: {
    bg: "bg-[var(--color-warning)]/10",
    border: "border-[var(--color-warning)]",
    icon: UserCheck,
    label: "Needs review",
  },
  completed: {
    bg: "bg-[var(--color-success)]/10",
    border: "border-[var(--color-success)]",
    icon: CheckCircle2,
    label: "Completed",
  },
  blocked: {
    bg: "bg-[var(--color-danger)]/10",
    border: "border-[var(--color-danger)]",
    icon: ShieldAlert,
    label: "Blocked",
  },
};

function DepartmentNodeInner({ data, selected }: NodeProps) {
  const d = data as unknown as WorkflowNodeData;
  const config = STATUS_CONFIG[d.status] ?? STATUS_CONFIG.pending;
  const StatusIcon = config.icon;

  return (
    <div
      className={`
        relative rounded-md border ${config.border} ${config.bg}
        px-3 py-2.5 w-full h-full flex flex-col justify-center gap-1
        transition-shadow
        ${selected ? "ring-2 ring-[var(--color-brand)] ring-offset-1" : ""}
      `}
      style={{
        boxShadow: "0 2px 5px rgba(50,50,93,0.1), 0 1px 2px rgba(0,0,0,0.08)",
        fontFeatureSettings: '"ss01"',
      }}
    >
      <Handle type="target" position={Position.Left} className="!bg-[var(--color-border-strong)] !w-2 !h-2" />
      <Handle type="source" position={Position.Right} className="!bg-[var(--color-border-strong)] !w-2 !h-2" />

      <div className="flex items-center gap-1.5">
        <span
          className="size-4 shrink-0 rounded-[5px] flex items-center justify-center text-[8px] font-semibold text-white"
          style={{ background: departmentColor(d.department) }}
          title={d.department}
        >
          {departmentInitials(d.department)}
        </span>
        <span className="text-[10px] text-[var(--color-fg-muted)] uppercase tracking-wide truncate">
          {d.department}
        </span>
      </div>

      <div className="flex items-center justify-between gap-1">
        <span className="text-xs font-medium text-[var(--color-fg)] truncate leading-tight">
          {d.name}
        </span>
        <StatusIcon
          size={14}
          className={`shrink-0 ${
            d.status === "in_progress"
              ? "text-[var(--color-brand)] animate-spin"
              : d.status === "completed"
                ? "text-[var(--color-success)]"
                : d.status === "awaiting_review"
                  ? "text-[var(--color-warning-fg)]"
                  : d.status === "blocked"
                    ? "text-[var(--color-danger)]"
                    : "text-[var(--color-fg-subtle)]"
          }`}
        />
      </div>

      {d.assignees && d.assignees.length > 0 && (
        <div className="flex items-center -space-x-1 mt-0.5">
          {d.assignees.slice(0, 4).map((name, i) => (
            <span key={i} className="ring-1 ring-[var(--color-surface)] rounded-full">
              <Avatar name={name} size={16} />
            </span>
          ))}
          {d.assignees.length > 4 && (
            <span className="text-[9px] text-[var(--color-fg-muted)] pl-1.5">+{d.assignees.length - 4}</span>
          )}
        </div>
      )}

      {isNotableOutcome(d.decision_outcome) && d.decision_outcome && (
        <span
          className={`mt-0.5 self-start rounded px-1.5 py-0.5 text-[9px] font-medium uppercase tracking-wide ${decisionOutcomeBadgeClass(d.decision_outcome)}`}
        >
          {decisionOutcomeLabel(d.decision_outcome)}
        </span>
      )}

      {d.status === "blocked" ? (
        <div className="flex items-center gap-1 mt-0.5">
          <ShieldAlert size={10} className="text-[var(--color-danger)] shrink-0" />
          <p className="text-[10px] text-[var(--color-danger)] truncate leading-tight">
            {d.status_text || "Waiting for dependency"}
          </p>
        </div>
      ) : d.status_text ? (
        <p className="text-[10px] text-[var(--color-fg-muted)] truncate leading-tight mt-0.5">
          {d.status_text}
        </p>
      ) : null}
    </div>
  );
}

export const DepartmentNode = memo(DepartmentNodeInner);
