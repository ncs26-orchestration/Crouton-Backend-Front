import { useState, useMemo } from "react";
import { useQuery } from "@tanstack/react-query";
import {
  BarChart3,
  Calendar,
  FileText,
  Filter,
  Search,
  RefreshCw,
  CheckCircle2,
  Clock,
  ShieldAlert,
  AlertTriangle,
  Loader2,
  ThumbsUp,
  ThumbsDown,
  Printer,
  X,
  ArrowRight,
} from "lucide-react";

import { api } from "../lib/api";
import { useOrg } from "../contexts/OrgContext";
import {
  relativeTime,
  humanizeDuration,
  decisionOutcomeBadgeClass,
  decisionOutcomeLabel,
  isNotableOutcome,
} from "../lib/request-format";
import type { AuditEvent, FinalReport, ReportStage } from "../lib/types";

export function ReportsView() {
  const { activeOrg } = useOrg();
  const orgId = activeOrg?.id;

  const [search, setSearch] = useState("");
  const [actorFilter, setActorFilter] = useState("");
  const [actionFilter, setActionFilter] = useState("");
  const [selectedReport, setSelectedReport] = useState<string | null>(null);

  const { data, isLoading, error, refetch } = useQuery({
    queryKey: ["org-audit", orgId],
    queryFn: () => api.listOrgAudit(orgId!),
    enabled: !!orgId,
  });
  const { data: requestsData } = useQuery({
    queryKey: ["requests", orgId],
    queryFn: () => api.listRequests(orgId!),
    enabled: !!orgId,
  });
  const { data: reportData, isLoading: reportLoading } = useQuery({
    queryKey: ["report", selectedReport],
    queryFn: () => api.getReport(selectedReport!),
    enabled: !!selectedReport,
  });

  const events = data?.events ?? [];
  const allRequests = requestsData?.requests ?? [];
  const completedRequests = useMemo(
    () => allRequests.filter((r) => r.status === "completed"),
    [allRequests],
  );

  const actors = useMemo(() => Array.from(new Set(events.map((e) => e.actor))).sort(), [events]);
  const actions = useMemo(() => Array.from(new Set(events.map((e) => e.action))).sort(), [events]);

  const filtered = useMemo(() => {
    let out = events;
    if (search) {
      const q = search.toLowerCase();
      out = out.filter(
        (e) =>
          e.actor.toLowerCase().includes(q) ||
          e.action.toLowerCase().includes(q) ||
          e.reason.toLowerCase().includes(q) ||
          e.request_id.toLowerCase().includes(q),
      );
    }
    if (actorFilter) out = out.filter((e) => e.actor === actorFilter);
    if (actionFilter) out = out.filter((e) => e.action === actionFilter);
    return out;
  }, [events, search, actorFilter, actionFilter]);

  return (
    <div className="flex-1 flex flex-col bg-[var(--color-bg)] text-[var(--color-fg)] overflow-auto nice-scroll">
      <div className="border-b border-[var(--color-border)] px-8 py-5 flex items-center justify-between">
        <div>
          <h1 className="text-xl font-medium" style={{ fontFeatureSettings: '"ss01"' }}>
            Reports
          </h1>
          <p className="text-sm text-[var(--color-fg-muted)] mt-0.5">
            Completed workflow reports and the organization-wide audit trail
          </p>
        </div>
        <button
          type="button"
          onClick={() => refetch()}
          className="flex items-center gap-1.5 text-xs text-[var(--color-fg-muted)] hover:text-[var(--color-fg)] transition-colors"
        >
          <RefreshCw size={14} />
          Refresh
        </button>
      </div>

      <div className="px-8 py-6 w-full flex flex-col gap-6">
        {/* Metrics */}
        <div className="grid grid-cols-2 md:grid-cols-3 gap-3">
          <Metric icon={FileText} label="Completed reports" value={completedRequests.length} />
          <Metric icon={BarChart3} label="Audit events" value={events.length} />
          <Metric icon={Clock} label="Requests tracked" value={allRequests.length} />
        </div>

        {/* Completed reports */}
        <section>
          <h2 className="text-xs font-semibold uppercase tracking-wide text-[var(--color-fg-muted)] mb-3">
            Completed reports
          </h2>
          {completedRequests.length === 0 ? (
            <p className="rounded-lg border border-[var(--color-border)] bg-[var(--color-surface)] px-4 py-8 text-center text-xs text-[var(--color-fg-muted)]">
              No completed reports yet. When a request finishes, its report shows up here.
            </p>
          ) : (
            <div className="grid grid-cols-1 md:grid-cols-2 gap-3">
              {completedRequests.map((req) => (
                <button
                  key={req.id}
                  type="button"
                  onClick={() => setSelectedReport(req.id)}
                  className="group flex items-center gap-3 rounded-lg border border-[var(--color-border)] bg-[var(--color-surface)] px-4 py-3 text-left shadow-stripe-ambient transition-colors hover:border-[var(--color-border-strong)]"
                >
                  <div className="size-9 rounded-md bg-[var(--color-success)]/12 flex items-center justify-center shrink-0">
                    <CheckCircle2 size={18} className="text-[var(--color-success)]" />
                  </div>
                  <div className="min-w-0 flex-1">
                    <p className="text-sm font-medium text-[var(--color-fg)] truncate">{req.title}</p>
                    <p className="text-[11px] text-[var(--color-fg-muted)] mt-0.5">
                      {req.requester_name} · {new Date(req.created_at).toLocaleDateString()}
                    </p>
                  </div>
                  <ArrowRight
                    size={15}
                    className="text-[var(--color-fg-subtle)] group-hover:text-[var(--color-brand)] shrink-0"
                  />
                </button>
              ))}
            </div>
          )}
        </section>

        {/* Audit trail */}
        <section>
          <div className="flex items-center gap-3 text-xs mb-3 flex-wrap">
            <div className="flex items-center gap-1.5 text-[var(--color-fg-muted)]">
              <Filter size={13} />
              <span className="font-semibold uppercase tracking-wide">Audit trail</span>
            </div>
            <div className="relative flex-1 min-w-[180px] max-w-xs">
              <Search size={13} className="absolute left-2.5 top-1/2 -translate-y-1/2 text-[var(--color-fg-subtle)]" />
              <input
                type="text"
                aria-label="Search audit events"
                placeholder="Search actor, action, reason, request..."
                value={search}
                onChange={(e) => setSearch(e.target.value)}
                className="w-full pl-8 pr-3 py-1.5 text-xs bg-[var(--color-surface)] border border-[var(--color-border)] rounded-md text-[var(--color-fg)] placeholder:text-[var(--color-fg-subtle)] focus:outline-none focus:ring-1 focus:ring-[var(--color-brand)]"
              />
            </div>
            <select
              value={actorFilter}
              onChange={(e) => setActorFilter(e.target.value)}
              className="px-2 py-1.5 text-xs bg-[var(--color-surface)] border border-[var(--color-border)] rounded-md text-[var(--color-fg)] focus:outline-none focus:ring-1 focus:ring-[var(--color-brand)]"
            >
              <option value="">All actors</option>
              {actors.map((a) => (
                <option key={a} value={a}>{a}</option>
              ))}
            </select>
            <select
              value={actionFilter}
              onChange={(e) => setActionFilter(e.target.value)}
              className="px-2 py-1.5 text-xs bg-[var(--color-surface)] border border-[var(--color-border)] rounded-md text-[var(--color-fg)] focus:outline-none focus:ring-1 focus:ring-[var(--color-brand)]"
            >
              <option value="">All actions</option>
              {actions.map((a) => (
                <option key={a} value={a}>{a.replace(/\./g, " ")}</option>
              ))}
            </select>
            <span className="text-[var(--color-fg-subtle)] ml-auto">
              {filtered.length} of {events.length} events
            </span>
          </div>

          <div className="rounded-lg border border-[var(--color-border)] bg-[var(--color-surface)]">
            {isLoading ? (
              <div className="flex items-center justify-center h-32">
                <div className="size-5 rounded-full border-2 border-[var(--color-brand)] border-t-transparent animate-spin" />
              </div>
            ) : error ? (
              <div className="flex items-center justify-center h-32 text-sm text-[var(--color-danger)]">
                Failed to load audit trail
              </div>
            ) : filtered.length === 0 ? (
              <p className="px-4 py-8 text-center text-xs text-[var(--color-fg-muted)]">
                {events.length === 0 ? "Run a request first; every state change is recorded here." : "No events match your filters."}
              </p>
            ) : (
              <div className="divide-y divide-[var(--color-border)]">
                {filtered.map((e) => (
                  <AuditRow key={e.id} event={e} />
                ))}
              </div>
            )}
          </div>
        </section>
      </div>

      {selectedReport && (
        <ReportModal
          loading={reportLoading}
          report={reportData ?? null}
          onClose={() => setSelectedReport(null)}
        />
      )}
    </div>
  );
}

