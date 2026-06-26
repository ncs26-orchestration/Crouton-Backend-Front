import { useState, useMemo } from "react";
import { useQuery } from "@tanstack/react-query";
import {
  BarChart3,
  Calendar,
  Filter,
  Search,
  RefreshCw,
} from "lucide-react";

import { api } from "../lib/api";
import { useOrg } from "../contexts/OrgContext";
import type { AuditEvent } from "../lib/types";

export function ReportsView() {
  const { activeOrg } = useOrg();
  const orgId = activeOrg?.id;

  const [search, setSearch] = useState("");
  const [actorFilter, setActorFilter] = useState("");
  const [actionFilter, setActionFilter] = useState("");

  const { data, isLoading, error, refetch } = useQuery({
    queryKey: ["org-audit", orgId],
    queryFn: () => api.listOrgAudit(orgId!),
    enabled: !!orgId,
  });

  const events = data?.events ?? [];

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
              Audit trail and completed reports
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

      {/* Filters */}
      <div className="border-b border-[var(--color-border)] px-8 py-3 flex items-center gap-3 text-xs">
        <div className="flex items-center gap-1.5 text-[var(--color-fg-muted)]">
          <Filter size={13} />
          <span>Filters</span>
        </div>

        {/* Search */}
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

        {/* Actor filter */}
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

        {/* Action filter */}
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

      {/* Content */}
      <div className="flex-1 overflow-auto">
        {isLoading ? (
          <div className="flex items-center justify-center h-full">
            <div className="size-5 rounded-full border-2 border-[var(--color-brand)] border-t-transparent animate-spin" />
          </div>
        ) : error ? (
          <div className="flex items-center justify-center h-full text-sm text-[var(--color-danger)]">
            Failed to load audit trail
          </div>
        ) : filtered.length === 0 ? (
          <div className="flex-1 flex flex-col items-center justify-center gap-4 h-full">
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
  );
}

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
      {/* Timeline dot */}
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
