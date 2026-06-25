import { useQuery } from "@tanstack/react-query";
import { motion } from "framer-motion";
import { Activity, AlertTriangle, ExternalLink, Loader2, Play, RefreshCw, Rocket, Trash2, Workflow } from "lucide-react";

import { api } from "../lib/api";
import { useToasts } from "../components/Toasts";

interface ProcessDefinition {
  id: string;
  key: string;
  name: string;
  version: number;
  deployment_id: string;
  running_instances: number;
  history_time_to_live: number;
}

interface RunsResponse {
  engine_id: string;
  endpoint: string;
  cockpit_url?: string;
  tasklist_url?: string;
  last_synced_at?: string;
  process_definitions: ProcessDefinition[];
}

async function fetchRuns(engineId: string): Promise<RunsResponse> {
  const res = await fetch(`/api/engines/${encodeURIComponent(engineId)}/runs`);
  if (!res.ok) throw new Error(`runs ${res.status}`);
  return res.json();
}

async function startInstance(engineId: string, key: string): Promise<{ id: string }> {
  const res = await fetch(
    `/api/engines/${encodeURIComponent(engineId)}/processes/${encodeURIComponent(key)}/start`,
    { method: "POST", headers: { "content-type": "application/json" }, body: "{}" },
  );
  if (!res.ok) throw new Error(await res.text());
  return res.json();
}

interface EngineRef {
  id: string;
  endpoint: string;
  lastSyncedAt?: string;
}

interface Props {
  engines: EngineRef[];
  onEngineRemoved: () => void;
}

export function RunsView({ engines, onEngineRemoved }: Props) {
  if (engines.length === 0) {
    return (
      <EmptyState
        title="No engine connected"
        body="Runs show deployed process definitions and their live instance counts. Connect a Camunda 7 engine to see yours."
      />
    );
  }
  return (
    <div className="flex-1 overflow-y-auto nice-scroll">
      <div className="max-w-5xl mx-auto px-6 py-8 space-y-6">
        <Header />
        {engines.map((e) => (
          <EngineRuns key={e.id} engine={e} onRemoved={onEngineRemoved} />
        ))}
      </div>
    </div>
  );
}

function Header() {
  return (
    <div className="flex items-start justify-between gap-4">
      <div>
        <div className="flex items-center gap-2 mb-1 text-[11px] uppercase tracking-[0.14em] text-[var(--color-fg-muted)]" style={{ fontWeight: 400 }}>
          <Activity size={12} />
          Runs
        </div>
        <div className="text-[28px] leading-tight text-[var(--color-fg)]" style={{ fontWeight: 300, letterSpacing: "-0.015em" }}>
          Deployed workflows on your engines
        </div>
        <div className="mt-1 text-[14px] text-[var(--color-fg-muted)]" style={{ fontWeight: 300 }}>
          Process definitions Pablo has deployed, plus live instance counts reported by each engine.
        </div>
      </div>
    </div>
  );
}

