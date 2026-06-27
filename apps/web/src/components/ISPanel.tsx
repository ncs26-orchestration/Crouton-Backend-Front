import { useState } from "react";
import { motion } from "framer-motion";
import {
  Building2,
  ChevronLeft,
  CircleDot,
  Database,
  Mail,
  Plus,
  RefreshCw,
  Server,
  Trash2,
  Users,
  UsersRound,
} from "lucide-react";

import type { ISRegistry, ISSystem } from "../lib/types";
import { api } from "../lib/api";
import { ConnectEngineForm } from "./ConnectEngineForm";
import { DeclareSystemForm } from "./DeclareSystemForm";
import { useToasts } from "./Toasts";

interface Props {
  is?: ISRegistry;
  loading: boolean;
  onRefresh: () => void;
  error?: string | null;
  highlightUserIds?: Set<string>;
  highlightGroupIds?: Set<string>;
  highlightSystemIds?: Set<string>;
  collapsed?: boolean;
  onToggleCollapsed?: () => void;
  showEngineForm?: boolean;
  showSystemForm?: boolean;
  onCloseForms?: () => void;
}

type TabId = "engines" | "users" | "groups" | "systems";

const SYSTEM_ICONS: Record<ISSystem["kind"], React.ComponentType<{ size?: number; strokeWidth?: number; className?: string }>> = {
  ecm: Database,
  erp: Building2,
  comms: Mail,
  idp: UsersRound,
  crm: Users,
  signer: CircleDot,
  other: Server,
};

