import type { NodeStatus, RequestPriority, RequestStatus } from "./types";

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
