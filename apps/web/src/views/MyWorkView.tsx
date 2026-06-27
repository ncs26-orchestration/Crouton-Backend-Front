import { useEffect, useMemo, useRef, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { AlertCircle, CheckCircle2, Inbox, Loader2, ShieldCheck, X } from "lucide-react";

import { api } from "../lib/api";
import { useToasts } from "../components/Toasts";
import { prettyLabel, priorityTextClass, statusBadgeClass } from "../lib/request-format";
import { PageHeader, StatCard, EmptyState as UIEmptyState } from "../components/ui";
import type { ApprovalDecision, OrgRequest } from "../lib/types";

const MAX_JUSTIFICATION_LEN = 2000;
const DECIDED_STATUSES = new Set(["approved", "rejected", "completed"]);

interface Props {
  orgId: string;
  /** The caller's role in the org. Only an approver (admin) may decide. */
  role: string;
  onOpenWorkflow: (requestId: string) => void;
}

export function MyWorkView({ orgId, role, onOpenWorkflow }: Props) {
  const isApprover = role === "admin";
  const [deciding, setDeciding] = useState<OrgRequest | null>(null);

  // Poll while the live SSE stream (F4) is not yet wired, so a request
  // reaching the gate shows up here without a manual refresh.
  const { data, isLoading, error } = useQuery({
    queryKey: ["requests", orgId],
    queryFn: () => api.listRequests(orgId),
    refetchInterval: 4000,
  });

  const requests = useMemo(() => data?.requests ?? [], [data]);
  const pending = useMemo(
    () => requests.filter((r) => r.status === "awaiting_approval"),
    [requests],
  );
  const active = useMemo(
    () => requests.filter((r) => r.status === "submitted" || r.status === "in_progress"),
    [requests],
  );
  const decided = useMemo(
    () => requests.filter((r) => DECIDED_STATUSES.has(r.status)).slice(0, 6),
    [requests],
  );

  return (
    <div className="flex-1 flex flex-col overflow-hidden">
      <PageHeader title="My Work" subtitle="Approvals, work in flight, and recent decisions" />

      <div className="flex-1 overflow-auto px-8 py-6">
        {isLoading && (
          <div className="flex items-center justify-center h-40">
            <div className="size-6 rounded-full border-2 border-[var(--color-brand)] border-t-transparent animate-spin" />
          </div>
        )}

        {error && (
          <div className="flex items-center justify-center h-40 gap-2 text-sm text-[var(--color-danger)]">
            <AlertCircle size={16} />
            Failed to load your work
          </div>
        )}

        {!isLoading && !error && (
          <div className="flex flex-col gap-8 w-full">
            {/* Summary */}
            <div className="grid grid-cols-3 gap-3">
              <StatCard icon={ShieldCheck} label="Pending approvals" value={pending.length} tone={pending.length ? "warning" : "neutral"} />
              <StatCard icon={Loader2} label="In progress" value={active.length} tone="brand" />
              <StatCard icon={CheckCircle2} label="Recently decided" value={decided.length} tone="success" />
            </div>

            <section className="flex flex-col gap-3">
              <SectionHeader
                icon={<ShieldCheck size={14} className="text-[var(--color-brand)]" />}
                title="Pending approvals"
                count={pending.length}
              />

              {pending.length === 0 ? (
                <UIEmptyState icon={Inbox} title="Nothing waiting on you" hint="Requests reaching the executive gate appear here." />
              ) : (
                <div className="grid grid-cols-1 lg:grid-cols-2 gap-3">
                  {pending.map((r) => (
                    <PendingCard
                      key={r.id}
                      request={r}
                      isApprover={isApprover}
                      onOpen={() => onOpenWorkflow(r.id)}
                      onDecide={() => setDeciding(r)}
                    />
                  ))}
                </div>
              )}
            </section>

            {active.length > 0 && (
              <section className="flex flex-col gap-3">
                <SectionHeader
                  icon={<Loader2 size={14} className="text-[var(--color-fg-subtle)]" />}
                  title="In progress"
                  count={active.length}
                />
                <div className="rounded-lg border border-[var(--color-border)] divide-y divide-[var(--color-border)]">
                  {active.map((r) => (
                    <button
                      key={r.id}
                      type="button"
                      onClick={() => onOpenWorkflow(r.id)}
                      className="w-full flex items-center gap-3 px-4 py-3 text-left hover:bg-[var(--color-surface-2)] transition-colors"
                    >
                      <div className="min-w-0 flex-1">
                        <p className="text-sm text-[var(--color-fg)] truncate" style={{ fontFeatureSettings: '"ss01"' }}>
                          {r.title}
                        </p>
                        <p className="text-xs text-[var(--color-fg-muted)] mt-0.5 truncate">
                          {r.requester_name} ·{" "}
                          <span className={`capitalize ${priorityTextClass(r.priority)}`}>{r.priority}</span>
                        </p>
                      </div>
                      <div className="hidden sm:block w-28 shrink-0">
                        <div className="h-1.5 rounded-full bg-[var(--color-surface-3)] overflow-hidden">
                          <div className="h-full rounded-full bg-[var(--color-brand)]" style={{ width: `${r.progress}%` }} />
                        </div>
                      </div>
                      <span className="tnum text-xs text-[var(--color-fg-muted)] w-9 text-right">{r.progress}%</span>
                      <span className={`shrink-0 rounded-md px-2 py-0.5 text-xs font-medium ${statusBadgeClass(r.status)}`}>
                        {prettyLabel(r.status)}
                      </span>
                    </button>
                  ))}
                </div>
              </section>
            )}

            {decided.length > 0 && (
              <section className="flex flex-col gap-3">
                <SectionHeader
                  icon={<CheckCircle2 size={14} className="text-[var(--color-fg-subtle)]" />}
                  title="Recently decided"
                  count={decided.length}
                />
                <div className="rounded-lg border border-[var(--color-border)] divide-y divide-[var(--color-border)]">
                  {decided.map((r) => (
                    <button
                      key={r.id}
                      type="button"
                      onClick={() => onOpenWorkflow(r.id)}
                      className="w-full flex items-center justify-between px-4 py-2.5 text-left hover:bg-[var(--color-surface-2)] transition-colors"
                    >
                      <span
                        className="text-sm text-[var(--color-fg)] truncate"
                        style={{ fontFeatureSettings: '"ss01"' }}
                      >
                        {r.title}
                      </span>
                      <span
                        className={`shrink-0 ml-3 inline-block rounded-md px-2 py-0.5 text-xs font-medium ${statusBadgeClass(r.status)}`}
                      >
                        {prettyLabel(r.status)}
                      </span>
                    </button>
                  ))}
                </div>
              </section>
            )}
          </div>
        )}
      </div>

      {deciding && (
        <ApprovalModal orgId={orgId} request={deciding} onClose={() => setDeciding(null)} />
      )}
    </div>
  );
}

function SectionHeader({
  icon,
  title,
  count,
}: {
  icon: React.ReactNode;
  title: string;
  count: number;
}) {
  return (
    <div className="flex items-center gap-2">
      {icon}
      <h2 className="text-sm font-medium text-[var(--color-fg)]">{title}</h2>
      <span className="text-xs text-[var(--color-fg-subtle)] tabular-nums">{count}</span>
    </div>
  );
}

function PendingCard({
  request: r,
  isApprover,
  onOpen,
  onDecide,
}: {
  request: OrgRequest;
  isApprover: boolean;
  onOpen: () => void;
  onDecide: () => void;
}) {
  const urgent = r.priority === "urgent" || r.priority === "high";
  return (
    <div
      className={`rounded-lg border bg-[var(--color-surface)] p-4 shadow-stripe-ambient ${
        urgent ? "border-[var(--color-warning)]/40" : "border-[var(--color-border)]"
      }`}
    >
      <div className="flex items-start justify-between gap-3">
        <div className="min-w-0">
          <div className="flex items-center gap-2">
            {urgent && (
              <span className="shrink-0 rounded px-1.5 py-0.5 text-[10px] font-semibold uppercase tracking-wide bg-[var(--color-warning)]/15 text-[var(--color-warning-fg)]">
                {r.priority}
              </span>
            )}
            <h3 className="text-sm font-medium text-[var(--color-fg)] truncate" style={{ fontFeatureSettings: '"ss01"' }}>
              {r.title}
            </h3>
          </div>
          <p className="text-xs text-[var(--color-fg-muted)] mt-0.5">
            {r.requester_name || "Unknown"} ·{" "}
            <span className={`capitalize ${priorityTextClass(r.priority)}`}>{r.priority}</span>{" "}
            priority
          </p>
        </div>
        <span
          className={`shrink-0 inline-block rounded-md px-2 py-0.5 text-xs font-medium ${statusBadgeClass(r.status)}`}
        >
          {prettyLabel(r.status)}
        </span>
      </div>

      {r.description && (
        <p className="text-xs text-[var(--color-fg-muted)] mt-2 line-clamp-2">{r.description}</p>
      )}

      <div className="flex items-center gap-2 mt-4">
        {isApprover ? (
          <button
            type="button"
            onClick={onDecide}
            className="px-3 py-1.5 text-sm rounded bg-[var(--color-brand)] text-white font-medium hover:bg-[var(--color-brand-hover)] transition-colors"
            style={{ fontFeatureSettings: '"ss01"' }}
          >
            Review &amp; decide
          </button>
        ) : (
          <span className="text-xs text-[var(--color-fg-subtle)]">
            An approver needs to sign off.
          </span>
        )}
        <button
          type="button"
          onClick={onOpen}
          className="px-3 py-1.5 text-sm rounded border border-[var(--color-border)] text-[var(--color-fg-muted)] hover:bg-[var(--color-surface-2)] transition-colors"
        >
          Open in canvas
        </button>
      </div>
    </div>
  );
}

function ApprovalModal({
  orgId,
  request,
  onClose,
}: {
  orgId: string;
  request: OrgRequest;
  onClose: () => void;
}) {
  const qc = useQueryClient();
  const toasts = useToasts();
  const dialogRef = useRef<HTMLDivElement>(null);
  const textareaRef = useRef<HTMLTextAreaElement>(null);
  const [justification, setJustification] = useState("");
  const [submitting, setSubmitting] = useState<ApprovalDecision | null>(null);

  // Move focus to the justification field when the dialog opens, without an
  // autoFocus attribute (better for screen-reader announcement order).
  useEffect(() => {
    textareaRef.current?.focus();
  }, []);

  const mutation = useMutation({
    mutationFn: (decision: ApprovalDecision) =>
      api.approve(request.id, { decision, justification: justification.trim() }),
    onSuccess: (_data, decision) => {
      qc.invalidateQueries({ queryKey: ["requests", orgId] });
      qc.invalidateQueries({ queryKey: ["request", request.id] });
      toasts.push({
        kind: "success",
        title: decision === "approve" ? "Request approved" : "Request rejected",
      });
      onClose();
    },
    onError: (e: Error) => {
      setSubmitting(null);
      toasts.push({ kind: "error", title: e.message });
    },
  });

  const decide = (decision: ApprovalDecision) => {
    if (!justification.trim() || mutation.isPending) return;
    setSubmitting(decision);
    mutation.mutate(decision);
  };

  // Escape to close + a focus trap so keyboard users stay in the dialog.
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

  const canDecide = justification.trim().length > 0 && !mutation.isPending;

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center">
      <div className="absolute inset-0 bg-black/40" onClick={onClose} />
      <div
        ref={dialogRef}
        role="dialog"
        aria-modal="true"
        aria-labelledby="approval-title"
        className="relative bg-[var(--color-surface)] rounded-lg shadow-stripe-elevated w-full max-w-md p-6 border border-[var(--color-border)]"
      >
        <div className="flex items-start justify-between mb-1">
          <h2
            id="approval-title"
            className="text-base font-medium text-[var(--color-fg)]"
            style={{ fontFeatureSettings: '"ss01"' }}
          >
            Executive decision
          </h2>
          <button
            type="button"
            onClick={onClose}
            aria-label="Close"
            className="text-[var(--color-fg-muted)] hover:text-[var(--color-fg)] transition-colors"
          >
            <X size={18} />
          </button>
        </div>
        <p className="text-sm text-[var(--color-fg-muted)] mb-4">{request.title}</p>

        <label
          htmlFor="approval-justification"
          className="block text-xs font-medium text-[var(--color-fg-label)] mb-1"
        >
          Justification <span className="text-[var(--color-danger)]">*</span>
        </label>
        <textarea
          id="approval-justification"
          ref={textareaRef}
          value={justification}
          maxLength={MAX_JUSTIFICATION_LEN}
          onChange={(e) => setJustification(e.target.value)}
          rows={4}
          placeholder="Explain the decision. This is recorded on the request."
          className="w-full px-3 py-2 text-sm rounded border border-[var(--color-border)] bg-[var(--color-bg)] text-[var(--color-fg)] placeholder:text-[var(--color-fg-subtle)] focus:outline-none focus:ring-2 focus:ring-[var(--color-brand)] focus:border-transparent resize-none"
        />
        <p className="text-xs text-[var(--color-fg-subtle)] mt-1">
          A written reason is required for both approve and reject.
        </p>

        <div className="flex justify-end gap-2 mt-5">
          <button
            type="button"
            onClick={() => decide("reject")}
            disabled={!canDecide}
            className="flex items-center gap-1.5 px-3 py-2 text-sm rounded border border-[var(--color-danger)] text-[var(--color-danger)] font-medium hover:bg-[var(--color-danger)]/10 transition-colors disabled:opacity-50"
          >
            {submitting === "reject" && <Loader2 size={13} className="animate-spin" />}
            Reject
          </button>
          <button
            type="button"
            onClick={() => decide("approve")}
            disabled={!canDecide}
            className="flex items-center gap-1.5 px-3 py-2 text-sm rounded bg-[var(--color-brand)] text-white font-medium hover:bg-[var(--color-brand-hover)] transition-colors disabled:opacity-50"
            style={{ fontFeatureSettings: '"ss01"' }}
          >
            {submitting === "approve" && <Loader2 size={13} className="animate-spin" />}
            Approve
          </button>
        </div>
      </div>
    </div>
  );
}
