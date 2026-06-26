import { useState, useMemo } from "react";
import { useQuery } from "@tanstack/react-query";
import {
  BarChart3,
  Calendar,
  FileText,
  Filter,
  Search,
  RefreshCw,
  ChevronRight,
  CheckCircle2,
  Clock,
  ShieldAlert,
  AlertTriangle,
  Loader2,
  User,
  ThumbsUp,
  ThumbsDown,
  X,
} from "lucide-react";

import { api } from "../lib/api";
import { useOrg } from "../contexts/OrgContext";
import type { AuditEvent, FinalReport, ReportStage } from "../lib/types";

export function ReportsView() {
  const { activeOrg } = useOrg();
  const orgId = activeOrg?.id;

  const [search, setSearch] = useState("");
  const [actorFilter, setActorFilter] = useState("");
  const [actionFilter, setActionFilter] = useState("");
  const [selectedReport, setSelectedReport] = useState<string | null>(null);

  // Audit trail query.
  const { data, isLoading, error, refetch } = useQuery({
    queryKey: ["org-audit", orgId],
    queryFn: () => api.listOrgAudit(orgId!),
    enabled: !!orgId,
  });

  // Completed requests query (for reports).
  const { data: requestsData } = useQuery({
    queryKey: ["org-requests", orgId],
    queryFn: () => api.listRequests(orgId!),
    enabled: !!orgId,
  });

  // Report data for the selected request.
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

  const actors = useMemo(() => {
    const s = new Set(events.map((e) => e.actor));
    return Array.from(s).sort();
  }, [events]);

  const actions = useMemo(() => {
    const s = new Set(events.map((e) => e.action));
    return Array.from(s).sort();
  }, [events]);

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
    if (actorFilter) {
      out = out.filter((e) => e.actor === actorFilter);
    }
    if (actionFilter) {
      out = out.filter((e) => e.action === actionFilter);
    }
    return out;
  }, [events, search, actorFilter, actionFilter]);

  return (
    <div className="flex-1 flex flex-col bg-[var(--color-bg)] text-[var(--color-fg)]">
      {/* Header */}
      <div className="border-b border-[var(--color-border)] px-8 py-5">
        <div className="flex items-center justify-between">
          <div>
            <h1
              className="text-xl font-semibold text-[var(--color-fg)]"
              style={{ fontFeatureSettings: '"ss01"' }}
            >
              Reports
            </h1>
            <p className="text-sm text-[var(--color-fg-muted)] mt-0.5">
              Completed reports and audit trail
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
      </div>

      <div className="flex-1 overflow-auto">
        {/* Completed Requests / Reports section */}
        {completedRequests.length > 0 && (
          <div className="border-b border-[var(--color-border)]">
            <div className="px-8 py-4 flex flex-col gap-3">
              <h2 className="text-xs font-semibold uppercase tracking-wide text-[var(--color-fg-muted)] flex items-center gap-1.5">
                <FileText size={13} />
                Completed Reports
              </h2>
              {completedRequests.map((req) => (
                <div key={req.id}>
                  <button
                    type="button"
                    onClick={() =>
                      setSelectedReport(
                        selectedReport === req.id ? null : req.id,
                      )
                    }
                    className="w-full flex items-center gap-3 px-3 py-2.5 rounded-md hover:bg-[var(--color-surface-2)] transition-colors text-left"
                  >
                    <CheckCircle2
                      size={16}
                      className="text-[var(--color-success)] shrink-0"
                    />
                    <div className="flex-1 min-w-0">
                      <div className="text-sm font-medium text-[var(--color-fg)] truncate">
                        {req.title}
                      </div>
                      <div className="text-[11px] text-[var(--color-fg-muted)] mt-0.5">
                        {req.requester_name} &middot;{" "}
                        {new Date(req.created_at).toLocaleDateString()}
                      </div>
                    </div>
                    <ChevronRight
                      size={14}
                      className={`text-[var(--color-fg-subtle)] transition-transform ${
                        selectedReport === req.id ? "rotate-90" : ""
                      }`}
                    />
                  </button>

                  {/* Report content */}
                  {selectedReport === req.id && (
                    <div className="ml-3 pl-4 border-l-2 border-[var(--color-brand)] mt-1 mb-2">
                      {reportLoading ? (
                        <div className="flex items-center gap-2 px-3 py-4 text-xs text-[var(--color-fg-muted)]">
                          <Loader2
                            size={14}
                            className="animate-spin"
                          />
                          Loading report...
                        </div>
                      ) : reportData ? (
                        <ReportDetail report={reportData} />
                      ) : null}
                    </div>
                  )}
                </div>
              ))}
            </div>
          </div>
        )}

        {/* Audit trail header */}
        <div className="border-b border-[var(--color-border)] px-8 py-3 flex items-center gap-3 text-xs">
          <div className="flex items-center gap-1.5 text-[var(--color-fg-muted)]">
            <Filter size={13} />
            <span>Filters</span>
          </div>

          <div className="relative flex-1 max-w-xs">
            <Search
              size={13}
              className="absolute left-2.5 top-1/2 -translate-y-1/2 text-[var(--color-fg-subtle)]"
            />
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

        {/* Audit trail content */}
        <div className="flex-1">
          {isLoading ? (
            <div className="flex items-center justify-center h-32">
              <div className="size-5 rounded-full border-2 border-[var(--color-brand)] border-t-transparent animate-spin" />
            </div>
          ) : error ? (
            <div className="flex items-center justify-center h-32 text-sm text-[var(--color-danger)]">
              Failed to load audit trail
            </div>
          ) : filtered.length === 0 ? (
            <div className="flex flex-col items-center justify-center gap-4 px-8 py-16">
              <div className="size-14 rounded-2xl bg-[var(--color-accent-bg)] border border-[var(--color-accent-border)] flex items-center justify-center">
                <BarChart3 size={24} className="text-[var(--color-brand)]" strokeWidth={1.5} />
              </div>
              <div className="text-center">
                <p className="text-sm font-medium text-[var(--color-fg)]">No audit events found</p>
                <p className="text-xs text-[var(--color-fg-muted)] mt-1">
                  {events.length === 0
                    ? "Run a request first — every state change is recorded here"
                    : "Try adjusting your filters"}
                </p>
              </div>
            </div>
          ) : (
            <div className="px-8 py-4">
              <div className="flex flex-col gap-1">
                {filtered.map((e: AuditEvent) => (
                  <AuditRow key={e.id} event={e} />
                ))}
              </div>
            </div>
          )}
        </div>
      </div>
    </div>
  );
}

