import { useMemo } from "react";
import { useQuery, useQueries } from "@tanstack/react-query";
import {
  Inbox,
  Compass,
  DollarSign,
  Scale,
  Server,
  Users,
  Boxes,
  ShieldCheck,
  Hammer,
  FileText,
  Bot,
  type LucideIcon,
} from "lucide-react";

import { api } from "../lib/api";
import { prettyLabel } from "../lib/request-format";
import type { RequestGraph, WorkflowNodeData } from "../lib/types";

// Reference descriptors for each agent type the system runs. Names and
// capability blurbs are presentation only; all status and counts below come
// from live node data aggregated across the org's requests.
const AGENT_META: Record<string, { name: string; icon: LucideIcon; blurb: string }> = {
  intake: {
    name: "Intake Processor",
    icon: Inbox,
    blurb: "Reads each request and plans which departments and stages it needs.",
  },
  planning: {
    name: "Planning Analyst",
    icon: Compass,
    blurb: "Breaks the request into scope and identifies the resources required.",
  },
  finance: {
    name: "Finance Reviewer",
    icon: DollarSign,
    blurb: "Checks budget feasibility, financial impact and ROI.",
  },
  legal: {
    name: "Legal Reviewer",
    icon: Scale,
    blurb: "Reviews compliance, regulation and contract risk.",
  },
  it: {
    name: "IT Manager",
    icon: Server,
    blurb: "Assesses technical feasibility, infrastructure and security.",
  },
  hr: {
    name: "HR Manager",
    icon: Users,
    blurb: "Plans staffing, hiring and policy alignment.",
  },
  ops: {
    name: "Operations Manager",
    icon: Boxes,
    blurb: "Handles logistics, facilities and the operational timeline.",
  },
  approval: {
    name: "Executive Approver",
    icon: ShieldCheck,
    blurb: "Makes the final approve or reject call with written justification.",
  },
  implementation: {
    name: "Implementation Lead",
    icon: Hammer,
    blurb: "Carries out the approved work once planning clears.",
  },
  report: {
    name: "Report Writer",
    icon: FileText,
    blurb: "Summarises what was decided, flagged and executed.",
  },
};

interface AgentRow {
  agentType: string;
  name: string;
  department: string;
  icon: LucideIcon;
  blurb: string;
  total: number;
  completed: number;
  active: number;
  blocked: number;
  requestCount: number;
  latestStatus: string;
}

export function AgentsView({ orgId }: { orgId: string }) {
  const requestsQuery = useQuery({
    queryKey: ["requests", orgId],
    queryFn: () => api.listRequests(orgId),
  });
  const requestIds = (requestsQuery.data?.requests ?? []).map((r) => r.id);

  const graphQueries = useQueries({
    queries: requestIds.map((id) => ({
      queryKey: ["request", id],
      queryFn: () => api.getRequest(id),
    })),
  });

  const graphsLoading = graphQueries.some((q) => q.isLoading);
  const graphs = graphQueries
    .map((q) => q.data)
    .filter((g): g is RequestGraph => Boolean(g));

  const agents = useMemo(() => aggregateAgents(graphs), [graphs]);
  const loading = requestsQuery.isLoading || (requestIds.length > 0 && graphsLoading && agents.length === 0);

  return (
    <div className="flex-1 flex flex-col bg-[var(--color-bg)] text-[var(--color-fg)] overflow-auto nice-scroll">
      <div className="border-b border-[var(--color-border)] px-8 py-5">
        <h1 className="text-xl font-medium" style={{ fontFeatureSettings: '"ss01"' }}>
          Agents
        </h1>
        <p className="text-sm text-[var(--color-fg-muted)] mt-0.5">
          The department agents and what they're working on across the org
        </p>
      </div>

      <div className="px-8 py-6 w-full">
        {loading ? (
          <div className="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-3 gap-4">
            {Array.from({ length: 4 }).map((_, i) => (
              <div key={i} className="h-32 rounded-lg bg-[var(--color-surface-2)] animate-pulse" />
            ))}
          </div>
        ) : agents.length === 0 ? (
          <p className="text-sm text-[var(--color-fg-muted)]">
            No agent activity yet. Submit a request and the agents will pick it up here.
          </p>
        ) : (
          <div className="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-3 gap-4">
            {agents.map((a) => (
              <AgentCard key={a.agentType} agent={a} />
            ))}
          </div>
        )}
      </div>
    </div>
  );
}