function Metric({ icon: Icon, label, value }: { icon: typeof FileText; label: string; value: number }) {
  return (
    <div className="rounded-lg border border-[var(--color-border)] bg-[var(--color-surface)] px-4 py-3 shadow-stripe-ambient">
      <div className="flex items-center gap-1.5 text-[11px] uppercase tracking-wide text-[var(--color-fg-muted)]">
        <Icon size={13} className="text-[var(--color-fg-muted)]" />
        {label}
      </div>
      <p className="mt-1.5 text-2xl font-light tnum">{value}</p>
    </div>
  );
}

// --- Report modal (printable) ---

function ReportModal({
  loading,
  report,
  onClose,
}: {
  loading: boolean;
  report: FinalReport | null;
  onClose: () => void;
}) {
  return (
    <div
      className="fixed inset-0 z-50 flex items-start justify-center bg-black/40 p-4 sm:p-8 overflow-auto anim-fade-in"
      data-print-hide
      onClick={onClose}
    >
      <div
        data-print-root
        className="relative w-full max-w-[760px] rounded-lg border border-[var(--color-border)] bg-[var(--color-surface)] shadow-stripe-elevated"
        onClick={(e) => e.stopPropagation()}
      >
        <div
          className="flex items-center justify-between gap-3 px-6 py-4 border-b border-[var(--color-border)]"
          data-print-hide
        >
          <h2 className="text-sm font-medium">Workflow report</h2>
          <div className="flex items-center gap-2">
            <button
              type="button"
              onClick={() => window.print()}
              disabled={!report}
              className="flex items-center gap-1.5 rounded-md bg-[var(--color-brand)] px-3 py-1.5 text-xs font-medium text-white transition-colors hover:bg-[var(--color-brand-hover)] disabled:opacity-50"
            >
              <Printer size={13} />
              Print / Export PDF
            </button>
            <button
              type="button"
              onClick={onClose}
              aria-label="Close report"
              className="size-7 flex items-center justify-center rounded-md text-[var(--color-fg-muted)] hover:bg-[var(--color-surface-2)]"
            >
              <X size={15} />
            </button>
          </div>
        </div>

        {loading || !report ? (
          <div className="flex items-center gap-2 px-6 py-16 justify-center text-xs text-[var(--color-fg-muted)]">
            <Loader2 size={14} className="animate-spin" />
            Loading report...
          </div>
        ) : (
          <ReportBody report={report} />
        )}
      </div>
    </div>
  );
}