function EngineRuns({ engine, onRemoved }: { engine: EngineRef; onRemoved: () => void }) {
  const toasts = useToasts();
  const q = useQuery({
    queryKey: ["runs", engine.id],
    queryFn: () => fetchRuns(engine.id),
    refetchInterval: (query) => (query.state.error ? false : 8000),
    retry: false,
  });

  const unreachable = !!q.error;

  const handleRemove = async () => {
    if (!confirm(`Remove engine '${engine.id}' from Pablo's projection? The engine itself is untouched.`)) return;
    try {
      await api.deleteEngine(engine.id);
      toasts.push({ kind: "success", title: "Engine removed", body: engine.id });
      onRemoved();
    } catch (err) {
      toasts.push({
        kind: "error",
        title: "Couldn't remove engine",
        body: err instanceof Error ? err.message : String(err),
      });
    }
  };

  return (
    <motion.section
      initial={{ opacity: 0, y: 8 }}
      animate={{ opacity: 1, y: 0 }}
      transition={{ type: "spring", stiffness: 360, damping: 32 }}
      className="border border-[var(--color-border)] rounded-md bg-[var(--color-surface)] shadow-stripe-ambient"
    >
      <header className="px-4 py-3 border-b border-[var(--color-border)] flex items-center justify-between gap-3">
        <div className="min-w-0">
          <div className="flex items-center gap-2">
            <span className={`size-1.5 rounded-full ${unreachable ? "bg-amber-500" : "bg-emerald-500"}`} />
            <span className="text-[14px] text-[var(--color-fg)] truncate" style={{ fontWeight: 400 }}>
              {engine.id}
            </span>
            <span className="text-[10px] uppercase tracking-[0.12em] text-[var(--color-fg-subtle)]" style={{ fontWeight: 400 }}>
              camunda 7
            </span>
            {unreachable && (
              <span className="text-[10px] uppercase tracking-[0.12em] text-amber-600 dark:text-amber-400 flex items-center gap-1" style={{ fontWeight: 400 }}>
                <AlertTriangle size={10} /> unreachable
              </span>
            )}
          </div>
          <div className="text-[11px] font-mono text-[var(--color-fg-muted)] truncate mt-0.5">
            {engine.endpoint}
          </div>
        </div>
        <div className="flex items-center gap-1">
          <IconButton
            onClick={() => q.refetch()}
            label={unreachable ? "Retry" : "Refresh"}
            busy={q.isFetching}
            icon={<RefreshCw size={12} className={q.isFetching ? "animate-spin" : ""} />}
          />
          {q.data?.cockpit_url && !unreachable && (
            <ExternalButton href={q.data.cockpit_url} label="Cockpit" />
          )}
          {q.data?.tasklist_url && !unreachable && (
            <ExternalButton href={q.data.tasklist_url} label="Tasklist" />
          )}
          <IconButton
            onClick={handleRemove}
            label="Remove"
            icon={<Trash2 size={12} />}
            tone="danger"
          />
        </div>
      </header>

      <div className="p-3">
        {q.isLoading && (
          <div className="text-[12px] text-[var(--color-fg-muted)] flex items-center gap-2 px-2 py-1">
            <Loader2 size={12} className="animate-spin" /> loading deployments…
          </div>
        )}
        {unreachable && !q.isLoading && (
          <div className="px-3 py-3 text-[12px] text-[var(--color-fg-muted)]" style={{ fontWeight: 300 }}>
            Pablo couldn't reach this endpoint. Check it's healthy, or remove it from the projection.
          </div>
        )}
        {q.data && !unreachable && q.data.process_definitions.length === 0 && (
          <div className="text-[12px] text-[var(--color-fg-muted)] px-2 py-4 text-center">
            No process definitions yet. Compile a workflow and hit <span className="font-medium text-[var(--color-fg)]">Deploy</span> in the top bar.
          </div>
        )}
        {q.data && !unreachable && q.data.process_definitions.length > 0 && (
          <ul className="grid gap-2 sm:grid-cols-2">
            {q.data.process_definitions.map((pd) => (
              <li
                key={pd.id}
                className="group rounded-md border border-[var(--color-border)] bg-[var(--color-surface)] px-3 py-2.5 hover:border-[var(--color-border-strong)] hover:shadow-stripe-ambient transition-all"
              >
                <div className="flex items-start justify-between gap-2">
                  <div className="min-w-0">
                    <div className="flex items-center gap-1.5">
                      <Workflow size={12} className="text-[var(--color-fg-muted)]" />
                      <span className="text-[13px] text-[var(--color-fg)] truncate" style={{ fontWeight: 400 }}>
                        {pd.name}
                      </span>
                      <span className="text-[10px] font-mono text-[var(--color-fg-subtle)] tnum">v{pd.version}</span>
                    </div>
                    <div className="text-[11px] font-mono text-[var(--color-fg-muted)] truncate mt-0.5">
                      {pd.key}
                    </div>
                  </div>
                  <span
                    className={`text-[10px] font-mono tnum px-1.5 py-0.5 rounded border shrink-0 ${
                      pd.running_instances > 0
                        ? "bg-emerald-500/10 border-emerald-500/30 text-emerald-600 dark:text-emerald-400"
                        : "bg-[var(--color-surface-2)] border-[var(--color-border)] text-[var(--color-fg-muted)]"
                    }`}
                  >
                    {pd.running_instances} running
                  </span>
                </div>

                <div className="mt-2 flex items-center gap-1 opacity-0 group-hover:opacity-100 transition-opacity">
                  <button
                    onClick={async () => {
                      try {
                        const inst = await startInstance(engine.id, pd.key);
                        toasts.push({
                          kind: "success",
                          title: "Instance started",
                          body: inst.id.slice(0, 16) + "…",
                        });
                        q.refetch();
                      } catch (err) {
                        toasts.push({
                          kind: "error",
                          title: "Couldn't start instance",
                          body: err instanceof Error ? err.message : String(err),
                        });
                      }
                    }}
                    className="flex items-center gap-1 text-[11px] text-[var(--color-brand)] hover:underline"
                    style={{ fontWeight: 400 }}
                  >
                    <Play size={11} /> start
                  </button>
                  <span className="text-[var(--color-fg-faint)]">·</span>
                  <a
                    href={`http://localhost:8180/camunda/app/cockpit/default/#/process-definition/${pd.id}`}
                    target="_blank"
                    rel="noreferrer"
                    className="flex items-center gap-1 text-[11px] text-[var(--color-fg-muted)] hover:text-[var(--color-brand)]"
                    style={{ fontWeight: 400 }}
                  >
                    <ExternalLink size={11} /> cockpit
                  </a>
                </div>
              </li>
            ))}
          </ul>
        )}
      </div>
    </motion.section>
  );
}

