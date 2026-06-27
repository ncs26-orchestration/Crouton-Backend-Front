import { useEffect, useMemo, useRef, useState } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { AlertCircle, FileText, Loader2, Plus, X } from "lucide-react";

import { api } from "../lib/api";
import { useToasts } from "../components/Toasts";
import {
  MAX_REQUEST_DESCRIPTION_LEN,
  MAX_REQUEST_TITLE_LEN,
  prettyLabel,
  priorityTextClass,
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

interface Props {
  orgId: string;
  onOpenWorkflow: (requestId: string) => void;
}

export function RequestsView({ orgId, onOpenWorkflow }: Props) {
  const qc = useQueryClient();
  const [modalOpen, setModalOpen] = useState(false);
  const [statusFilter, setStatusFilter] = useState<RequestStatus | "all">("all");
  const [priorityFilter, setPriorityFilter] = useState<RequestPriority | "all">("all");

  const { data, isLoading, error } = useQuery({
    queryKey: ["requests", orgId],
    queryFn: () => api.listRequests(orgId),
  });

  const requests = useMemo(() => data?.requests ?? [], [data]);
  const filtered = useMemo(
    () =>
      requests.filter(
        (r) =>
          (statusFilter === "all" || r.status === statusFilter) &&
          (priorityFilter === "all" || r.priority === priorityFilter),
      ),
    [requests, statusFilter, priorityFilter],
  );

  return (
    <div className="flex-1 flex flex-col overflow-hidden">
      {/* Header */}
      <div className="shrink-0 px-4 md:px-6 py-4 border-b border-[var(--color-border)] flex items-center justify-between gap-3">
        <div>
          <h1
            className="text-lg font-medium text-[var(--color-fg)]"
            style={{ fontFeatureSettings: '"ss01"' }}
          >
            Requests
          </h1>
          <p className="text-xs text-[var(--color-fg-muted)] mt-0.5">
            Submit and track business requests across the organization
          </p>
        </div>
        <button
          onClick={() => setModalOpen(true)}
          className="flex items-center gap-1.5 px-3 py-2 rounded bg-[var(--color-brand)] text-white text-sm font-medium hover:bg-[var(--color-brand-hover)] transition-colors"
          style={{ fontFeatureSettings: '"ss01"' }}
        >
          <Plus size={14} strokeWidth={2} />
          New Request
        </button>
      </div>

      {/* Filters */}
      <div className="shrink-0 px-4 md:px-6 py-2.5 border-b border-[var(--color-border)] flex items-center gap-3 flex-wrap">
        <FilterSelect
          label="Status"
          value={statusFilter}
          onChange={(v) => setStatusFilter(v as RequestStatus | "all")}
          options={STATUSES}
        />
        <FilterSelect
          label="Priority"
          value={priorityFilter}
          onChange={(v) => setPriorityFilter(v as RequestPriority | "all")}
          options={PRIORITIES}
        />
      </div>

      {/* Table */}
      <div className="flex-1 overflow-auto">
        {isLoading && (
          <div className="flex items-center justify-center h-40">
            <div className="size-6 rounded-full border-2 border-[var(--color-brand)] border-t-transparent animate-spin" />
          </div>
        )}

        {error && (
          <div className="flex items-center justify-center h-40 gap-2 text-sm text-[var(--color-danger)]">
            <AlertCircle size={16} />
            Failed to load requests
          </div>
        )}

        {!isLoading && !error && filtered.length === 0 && (
          <div className="flex flex-col items-center justify-center h-60 gap-3 text-center">
            <div className="size-10 rounded-lg bg-[var(--color-surface-2)] flex items-center justify-center">
              <FileText size={20} className="text-[var(--color-fg-subtle)]" />
            </div>
            <p className="text-sm text-[var(--color-fg-muted)]">
              {requests.length === 0 ? "No requests yet" : "No requests match these filters"}
            </p>
            {requests.length === 0 && (
              <button
                onClick={() => setModalOpen(true)}
                className="text-sm text-[var(--color-brand)] hover:underline"
              >
                Submit your first request
              </button>
            )}
          </div>
        )}

        {filtered.length > 0 && (
          <>
            {/* Desktop table */}
            <table className="w-full text-sm hidden md:table">
              <thead>
                <tr className="text-left text-xs text-[var(--color-fg-muted)] border-b border-[var(--color-border)]">
                  <th className="px-6 py-2.5 font-medium">Title</th>
                  <th className="px-4 py-2.5 font-medium">Requester</th>
                  <th className="px-4 py-2.5 font-medium">Priority</th>
                  <th className="px-4 py-2.5 font-medium">Status</th>
                  <th className="px-4 py-2.5 font-medium text-right">Progress</th>
                </tr>
              </thead>
              <tbody>
                {filtered.map((r) => (
                  <RequestRow key={r.id} request={r} onClick={() => onOpenWorkflow(r.id)} />
                ))}
              </tbody>
            </table>

            {/* Mobile cards */}
            <div className="md:hidden flex flex-col gap-2 p-4">
              {filtered.map((r) => (
                <button
                  key={r.id}
                  onClick={() => onOpenWorkflow(r.id)}
                  className="flex items-center gap-3 rounded-lg border border-[var(--color-border)] bg-[var(--color-surface)] px-4 py-3 text-left shadow-stripe-ambient hover:border-[var(--color-border-strong)] transition-colors"
                >
                  <div className="min-w-0 flex-1">
                    <p className="text-sm font-medium text-[var(--color-fg)] truncate" style={{ fontFeatureSettings: '"ss01"' }}>
                      {r.title}
                    </p>
                    <p className="text-xs text-[var(--color-fg-muted)] mt-0.5 truncate">
                      {r.requester_name}
                      {r.request_type && r.request_type !== "general" && (
                        <> · <span className="rounded bg-[var(--color-surface-3)] px-1 py-0.5 text-[10px]">{prettyLabel(r.request_type)}</span></>
                      )}
                      <> · <span className={`capitalize ${priorityTextClass(r.priority)}`}>{r.priority}</span></>
                    </p>
                  </div>
                  <div className="flex items-center gap-2 shrink-0">
                    <span className="tnum text-xs text-[var(--color-fg-muted)]">{r.progress}%</span>
                    <span className={`inline-block rounded-md px-2 py-0.5 text-xs font-medium ${statusBadgeClass(r.status)}`}>
                      {prettyLabel(r.status)}
                    </span>
                  </div>
                </button>
              ))}
            </div>
          </>
        )}
      </div>

      {modalOpen && (
        <NewRequestModal
          orgId={orgId}
          onClose={() => setModalOpen(false)}
          onCreated={(id) => {
            setModalOpen(false);
            qc.invalidateQueries({ queryKey: ["requests", orgId] });
            onOpenWorkflow(id);
          }}
        />
      )}
    </div>
  );
}

function FilterSelect<T extends string>({
  label,
  value,
  onChange,
  options,
}: {
  label: string;
  value: T | "all";
  onChange: (v: string) => void;
  options: readonly T[];
}) {
  return (
    <label className="flex items-center gap-1.5 text-xs text-[var(--color-fg-muted)]">
      {label}
      <select
        value={value}
        onChange={(e) => onChange(e.target.value)}
        className="rounded border border-[var(--color-border)] bg-[var(--color-bg)] px-2 py-1 text-xs text-[var(--color-fg)] focus:outline-none focus:ring-2 focus:ring-[var(--color-brand)]"
      >
        <option value="all">All</option>
        {options.map((o) => (
          <option key={o} value={o}>
            {prettyLabel(o)}
          </option>
        ))}
      </select>
    </label>
  );
}

function RequestRow({ request: r, onClick }: { request: OrgRequest; onClick: () => void }) {
  return (
    <tr
      onClick={onClick}
      className="border-b border-[var(--color-border)] hover:bg-[var(--color-surface-2)] cursor-pointer transition-colors"
    >
      <td className="px-6 py-3">
        <div className="flex items-center gap-2">
          <span className="font-medium text-[var(--color-fg)]" style={{ fontFeatureSettings: '"ss01"' }}>
            {r.title}
          </span>
          {r.request_type && r.request_type !== "general" && (
            <span className="rounded bg-[var(--color-surface-3)] px-1.5 py-0.5 text-[10px] font-medium text-[var(--color-fg-muted)]">
              {prettyLabel(r.request_type)}
            </span>
          )}
        </div>
      </td>
      <td className="px-4 py-3 text-[var(--color-fg-muted)]">{r.requester_name}</td>
      <td className="px-4 py-3">
        <span className={`text-xs font-medium capitalize ${priorityTextClass(r.priority)}`}>
          {r.priority}
        </span>
      </td>
      <td className="px-4 py-3">
        <span className={`inline-block rounded-md px-2 py-0.5 text-xs font-medium ${statusBadgeClass(r.status)}`}>
          {prettyLabel(r.status)}
        </span>
      </td>
      <td className="px-4 py-3 text-right text-[var(--color-fg-muted)]">{r.progress}%</td>
    </tr>
  );
}

function NewRequestModal({
  orgId,
  onClose,
  onCreated,
}: {
  orgId: string;
  onClose: () => void;
  onCreated: (requestId: string) => void;
}) {
  const qc = useQueryClient();
  const toasts = useToasts();
  const dialogRef = useRef<HTMLDivElement>(null);
  const [title, setTitle] = useState("");
  const [description, setDescription] = useState("");
  const [priority, setPriority] = useState<RequestPriority>("medium");

  const mutation = useMutation({
    mutationFn: () =>
      api.createRequest(orgId, { title: title.trim(), description: description.trim(), priority }),
    onSuccess: (data) => {
      qc.invalidateQueries({ queryKey: ["requests", orgId] });
      onCreated(data.request.id);
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
    <div className="fixed inset-0 z-50 flex items-center justify-center">
      <div className="absolute inset-0 bg-black/40" onClick={onClose} />
      <div
        ref={dialogRef}
        role="dialog"
        aria-modal="true"
        aria-labelledby="new-request-title"
        className="relative bg-[var(--color-surface)] rounded-lg shadow-stripe-elevated w-full max-w-md p-6 border border-[var(--color-border)]"
      >
        <div className="flex items-start justify-between mb-4">
          <h2
            id="new-request-title"
            className="text-base font-medium text-[var(--color-fg)]"
            style={{ fontFeatureSettings: '"ss01"' }}
          >
            New Request
          </h2>
          <button
            onClick={onClose}
            aria-label="Close"
            className="text-[var(--color-fg-muted)] hover:text-[var(--color-fg)] transition-colors"
          >
            <X size={18} />
          </button>
        </div>

        <div className="flex flex-col gap-3">
          <div>
            <label className="block text-xs font-medium text-[var(--color-fg-label)] mb-1">Title</label>
            <input
              autoFocus
              type="text"
              value={title}
              maxLength={MAX_REQUEST_TITLE_LEN}
              onChange={(e) => setTitle(e.target.value)}
              placeholder="Open a new office in Berlin"
              className="w-full px-3 py-2 text-sm rounded border border-[var(--color-border)] bg-[var(--color-bg)] text-[var(--color-fg)] placeholder:text-[var(--color-fg-subtle)] focus:outline-none focus:ring-2 focus:ring-[var(--color-brand)] focus:border-transparent"
            />
          </div>

          <div>
            <label className="block text-xs font-medium text-[var(--color-fg-label)] mb-1">Description</label>
            <textarea
              value={description}
              maxLength={MAX_REQUEST_DESCRIPTION_LEN}
              onChange={(e) => setDescription(e.target.value)}
              rows={3}
              placeholder="Describe the request..."
              className="w-full px-3 py-2 text-sm rounded border border-[var(--color-border)] bg-[var(--color-bg)] text-[var(--color-fg)] placeholder:text-[var(--color-fg-subtle)] focus:outline-none focus:ring-2 focus:ring-[var(--color-brand)] focus:border-transparent resize-none"
            />
          </div>

          <div>
            <label className="block text-xs font-medium text-[var(--color-fg-label)] mb-1">Priority</label>
            <select
              value={priority}
              onChange={(e) => setPriority(e.target.value as RequestPriority)}
              className="w-full px-3 py-2 text-sm rounded border border-[var(--color-border)] bg-[var(--color-bg)] text-[var(--color-fg)] focus:outline-none focus:ring-2 focus:ring-[var(--color-brand)] focus:border-transparent"
            >
              <option value="low">Low</option>
              <option value="medium">Medium</option>
              <option value="high">High</option>
              <option value="urgent">Urgent</option>
            </select>
          </div>
        </div>

        <div className="flex justify-end gap-2 mt-5">
          <button
            onClick={onClose}
            className="px-3 py-2 text-sm rounded border border-[var(--color-border)] text-[var(--color-fg-muted)] hover:bg-[var(--color-surface-2)] transition-colors"
          >
            Cancel
          </button>
          <button
            onClick={() => mutation.mutate()}
            disabled={!title.trim() || mutation.isPending}
            className="flex items-center gap-1.5 px-3 py-2 text-sm rounded bg-[var(--color-brand)] text-white font-medium hover:bg-[var(--color-brand-hover)] transition-colors disabled:opacity-50"
            style={{ fontFeatureSettings: '"ss01"' }}
          >
            {mutation.isPending && <Loader2 size={13} className="animate-spin" />}
            {mutation.isPending ? "Creating..." : "Submit Request"}
          </button>
        </div>
      </div>
    </div>
  );
}
