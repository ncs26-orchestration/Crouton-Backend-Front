import { Handle, Position, type NodeProps, type Node } from "@xyflow/react";
import { Play, Square } from "lucide-react";

import type { EventNodeData } from "../../lib/ir-to-flow";

type Props = NodeProps<Node<EventNodeData, "event">>;

export function EventNode({ data, selected }: Props) {
  const isStart = data.eventType === "start";
  return (
    <div
      className="relative flex items-center justify-center cursor-grab active:cursor-grabbing"
      style={{ width: data.width, height: data.height }}
    >
      <Handle
        type="target"
        position={Position.Left}
        className="!size-2 !bg-[var(--color-border-strong)] !border-2 !border-[var(--color-surface)]"
      />
      <div
        className={`rounded-full bg-[var(--color-surface)] flex items-center justify-center transition-all ${
          selected
            ? isStart
              ? "border-2 border-[var(--color-brand)] shadow-[0_0_0_3px_var(--color-accent-bg)]"
              : "border-[3px] border-[var(--color-brand)] shadow-[0_0_0_3px_var(--color-accent-bg)]"
            : isStart
            ? "border border-emerald-500/60 shadow-stripe-ambient"
            : "border-[3px] border-[var(--color-fg)] shadow-stripe-ambient"
        }`}
        style={{ width: data.width - 6, height: data.height - 6 }}
      >
        {isStart ? (
          <Play size={10} strokeWidth={2.25} className="text-emerald-500" fill="currentColor" />
        ) : (
          <Square size={10} strokeWidth={2.25} className="text-[var(--color-fg)]" fill="currentColor" />
        )}
      </div>
      <Handle
        type="source"
        position={Position.Right}
        className="!size-2 !bg-[var(--color-border-strong)] !border-2 !border-[var(--color-surface)]"
      />
    </div>
  );
}