function IconButton({
  label,
  icon,
  onClick,
  busy,
  tone,
}: {
  label: string;
  icon: React.ReactNode;
  onClick: () => void;
  busy?: boolean;
  tone?: "danger";
}) {
  const toneClass =
    tone === "danger"
      ? "text-[var(--color-fg-muted)] hover:bg-rose-500/10 hover:text-rose-500"
      : "text-[var(--color-fg-muted)] hover:bg-[var(--color-surface-2)] hover:text-[var(--color-fg)]";
  return (
    <button
      onClick={onClick}
      disabled={busy}
      title={label}
      className={`flex items-center gap-1 text-[12px] px-2 py-1 rounded-md transition-colors disabled:opacity-50 ${toneClass}`}
      style={{ fontWeight: 400 }}
    >
      {icon}
      {label}
    </button>
  );
}

function ExternalButton({ href, label }: { href: string; label: string }) {
  return (
    <a
      href={href}
      target="_blank"
      rel="noreferrer"
      className="flex items-center gap-1 text-[12px] px-2 py-1 rounded-md text-[var(--color-fg-muted)] hover:bg-[var(--color-surface-2)] hover:text-[var(--color-brand)] transition-colors"
      style={{ fontWeight: 400 }}
    >
      <ExternalLink size={12} />
      {label}
    </a>
  );
}

function EmptyState({ title, body }: { title: string; body: string }) {
  return (
    <div className="flex-1 flex items-center justify-center">
      <div className="max-w-sm text-center px-6">
        <div className="mx-auto mb-4 size-12 rounded-xl bg-[var(--color-accent-bg)] text-[var(--color-brand)] flex items-center justify-center">
          <Rocket size={20} strokeWidth={1.75} />
        </div>
        <div className="text-[18px] text-[var(--color-fg)]" style={{ fontWeight: 300, letterSpacing: "-0.01em" }}>
          {title}
        </div>
        <div className="mt-1 text-[13px] text-[var(--color-fg-muted)]" style={{ fontWeight: 300 }}>
          {body}
        </div>
      </div>
    </div>
  );
}