function aggregateAgents(graphs: RequestGraph[]): AgentRow[] {
  const byType = new Map<string, { nodes: WorkflowNodeData[]; requests: Set<string> }>();

  for (const g of graphs) {
    for (const n of g.nodes) {
      const entry = byType.get(n.agent_type) ?? { nodes: [], requests: new Set<string>() };
      entry.nodes.push(n);
      entry.requests.add(g.request.id);
      byType.set(n.agent_type, entry);
    }
  }

  const rows: AgentRow[] = [];
  for (const [agentType, { nodes, requests }] of byType) {
    const meta = AGENT_META[agentType];
    const latest = [...nodes]
      .filter((n) => n.status_text)
      .sort((a, b) => +new Date(b.completed_at ?? b.started_at ?? 0) - +new Date(a.completed_at ?? a.started_at ?? 0))[0];
    rows.push({
      agentType,
      name: meta?.name ?? prettyLabel(agentType),
      department: nodes[0]?.department || prettyLabel(agentType),
      icon: meta?.icon ?? Bot,
      blurb: meta?.blurb ?? "",
      total: nodes.length,
      completed: nodes.filter((n) => n.status === "completed").length,
      active: nodes.filter((n) => n.status === "in_progress").length,
      blocked: nodes.filter((n) => n.status === "blocked").length,
      requestCount: requests.size,
      latestStatus: latest?.status_text ?? "",
    });
  }

  // Stable order: the canonical pipeline order, unknown types last.
  const order = Object.keys(AGENT_META);
  return rows.sort((a, b) => {
    const ai = order.indexOf(a.agentType);
    const bi = order.indexOf(b.agentType);
    return (ai === -1 ? 99 : ai) - (bi === -1 ? 99 : bi);
  });
}

function AgentCard({ agent }: { agent: AgentRow }) {
  const { icon: Icon } = agent;
  const state =
    agent.blocked > 0
      ? { label: "Blocked", cls: "bg-[var(--color-danger)]/12 text-[var(--color-danger)]" }
      : agent.active > 0
        ? { label: "Active", cls: "bg-[var(--color-accent-bg)] text-[var(--color-brand)]" }
        : { label: "Idle", cls: "bg-[var(--color-surface-2)] text-[var(--color-fg-muted)]" };

  return (
    <div className="rounded-lg border border-[var(--color-border)] bg-[var(--color-surface)] p-4 shadow-stripe-ambient flex flex-col gap-3">
      <div className="flex items-start gap-3">
        <div className="size-10 rounded-lg bg-[var(--color-accent-bg)] border border-[var(--color-accent-border)] flex items-center justify-center shrink-0">
          <Icon size={18} className="text-[var(--color-brand)]" strokeWidth={1.75} />
        </div>
        <div className="min-w-0 flex-1">
          <div className="flex items-center justify-between gap-2">
            <h3 className="text-sm font-medium text-[var(--color-fg)] truncate">{agent.name}</h3>
            <span className={`shrink-0 rounded-full px-2 py-0.5 text-[11px] font-medium ${state.cls}`}>
              {state.label}
            </span>
          </div>
          <p className="text-[11px] uppercase tracking-wide text-[var(--color-fg-muted)] mt-0.5">
            {agent.department} Team
          </p>
        </div>
      </div>

      {agent.blurb && <p className="text-xs text-[var(--color-fg-muted)] leading-relaxed">{agent.blurb}</p>}

      {agent.latestStatus && (
        <p className="text-xs text-[var(--color-fg-label)] leading-snug border-l-0">
          <span className="text-[var(--color-fg-muted)]">Latest: </span>
          {agent.latestStatus}
        </p>
      )}

      <div className="mt-auto pt-2 border-t border-[var(--color-border)] grid grid-cols-3 gap-2 text-center">
        <Stat label="Completed" value={agent.completed} />
        <Stat label="Active" value={agent.active} />
        <Stat label="Requests" value={agent.requestCount} />
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
