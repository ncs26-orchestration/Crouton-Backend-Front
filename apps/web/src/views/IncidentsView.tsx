import { useMemo, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import {
  AlertTriangle,
  AlertCircle,
  Info,
  Loader2,
  MessageSquare,
  Wrench,
  CheckCircle2,
  ChevronRight,
} from "lucide-react";

import { api } from "../lib/api";
import type { Incident, IncidentSeverity, IncidentMessage } from "../lib/types";

interface Props {
  orgId: string;
}

const SEVERITY_ICON: Record<string, typeof AlertTriangle> = {
  critical: AlertCircle,
  high: AlertTriangle,
  medium: Info,
  low: Info,
};

const SEVERITY_BADGE: Record<string, string> = {
  critical:
    "text-red-700 bg-red-50 dark:text-red-300 dark:bg-red-950",
  high: "text-amber-700 bg-amber-50 dark:text-amber-300 dark:bg-amber-950",
  medium: "text-blue-700 bg-blue-50 dark:text-blue-300 dark:bg-blue-950",
  low: "text-slate-700 bg-slate-50 dark:text-slate-300 dark:bg-slate-950",
};

const STATUS_BADGE: Record<string, string> = {
  open: "text-red-700 bg-red-50 dark:text-red-300 dark:bg-red-950",
  in_progress: "text-amber-700 bg-amber-50 dark:text-amber-300 dark:bg-amber-950",
  resolved: "text-emerald-700 bg-emerald-50 dark:text-emerald-300 dark:bg-emerald-950",
};

export function IncidentsView({ orgId }: Props) {
  const [selected, setSelected] = useState<Incident | null>(null);
  const [filterStatus, setFilterStatus] = useState<string>("all");

  const { data, isLoading, error } = useQuery({
    queryKey: ["incidents", orgId],
    queryFn: () => api.listIncidents(orgId),
  });

  const incidents = useMemo(() => data?.incidents ?? [], [data]);

  const filtered = useMemo(
    () =>
      filterStatus === "all"
        ? incidents
        : incidents.filter((i) => i.status === filterStatus),
    [incidents, filterStatus],
  );

  const openCount = incidents.filter((i) => i.status === "open" || i.status === "in_progress").length;

  return (
    <div className="flex-1 flex flex-col overflow-hidden">
      <div className="shrink-0 px-4 md:px-6 py-4 border-b border-[var(--color-border)] flex flex-col md:flex-row md:items-center md:justify-between gap-3">
        <div>
          <h1
            className="text-lg font-medium text-[var(--color-fg)]"
            style={{ fontFeatureSettings: '"ss01"' }}
          >
            Incidents
          </h1>
          <p className="text-xs text-[var(--color-fg-muted)] mt-0.5">
            {openCount} open {openCount === 1 ? "incident" : "incidents"} across all machines
          </p>
        </div>
      </div>

      <div className="shrink-0 px-4 md:px-6 py-2.5 border-b border-[var(--color-border)] flex items-center gap-3 flex-wrap">
        <label className="flex items-center gap-1.5 text-xs text-[var(--color-fg-muted)]">
          Status
          <select
            value={filterStatus}
            onChange={(e) => setFilterStatus(e.target.value)}
            className="rounded border border-[var(--color-border)] bg-[var(--color-bg)] px-2 py-1 text-xs text-[var(--color-fg)] focus:outline-none focus:ring-2 focus:ring-[var(--color-brand)]"
          >
            <option value="all">All</option>
            <option value="open">Open</option>
            <option value="in_progress">In Progress</option>
            <option value="resolved">Resolved</option>
          </select>
        </label>
      </div>

      <div className="flex-1 flex overflow-hidden">
        <div className={`flex-1 overflow-auto ${selected ? "hidden md:block border-r border-[var(--color-border)]" : ""}`}>
          {isLoading && (
            <div className="flex items-center justify-center h-40">
              <div className="size-6 rounded-full border-2 border-[var(--color-brand)] border-t-transparent animate-spin" />
            </div>
          )}

          {error && (
            <div className="flex items-center justify-center h-40 gap-2 text-sm text-[var(--color-danger)]">
              <AlertCircle size={16} />
              Failed to load incidents
            </div>
          )}

          {!isLoading && !error && filtered.length === 0 && (
            <div className="flex flex-col items-center justify-center h-60 gap-3 text-center">
              <div className="size-10 rounded-lg bg-[var(--color-surface-2)] flex items-center justify-center">
                <CheckCircle2 size={20} className="text-[var(--color-fg-subtle)]" />
              </div>
              <p className="text-sm text-[var(--color-fg-muted)]">
                {filterStatus !== "all"
                  ? "No incidents match this filter"
                  : "No incidents reported yet"}
              </p>
            </div>
          )}

          {filtered.map((inc) => (
            <IncidentRow
              key={inc.id}
              incident={inc}
              active={selected?.id === inc.id}
              onClick={() => setSelected(selected?.id === inc.id ? null : inc)}
            />
          ))}
        </div>

        {selected && (
          <>
            {/* Mobile overlay */}
            <div className="md:hidden fixed inset-0 z-40 bg-[var(--color-bg)] overflow-auto" onClick={() => setSelected(null)}>
              <div className="min-h-full" onClick={(e) => e.stopPropagation()}>
                <IncidentDetail incident={selected} orgId={orgId} onBack={() => setSelected(null)} />
              </div>
            </div>
            {/* Desktop panel */}
            <div className="hidden md:block w-[420px] shrink-0 overflow-auto">
              <IncidentDetail incident={selected} orgId={orgId} onBack={() => setSelected(null)} />
            </div>
          </>
        )}
      </div>
    </div>
  );
}

function IncidentRow({
  incident: inc,
  active,
  onClick,
}: {
  incident: Incident;
  active: boolean;
  onClick: () => void;
}) {
  const SevIcon = SEVERITY_ICON[inc.severity] ?? Info;
  return (
    <button
      onClick={onClick}
      className={`w-full flex items-center gap-3 px-6 py-3.5 text-left border-b border-[var(--color-border)] hover:bg-[var(--color-surface-2)] transition-colors ${
        active ? "bg-[var(--color-accent-bg)]" : ""
      }`}
    >
      <SevIcon
        size={16}
        className={
          inc.severity === "critical"
            ? "text-red-500"
            : inc.severity === "high"
              ? "text-amber-500"
              : "text-[var(--color-fg-muted)]"
        }
      />
      <div className="flex-1 min-w-0">
        <div className="flex items-center gap-2">
          <span
            className="text-sm font-medium text-[var(--color-fg)] truncate"
            style={{ fontFeatureSettings: '"ss01"' }}
          >
            {inc.title}
          </span>
          <span
            className={`shrink-0 inline-block rounded-md px-1.5 py-0.5 text-[10px] font-medium ${STATUS_BADGE[inc.status] ?? ""}`}
          >
            {inc.status === "in_progress" ? "in progress" : inc.status}
          </span>
        </div>
        <p className="text-xs text-[var(--color-fg-muted)] mt-0.5 truncate">
          <Wrench size={11} className="inline mr-1" />
          {inc.machine_name || inc.machine_id}
          {inc.description && <> &middot; {inc.description}</>}
        </p>
      </div>
      <ChevronRight size={14} className="text-[var(--color-fg-subtle)]" />
    </button>
  );
}

function IncidentDetail({
  incident: inc,
  orgId,
  onBack,
}: {
  incident: Incident;
  orgId: string;
  onBack: () => void;
}) {
  const [messages, setMessages] = useState<IncidentMessage[]>([]);
  const [newMsg, setNewMsg] = useState("");
  const [sending, setSending] = useState(false);
  const [resolving, setResolving] = useState(false);
  const [resolveNotes, setResolveNotes] = useState("");
  const [showResolve, setShowResolve] = useState(false);

  const { isLoading: msgsLoading, refetch: refetchMsgs } = useQuery({
    queryKey: ["incident-messages", inc.id],
    queryFn: async () => {
      const res = await api.listIncidentMessages(inc.id);
      setMessages(res.messages ?? []);
      return res;
    },
  });

  const sendMessage = async () => {
    if (!newMsg.trim()) return;
    setSending(true);
    await api.appendIncidentMessage(inc.id, newMsg.trim());
    setNewMsg("");
    setSending(false);
    refetchMsgs();
  };

  const handleResolve = async () => {
    setResolving(true);
    await api.resolveIncident(inc.id, resolveNotes);
    setResolving(false);
    setShowResolve(false);
    refetchMsgs();
  };

  const handleDiagnose = async () => {
    await api.requestDiagnosis(inc.id);
  };

  const SevIcon = SEVERITY_ICON[inc.severity] ?? Info;

  return (
    <div className="flex flex-col h-full">
      <div className="shrink-0 px-4 py-3 border-b border-[var(--color-border)]">
        <button
          onClick={onBack}
          className="text-xs text-[var(--color-fg-muted)] hover:text-[var(--color-fg)] transition-colors"
        >
          &larr; Back to list
        </button>
        <h2
          className="text-base font-medium text-[var(--color-fg)] mt-1"
          style={{ fontFeatureSettings: '"ss01"' }}
        >
          {inc.title}
        </h2>
        <div className="flex items-center gap-2 mt-1.5">
          <span
            className={`inline-flex items-center gap-1 rounded-md px-1.5 py-0.5 text-[10px] font-medium ${SEVERITY_BADGE[inc.severity] ?? ""}`}
          >
            <SevIcon size={10} />
            {inc.severity}
          </span>
          <span
            className={`inline-flex items-center gap-1 rounded-md px-1.5 py-0.5 text-[10px] font-medium ${STATUS_BADGE[inc.status] ?? ""}`}
          >
            {inc.status === "in_progress" ? "in progress" : inc.status}
          </span>
        </div>
        <p className="text-xs text-[var(--color-fg-muted)] mt-2">
          <Wrench size={11} className="inline mr-1" />
          {inc.machine_name || inc.machine_id}
        </p>
        {inc.description && (
          <p className="text-xs text-[var(--color-fg)] mt-1">{inc.description}</p>
        )}

        {inc.status !== "resolved" && (
          <div className="flex gap-2 mt-3">
            <button
              onClick={handleDiagnose}
              className="flex items-center gap-1 px-2.5 py-1.5 rounded-md bg-[var(--color-brand)] text-white text-xs font-medium hover:opacity-90 transition-opacity min-h-[44px] md:min-h-auto"
            >
              <AlertTriangle size={11} />
              Diagnose with AI
            </button>
            <button
              onClick={() => setShowResolve(!showResolve)}
              className="flex items-center gap-1 px-2.5 py-1.5 rounded-md border border-[var(--color-border)] text-xs text-[var(--color-fg-muted)] hover:text-emerald-600 hover:border-emerald-400 transition-colors"
            >
              <CheckCircle2 size={11} />
              Resolve
            </button>
          </div>
        )}

        {showResolve && (
          <div className="mt-3 flex gap-2">
            <input
              type="text"
              value={resolveNotes}
              onChange={(e) => setResolveNotes(e.target.value)}
              placeholder="Resolution notes..."
              className="flex-1 px-2 py-1.5 rounded border border-[var(--color-border)] bg-[var(--color-bg)] text-xs text-[var(--color-fg)] outline-none focus:border-[var(--color-brand)]"
            />
            <button
              onClick={handleResolve}
              disabled={resolving || !resolveNotes.trim()}
              className="px-2.5 py-1.5 rounded-md bg-emerald-600 text-white text-xs font-medium hover:bg-emerald-700 disabled:opacity-40 transition-colors"
            >
              {resolving ? "..." : "Confirm"}
            </button>
          </div>
        )}
      </div>

      <div className="flex-1 overflow-auto p-4 space-y-3">
        {msgsLoading && (
          <div className="flex justify-center py-6">
            <div className="size-5 rounded-full border-2 border-[var(--color-brand)] border-t-transparent animate-spin" />
          </div>
        )}

        {messages.length === 0 && !msgsLoading && (
          <p className="text-xs text-[var(--color-fg-muted)] text-center py-6">No messages yet</p>
        )}

        {messages.map((msg) => (
          <div
            key={msg.id}
            className="rounded-lg border border-[var(--color-border)] bg-[var(--color-surface)] px-3 py-2"
          >
            <div className="flex items-center gap-2 mb-1">
              <span className="text-xs font-medium text-[var(--color-fg)]">
                {msg.sender_name}
              </span>
              <span className="text-[10px] text-[var(--color-fg-muted)]">{msg.sender_role}</span>
              <span className="text-[10px] text-[var(--color-fg-subtle)] ml-auto">
                {new Date(msg.created_at).toLocaleString()}
              </span>
            </div>
            <p className="text-xs text-[var(--color-fg)]">{msg.content}</p>
          </div>
        ))}
      </div>

      <div className="shrink-0 px-4 py-3 border-t border-[var(--color-border)]">
        <div className="flex gap-2">
          <input
            type="text"
            value={newMsg}
            onChange={(e) => setNewMsg(e.target.value)}
            onKeyDown={(e) => e.key === "Enter" && sendMessage()}
            placeholder="Add a message..."
            className="flex-1 px-3 py-2 rounded-lg border border-[var(--color-border)] bg-[var(--color-bg)] text-sm text-[var(--color-fg)] placeholder:text-[var(--color-fg-subtle)] outline-none focus:border-[var(--color-brand)] transition-colors"
          />
          <button
            onClick={sendMessage}
            disabled={!newMsg.trim() || sending}
            className="flex items-center gap-1 px-3 py-2 rounded-lg bg-[var(--color-brand)] text-white text-sm font-medium hover:opacity-90 disabled:opacity-40 transition-opacity min-h-[44px] md:min-h-auto"
          >
            {sending ? <Loader2 size={13} className="animate-spin" /> : <MessageSquare size={13} />}
          </button>
        </div>
      </div>
    </div>
  );
}