export function ISPanel({
  is,
  loading,
  onRefresh,
  error,
  highlightUserIds,
  highlightGroupIds,
  highlightSystemIds,
  collapsed,
  onToggleCollapsed,
  showEngineForm: showEngineFormExternal,
  showSystemForm: showSystemFormExternal,
  onCloseForms,
}: Props) {
  const toasts = useToasts();
  const [tab, setTab] = useState<TabId>("engines");
  const [localEngineForm, setLocalEngineForm] = useState(false);
  const [localSystemForm, setLocalSystemForm] = useState(false);
  const showEngineForm = showEngineFormExternal || localEngineForm;
  const showSystemForm = showSystemFormExternal || localSystemForm;


  const hasHighlight =
    (highlightUserIds && highlightUserIds.size > 0) ||
    (highlightGroupIds && highlightGroupIds.size > 0) ||
    (highlightSystemIds && highlightSystemIds.size > 0);

  const hl = (on: boolean) =>
    hasHighlight
      ? on
        ? "bg-[var(--color-accent-bg)] ring-1 ring-[var(--color-accent-border)] transition-all"
        : "opacity-45 transition-opacity"
      : "";

  const deleteEngine = async (id: string) => {
    if (!confirm(`Remove engine ${id}? Crouton never touches the engine itself — only this cached projection disappears.`))
      return;
    try {
      await api.deleteEngine(id);
      toasts.push({ kind: "success", title: "Engine removed", body: id });
      onRefresh();
    } catch (err) {
      toasts.push({
        kind: "error",
        title: "Couldn't remove engine",
        body: err instanceof Error ? err.message : String(err),
      });
    }
  };

  const counts: Record<TabId, number> = {
    engines: is?.engine_connections?.length ?? 0,
    users: is?.users.length ?? 0,
    groups: is?.groups.length ?? 0,
    systems: is?.systems.length ?? 0,
  };

  const TABS: { id: TabId; label: string; icon: React.ComponentType<{ size?: number; strokeWidth?: number }> }[] = [
    { id: "engines", label: "Engines", icon: Server },
    { id: "users", label: "Users", icon: Users },
    { id: "groups", label: "Groups", icon: UsersRound },
    { id: "systems", label: "Systems", icon: Database },
  ];

  return (
    <motion.aside
      initial={false}
      animate={{
        width: collapsed ? 0 : 300,
        opacity: collapsed ? 0 : 1,
      }}
      transition={{ type: "spring", stiffness: 320, damping: 34 }}
      className="shrink-0 bg-[var(--color-surface)] border-r border-[var(--color-border)] flex flex-col h-full overflow-hidden">
      {/* Header */}
      <div className="px-5 pt-4 pb-3 flex items-start justify-between gap-2">
        <div className="min-w-0">
          <div className="text-[11px] uppercase tracking-[0.14em] text-[var(--color-fg-muted)]" style={{ fontWeight: 400 }}>
            Information system
          </div>
          <div className="text-[20px] text-[var(--color-fg)] truncate" style={{ fontWeight: 300, letterSpacing: "-0.01em" }}>
            {is?.tenant_id ?? "…"}
          </div>
        </div>
        <div className="flex items-center gap-1">
          <button
            onClick={onRefresh}
            disabled={loading}
title="Refresh projection"
             className="btn-sm size-7 rounded-md flex items-center justify-center text-[var(--color-fg-muted)] hover:text-[var(--color-fg)] hover:bg-[var(--color-surface-2)] disabled:text-[var(--color-fg-faint)] transition-colors"
          >
            <RefreshCw size={13} className={loading ? "animate-spin" : ""} />
          </button>
          {onToggleCollapsed && (
            <button
              onClick={onToggleCollapsed}
              title="Collapse panel"
              className="btn-sm size-7 rounded-md flex items-center justify-center text-[var(--color-fg-muted)] hover:text-[var(--color-fg)] hover:bg-[var(--color-surface-2)] transition-colors"
            >
              <ChevronLeft size={13} />
            </button>
          )}
        </div>
      </div>

      {/* Tabs — icon + count when inactive, icon + label + count when active.
          This keeps the 4-tab strip comfortably inside 300 px. */}
      <div className="px-3 pb-2 border-b border-[var(--color-border)]">
        <nav className="flex gap-0.5 bg-[var(--color-surface-2)] rounded-md p-1">
          {TABS.map((t) => {
            const Icon = t.icon;
            const isActive = tab === t.id;
            return (
              <button
                key={t.id}
                onClick={() => setTab(t.id)}
                title={t.label}
                aria-label={t.label}
                className={`group flex items-center justify-center gap-1.5 rounded px-2 py-1 min-w-0 transition-all ${
                  isActive
                    ? "flex-1 bg-[var(--color-surface)] text-[var(--color-fg)] shadow-stripe-ambient"
                    : "shrink-0 text-[var(--color-fg-muted)] hover:text-[var(--color-fg)]"
                }`}
                style={{ fontWeight: 400 }}
              >
                <Icon size={12} strokeWidth={isActive ? 2.25 : 1.75} />
                {isActive && (
                  <span className="text-[11px] truncate">{t.label}</span>
                )}
                <span className="font-mono text-[10px] text-[var(--color-fg-subtle)] tnum">
                  {counts[t.id]}
                </span>
              </button>
            );
          })}
        </nav>
      </div>

      {/* Body */}
      <div className="flex-1 overflow-y-auto nice-scroll px-3 py-3">
        {error && (
          <div className="mb-3 text-xs text-rose-600 dark:text-rose-300 bg-rose-500/10 border border-rose-500/30 rounded-md px-3 py-2">
            {error}
          </div>
        )}

        {!is && !loading && !error && (
          <EmptyHint label="No IS data yet" body="Connect a Camunda 7 engine to start projecting identities." />
        )}

        {is && tab === "engines" && (
          <div className="space-y-2">
            {showEngineForm && (
              <ConnectEngineForm
                onClose={() => {
                  setLocalEngineForm(false);
                  onCloseForms?.();
                }}
                onConnected={onRefresh}
              />
            )}
            {(is.engine_connections ?? []).map((ec) => (
              <article
                key={ec.id}
                className="group rounded-md border border-[var(--color-border)] bg-[var(--color-surface)] px-3 py-2.5 hover:border-[var(--color-border-strong)] hover:shadow-stripe-ambient transition-all"
              >
                <div className="flex items-center gap-2">
                  <span className="size-1.5 rounded-full bg-emerald-500 shrink-0" />
                  <span className="text-[13px] text-[var(--color-fg)] truncate" style={{ fontWeight: 400 }}>
                    {ec.id}
                  </span>
                  <span className="text-[10px] uppercase tracking-[0.12em] text-[var(--color-fg-subtle)]" style={{ fontWeight: 400 }}>
                    {ec.kind}
                  </span>
                  <div className="flex-1" />
                  <button
                    onClick={() => deleteEngine(ec.id)}
                    className="opacity-0 group-hover:opacity-100 text-[var(--color-fg-subtle)] hover:text-rose-500 transition-opacity"
                    title="Remove from Crouton's projection"
                  >
                    <Trash2 size={12} />
                  </button>
                </div>
                <div className="mt-1 text-[11px] font-mono text-[var(--color-fg-muted)] truncate">
                  {ec.endpoint}
                </div>
              </article>
            ))}
            {(is.engine_connections?.length ?? 0) === 0 && !showEngineForm && (
              <EmptyHint label="No engine connected" body="Add a Camunda 7 endpoint to start projecting." />
            )}
            {!showEngineForm && (
              <AddButton label="Connect engine" onClick={() => setLocalEngineForm(true)} />
            )}
          </div>
        )}

        {is && tab === "users" && (
          <ul className="space-y-1">
            {is.users.map((u) => (
              <li
                key={u.id}
                className={`rounded-md px-2.5 py-1.5 flex items-center justify-between gap-2 ${hl(!!highlightUserIds?.has(u.id))}`}
              >
                <div className="flex flex-col min-w-0">
                  <span className="text-[13px] text-[var(--color-fg)] truncate" style={{ fontWeight: 400 }}>
                    {u.name}
                  </span>
                  <span className="text-[11px] font-mono text-[var(--color-fg-subtle)] truncate">{u.id}</span>
                </div>
                {u.group_ids && u.group_ids.length > 0 && (
                  <span className="text-[10px] font-mono text-[var(--color-fg-subtle)] shrink-0 tnum">
                    {u.group_ids.length} grp
                  </span>
                )}
              </li>
            ))}
            {is.users.length === 0 && <EmptyHint label="No users projected" />}
          </ul>
        )}

        {is && tab === "groups" && (
          <ul className="space-y-1">
            {is.groups.map((g) => (
              <li
                key={g.id}
                className={`rounded-md px-2.5 py-1.5 flex items-center justify-between gap-2 ${hl(!!highlightGroupIds?.has(g.id))}`}
              >
                <span className="text-[13px] text-[var(--color-fg)] truncate" style={{ fontWeight: 400 }}>
                  {g.name}
                </span>
                <span className="text-[11px] font-mono text-[var(--color-fg-subtle)] shrink-0">{g.id}</span>
              </li>
            ))}
            {is.groups.length === 0 && <EmptyHint label="No groups projected" />}
          </ul>
        )}

        {is && tab === "systems" && (
          <div className="space-y-2 min-w-[268px]">
            {showSystemForm && (
              <DeclareSystemForm
                onClose={() => {
                  setLocalSystemForm(false);
                  onCloseForms?.();
                }}
                onDeclared={onRefresh}
              />
            )}
            {is.systems.map((s) => {
              const Icon = SYSTEM_ICONS[s.kind];
              return (
                <article
                  key={s.id}
                  className={`rounded-md border border-[var(--color-border)] bg-[var(--color-surface)] px-3 py-2.5 hover:border-[var(--color-border-strong)] hover:shadow-stripe-ambient transition-all ${hl(!!highlightSystemIds?.has(s.id))}`}
                >
                  <div className="flex items-center gap-2">
                    <Icon size={13} strokeWidth={1.75} className="text-[var(--color-fg-muted)]" />
                    <span className="text-[13px] text-[var(--color-fg)]" style={{ fontWeight: 400 }}>
                      {s.id}
                    </span>
                    <span className="text-[10px] uppercase tracking-[0.12em] text-[var(--color-fg-subtle)]" style={{ fontWeight: 400 }}>
                      {s.kind}
                    </span>
                  </div>
                  <div className="mt-1.5 flex flex-wrap gap-1">
                    {s.capabilities.map((c) => (
                      <span
                        key={c}
                        className="text-[10px] font-mono text-[var(--color-fg-muted)] bg-[var(--color-surface-2)] px-1.5 py-0.5 rounded"
                      >
                        {c}
                      </span>
                    ))}
                  </div>
                </article>
              );
            })}
            {is.systems.length === 0 && !showSystemForm && <EmptyHint label="No systems declared" />}
            {!showSystemForm && (
              <AddButton label="Declare a system" onClick={() => setLocalSystemForm(true)} />
            )}
          </div>
        )}
      </div>
    </motion.aside>
  );
}

function AddButton({ label, onClick }: { label: string; onClick: () => void }) {
  return (
    <button
      onClick={onClick}
      className="w-full flex items-center justify-center gap-1.5 rounded-md border border-dashed border-[var(--color-border-purple)] bg-[var(--color-accent-bg)] px-3 py-2 text-[12px] text-[var(--color-brand)] hover:bg-[var(--color-brand)] hover:text-white hover:border-[var(--color-brand)] transition-colors"
      style={{ fontWeight: 400 }}
    >
      <Plus size={12} />
      {label}
    </button>
  );
}

function EmptyHint({ label, body }: { label: string; body?: string }) {
  return (
    <div className="px-3 py-5 text-center">
      <div className="text-[12px] text-[var(--color-fg-label)]" style={{ fontWeight: 400 }}>
        {label}
      </div>
      {body && <div className="mt-0.5 text-[11px] text-[var(--color-fg-subtle)]">{body}</div>}
    </div>
  );
}