// --- Report Detail Component ---

function ReportDetail({ report }: { report: FinalReport }) {
  return (
    <div className="flex flex-col gap-3 px-3 py-3 text-xs">
      {/* Request overview */}
      <div className="flex flex-col gap-1.5">
        <h4 className="text-[10px] uppercase tracking-wide text-[var(--color-fg-muted)]">
          Request
        </h4>
        <div className="text-sm font-medium text-[var(--color-fg)]">
          {report.request.title}
        </div>
        {report.request.description && (
          <p className="text-[var(--color-fg-muted)] leading-snug">
            {report.request.description}
          </p>
        )}
        <div className="flex flex-wrap gap-x-4 gap-y-1 mt-1">
          <span className="text-[var(--color-fg-muted)]">
            Priority: <span className="text-[var(--color-fg)] font-medium capitalize">{report.request.priority}</span>
          </span>
          <span className="text-[var(--color-fg-muted)]">
            Requester: <span className="text-[var(--color-fg)] font-medium">{report.request.requester_name}</span>
          </span>
          <span className="text-[var(--color-fg-muted)]">
            Status: <span className="text-[var(--color-success)] font-medium capitalize">{report.request.status}</span>
          </span>
        </div>
      </div>

      {/* Summary */}
      <div className="flex flex-wrap gap-3">
        <div className="flex-1 min-w-[100px] bg-[var(--color-surface-2)] rounded-md px-2.5 py-2">
          <div className="text-[10px] text-[var(--color-fg-muted)] uppercase">Stages</div>
          <div className="text-sm font-semibold text-[var(--color-fg)]">
            {report.summary.completed_stages}/{report.summary.total_stages}
          </div>
        </div>
        <div className="flex-1 min-w-[100px] bg-[var(--color-surface-2)] rounded-md px-2.5 py-2">
          <div className="text-[10px] text-[var(--color-fg-muted)] uppercase">Total Time</div>
          <div className="text-sm font-semibold text-[var(--color-fg)]">
            {report.summary.total_time_human}
          </div>
        </div>
        <div className="flex-1 min-w-[100px] bg-[var(--color-surface-2)] rounded-md px-2.5 py-2">
          <div className="text-[10px] text-[var(--color-fg-muted)] uppercase">Flags</div>
          <div className="text-sm font-semibold text-[var(--color-fg)]">
            {report.flags.length}
          </div>
        </div>
      </div>

      {/* Approval info */}
      {report.approval && (
        <div className="flex flex-col gap-1.5">
          <h4 className="text-[10px] uppercase tracking-wide text-[var(--color-fg-muted)] flex items-center gap-1">
            {report.approval.decision === "approve" ? (
              <ThumbsUp size={11} className="text-[var(--color-success)]" />
            ) : (
              <ThumbsDown size={11} className="text-[var(--color-danger)]" />
            )}
            Approval
          </h4>
          <div className="bg-[var(--color-surface-2)] rounded-md px-2.5 py-2">
            <div className="flex items-center gap-2 mb-1">
              <span className="font-medium text-[var(--color-fg)] capitalize">
                {report.approval.decision === "approve" ? "Approved" : "Rejected"}
              </span>
              <span className="text-[var(--color-fg-muted)]">by {report.approval.approved_by}</span>
            </div>
            <p className="text-[var(--color-fg-muted)] leading-snug">
              &ldquo;{report.approval.justification}&rdquo;
            </p>
            <div className="flex items-center gap-1 mt-1 text-[10px] text-[var(--color-fg-subtle)]">
              <Calendar size={9} />
              {new Date(report.approval.approved_at).toLocaleString()}
            </div>
          </div>
        </div>
      )}

      {/* Flags */}
      {report.flags.length > 0 && (
        <div className="flex flex-col gap-1.5">
          <h4 className="text-[10px] uppercase tracking-wide text-[var(--color-fg-muted)] flex items-center gap-1">
            <AlertTriangle size={11} className="text-[var(--color-warning)]" />
            Flags ({report.flags.length})
          </h4>
          {report.flags.map((f, i) => (
            <div
              key={`${f.stage_key}-${i}`}
              className={`flex items-start gap-1.5 rounded-md px-2.5 py-1.5 ${
                f.severity === "warning"
                  ? "bg-amber-50 text-amber-800"
                  : "bg-blue-50 text-blue-800"
              }`}
            >
              {f.severity === "warning" ? (
                <ShieldAlert size={12} className="shrink-0 mt-0.5" />
              ) : (
                <AlertTriangle size={12} className="shrink-0 mt-0.5" />
              )}
              <div>
                <span className="font-medium">{f.stage_name}:</span>{" "}
                {f.message}
              </div>
            </div>
          ))}
        </div>
      )}

      {/* Stages */}
      <div className="flex flex-col gap-1.5">
        <h4 className="text-[10px] uppercase tracking-wide text-[var(--color-fg-muted)] flex items-center gap-1">
          <FileText size={11} />
          Stages ({report.stages.length})
        </h4>
        <div className="flex flex-col gap-1">
          {report.stages.map((s) => (
            <StageCard key={s.key} stage={s} />
          ))}
        </div>
      </div>
    </div>
  );
}

