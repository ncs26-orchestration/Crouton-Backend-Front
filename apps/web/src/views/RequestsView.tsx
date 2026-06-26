import { useEffect, useMemo, useRef, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { FileText, Loader2, Plus, X } from "lucide-react";

import { api } from "../lib/api";
import { useOrg } from "../contexts/OrgContext";
import { useToasts } from "../components/Toasts";
import {
  MAX_REQUEST_DESCRIPTION_LEN,
  MAX_REQUEST_TITLE_LEN,
  priorityBadgeClass,
  prettyStatus,
  statusBadgeClass,
} from "../lib/request-format";
import type { OrgRequest, RequestPriority, RequestStatus } from "../lib/types";

const PRIORITIES: RequestPriority[] = ["low", "medium", "high", "urgent"];
const STATUSES: RequestStatus[] = [
  "submitted",
  "in_progress",
  "awaiting_approval",
  "approved",
  "rejected",
  "completed",
];

export function RequestsView({ onOpenRequest }: { onOpenRequest: (id: string) => void }) {
  const { activeOrg } = useOrg();
  const [showCreate, setShowCreate] = useState(false);
  const [statusFilter, setStatusFilter] = useState<RequestStatus | "all">("all");
  const [priorityFilter, setPriorityFilter] = useState<RequestPriority | "all">("all");

  const orgId = activeOrg?.id ?? "";

  const requestsQ = useQuery({
    queryKey: ["requests", orgId],
    queryFn: () => api.listRequests(orgId).then((r) => r.requests),
    enabled: !!orgId,
  });

  const requests = requestsQ.data ?? [];
  const filtered = useMemo(
    () =>
      requests.filter(
        (r) =>
          (statusFilter === "all" || r.status === statusFilter) &&
          (priorityFilter === "all" || r.priority === priorityFilter),
      ),
    [requests, statusFilter, priorityFilter],
  );

  if (!activeOrg) return null;

  return (
    <div className="flex-1 flex flex-col overflow-hidden bg-[var(--color-bg)] text-[var(--color-fg)]">
      {/* Header */}
      <div className="border-b border-[var(--color-border)] px-8 py-5 shrink-0 flex items-start justify-between">
        <div>
          <h1 className="text-xl font-semibold">Requests</h1>
          <p className="text-sm text-[var(--color-fg-muted)] mt-0.5">
            Business requests submitted into {activeOrg.name}
          </p>
        </div>
        <button
          onClick={() => setShowCreate(true)}
          className="flex items-center gap-2 px-3 py-1.5 rounded-lg bg-[var(--color-brand)] text-white text-sm font-medium hover:opacity-90 transition-opacity"
        >
          <Plus size={14} /> New request
        </button>
      </div>

      {/* Filters */}
      <div className="px-8 py-3 border-b border-[var(--color-border)] flex items-center gap-3 shrink-0">
        <Filter
          label="Status"
          value={statusFilter}
          onChange={(v) => setStatusFilter(v as RequestStatus | "all")}
          options={STATUSES}
          render={prettyStatus}
        />
        <Filter
          label="Priority"
          value={priorityFilter}
          onChange={(v) => setPriorityFilter(v as RequestPriority | "all")}
          options={PRIORITIES}
          render={(v) => v}
        />
      </div>

      {/* Body */}
      <div className="flex-1 overflow-y-auto px-8 py-6">
        {requestsQ.isLoading && (
          <div className="flex justify-center py-16">
            <Loader2 size={20} className="animate-spin text-[var(--color-fg-muted)]" />
          </div>
        )}

        {requestsQ.isError && (
          <div className="rounded-lg border border-[#ea2261]/30 bg-[#ea2261]/8 px-4 py-3 text-sm text-[#ea2261]">
            Failed to load requests: {(requestsQ.error as Error).message}
          </div>
        )}

        {!requestsQ.isLoading && !requestsQ.isError && filtered.length === 0 && (
          <div className="flex flex-col items-center gap-3 py-20 text-center">
            <div className="size-14 rounded-2xl bg-[var(--color-accent-bg)] border border-[var(--color-accent-border)] flex items-center justify-center">
              <FileText size={24} className="text-[var(--color-brand)]" strokeWidth={1.5} />
            </div>
            <div>
              <p className="text-sm font-medium">
                {requests.length === 0 ? "No requests yet" : "No requests match these filters"}
              </p>
              <p className="text-xs text-[var(--color-fg-muted)] mt-1">
                {requests.length === 0
                  ? "Submit a request to start a department workflow."
                  : "Try clearing the status or priority filter."}
              </p>
            </div>
          </div>
        )}

        {filtered.length > 0 && (
          <div className="overflow-hidden rounded-xl border border-[var(--color-border)]">
            <table className="w-full text-sm">
              <thead>
                <tr className="bg-[var(--color-surface)] text-left text-xs uppercase tracking-wide text-[var(--color-fg-muted)]">
                  <th className="px-4 py-2.5 font-medium">Title</th>
                  <th className="px-4 py-2.5 font-medium">Requester</th>
                  <th className="px-4 py-2.5 font-medium">Priority</th>
                  <th className="px-4 py-2.5 font-medium">Status</th>
                  <th className="px-4 py-2.5 font-medium w-40">Progress</th>
                </tr>
              </thead>
              <tbody>
                {filtered.map((r) => (
                  <tr
                    key={r.id}
                    onClick={() => onOpenRequest(r.id)}
                    className="border-t border-[var(--color-border)] cursor-pointer hover:bg-[var(--color-surface-2)] transition-colors"
                  >
                    <td className="px-4 py-3 font-medium">{r.title}</td>
                    <td className="px-4 py-3 text-[var(--color-fg-muted)]">{r.requester_name || "—"}</td>
                    <td className="px-4 py-3">
                      <span className={`inline-block rounded-md px-2 py-0.5 text-xs font-medium capitalize ${priorityBadgeClass(r.priority)}`}>
                        {r.priority}
                      </span>
                    </td>
                    <td className="px-4 py-3">
                      <span className={`inline-block rounded-md px-2 py-0.5 text-xs font-medium capitalize ${statusBadgeClass(r.status)}`}>
                        {prettyStatus(r.status)}
                      </span>
                    </td>
                    <td className="px-4 py-3">
                      <div className="flex items-center gap-2">
                        <div className="h-1.5 flex-1 rounded-full bg-[var(--color-surface-2)] overflow-hidden">
                          <div
                            className="h-full rounded-full bg-[var(--color-brand)]"
                            style={{ width: `${r.progress}%` }}
                          />
                        </div>
                        <span className="text-xs text-[var(--color-fg-muted)] tabular-nums w-8 text-right">
                          {r.progress}%
                        </span>
                      </div>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </div>

      {showCreate && (
        <NewRequestModal
          orgId={orgId}
          onClose={() => setShowCreate(false)}
          onCreated={(id) => {
            setShowCreate(false);
            onOpenRequest(id);
          }}
        />
      )}
    </div>
  );
}

function Filter<T extends string>({
  label,
  value,
  onChange,
  options,
  render,
}: {
  label: string;
  value: T | "all";
  onChange: (v: string) => void;
  options: readonly T[];
  render: (v: T) => string;
}) {
  return (
    <label className="flex items-center gap-2 text-xs text-[var(--color-fg-muted)]">
      {label}
      <select
        value={value}
        onChange={(e) => onChange(e.target.value)}
        className="rounded-lg border border-[var(--color-border)] bg-[var(--color-bg)] px-2 py-1 text-sm text-[var(--color-fg)] outline-none focus:border-[var(--color-brand)] transition-colors capitalize"
      >
        <option value="all">All</option>
        {options.map((o) => (
          <option key={o} value={o} className="capitalize">
            {render(o)}
          </option>
        ))}
      </select>
    </label>
  );
}

function NewRequestModal({
  orgId,
  onClose,
  onCreated,
}: {
  orgId: string;
  onClose: () => void;
  onCreated: (id: string) => void;
}) {
  const qc = useQueryClient();
  const toasts = useToasts();
  const dialogRef = useRef<HTMLDivElement>(null);
  const [title, setTitle] = useState("");
  const [description, setDescription] = useState("");
  const [priority, setPriority] = useState<RequestPriority>("medium");

  const createMut = useMutation({
    mutationFn: () =>
      api.createRequest(orgId, { title: title.trim(), description: description.trim(), priority }),
    onSuccess: (res: { request: OrgRequest }) => {
      qc.invalidateQueries({ queryKey: ["requests", orgId] });
      onCreated(res.request.id);
    },
    onError: (e: Error) => toasts.push({ kind: "error", title: e.message }),
  });

  // Escape to close + a simple focus trap so keyboard users stay inside
  // the dialog while it's open.
  useEffect(() => {
    const onKeyDown = (e: KeyboardEvent) => {
      if (e.key === "Escape") {
        e.stopPropagation();
        onClose();
        return;
      }
      if (e.key !== "Tab") return;
      const root = dialogRef.current;
      if (!root) return;
      const focusable = root.querySelectorAll<HTMLElement>(
        'a[href], button:not([disabled]), input, textarea, select, [tabindex]:not([tabindex="-1"])',
      );
      if (focusable.length === 0) return;
      const first = focusable[0]!;
      const last = focusable[focusable.length - 1]!;
      if (e.shiftKey && document.activeElement === first) {
        e.preventDefault();
        last.focus();
      } else if (!e.shiftKey && document.activeElement === last) {
        e.preventDefault();
        first.focus();
      }
    };
    document.addEventListener("keydown", onKeyDown);
    return () => document.removeEventListener("keydown", onKeyDown);
  }, [onClose]);

  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center bg-black/40 px-4"
      onClick={onClose}
    >
      <div
        ref={dialogRef}
        role="dialog"
        aria-modal="true"
        aria-labelledby="new-request-title"
        className="w-full max-w-lg rounded-2xl border border-[var(--color-border)] bg-[var(--color-bg)] p-6 shadow-stripe-elevated"
        onClick={(e) => e.stopPropagation()}
      >
        <div className="flex items-start justify-between mb-4">
          <div>
            <h2 id="new-request-title" className="text-lg font-semibold text-[var(--color-fg)]">New request</h2>
            <p className="text-sm text-[var(--color-fg-muted)] mt-0.5">
              Describe what you need; the right departments get pulled in automatically.
            </p>
          </div>
          <button
            onClick={onClose}
            aria-label="Close"
            className="text-[var(--color-fg-muted)] hover:text-[var(--color-fg)] transition-colors"
          >
            <X size={18} />
          </button>
        </div>

        <div className="flex flex-col gap-3">
          <label className="flex flex-col gap-1">
            <span className="text-xs font-medium text-[var(--color-fg-muted)]">Title</span>
            <input
              autoFocus
              value={title}
              maxLength={MAX_REQUEST_TITLE_LEN}
              onChange={(e) => setTitle(e.target.value)}
              placeholder="Open a new office in Berlin"
              className="rounded-lg border border-[var(--color-border)] bg-[var(--color-surface)] px-3 py-2 text-sm text-[var(--color-fg)] placeholder-[var(--color-fg-muted)] outline-none focus:border-[var(--color-brand)] transition-colors"
            />
          </label>

          <label className="flex flex-col gap-1">
            <span className="text-xs font-medium text-[var(--color-fg-muted)]">Description</span>
            <textarea
              value={description}
              maxLength={MAX_REQUEST_DESCRIPTION_LEN}
              onChange={(e) => setDescription(e.target.value)}
              placeholder="Add any context the departments should know."
              rows={4}
              className="rounded-lg border border-[var(--color-border)] bg-[var(--color-surface)] px-3 py-2 text-sm text-[var(--color-fg)] placeholder-[var(--color-fg-muted)] outline-none focus:border-[var(--color-brand)] transition-colors resize-none"
            />
          </label>

          <label className="flex flex-col gap-1">
            <span className="text-xs font-medium text-[var(--color-fg-muted)]">Priority</span>
            <select
              value={priority}
              onChange={(e) => setPriority(e.target.value as RequestPriority)}
              className="rounded-lg border border-[var(--color-border)] bg-[var(--color-surface)] px-3 py-2 text-sm text-[var(--color-fg)] outline-none focus:border-[var(--color-brand)] transition-colors capitalize"
            >
              {PRIORITIES.map((p) => (
                <option key={p} value={p} className="capitalize">
                  {p}
                </option>
              ))}
            </select>
          </label>
        </div>

        <div className="flex justify-end gap-2 mt-5">
          <button
            onClick={onClose}
            className="px-3 py-1.5 rounded-lg text-sm text-[var(--color-fg-muted)] hover:bg-[var(--color-surface-2)] transition-colors"
          >
            Cancel
          </button>
          <button
            disabled={!title.trim() || createMut.isPending}
            onClick={() => createMut.mutate()}
            className="flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-[var(--color-brand)] text-white text-sm font-medium hover:opacity-90 disabled:opacity-40 transition-opacity"
          >
            {createMut.isPending && <Loader2 size={13} className="animate-spin" />}
            Submit request
          </button>
        </div>
      </div>
    </div>
  );
}
