import { useMemo } from "react";
import { useQuery } from "@tanstack/react-query";
import { Plus, FileText, Loader2, CheckCircle2, Activity, ArrowRight } from "lucide-react";

import { api } from "../lib/api";
import {
  statusBadgeClass,
  prettyLabel,
  priorityTextClass,
  relativeTime,
} from "../lib/request-format";
import type { ShellSection } from "../components/ShellRail";
import type { OrgRequest, RequestStatus } from "../lib/types";

interface Props {
  orgId: string;
  onOpenWorkflow: (requestId: string) => void;
  onNavigate: (section: ShellSection) => void;
}

const ACTIVE_STATUSES: RequestStatus[] = ["submitted", "in_progress", "awaiting_approval"];

export function HomeView({ orgId, onOpenWorkflow, onNavigate }: Props) {
  const requestsQuery = useQuery({
    queryKey: ["requests", orgId],
    queryFn: () => api.listRequests(orgId),
  });
  const auditQuery = useQuery({
    queryKey: ["org-audit", orgId],
    queryFn: () => api.listOrgAudit(orgId),
  });

  const requests = requestsQuery.data?.requests ?? [];
  const events = auditQuery.data?.events ?? [];

  const stats = useMemo(() => {
    const total = requests.length;
    const completed = requests.filter(
      (r) => r.status === "completed" || r.status === "approved",
    ).length;
    const active = requests.filter((r) => ACTIVE_STATUSES.includes(r.status)).length;
    const completionRate = total === 0 ? 0 : Math.round((completed / total) * 100);
    return { total, completed, active, completionRate };
  }, [requests]);

  const recentRequests = useMemo(
    () =>
      [...requests]
        .sort((a, b) => +new Date(b.created_at) - +new Date(a.created_at))
        .slice(0, 6),
    [requests],
  );
  const recentEvents = useMemo(
    () =>
      [...events]
        .sort((a, b) => +new Date(b.created_at) - +new Date(a.created_at))
        .slice(0, 8),
    [events],
  );

  return (
    <div className="flex-1 flex flex-col bg-[var(--color-bg)] text-[var(--color-fg)] overflow-auto nice-scroll">
      <div className="border-b border-[var(--color-border)] px-8 py-5 flex items-center justify-between">
        <div>
          <h1 className="text-xl font-medium" style={{ fontFeatureSettings: '"ss01"' }}>
            Home
          </h1>
          <p className="text-sm text-[var(--color-fg-muted)] mt-0.5">
            What's moving through the organization right now
          </p>
        </div>
        <button
          onClick={() => onNavigate("requests")}
          className="flex items-center gap-1.5 rounded-md bg-[var(--color-brand)] px-3 py-2 text-sm font-medium text-white transition-colors hover:bg-[var(--color-brand-hover)]"
        >
          <Plus size={15} />
          New request
        </button>
      </div>

      <div className="px-8 py-6 flex flex-col gap-6 w-full">
        <div className="grid grid-cols-2 md:grid-cols-4 gap-3">
          <StatTile icon={FileText} label="Total requests" value={stats.total} tone="neutral" />
          <StatTile icon={Loader2} label="In progress" value={stats.active} tone="brand" />
          <StatTile icon={CheckCircle2} label="Completed" value={stats.completed} tone="success" />
          <StatTile icon={Activity} label="Completion rate" value={`${stats.completionRate}%`} tone="neutral" />
        </div>

        <div className="grid grid-cols-1 lg:grid-cols-5 gap-6">
          <section className="lg:col-span-3 rounded-lg border border-[var(--color-border)] bg-[var(--color-surface)] shadow-stripe-ambient">
            <header className="flex items-center justify-between px-4 py-3 border-b border-[var(--color-border)]">
              <h2 className="text-sm font-medium">Recent requests</h2>
              <button
                onClick={() => onNavigate("requests")}
                className="flex items-center gap-1 text-xs text-[var(--color-brand)] hover:underline"
              >
                View all
                <ArrowRight size={12} />
              </button>
            </header>
            {requestsQuery.isLoading ? (
              <ListSkeleton rows={4} />
            ) : recentRequests.length === 0 ? (
              <EmptyHint text="No requests yet. Create the first one to start a workflow." />
            ) : (
              <ul className="divide-y divide-[var(--color-border)]">
                {recentRequests.map((r) => (
                  <RequestRow key={r.id} request={r} onOpen={() => onOpenWorkflow(r.id)} />
                ))}
              </ul>
            )}
          </section>

          <section className="lg:col-span-2 rounded-lg border border-[var(--color-border)] bg-[var(--color-surface)] shadow-stripe-ambient">
            <header className="px-4 py-3 border-b border-[var(--color-border)]">
              <h2 className="text-sm font-medium">Recent activity</h2>
            </header>
            {auditQuery.isLoading ? (
              <ListSkeleton rows={5} />
            ) : recentEvents.length === 0 ? (
              <EmptyHint text="Agent decisions and handoffs will show up here." />
            ) : (
              <ul className="px-4 py-3 flex flex-col gap-3">
                {recentEvents.map((e) => (
                  <li key={e.id} className="flex gap-2.5 text-xs">
                    <span className="mt-1 size-1.5 shrink-0 rounded-full bg-[var(--color-brand)]" />
                    <div className="min-w-0">
                      <p className="text-[var(--color-fg)] leading-snug">
                        <span className="font-medium">{e.actor}</span>{" "}
                        <span className="text-[var(--color-fg-muted)]">{prettyLabel(e.action)}</span>
                      </p>
                      {e.reason && (
                        <p className="text-[var(--color-fg-muted)] leading-snug truncate">{e.reason}</p>
                      )}
                      <p className="text-[10px] text-[var(--color-fg-subtle)] mt-0.5">
                        {relativeTime(e.created_at)}
                      </p>
                    </div>
                  </li>
                ))}
              </ul>
            )}
          </section>
        </div>
      </div>
    </div>
  );
}

