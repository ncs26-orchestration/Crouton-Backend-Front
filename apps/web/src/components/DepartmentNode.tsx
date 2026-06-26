import { memo } from "react";
import { Handle, Position, type NodeProps } from "@xyflow/react";
import { Bot, CheckCircle2, Clock, Loader2, ShieldAlert } from "lucide-react";

import type { NodeStatus, WorkflowNodeData } from "../lib/types";

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
        <Bot size={12} className="text-[var(--color-fg-muted)] shrink-0" />
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
                : d.status === "blocked"
                  ? "text-[var(--color-danger)]"
                  : "text-[var(--color-fg-subtle)]"
          }`}
        />
      </div>

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
