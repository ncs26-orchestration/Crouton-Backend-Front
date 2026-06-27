import { useMemo } from "react";
import { useQuery } from "@tanstack/react-query";
import { Plus, FileText, ArrowRight } from "lucide-react";

import { api } from "../lib/api";
import { statusBadgeClass, prettyLabel, priorityTextClass, relativeTime } from "../lib/request-format";
import { PageHeader, SectionCard, ProgressRing, EmptyState } from "../components/ui";
import type { ShellSection } from "../components/ShellRail";
import type { OrgRequest, RequestStatus } from "../lib/types";

interface Props {
  orgId: string;
  onOpenWorkflow: (requestId: string) => void;
  onNavigate: (section: ShellSection) => void;
}

// The pipeline a request flows through, in order, for the funnel.
const PIPELINE: { key: RequestStatus[]; label: string; tone: string }[] = [
  { key: ["submitted"], label: "Submitted", tone: "var(--color-fg-subtle)" },
  { key: ["in_progress"], label: "In progress", tone: "var(--color-brand)" },
  { key: ["awaiting_approval"], label: "Awaiting approval", tone: "var(--color-warning)" },
  { key: ["approved", "completed"], label: "Completed", tone: "var(--color-success)" },
];

export function HomeView({ orgId, onOpenWorkflow, onNavigate }: Props) {
  const requestsQuery = useQuery({ queryKey: ["requests", orgId], queryFn: () => api.listRequests(orgId) });
  const auditQuery = useQuery({ queryKey: ["org-audit", orgId], queryFn: () => api.listOrgAudit(orgId) });

  const requests = requestsQuery.data?.requests ?? [];
  const events = auditQuery.data?.events ?? [];

  const stats = useMemo(() => {
    const total = requests.length;
    const completed = requests.filter((r) => r.status === "completed" || r.status === "approved").length;
    const active = requests.filter((r) => r.status === "in_progress" || r.status === "submitted").length;
    const awaiting = requests.filter((r) => r.status === "awaiting_approval").length;
    const rate = total === 0 ? 0 : Math.round((completed / total) * 100);
    return { total, completed, active, awaiting, rate };
  }, [requests]);

  const funnel = useMemo(() => {
    const max = Math.max(1, requests.length);
    return PIPELINE.map((s) => {
      const count = requests.filter((r) => s.key.includes(r.status)).length;
      return { ...s, count, pct: Math.round((count / max) * 100) };
    });
  }, [requests]);

  const recentRequests = useMemo(
    () => [...requests].sort((a, b) => +new Date(b.created_at) - +new Date(a.created_at)).slice(0, 6),
    [requests],
  );
  const recentEvents = useMemo(
    () => [...events].sort((a, b) => +new Date(b.created_at) - +new Date(a.created_at)).slice(0, 9),
    [events],
  );

  return (
    <div className="flex-1 flex flex-col bg-[var(--color-bg)] text-[var(--color-fg)] overflow-auto nice-scroll">
      <PageHeader
        title="Home"
        subtitle="What's moving through the organization right now"
        actions={
          <button
            onClick={() => onNavigate("requests")}
            className="flex items-center gap-1.5 rounded-md bg-[var(--color-brand)] px-3 py-2 text-sm font-medium text-white transition-colors hover:bg-[var(--color-brand-hover)]"
          >
            <Plus size={15} /> New request
          </button>
        }
      />

      <div className="px-8 py-6 flex flex-col gap-6 w-full">
        {/* Hero: completion ring + pipeline funnel */}
        <div className="grid grid-cols-1 lg:grid-cols-3 gap-4">
          <SectionCard bodyClassName="p-5 flex items-center gap-5 w-full">
            <ProgressRing value={stats.rate} label="complete" />
            <div className="min-w-0">
              <p className="text-[11px] uppercase tracking-wide text-[var(--color-fg-muted)]">Completion rate</p>
              <p className="text-sm text-[var(--color-fg-muted)] mt-1 leading-snug">
                <span className="text-[var(--color-fg)] font-medium tnum">{stats.completed}</span> of{" "}
                <span className="text-[var(--color-fg)] font-medium tnum">{stats.total}</span> requests done
              </p>
              <p className="text-xs text-[var(--color-fg-subtle)] mt-1">
                {stats.active} in progress · {stats.awaiting} awaiting approval
              </p>
            </div>
          </SectionCard>

          <SectionCard title="Pipeline" className="lg:col-span-2" bodyClassName="px-4 py-4 flex flex-col gap-3">
            {funnel.map((s, i) => (
              <button key={s.label} onClick={() => onNavigate("requests")} className="group flex items-center gap-3 text-left">
                <span className="w-32 shrink-0 text-xs text-[var(--color-fg-muted)] group-hover:text-[var(--color-fg)] flex items-center gap-2">
                  <span className="size-2 rounded-full shrink-0" style={{ background: s.tone }} />
                  {s.label}
                </span>
                <span className="flex-1 h-2.5 rounded-full bg-[var(--color-surface-3)] overflow-hidden">
                  <span
                    className="block h-full rounded-full"
                    style={{ width: `${s.pct}%`, background: s.tone, transition: `width 500ms cubic-bezier(0.16,1,0.3,1) ${i * 60}ms` }}
                  />
                </span>
                <span className="w-7 text-right text-sm tnum text-[var(--color-fg)]">{s.count}</span>
              </button>
            ))}
          </SectionCard>
        </div>

        {/* Recent requests + activity */}
        <div className="grid grid-cols-1 lg:grid-cols-5 gap-6">
          <SectionCard
            title="Recent requests"
            className="lg:col-span-3"
            bodyClassName=""
            action={
              <button onClick={() => onNavigate("requests")} className="flex items-center gap-1 text-xs text-[var(--color-brand)] hover:underline">
                View all <ArrowRight size={12} />
              </button>
            }
          >
            {requestsQuery.isLoading ? (
              <ListSkeleton rows={4} />
            ) : recentRequests.length === 0 ? (
              <EmptyState icon={FileText} title="No requests yet" hint="Create the first one to start a workflow." />
            ) : (
              <ul className="divide-y divide-[var(--color-border)]">
                {recentRequests.map((r) => (
                  <RequestRow key={r.id} request={r} onOpen={() => onOpenWorkflow(r.id)} />
                ))}
              </ul>
            )}
          </SectionCard>

          <SectionCard title="Recent activity" className="lg:col-span-2" bodyClassName="px-4 py-3">
            {auditQuery.isLoading ? (
              <ListSkeleton rows={6} />
            ) : recentEvents.length === 0 ? (
              <p className="py-8 text-center text-xs text-[var(--color-fg-muted)]">Agent decisions show up here.</p>
            ) : (
              <ol className="relative flex flex-col gap-3 before:absolute before:left-[3px] before:top-1.5 before:bottom-1.5 before:w-px before:bg-[var(--color-border)]">
                {recentEvents.map((e) => (
                  <li key={e.id} className="relative flex gap-3 text-xs pl-4">
                    <span className="absolute left-0 top-1 size-1.5 rounded-full bg-[var(--color-brand)] ring-2 ring-[var(--color-surface)]" />
                    <div className="min-w-0">
                      <p className="leading-snug">
                        <span className="font-medium text-[var(--color-fg)]">{e.actor}</span>{" "}
                        <span className="text-[var(--color-fg-muted)]">{prettyLabel(e.action)}</span>
                      </p>
                      {e.reason && <p className="text-[var(--color-fg-muted)] leading-snug truncate">{e.reason}</p>}
                      <p className="text-[10px] text-[var(--color-fg-subtle)] mt-0.5">{relativeTime(e.created_at)}</p>
                    </div>
                  </li>
                ))}
              </ol>
            )}
          </SectionCard>
        </div>
      </div>
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
        <span className={`shrink-0 rounded-full px-2 py-0.5 text-[11px] font-medium ${statusBadgeClass(request.status)}`}>
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