function ReportBody({ report }: { report: FinalReport }) {
  // The API omits flags/stages (null) when there are none; normalise to arrays.
  const flags = report.flags ?? [];
  const stages = report.stages ?? [];
  return (
    <div className="px-6 py-5 flex flex-col gap-5">
      {/* Title block */}
      <div>
        <p className="text-[10px] uppercase tracking-wide text-[var(--color-fg-muted)]">Final report</p>
        <h3 className="text-lg font-medium text-[var(--color-fg)] mt-0.5">{report.request.title}</h3>
        {report.request.description && (
          <p className="text-sm text-[var(--color-fg-muted)] leading-relaxed mt-1 max-w-[65ch]">
            {report.request.description}
          </p>
        )}
        <div className="flex flex-wrap gap-x-5 gap-y-1 mt-2 text-xs">
          <Meta label="Requester" value={report.request.requester_name} />
          <Meta label="Priority" value={report.request.priority} capitalize />
          <Meta label="Status" value={report.request.status} capitalize tone="success" />
        </div>
      </div>

      {/* Summary tiles */}
      <div className="grid grid-cols-3 gap-3">
        <SummaryTile label="Stages completed" value={`${report.summary.completed_stages}/${report.summary.total_stages}`} />
        <SummaryTile label="Total time" value={report.summary.total_time_human} />
        <SummaryTile label="Flags raised" value={String(flags.length)} />
      </div>

      {/* Approval */}
      {report.approval && (
        <Section
          title="Executive decision"
          icon={report.approval.decision === "approve" ? ThumbsUp : ThumbsDown}
          iconTone={report.approval.decision === "approve" ? "success" : "danger"}
        >
          <div className="rounded-md bg-[var(--color-surface-2)] px-3 py-2.5">
            <div className="flex items-center gap-2 text-sm">
              <span className="font-medium capitalize">
                {report.approval.decision === "approve" ? "Approved" : "Rejected"}
              </span>
              <span className="text-[var(--color-fg-muted)]">by {report.approval.approved_by}</span>
            </div>
            <p className="text-sm text-[var(--color-fg-muted)] leading-snug mt-1">
              &ldquo;{report.approval.justification}&rdquo;
            </p>
            <p className="text-[10px] text-[var(--color-fg-subtle)] mt-1.5">
              {new Date(report.approval.approved_at).toLocaleString()}
            </p>
          </div>
        </Section>
      )}

      {/* Flags */}
      {flags.length > 0 && (
        <Section title={`Flags (${flags.length})`} icon={AlertTriangle} iconTone="warning">
          <div className="flex flex-col gap-1.5">
            {flags.map((f, i) => (
              <div
                key={`${f.stage_key}-${i}`}
                className="flex items-start gap-2 rounded-md bg-[var(--color-surface-2)] px-3 py-2 text-sm"
              >
                <ShieldAlert
                  size={14}
                  className={`shrink-0 mt-0.5 ${f.severity === "warning" ? "text-[var(--color-warning)]" : "text-[var(--color-brand)]"}`}
                />
                <div className="text-[var(--color-fg-label)]">
                  <span className="font-medium text-[var(--color-fg)]">{f.stage_name}:</span> {f.message}
                </div>
              </div>
            ))}
          </div>
        </Section>
      )}

      {/* Stages timeline */}
      <Section title={`Stages (${stages.length})`} icon={FileText}>
        <div className="flex flex-col">
          {stages.map((s) => (
            <StageRow key={s.key} stage={s} />
          ))}
        </div>
      </Section>

      <p className="text-[10px] text-[var(--color-fg-subtle)] pt-2 border-t border-[var(--color-border)]">
        Generated {new Date().toLocaleString()} · AI Organization OS
      </p>
    </div>
  );
}