function StageCard({ stage }: { stage: ReportStage }) {
  const statusIcon = {
    pending: <Clock size={12} className="text-[var(--color-fg-subtle)] shrink-0 mt-0.5" />,
    in_progress: (
      <Loader2 size={12} className="text-[var(--color-brand)] animate-spin shrink-0 mt-0.5" />
    ),
    completed: (
      <CheckCircle2 size={12} className="text-[var(--color-success)] shrink-0 mt-0.5" />
    ),
    blocked: (
      <ShieldAlert size={12} className="text-[var(--color-danger)] shrink-0 mt-0.5" />
    ),
  }[stage.status] ?? (
    <Clock size={12} className="text-[var(--color-fg-subtle)] shrink-0 mt-0.5" />
  );

  const borderColor = {
    pending: "border-[var(--color-fg-subtle)]",
    in_progress: "border-[var(--color-brand)]",
    completed: "border-[var(--color-success)]",
    blocked: "border-[var(--color-danger)]",
  }[stage.status] ?? "border-[var(--color-fg-subtle)]";

  return (
    <div className={`border-l-2 ${borderColor} pl-2.5 py-1.5`}>
      <div className="flex items-start gap-1.5">
        {statusIcon}
        <div className="flex-1 min-w-0">
          <div className="flex items-center gap-2">
            <span className="text-xs font-medium text-[var(--color-fg)]">
              {stage.name}
            </span>
            <span className="text-[10px] text-[var(--color-fg-muted)]">
              {stage.department}
            </span>
            {stage.duration_seconds > 0 && (
              <span className="text-[10px] text-[var(--color-fg-subtle)] ml-auto font-mono">
                {stage.duration_seconds}s
              </span>
            )}
          </div>
          {stage.status_text && (
            <p className="text-[11px] text-[var(--color-fg-muted)] mt-0.5 leading-snug">
              {stage.status_text}
            </p>
          )}
          {stage.tasks.length > 0 && (
            <div className="flex flex-wrap gap-x-3 gap-y-0.5 mt-1">
              {stage.tasks.map((t, i) => (
                <span
                  key={t.title}
                  className="text-[10px] flex items-center gap-0.5"
                >
                  {t.status === "completed" ? (
                    <CheckCircle2 size={9} className="text-[var(--color-success)]" />
                  ) : (
                    <Clock size={9} className="text-[var(--color-fg-subtle)]" />
                  )}
                  <span className="text-[var(--color-fg-muted)]">{t.title}</span>
                </span>
              ))}
            </div>
          )}
        </div>
      </div>
    </div>
  );
}

