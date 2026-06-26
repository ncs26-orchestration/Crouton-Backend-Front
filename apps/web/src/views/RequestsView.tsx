import { useState } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { FileText, Plus, AlertCircle } from "lucide-react";

import { api } from "../lib/api";
import type { OrgRequest, RequestPriority, RequestStatus } from "../lib/types";

const STATUS_COLORS: Record<RequestStatus, string> = {
  submitted: "bg-[var(--color-fg-subtle)]",
  in_progress: "bg-[var(--color-brand)]",
  awaiting_approval: "bg-[#f59e0b]",
  approved: "bg-[#15be53]",
  rejected: "bg-[#ea2261]",
  completed: "bg-[#15be53]",
};

const PRIORITY_LABELS: Record<RequestPriority, { label: string; cls: string }> = {
  low: { label: "Low", cls: "text-[var(--color-fg-subtle)]" },
  medium: { label: "Medium", cls: "text-[var(--color-fg-muted)]" },
  high: { label: "High", cls: "text-[#f59e0b]" },
  urgent: { label: "Urgent", cls: "text-[#ea2261]" },
};

function statusLabel(s: RequestStatus): string {
  return s.replace(/_/g, " ").replace(/\b\w/g, (c) => c.toUpperCase());
}

interface Props {
  orgId: string;
  onOpenWorkflow: (requestId: string) => void;
}

export function RequestsView({ orgId, onOpenWorkflow }: Props) {
  const qc = useQueryClient();
  const [modalOpen, setModalOpen] = useState(false);

  const { data, isLoading, error } = useQuery({
    queryKey: ["requests", orgId],
    queryFn: () => api.listRequests(orgId),
  });

  const requests = data?.requests ?? [];

  return (
    <div className="flex-1 flex flex-col overflow-hidden">
      {/* Header */}
      <div className="shrink-0 px-6 py-4 border-b border-[var(--color-border)] flex items-center justify-between">
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

      {/* Table */}
      <div className="flex-1 overflow-auto">
        {isLoading && (
          <div className="flex items-center justify-center h-40">
            <div className="size-6 rounded-full border-2 border-[var(--color-brand)] border-t-transparent animate-spin" />
          </div>
        )}

        {error && (
          <div className="flex items-center justify-center h-40 gap-2 text-sm text-[#ea2261]">
            <AlertCircle size={16} />
            Failed to load requests
          </div>
        )}

        {!isLoading && !error && requests.length === 0 && (
          <div className="flex flex-col items-center justify-center h-60 gap-3 text-center">
            <div className="size-10 rounded-lg bg-[var(--color-surface-2)] flex items-center justify-center">
              <FileText size={20} className="text-[var(--color-fg-subtle)]" />
            </div>
            <p className="text-sm text-[var(--color-fg-muted)]">No requests yet</p>
            <button
              onClick={() => setModalOpen(true)}
              className="text-sm text-[var(--color-brand)] hover:underline"
            >
              Submit your first request
            </button>
          </div>
        )}

        {requests.length > 0 && (
          <table className="w-full text-sm">
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
              {requests.map((r) => (
                <RequestRow key={r.id} request={r} onClick={() => onOpenWorkflow(r.id)} />
              ))}
            </tbody>
          </table>
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

function RequestRow({ request: r, onClick }: { request: OrgRequest; onClick: () => void }) {
  const pri = PRIORITY_LABELS[r.priority] ?? PRIORITY_LABELS.medium;
  return (
    <tr
      onClick={onClick}
      className="border-b border-[var(--color-border)] hover:bg-[var(--color-surface-2)] cursor-pointer transition-colors"
    >
      <td className="px-6 py-3">
        <span className="font-medium text-[var(--color-fg)]" style={{ fontFeatureSettings: '"ss01"' }}>
          {r.title}
        </span>
      </td>
      <td className="px-4 py-3 text-[var(--color-fg-muted)]">{r.requester_name}</td>
      <td className="px-4 py-3">
        <span className={`text-xs font-medium ${pri.cls}`}>{pri.label}</span>
      </td>
      <td className="px-4 py-3">
        <span className="inline-flex items-center gap-1.5">
          <span className={`size-1.5 rounded-full ${STATUS_COLORS[r.status] ?? ""}`} />
          <span className="text-xs text-[var(--color-fg-muted)]">{statusLabel(r.status)}</span>
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
  const [title, setTitle] = useState("");
  const [description, setDescription] = useState("");
  const [priority, setPriority] = useState<RequestPriority>("medium");

  const mutation = useMutation({
    mutationFn: () =>
      api.createRequest(orgId, { title, description, priority }),
    onSuccess: (data) => onCreated(data.request.id),
  });

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center">
      <div className="absolute inset-0 bg-black/40" onClick={onClose} />
      <div className="relative bg-[var(--color-surface)] rounded-lg shadow-stripe-elevated w-full max-w-md p-6 border border-[var(--color-border)]">
        <h2
          className="text-base font-medium text-[var(--color-fg)] mb-4"
          style={{ fontFeatureSettings: '"ss01"' }}
        >
          New Request
        </h2>

        <div className="flex flex-col gap-3">
          <div>
            <label className="block text-xs font-medium text-[var(--color-fg-label)] mb-1">Title</label>
            <input
              type="text"
              value={title}
              onChange={(e) => setTitle(e.target.value)}
              placeholder="Open a new office in Berlin"
              className="w-full px-3 py-2 text-sm rounded border border-[var(--color-border)] bg-[var(--color-bg)] text-[var(--color-fg)] placeholder:text-[var(--color-fg-subtle)] focus:outline-none focus:ring-2 focus:ring-[var(--color-brand)] focus:border-transparent"
            />
          </div>

          <div>
            <label className="block text-xs font-medium text-[var(--color-fg-label)] mb-1">Description</label>
            <textarea
              value={description}
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

        {mutation.error && (
          <p className="text-xs text-[#ea2261] mt-2">
            {mutation.error instanceof Error ? mutation.error.message : "Failed to create request"}
          </p>
        )}

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
            className="px-3 py-2 text-sm rounded bg-[var(--color-brand)] text-white font-medium hover:bg-[var(--color-brand-hover)] transition-colors disabled:opacity-50"
            style={{ fontFeatureSettings: '"ss01"' }}
          >
            {mutation.isPending ? "Creating..." : "Submit Request"}
          </button>
        </div>
      </div>
    </div>
  );
}