function Meta({ label, value, capitalize, tone }: { label: string; value: string; capitalize?: boolean; tone?: "success" }) {
  return (
    <span className="text-[var(--color-fg-muted)]">
      {label}:{" "}
      <span
        className={`font-medium ${tone === "success" ? "text-[var(--color-success)]" : "text-[var(--color-fg)]"} ${capitalize ? "capitalize" : ""}`}
      >
        {value}
      </span>
    </span>
  );
}

function SummaryTile({ label, value }: { label: string; value: string }) {
  return (
    <div className="rounded-md bg-[var(--color-surface-2)] px-3 py-2.5">
      <div className="text-[10px] uppercase tracking-wide text-[var(--color-fg-muted)]">{label}</div>
      <div className="text-base font-medium text-[var(--color-fg)] mt-0.5">{value}</div>
    </div>
  );
}

function Section({
  title,
  icon: Icon,
  iconTone,
  children,
}: {
  title: string;
  icon: typeof FileText;
  iconTone?: "success" | "danger" | "warning";
  children: React.ReactNode;
}) {
  const tone =
    iconTone === "success"
      ? "text-[var(--color-success)]"
      : iconTone === "danger"
        ? "text-[var(--color-danger)]"
        : iconTone === "warning"
          ? "text-[var(--color-warning)]"
          : "text-[var(--color-fg-muted)]";
  return (
    <div className="flex flex-col gap-2">
      <h4 className="text-[11px] uppercase tracking-wide text-[var(--color-fg-muted)] flex items-center gap-1.5">
        <Icon size={12} className={tone} />
        {title}
      </h4>
      {children}
    </div>
  );
}