// --- Audit Trail Components ---

const auditActionColor: Record<string, string> = {
  "node.started": "text-blue-600 bg-blue-50",
  "node.completed": "text-green-600 bg-green-50",
  "request.completed": "text-green-600 bg-green-50",
  "agent.fallback": "text-yellow-600 bg-yellow-50",
  "node.blocked": "text-red-600 bg-red-50",
  "node.unblocked": "text-teal-600 bg-teal-50",
  "approval.granted": "text-purple-600 bg-purple-50",
  "approval.rejected": "text-red-600 bg-red-50",
  "request.created": "text-blue-600 bg-blue-50",
};

function AuditRow({ event }: { event: AuditEvent }) {
  const badge = auditActionColor[event.action] ?? "text-gray-600 bg-gray-50";

  return (
    <div className="flex items-start gap-3 py-2 px-3 rounded-md hover:bg-[var(--color-surface-2)] transition-colors">
      <div className="shrink-0 mt-1">
        <div className={`size-2 rounded-full ${(badge.split(" ")[0] ?? "").replace("text-", "bg-")}`} />
      </div>

      <div className="flex-1 min-w-0">
        <div className="flex items-center gap-2">
          <span className="text-xs font-medium text-[var(--color-fg)]">{event.actor}</span>
          <span className={`text-[10px] px-1.5 py-0.5 rounded font-medium ${badge}`}>
            {event.action.replace(/\./g, " ")}
          </span>
          {event.request_id && (
            <span className="text-[10px] font-mono text-[var(--color-fg-subtle)] truncate">
              {event.request_id}
            </span>
          )}
        </div>
        {event.reason && (
          <p className="text-xs text-[var(--color-fg-muted)] mt-0.5 leading-snug">
            {event.reason}
          </p>
        )}
        <div className="flex items-center gap-2 mt-0.5">
          <Calendar size={10} className="text-[var(--color-fg-subtle)]" />
          <span className="text-[10px] text-[var(--color-fg-subtle)]">
            {new Date(event.created_at).toLocaleString()}
          </span>
        </div>
      </div>
    </div>
  );
}
