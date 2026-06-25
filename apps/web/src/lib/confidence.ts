// Shared confidence tiering. Keeps the "what counts as low confidence"
// threshold in one place so TaskNode, Inspector, DiagnosticsBar, and
// the Copilot panel all agree.

import type { Provenance, Task, Workflow } from "./types";

export type ConfidenceTier = "high" | "medium" | "low" | "unknown";

export const CONFIDENCE_THRESHOLDS = {
  high: 0.8,
  medium: 0.6,
} as const;

export function tierOf(c: number | undefined | null): ConfidenceTier {
  if (c == null) return "unknown";
  if (c >= CONFIDENCE_THRESHOLDS.high) return "high";
  if (c >= CONFIDENCE_THRESHOLDS.medium) return "medium";
  return "low";
}

// Tailwind utility fragments per tier. Semantic not decorative:
// emerald = confident, amber = needs review, rose = low-confidence
// guess, zinc = no signal yet.
export const TIER_COLOR = {
  high: "bg-emerald-500",
  medium: "bg-amber-500",
  low: "bg-rose-500",
  unknown: "bg-[var(--color-fg-faint)]",
} as const satisfies Record<ConfidenceTier, string>;

export const TIER_TEXT = {
  high: "text-emerald-600 dark:text-emerald-400",
  medium: "text-amber-600 dark:text-amber-400",
  low: "text-rose-600 dark:text-rose-400",
  unknown: "text-[var(--color-fg-subtle)]",
} as const satisfies Record<ConfidenceTier, string>;

export const TIER_LABEL = {
  high: "confident",
  medium: "review",
  low: "uncertain",
  unknown: "no signal",
} as const satisfies Record<ConfidenceTier, string>;

// taskConfidence blends the task's own confidence with its binding's
// (if present) — we surface the weakest link because a confident
// "Archiver" paired with an uncertain OpenBee binding should read as
// uncertain at the node level.
export function taskConfidence(task: Task): number | undefined {
  const parts: number[] = [];
  if (task.confidence != null) parts.push(task.confidence);
  if (task.binding?.confidence != null) parts.push(task.binding.confidence);
  if (parts.length === 0) return undefined;
  return Math.min(...parts);
}

export interface LowConfidenceItem {
  kind: "task" | "actor" | "gateway" | "condition";
  id: string; // task/actor/gateway id, or flow id for conditions
  label: string;
  confidence: number;
  evidence?: string;
  tier: ConfidenceTier;
}

// Collect every element in the IR whose confidence is below the "high"
// threshold. Used by DiagnosticsBar's "Low confidence" tab and by the
// Copilot to seed Clarify bubbles.
export function collectLowConfidence(wf: Workflow | null): LowConfidenceItem[] {
  if (!wf) return [];
  const out: LowConfidenceItem[] = [];

  for (const t of wf.tasks) {
    const c = taskConfidence(t);
    if (c != null && c < CONFIDENCE_THRESHOLDS.high) {
      out.push({
        kind: "task",
        id: t.id,
        label: t.name,
        confidence: c,
        evidence: t.evidence ?? t.binding?.evidence,
        tier: tierOf(c),
      });
    }
  }
  for (const a of wf.actors) {
    if (a.confidence != null && a.confidence < CONFIDENCE_THRESHOLDS.high) {
      out.push({
        kind: "actor",
        id: a.id,
        label: a.name,
        confidence: a.confidence,
        evidence: a.evidence,
        tier: tierOf(a.confidence),
      });
    }
  }
  for (const g of wf.gateways ?? []) {
    if (g.confidence != null && g.confidence < CONFIDENCE_THRESHOLDS.high) {
      out.push({
        kind: "gateway",
        id: g.id,
        label: g.type,
        confidence: g.confidence,
        evidence: g.evidence,
        tier: tierOf(g.confidence),
      });
    }
  }
  for (const f of wf.flows) {
    const c = f.condition?.confidence;
    if (c != null && c < CONFIDENCE_THRESHOLDS.high) {
      out.push({
        kind: "condition",
        id: f.id,
        label: f.condition?.expression ?? "condition",
        confidence: c,
        evidence: f.condition?.evidence,
        tier: tierOf(c),
      });
    }
  }

  // Sort ascending so the least-confident items surface first — the
  // Diagnostics tab and Copilot's auto-clarify pick up on this order.
  return out.sort((a, b) => a.confidence - b.confidence);
}

// Grab the richest evidence quote for a task — task-level wins over
// binding-level when both are present.
export function evidenceOf(p: Provenance | undefined | null): string | undefined {
  return p?.evidence || undefined;
}
