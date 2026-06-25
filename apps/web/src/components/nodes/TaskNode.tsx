import { Handle, Position, type NodeProps, type Node } from "@xyflow/react";
import { Cog, FileCode2, User } from "lucide-react";

import type { TaskNodeData } from "../../lib/ir-to-flow";
import { TIER_COLOR, TIER_LABEL, TIER_TEXT } from "../../lib/confidence";

type Props = NodeProps<Node<TaskNodeData, "task">>;

const STATE_ACCENT: Record<TaskNodeData["bindingState"], string> = {
  ok: "bg-emerald-500",
  warn: "bg-amber-500",
  idle: "bg-[var(--color-border-strong)]",
  error: "bg-rose-500",
};

const STATE_CHIP: Record<TaskNodeData["bindingState"], string> = {
  ok: "text-emerald-600 dark:text-emerald-400 bg-emerald-500/10 border-emerald-500/30",
  warn: "text-amber-600 dark:text-amber-400 bg-amber-500/10 border-amber-500/30",
  idle: "text-[var(--color-fg-muted)] bg-[var(--color-surface-2)] border-[var(--color-border)]",
  error: "text-rose-600 dark:text-rose-400 bg-rose-500/10 border-rose-500/30",
};

const TYPE_BADGE: Record<"user" | "service" | "script", string> = {
  user: "bg-sky-500/10 text-sky-600 dark:text-sky-400 border-sky-500/25",
  service: "bg-violet-500/10 text-violet-600 dark:text-violet-400 border-violet-500/25",
  script: "bg-zinc-500/10 text-zinc-600 dark:text-zinc-400 border-zinc-500/25",
};

export function TaskNode({ data, selected }: Props) {
  const {
    task,
    bindingState,
    bindingLabel,
    assigneeLabel,
    confidence,
    confidenceTier,
    evidence,
  } = data;
  const Icon = task.type === "user" ? User : task.type === "service" ? Cog : FileCode2;
  const typeLabel = task.type;

  // Width of the confidence bar at the bottom. When we have no signal
  // (confidenceTier === "unknown") we hide the bar entirely rather
  // than showing a full grey one — silence beats noise.
  const barWidth = confidence != null ? `${Math.round(confidence * 100)}%` : "0%";
  const confidencePct = confidence != null ? Math.round(confidence * 100) : null;

  // One-line title for the tooltip; evidence quote gets appended so a
  // hover on any task reveals the source text that drove the
  // extraction without opening the Inspector.
  const confidenceTitle =
    confidence != null
      ? `Confidence ${confidencePct}% · ${TIER_LABEL[confidenceTier]}${evidence ? ` — "${evidence}"` : ""}`
      : "No confidence signal";

  return (
    <div
      className={`group relative bg-[var(--color-surface)] border rounded-lg transition-all cursor-grab active:cursor-grabbing overflow-hidden ${
        selected
          ? "border-[var(--color-brand)] shadow-[0_0_0_3px_var(--color-accent-bg)]"
          : "border-[var(--color-border)] shadow-stripe-ambient hover:shadow-stripe-standard hover:border-[var(--color-border-strong)]"
      }`}
      style={{ width: data.width, height: data.height }}
    >
      <Handle
        type="target"
        position={Position.Left}
        className="!size-2 !bg-[var(--color-border-strong)] !border-2 !border-[var(--color-surface)]"
      />

      {/* Top status strip — one semantic pixel-bar for binding state. */}
      <div className={`absolute top-0 left-0 right-0 h-[3px] ${STATE_ACCENT[bindingState]}`} />

      <div className="flex flex-col h-full px-3 pt-2.5 pb-2.5">
        <div className="flex items-center justify-between gap-2">
          <span
            className={`inline-flex items-center gap-1 text-[9.5px] uppercase tracking-[0.12em] px-1.5 py-0.5 rounded border ${TYPE_BADGE[task.type]}`}
            style={{ fontWeight: 500 }}
          >
            <Icon size={9} strokeWidth={2.25} />
            {typeLabel}
          </span>
          <div className="flex items-center gap-1.5 min-w-0">
            {/* Confidence dot — filled = high, half = medium, hollow = low */}
            {confidence != null && (
              <span
                title={confidenceTitle}
                className={`shrink-0 size-[7px] rounded-full ${
                  confidenceTier === "high"
                    ? TIER_COLOR.high
                    : confidenceTier === "medium"
                      ? `${TIER_COLOR.medium} opacity-70`
                      : `border ${TIER_TEXT.low} border-current bg-transparent`
                }`}
              />
            )}
            {assigneeLabel && (
              <span
                className="text-[10.5px] text-[var(--color-fg-muted)] font-mono truncate max-w-[11ch]"
                title={assigneeLabel}
              >
                {assigneeLabel}
              </span>
            )}
          </div>
        </div>

        <div
          className="mt-1 text-[13px] text-[var(--color-fg)] truncate leading-snug"
          style={{ fontWeight: 400 }}
          title={task.name}
        >
          {task.name}
        </div>

        <div className="mt-auto">
          {bindingLabel ? (
            <span
              className={`inline-block text-[10.5px] px-1.5 py-0.5 rounded border font-mono truncate max-w-full ${STATE_CHIP[bindingState]}`}
              title={bindingLabel}
            >
              {bindingLabel}
            </span>
          ) : (
            <span className="text-[10.5px] text-[var(--color-fg-subtle)] italic">unbound</span>
          )}
        </div>
      </div>

      {/* Bottom confidence bar — width is confidence * card width. Lets
          the user judge certainty at a glance across many nodes. */}
      {confidence != null && (
        <div
          className="absolute bottom-0 left-0 right-0 h-[3px] bg-[var(--color-surface-2)]"
          aria-hidden="true"
        >
          <div
            className={`h-full transition-[width] duration-300 ${TIER_COLOR[confidenceTier]}`}
            style={{ width: barWidth }}
            title={confidenceTitle}
          />
        </div>
      )}

      <Handle
        type="source"
        position={Position.Right}
        className="!size-2 !bg-[var(--color-border-strong)] !border-2 !border-[var(--color-surface)]"
      />
    </div>
  );
}
