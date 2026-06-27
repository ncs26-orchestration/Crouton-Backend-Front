import type { DecisionOutcome, NodeStatus, RequestPriority, RequestStatus } from "./types";

// Shared request/node status + priority presentation so the list,
// canvas, and node card can't drift. Colors come from the status tokens
// in index.css so theming and dark mode stay consistent.

// Length caps mirror the server (apps/api handler/requests.go) so the UI
// stops oversized input before it ever hits the API.
export const MAX_REQUEST_TITLE_LEN = 200;
export const MAX_REQUEST_DESCRIPTION_LEN = 5000;

export function statusBadgeClass(status: RequestStatus): string {
  switch (status) {
    case "in_progress":
    case "awaiting_approval":
      return "bg-[var(--color-accent-bg)] text-[var(--color-brand)]";
    case "completed":
    case "approved":
      return "bg-[var(--color-success)]/12 text-[var(--color-success)]";
    case "rejected":
      return "bg-[var(--color-danger)]/12 text-[var(--color-danger)]";
    default:
      return "bg-[var(--color-surface-2)] text-[var(--color-fg-muted)]";
  }
}

// Text-only color for a request status (used on label text rather than a
// filled badge).
export function requestStatusTextClass(status: RequestStatus): string {
  switch (status) {
    case "in_progress":
      return "text-[var(--color-brand)]";
    case "awaiting_approval":
      return "text-[var(--color-warning-fg)]";
    case "completed":
    case "approved":
      return "text-[var(--color-success)]";
    case "rejected":
      return "text-[var(--color-danger)]";
    default:
      return "text-[var(--color-fg-muted)]";
  }
}

// Text color for a priority label.
export function priorityTextClass(priority: RequestPriority): string {
  switch (priority) {
    case "urgent":
      return "text-[var(--color-danger)]";
    case "high":
      return "text-[var(--color-warning-fg)]";
    case "medium":
      return "text-[var(--color-fg-muted)]";
    default:
      return "text-[var(--color-fg-subtle)]";
  }
}

// Solid dot/background color for a node status (canvas + status dots).
export function nodeStatusColorClass(status: NodeStatus): string {
  switch (status) {
    case "completed":
      return "bg-[var(--color-success)]";
    case "in_progress":
      return "bg-[var(--color-brand)]";
    case "blocked":
      return "bg-[var(--color-danger)]";
    default:
      return "bg-[var(--color-fg-subtle)]";
  }
}

// Raw token value for a node status — used where a CSS color string is
// needed (e.g. React Flow edge stroke) rather than a Tailwind class.
export function nodeStatusToken(status: NodeStatus): string {
  switch (status) {
    case "completed":
      return "var(--color-success)";
    case "in_progress":
      return "var(--color-brand)";
    case "blocked":
      return "var(--color-danger)";
    default:
      return "var(--color-fg-subtle)";
  }
}

// Title-cases a snake_case status/priority into a human label
// ("awaiting_approval" -> "Awaiting Approval").
export function prettyLabel(value: string): string {
  return value.replace(/_/g, " ").replace(/\b\w/g, (c) => c.toUpperCase());
}

// Humanises a duration in seconds into a compact label ("45s", "30m", "4h",
// "2d"). Used for per-stage timing in reports.
export function humanizeDuration(seconds: number): string {
  if (!Number.isFinite(seconds) || seconds <= 0) return "0s";
  if (seconds < 60) return `${Math.round(seconds)}s`;
  const min = Math.round(seconds / 60);
  if (min < 60) return `${min}m`;
  const hr = Math.round(seconds / 3600);
  if (hr < 24) return `${hr}h`;
  return `${Math.round(seconds / 86400)}d`;
}

// Short relative-time label for activity feeds ("just now", "3m ago",
// "2h ago", "5d ago"), falling back to a date past a week.
export function relativeTime(iso: string): string {
  const then = new Date(iso).getTime();
  if (!Number.isFinite(then)) return "";
  const sec = Math.max(0, Math.floor((Date.now() - then) / 1000));
  if (sec < 60) return "just now";
  const min = Math.floor(sec / 60);
  if (min < 60) return `${min}m ago`;
  const hr = Math.floor(min / 60);
  if (hr < 24) return `${hr}h ago`;
  const day = Math.floor(hr / 24);
  if (day < 7) return `${day}d ago`;
  return new Date(iso).toLocaleDateString();
}

// ── Agent decision outcomes ─────────────────────────────────────────────────

// A decision worth showing on the canvas/desk. "approve" and "pending" are the
// quiet default; the rest are what a reviewer wants to see at a glance.
export function isNotableOutcome(outcome?: DecisionOutcome): boolean {
  return !!outcome && outcome !== "pending" && outcome !== "approve";
}

export function decisionOutcomeLabel(outcome: DecisionOutcome): string {
  switch (outcome) {
    case "approve":
      return "Approved";
    case "approve_with_conditions":
      return "Approved with conditions";
    case "flag":
      return "Flagged";
    case "reject":
      return "Rejected";
    case "block":
      return "Blocked";
    default:
      return "Pending";
  }
}

// Dot color for a flag severity.
export function flagSeverityDot(severity: string): string {
  switch (severity) {
    case "critical":
      return "bg-[var(--color-danger)]";
    case "warning":
      return "bg-[var(--color-warning)]";
    default:
      return "bg-[var(--color-fg-subtle)]";
  }
}

// Text color for a flag severity label.
export function flagSeverityText(severity: string): string {
  switch (severity) {
    case "critical":
      return "text-[var(--color-danger)]";
    case "warning":
      return "text-[var(--color-warning-fg)]";
    default:
      return "text-[var(--color-fg-subtle)]";
  }
}

// Filled badge color for a decision outcome.
export function decisionOutcomeBadgeClass(outcome: DecisionOutcome): string {
  switch (outcome) {
    case "approve":
      return "bg-[var(--color-success)]/12 text-[var(--color-success)]";
    case "approve_with_conditions":
    case "flag":
      return "bg-[var(--color-warning)]/15 text-[var(--color-warning-fg)]";
    case "reject":
      return "bg-[var(--color-danger)]/12 text-[var(--color-danger)]";
    case "block":
      return "bg-[var(--color-accent-bg)] text-[var(--color-brand)]";
    default:
      return "bg-[var(--color-surface-2)] text-[var(--color-fg-muted)]";
  }
}
