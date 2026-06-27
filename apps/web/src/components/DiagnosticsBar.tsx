import { useMemo, useState } from "react";
import { motion } from "framer-motion";
import { AlertCircle, AlertTriangle, ArrowUpRight, CheckCircle2, HelpCircle, Sparkles } from "lucide-react";

import { TIER_COLOR, TIER_TEXT, collectLowConfidence } from "../lib/confidence";
import type { Diagnostic, Workflow } from "../lib/types";

type TabId = "errors" | "low-confidence" | "suggestions";

interface Props {
  diagnostics: Diagnostic[];
  error?: string | null;
  workflow?: Workflow | null;
  onSelectTask?: (taskId: string) => void;
  // Called when the user clicks a low-confidence row. Every row is
  // actionable — task rows open the Inspector, everything else
  // invites the Copilot. This closes the "I can't respond" gap.
  onOpenCopilot?: () => void;
}

// DiagnosticsBar — compact, tabbed strip at the bottom of the canvas.
// Replaces the previous flat list. Three tabs:
//   - Errors:          validator errors (schema or cross-ref)
//   - Low confidence:  tasks/actors/gateways/conditions with
//                      confidence < 0.8, driven by the extractor
//   - Suggestions:     warnings + hints the compiler emits (e.g. the
//                      lowering pass introduced in Round 2)
// When nothing's wrong we stay out of the way with a single green row.
export function DiagnosticsBar({ diagnostics, error, workflow, onSelectTask, onOpenCopilot }: Props) {
  const errors = useMemo(
    () => diagnostics.filter((d) => d.severity === "error"),
    [diagnostics],
  );
  const suggestions = useMemo(
    () => diagnostics.filter((d) => d.severity !== "error"),
    [diagnostics],
  );
  const lowConfidence = useMemo(() => collectLowConfidence(workflow ?? null), [workflow]);

  // Preferred tab on open: errors > low-confidence > suggestions.
  const defaultTab: TabId = errors.length
    ? "errors"
    : lowConfidence.length
      ? "low-confidence"
      : "suggestions";

  const [tab, setTab] = useState<TabId>(defaultTab);
  const [expanded, setExpanded] = useState<boolean>(errors.length > 0);

  // Fatal-error short-circuit stays the same — it's a transport-level
  // failure, not an IR-level one, so we surface it prominently.
  if (error) {
    return (
      <div className="shrink-0 border-t border-rose-500/30 bg-rose-500/10 px-4 py-2 text-xs text-rose-600 dark:text-rose-400 flex items-center gap-2">
        <AlertCircle size={14} />
        <span className="font-medium">Error:</span>
        <span className="font-mono truncate">{error}</span>
      </div>
    );
  }

  const total = errors.length + lowConfidence.length + suggestions.length;
  if (total === 0) {
    return (
      <div className="shrink-0 border-t border-[var(--color-border)] bg-[var(--color-surface)] px-4 py-2 text-xs text-[var(--color-fg-muted)] flex items-center gap-2">
        <CheckCircle2 size={13} className="text-emerald-500" />
        <span>Everything resolves. IR is schema-valid, every reference grounds in the IS, and the extractor is confident on all elements.</span>
      </div>
    );
  }

  return (
    <div className="shrink-0 border-t border-[var(--color-border)] bg-[var(--color-surface)]">
      {/* Header row — tab strip + collapse toggle. Kept tight because the
          bar sits between the canvas and the composer. */}
      <div className="flex items-center justify-between px-3 py-1.5 text-[11px]">
        <nav className="flex gap-0.5 bg-[var(--color-surface-2)] rounded-md p-0.5">
          <TabButton
            active={tab === "errors"}
            disabled={errors.length === 0}
            onClick={() => {
              setTab("errors");
              setExpanded(true);
            }}
            icon={<AlertCircle size={11} strokeWidth={2} />}
            label="Errors"
            count={errors.length}
            tone="rose"
          />
          <TabButton
            active={tab === "low-confidence"}
            disabled={lowConfidence.length === 0}
            onClick={() => {
              setTab("low-confidence");
              setExpanded(true);
            }}
            icon={<Sparkles size={11} strokeWidth={2} />}
            label="Low confidence"
            count={lowConfidence.length}
            tone="amber"
          />
          <TabButton
            active={tab === "suggestions"}
            disabled={suggestions.length === 0}
            onClick={() => {
              setTab("suggestions");
              setExpanded(true);
            }}
            icon={<HelpCircle size={11} strokeWidth={2} />}
            label="Suggestions"
            count={suggestions.length}
            tone="sky"
          />
        </nav>
        <button
          onClick={() => setExpanded((v) => !v)}
          className="btn-inline text-[10px] text-[var(--color-fg-muted)] hover:text-[var(--color-fg)] font-mono"
          style={{ fontWeight: 400 }}
        >
          {expanded ? "collapse" : "expand"}
        </button>
      </div>

      {expanded && (
        <motion.div
          initial={{ height: 0, opacity: 0 }}
          animate={{ height: "auto", opacity: 1 }}
          exit={{ height: 0, opacity: 0 }}
          className="max-h-40 overflow-y-auto nice-scroll border-t border-[var(--color-border)]"
        >
          {tab === "errors" && <DiagList items={errors} />}
          {tab === "low-confidence" && (
            <LowConfidenceList items={lowConfidence} onSelectTask={onSelectTask} onOpenCopilot={onOpenCopilot} />
          )}
          {tab === "suggestions" && <DiagList items={suggestions} />}
        </motion.div>
      )}
    </div>
  );
}

