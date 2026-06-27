import { useEffect, useRef, useState } from "react";
import {
  BaseEdge,
  EdgeLabelRenderer,
  getSmoothStepPath,
  type EdgeProps,
} from "@xyflow/react";
import { Pencil, Plus, Trash2 } from "lucide-react";

export interface ConditionalEdgeData extends Record<string, unknown> {
  /** Expression text, empty string = no condition */
  expression?: string;
  /** "juel" | "feel" — shown in the popover hint */
  language?: string;
  /** Called when the user saves a new expression */
  onEditExpression: (flowId: string, expression: string) => void;
  /** Called when the user clears the condition entirely */
  onClearExpression: (flowId: string) => void;
}

export function ConditionalEdge(props: EdgeProps) {
  const {
    id,
    sourceX,
    sourceY,
    targetX,
    targetY,
    sourcePosition,
    targetPosition,
    markerEnd,
    data,
    selected,
  } = props;
  const [edgePath, labelX, labelY] = getSmoothStepPath({
    sourceX,
    sourceY,
    sourcePosition,
    targetX,
    targetY,
    targetPosition,
    borderRadius: 14,
  });

  const d = data as unknown as ConditionalEdgeData;
  const [editing, setEditing] = useState(false);
  const [draft, setDraft] = useState(d?.expression ?? "");
  const inputRef = useRef<HTMLInputElement | null>(null);

  useEffect(() => {
    setDraft(d?.expression ?? "");
  }, [d?.expression]);

  useEffect(() => {
    if (editing) {
      const t = window.setTimeout(() => inputRef.current?.focus(), 0);
      return () => window.clearTimeout(t);
    }
  }, [editing]);

  const commit = () => {
    const next = draft.trim();
    if (next === (d?.expression ?? "")) {
      setEditing(false);
      return;
    }
    d?.onEditExpression(id, next);
    setEditing(false);
  };

  const cancel = () => {
    setDraft(d?.expression ?? "");
    setEditing(false);
  };

  const hasCondition = !!d?.expression;

  return (
    <>
      <BaseEdge
        id={id}
        path={edgePath}
        markerEnd={markerEnd}
        style={{
          strokeWidth: selected ? 2 : 1.25,
          stroke: selected ? "var(--color-brand)" : "var(--color-fg-subtle)",
        }}
      />
      <EdgeLabelRenderer>
        <div
          style={{
            position: "absolute",
            transform: `translate(-50%, -50%) translate(${labelX}px, ${labelY}px)`,
            pointerEvents: "all",
          }}
          className="nodrag nopan"
        >
          {editing ? (
            <div className="flex items-center gap-1 bg-[var(--color-surface)] border border-[var(--color-brand)] rounded-md shadow-stripe-elevated px-1.5 py-1 min-w-[180px]">
              <span className="text-[10px] font-mono text-[var(--color-fg-subtle)]">
                {d?.language ?? "juel"}
              </span>
              <input
                ref={inputRef}
                value={draft}
                onChange={(e) => setDraft(e.target.value)}
                onKeyDown={(e) => {
                  if (e.key === "Enter") commit();
                  if (e.key === "Escape") cancel();
                }}
                onBlur={commit}
                placeholder='${amount >= 50000}'
                className="flex-1 min-w-0 bg-transparent font-mono text-[11px] text-[var(--color-fg)] placeholder:text-[var(--color-fg-subtle)] focus:outline-none"
              />
              {hasCondition && (
                <button
                  onClick={(e) => {
                    e.preventDefault();
                    e.stopPropagation();
                    d?.onClearExpression(id);
                    setDraft("");
                    setEditing(false);
                  }}
                  className="btn-sm text-[var(--color-fg-subtle)] hover:text-rose-500"
                  title="Remove condition"
                  tabIndex={-1}
                >
                  <Trash2 size={11} />
                </button>
              )}
            </div>
          ) : hasCondition ? (
            <button
              onClick={() => setEditing(true)}
              className="group inline-flex items-center gap-1 bg-[var(--color-surface)] border border-[var(--color-border)] hover:border-[var(--color-brand)] hover:text-[var(--color-fg)] rounded-full px-2 py-0.5 text-[11px] font-mono text-[var(--color-fg)] shadow-stripe-ambient transition-colors"
              title="Click to edit"
            >
              <span>{d?.expression}</span>
              <Pencil
                size={9}
                className="text-[var(--color-fg-subtle)] group-hover:text-[var(--color-brand)] opacity-0 group-hover:opacity-100 transition-opacity"
              />
            </button>
          ) : (
            <button
              onClick={() => setEditing(true)}
              className="btn-sm inline-flex items-center gap-1 bg-[var(--color-surface)] border border-dashed border-[var(--color-border)] hover:border-[var(--color-brand)] hover:text-[var(--color-brand)] rounded-full px-1.5 py-0.5 text-[10px] text-[var(--color-fg-subtle)] transition-colors"
              title="Add a condition"
            >
              <Plus size={9} /> condition
            </button>
          )}
        </div>
      </EdgeLabelRenderer>
    </>
  );
}
