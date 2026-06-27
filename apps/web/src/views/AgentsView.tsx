import { useMemo } from "react";
import { useQuery } from "@tanstack/react-query";
import {
  Compass,
  DollarSign,
  Scale,
  Server,
  Users,
  Boxes,
  ShieldCheck,
  Bot,
  type LucideIcon,
} from "lucide-react";

import { api } from "../lib/api";
import type { AgentRosterEntry, AgentStatus } from "../lib/types";

// Icon per agent type. Names, teams and capabilities come from the seeded
// roster (the API); only the icon is presentation-side.
const AGENT_ICON: Record<string, LucideIcon> = {
  finance: DollarSign,
  legal: Scale,
  it: Server,
  hr: Users,
  ops: Boxes,
  planning: Compass,
  approval: ShieldCheck,
};

const STATUS_BADGE: Record<AgentStatus, { label: string; cls: string }> = {
  busy: { label: "Busy", cls: "bg-[var(--color-accent-bg)] text-[var(--color-brand)]" },
  blocked: { label: "Blocked", cls: "bg-[var(--color-danger)]/12 text-[var(--color-danger)]" },
  idle: { label: "Idle", cls: "bg-[var(--color-surface-2)] text-[var(--color-fg-muted)]" },
};

interface DepartmentGroup {
  department: string;
  agents: AgentRosterEntry[];
}

export function AgentsView({ orgId }: { orgId: string }) {
  const { data, isLoading, isError, error } = useQuery({
    queryKey: ["agents", orgId],
    queryFn: () => api.listAgents(orgId),
    // Poll so the live status reflects the engine as requests run. The roster
    // is small and the query is cheap; this is the interval-poll fallback.
    refetchInterval: 4000,
  });

  const groups = useMemo(() => groupByDepartment(data?.agents ?? []), [data]);

  const busyCount = (data?.agents ?? []).filter((a) => a.status !== "idle").length;

  return (
    <div className="flex-1 flex flex-col bg-[var(--color-bg)] text-[var(--color-fg)] overflow-auto nice-scroll">
      <div className="border-b border-[var(--color-border)] px-4 md:px-8 py-4 md:py-5">
        <h1 className="text-xl font-medium" style={{ fontFeatureSettings: '"ss01"' }}>
          Agents
        </h1>
        <p className="text-sm text-[var(--color-fg-muted)] mt-0.5">
          The department agents that staff the organization, grouped by team
          {busyCount > 0 && (
            <span className="text-[var(--color-brand)]"> · {busyCount} working now</span>
          )}
        </p>
      </div>

      <div className="px-4 md:px-8 py-4 md:py-6 w-full max-w-[1100px]">
        {isLoading ? (
          <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
            {Array.from({ length: 4 }).map((_, i) => (
              <div key={i} className="h-40 rounded-lg bg-[var(--color-surface-2)] animate-pulse" />
            ))}
          </div>
        ) : isError ? (
          <p className="text-sm text-[var(--color-danger)]">
            Could not load the agent roster. {(error as Error)?.message}
          </p>
        ) : groups.length === 0 ? (
          <p className="text-sm text-[var(--color-fg-muted)]">
            No agents yet. The roster is seeded when the organization is created.
          </p>
        ) : (
          <div className="flex flex-col gap-8">
            {groups.map((g) => (
              <section key={g.department}>
                <h2 className="text-[11px] font-semibold uppercase tracking-wide text-[var(--color-fg-muted)] mb-3">
                  {g.department}
                </h2>
                <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                  {g.agents.map((a) => (
                    <AgentCard key={a.id} agent={a} />
                  ))}
                </div>
              </section>
            ))}
          </div>
        )}
      </div>
    </div>
  );
}

// groupByDepartment keeps the canonical pipeline order within a department and
// orders departments by where their first agent sits in that pipeline, so the
// roster reads the way the workflow runs (Finance, Legal, IT, …).
function groupByDepartment(agents: AgentRosterEntry[]): DepartmentGroup[] {
  const order = Object.keys(AGENT_ICON);
  const rank = (t: string | undefined) => {
    const i = t ? order.indexOf(t) : -1;
    return i === -1 ? 99 : i;
  };
  const byDept = new Map<string, AgentRosterEntry[]>();
  for (const a of agents) {
    const dept = a.team_name || "Other";
    const list = byDept.get(dept) ?? [];
    list.push(a);
    byDept.set(dept, list);
  }
  const groups: DepartmentGroup[] = [];
  for (const [department, list] of byDept) {
    list.sort((x, y) => rank(x.agent_type) - rank(y.agent_type));
    groups.push({ department, agents: list });
  }
  groups.sort((a, b) => rank(a.agents[0]?.agent_type) - rank(b.agents[0]?.agent_type));
  return groups;
}

// parseCapabilities splits the seeded comma-separated capability string into
// individual, trimmed chips.
function parseCapabilities(raw: string): string[] {
  return raw
    .split(",")
    .map((c) => c.trim())
    .filter(Boolean);
}

function AgentCard({ agent }: { agent: AgentRosterEntry }) {
  const Icon = AGENT_ICON[agent.agent_type] ?? Bot;
  const badge = STATUS_BADGE[agent.status];
  const capabilities = parseCapabilities(agent.capabilities);

  return (
    <div className="rounded-lg border border-[var(--color-border)] bg-[var(--color-surface)] p-4 shadow-stripe-ambient flex flex-col gap-3">
      <div className="flex items-start gap-3">
        <div className="size-10 rounded-lg bg-[var(--color-accent-bg)] border border-[var(--color-accent-border)] flex items-center justify-center shrink-0">
          <Icon size={18} className="text-[var(--color-brand)]" strokeWidth={1.75} />
        </div>
        <div className="min-w-0 flex-1">
          <div className="flex items-center justify-between gap-2">
            <h3 className="text-sm font-medium text-[var(--color-fg)] truncate">{agent.name}</h3>
            <span className={`shrink-0 inline-flex items-center gap-1.5 rounded-full px-2 py-0.5 text-[11px] font-medium ${badge.cls}`}>
              {agent.status === "busy" && (
                <span className="size-1.5 rounded-full bg-[var(--color-brand)] animate-pulse" />
              )}
              {badge.label}
            </span>
          </div>
          <p className="text-[11px] uppercase tracking-wide text-[var(--color-fg-muted)] mt-0.5">
            {agent.team_name ? `${agent.team_name} Team` : "Department"}
          </p>
        </div>
      </div>

      {capabilities.length > 0 && (
        <ul className="flex flex-wrap gap-1.5">
          {capabilities.map((c) => (
            <li
              key={c}
              className="rounded-md bg-[var(--color-surface-2)] px-2 py-0.5 text-[11px] text-[var(--color-fg-label)]"
            >
              {c}
            </li>
          ))}
        </ul>
      )}

      {agent.latest_status && (
        <p className="text-xs text-[var(--color-fg-label)] leading-snug">
          <span className="text-[var(--color-fg-muted)]">Latest: </span>
          {agent.latest_status}
        </p>
      )}

      <div className="mt-auto pt-2 border-t border-[var(--color-border)] grid grid-cols-3 gap-2 text-center">
        <Stat label="Completed" value={agent.completed} />
        <Stat label="Active" value={agent.active} />
        <Stat label="Requests" value={agent.request_count} />
      </div>
    </div>
  );
}

function Stat({ label, value }: { label: string; value: number }) {
  return (
    <div>
      <p className="text-base font-light tnum text-[var(--color-fg)]">{value}</p>
      <p className="text-[10px] uppercase tracking-wide text-[var(--color-fg-muted)]">{label}</p>
    </div>
  );
}
