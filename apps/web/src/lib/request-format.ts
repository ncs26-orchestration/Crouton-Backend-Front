import type { RequestPriority, RequestStatus } from "./types";

// Length caps mirror the server (apps/api handler/requests.go) so the UI
// stops oversized input before it ever hits the API.
export const MAX_REQUEST_TITLE_LEN = 200;
export const MAX_REQUEST_DESCRIPTION_LEN = 5000;

// Shared request status/priority presentation so the list and detail
// views can't drift. Colors come from the status tokens in index.css so
// theming and dark mode stay consistent.

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

export function priorityBadgeClass(priority: RequestPriority): string {
  switch (priority) {
    case "urgent":
      return "bg-[var(--color-danger)]/12 text-[var(--color-danger)]";
    case "high":
      return "bg-[var(--color-warning)]/15 text-[var(--color-warning-fg)]";
    case "medium":
      return "bg-[var(--color-accent-bg)] text-[var(--color-brand)]";
    default:
      return "bg-[var(--color-surface-2)] text-[var(--color-fg-muted)]";
  }
}

// Turns a snake_case status into a human label ("awaiting approval").
export function prettyStatus(status: RequestStatus): string {
  return status.replace(/_/g, " ");
}