function StageRow({ stage }: { stage: ReportStage }) {
  const icon = {
    pending: <Clock size={13} className="text-[var(--color-fg-subtle)]" />,
    in_progress: <Loader2 size={13} className="text-[var(--color-brand)] animate-spin" />,
    completed: <CheckCircle2 size={13} className="text-[var(--color-success)]" />,
    blocked: <ShieldAlert size={13} className="text-[var(--color-danger)]" />,
  }[stage.status] ?? <Clock size={13} className="text-[var(--color-fg-subtle)]" />;

  return (
    <div className="flex gap-3 py-2.5 border-b border-[var(--color-border)] last:border-0">
      <div className="mt-0.5 shrink-0">{icon}</div>
      <div className="flex-1 min-w-0">
        <div className="flex items-center gap-2">
          <span className="text-sm font-medium text-[var(--color-fg)]">{stage.name}</span>
          <span className="text-[10px] uppercase tracking-wide text-[var(--color-fg-muted)]">{stage.department}</span>
          {isNotableOutcome(stage.decision_outcome) && stage.decision_outcome && (
            <span className={`rounded px-1.5 py-0.5 text-[10px] font-medium ${decisionOutcomeBadgeClass(stage.decision_outcome)}`}>
              {decisionOutcomeLabel(stage.decision_outcome)}
            </span>
          )}
          {stage.duration_seconds > 0 && (
            <span className="text-[10px] text-[var(--color-fg-subtle)] ml-auto tnum">
              {humanizeDuration(stage.duration_seconds)}
            </span>
          )}
        </div>
        {stage.decision_summary ? (
          <p className="text-xs text-[var(--color-fg)] leading-snug mt-0.5">{stage.decision_summary}</p>
        ) : stage.status_text ? (
          <p className="text-xs text-[var(--color-fg-muted)] leading-snug mt-0.5">{stage.status_text}</p>
        ) : null}
        {stage.tasks.length > 0 && (
          <div className="flex flex-wrap gap-x-3 gap-y-1 mt-1.5">
            {stage.tasks.map((t) => (
              <span key={t.title} className="text-[11px] flex items-center gap-1 text-[var(--color-fg-muted)]">
                {t.status === "completed" ? (
                  <CheckCircle2 size={10} className="text-[var(--color-success)]" />
                ) : (
                  <Clock size={10} className="text-[var(--color-fg-subtle)]" />
                )}
                {t.title}
              </span>
            ))}
          </div>
        )}
      </div>
    </div>
  );
}

// --- Audit trail row ---

const auditActionColor: Record<string, string> = {
  "node.started": "text-blue-600 bg-blue-50",
  "node.completed": "text-green-600 bg-green-50",
  "request.completed": "text-green-600 bg-green-50",
  "agent.fallback": "text-yellow-600 bg-yellow-50",
  "node.blocked": "text-red-600 bg-red-50",
  "node.unblocked": "text-teal-600 bg-teal-50",
  "node.flagged": "text-yellow-600 bg-yellow-50",
  "agent.rejected": "text-red-600 bg-red-50",
  "approval.granted": "text-purple-600 bg-purple-50",
  "approval.rejected": "text-red-600 bg-red-50",
  "request.created": "text-blue-600 bg-blue-50",
};

function AuditRow({ event }: { event: AuditEvent }) {
  const badge = auditActionColor[event.action] ?? "text-gray-600 bg-gray-50";
  return (
    <div className="flex items-start gap-3 py-2.5 px-4 hover:bg-[var(--color-surface-2)] transition-colors">
      <div className={`shrink-0 mt-1.5 size-2 rounded-full ${(badge.split(" ")[0] ?? "").replace("text-", "bg-")}`} />
      <div className="flex-1 min-w-0">
        <div className="flex items-center gap-2 flex-wrap">
          <span className="text-xs font-medium text-[var(--color-fg)]">{event.actor}</span>
          <span className={`text-[10px] px-1.5 py-0.5 rounded font-medium ${badge}`}>
            {event.action.replace(/\./g, " ")}
          </span>
          {event.request_id && (
            <span className="text-[10px] font-mono text-[var(--color-fg-subtle)] truncate">{event.request_id}</span>
          )}
          <span className="text-[10px] text-[var(--color-fg-subtle)] ml-auto flex items-center gap-1">
            <Calendar size={9} />
            {relativeTime(event.created_at)}
          </span>
        </div>
        {event.reason && (
          <p className="text-xs text-[var(--color-fg-muted)] mt-0.5 leading-snug">{event.reason}</p>
        )}
      </div>
    </div>
  );
}