function TabButton({
  active,
  disabled,
  onClick,
  icon,
  label,
  count,
  tone,
}: {
  active: boolean;
  disabled?: boolean;
  onClick: () => void;
  icon: React.ReactNode;
  label: string;
  count: number;
  tone: "rose" | "amber" | "sky";
}) {
  const toneText =
    tone === "rose"
      ? "text-rose-600 dark:text-rose-400"
      : tone === "amber"
        ? "text-amber-600 dark:text-amber-400"
        : "text-sky-600 dark:text-sky-400";
  return (
    <button
      onClick={onClick}
      disabled={disabled}
      className={`flex items-center gap-1.5 px-2 py-0.5 rounded text-[11px] transition-colors disabled:opacity-40 disabled:cursor-not-allowed ${
        active
          ? `bg-[var(--color-surface)] ${toneText} shadow-stripe-ambient`
          : "text-[var(--color-fg-muted)] hover:text-[var(--color-fg)]"
      }`}
      style={{ fontWeight: 400 }}
    >
      <span className={active ? toneText : "text-[var(--color-fg-subtle)]"}>{icon}</span>
      <span>{label}</span>
      <span className="font-mono text-[10px] tnum">{count}</span>
    </button>
  );
}

function DiagList({ items }: { items: Diagnostic[] }) {
  if (items.length === 0) {
    return (
      <div className="px-4 py-3 text-[11px] text-[var(--color-fg-subtle)] italic">
        nothing here — the extractor had nothing to say.
      </div>
    );
  }
  return (
    <ul className="px-3 py-2 space-y-1.5 text-xs">
      {items.map((d, i) => (
        <li key={i} className="flex items-start gap-2">
          <span
            className={`font-mono text-[10px] px-1.5 py-0.5 rounded shrink-0 ${
              d.severity === "error"
                ? "bg-rose-500/15 text-rose-700 dark:text-rose-400"
                : "bg-sky-500/15 text-sky-700 dark:text-sky-400"
            }`}
          >
            {d.severity}
          </span>
          {d.ir_ref && (
            <code className="font-mono text-[10.5px] text-[var(--color-fg-subtle)] shrink-0 mt-[1px]">
              {d.ir_ref}
            </code>
          )}
          <span className="text-[var(--color-fg)] flex-1">{d.message}</span>
          {d.suggestion && <span className="text-[var(--color-fg-subtle)] italic">· {d.suggestion}</span>}
        </li>
      ))}
    </ul>
  );
}

function LowConfidenceList({
  items,
  onSelectTask,
  onOpenCopilot,
}: {
  items: ReturnType<typeof collectLowConfidence>;
  onSelectTask?: (taskId: string) => void;
  onOpenCopilot?: () => void;
}) {
  // Every row is actionable. Task rows jump to the Inspector; every
  // other kind opens the Copilot where the user can Clarify. Both
  // paths surface the element in a dedicated UI so the diagnostic
  // never feels like a dead-end.
  const handleClick = (item: ReturnType<typeof collectLowConfidence>[number]) => {
    if (item.kind === "task" && onSelectTask) {
      onSelectTask(item.id);
    } else if (onOpenCopilot) {
      onOpenCopilot();
    }
  };

  return (
    <ul className="py-1 text-xs">
      {items.map((item) => {
        const pct = Math.round(item.confidence * 100);
        return (
          <li
            key={`${item.kind}:${item.id}`}
            onClick={() => handleClick(item)}
            className="group flex items-center gap-3 px-3 py-1.5 cursor-pointer hover:bg-[var(--color-surface-2)] transition-colors"
          >
            {/* Confidence — dot + percent, fixed width so every row
                aligns vertically. */}
            <div className="flex items-center gap-1.5 shrink-0 w-[50px]">
              <span className={`size-[6px] rounded-full ${TIER_COLOR[item.tier]}`} />
              <span className={`text-[10px] font-mono tnum ${TIER_TEXT[item.tier]}`}>
                {pct}%
              </span>
            </div>

            {/* Kind — small-caps tag, fixed width. */}
            <span
              className="text-[10px] uppercase tracking-[0.12em] text-[var(--color-fg-subtle)] shrink-0 w-[64px]"
              style={{ fontWeight: 500 }}
            >
              {item.kind}
            </span>

            {/* Label — the element's display name. Min-width 0 so
                the flex child can actually truncate. */}
            <span
              className="text-[var(--color-fg)] truncate min-w-0 flex-[0_1_auto]"
              title={item.label}
              style={{ flexBasis: "20ch" }}
            >
              {item.label}
            </span>

            {/* Evidence — italic quote, grows to fill the rest of
                the row. Truncates at the right edge; hover shows
                the full text via title. */}
            {item.evidence && (
              <span
                className="text-[var(--color-fg-muted)] italic truncate min-w-0 flex-1"
                title={item.evidence}
              >
                &ldquo;{item.evidence}&rdquo;
              </span>
            )}

            {/* Action hint — always shown, brand-tinted on hover.
                Makes "click me" obvious for non-task rows. */}
            <span
              className="shrink-0 inline-flex items-center gap-1 text-[10px] text-[var(--color-fg-subtle)] group-hover:text-[var(--color-brand)] transition-colors"
              style={{ fontWeight: 500 }}
            >
              {item.kind === "task" ? "Inspect" : "Clarify"}
              <ArrowUpRight size={10} />
            </span>
          </li>
        );
      })}
      {items.length === 0 && (
        <li className="text-[11px] text-[var(--color-fg-subtle)] italic">
          extractor is confident on every element.
        </li>
      )}
    </ul>
  );
}

// expose AlertTriangle to keep tree-shaking deterministic (lucide-react).
export { AlertTriangle };