function StatTile({
  icon: Icon,
  label,
  value,
  tone,
}: {
  icon: typeof FileText;
  label: string;
  value: string | number;
  tone: "neutral" | "brand" | "success";
}) {
  const toneClass =
    tone === "brand"
      ? "text-[var(--color-brand)]"
      : tone === "success"
        ? "text-[var(--color-success)]"
        : "text-[var(--color-fg-muted)]";
  return (
    <div className="rounded-lg border border-[var(--color-border)] bg-[var(--color-surface)] px-4 py-3 shadow-stripe-ambient">
      <div className="flex items-center gap-1.5 text-[11px] uppercase tracking-wide text-[var(--color-fg-muted)]">
        <Icon size={13} className={toneClass} />
        {label}
      </div>
      <p className="mt-1.5 text-2xl font-light tnum">{value}</p>
    </div>
  );
}

function RequestRow({ request, onOpen }: { request: OrgRequest; onOpen: () => void }) {
  return (
    <li>
      <button
        onClick={onOpen}
        className="w-full flex items-center gap-3 px-4 py-3 text-left transition-colors hover:bg-[var(--color-surface-2)]"
      >
        <div className="min-w-0 flex-1">
          <p className="text-sm text-[var(--color-fg)] truncate">{request.title}</p>
          <p className="text-xs text-[var(--color-fg-muted)] mt-0.5 truncate">
            {request.requester_name} ·{" "}
            <span className={priorityTextClass(request.priority)}>{prettyLabel(request.priority)}</span> ·{" "}
            {relativeTime(request.created_at)}
          </p>
        </div>
        <span className="tnum text-xs text-[var(--color-fg-muted)] w-9 text-right">{request.progress}%</span>
        <span
          className={`shrink-0 rounded-full px-2 py-0.5 text-[11px] font-medium ${statusBadgeClass(request.status)}`}
        >
          {prettyLabel(request.status)}
        </span>
      </button>
    </li>
  );
}

function ListSkeleton({ rows }: { rows: number }) {
  return (
    <div className="px-4 py-3 flex flex-col gap-3">
      {Array.from({ length: rows }).map((_, i) => (
        <div key={i} className="h-8 rounded bg-[var(--color-surface-2)] animate-pulse" />
      ))}
    </div>
  );
}

function EmptyHint({ text }: { text: string }) {
  return <p className="px-4 py-8 text-center text-xs text-[var(--color-fg-muted)]">{text}</p>;
}
