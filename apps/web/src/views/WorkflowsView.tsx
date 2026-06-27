import { useMemo, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { ChevronDown, ChevronRight, Loader2, Pencil, Play, Plus, Trash2, Workflow as WorkflowIcon, X } from "lucide-react";

import { api } from "../lib/api";
import { useAuth } from "../contexts/AuthContext";
import { useToasts } from "../components/Toasts";
import { PageHeader, EmptyState } from "../components/ui";
import { PlanEditor, linearEdges } from "../components/PlanEditor";
import { departmentColor } from "../lib/department";
import { prettyLabel, statusBadgeClass } from "../lib/request-format";
import type { AgentRosterEntry, RequestStatus, WorkflowDef, WorkflowStep } from "../lib/types";

interface Props {
  orgId: string;
  role: string;
  onOpenWorkflow: (requestId: string) => void;
}

export function WorkflowsView({ orgId, role, onOpenWorkflow }: Props) {
  const { user } = useAuth();
  const isPrivileged = role === "admin" || role === "executor";

  const workflowsQ = useQuery({ queryKey: ["workflows", orgId], queryFn: () => api.listWorkflows(orgId).then((r) => r.workflows) });
  const agentsQ = useQuery({ queryKey: ["agents", orgId], queryFn: () => api.listAgents(orgId).then((r) => r.agents) });
  const teamsQ = useQuery({ queryKey: ["teams", orgId], queryFn: () => api.listTeams(orgId).then((r) => r.teams) });
  const membersQ = useQuery({ queryKey: ["org-members", orgId], queryFn: () => api.listOrgMembers(orgId).then((r) => r.members) });

  const workflows = workflowsQ.data ?? [];
  const agents = agentsQ.data ?? [];
  const teams = teamsQ.data ?? [];
  const members = membersQ.data ?? [];

  // Teams the current user leads, by id — they may author those teams' workflows.
  const me = members.find((m) => m.id === user?.id);
  const leadTeamNames = new Set((me?.team_roles ?? []).filter((tr) => tr.role === "lead").map((tr) => tr.team.toLowerCase()));
  const leadTeamIds = new Set(teams.filter((t) => leadTeamNames.has(t.name.toLowerCase())).map((t) => t.id));
  const authorableTeams = isPrivileged ? teams : teams.filter((t) => leadTeamIds.has(t.id));
  const canAuthor = isPrivileged || authorableTeams.length > 0;
  const canManage = (w: WorkflowDef) =>
    isPrivileged || (w.scope === "team" && w.team_id != null && leadTeamIds.has(w.team_id));

  const [editing, setEditing] = useState<WorkflowDef | "new" | null>(null);

  // Group: global first, then by team name.
  const groups = useMemo(() => {
    const g: { label: string; items: WorkflowDef[] }[] = [];
    const global = workflows.filter((w) => w.scope === "global");
    if (global.length) g.push({ label: "Global", items: global });
    const byTeam = new Map<string, WorkflowDef[]>();
    for (const w of workflows.filter((w) => w.scope === "team")) {
      const k = w.team_name || "Team";
      byTeam.set(k, [...(byTeam.get(k) ?? []), w]);
    }
    for (const [label, items] of [...byTeam.entries()].sort()) g.push({ label, items });
    return g;
  }, [workflows]);

  return (
    <div className="flex-1 flex flex-col overflow-hidden">
      <PageHeader
        title="Workflows"
        subtitle="Reusable internal processes you run on demand, like hiring or time off."
        actions={
          canAuthor ? (
            <button
              onClick={() => setEditing("new")}
              className="flex items-center gap-1.5 rounded-md bg-[var(--color-brand)] px-3 py-1.5 text-sm font-medium text-white hover:bg-[var(--color-brand-hover)] transition-colors"
            >
              <Plus size={14} /> New workflow
            </button>
          ) : undefined
        }
      />

      <div className="flex-1 overflow-auto px-8 py-6">
        {workflowsQ.isLoading ? (
          <div className="flex justify-center py-16"><Loader2 size={20} className="animate-spin text-[var(--color-fg-muted)]" /></div>
        ) : workflows.length === 0 ? (
          <EmptyState icon={WorkflowIcon} title="No workflows yet" hint="Define a reusable process once, then run it whenever you need it." />
        ) : (
          <div className="flex flex-col gap-7 max-w-4xl">
            {groups.map((g) => (
              <section key={g.label}>
                <h2 className="text-[11px] font-semibold uppercase tracking-wide text-[var(--color-fg-muted)] mb-3">{g.label}</h2>
                <div className="flex flex-col gap-3">
                  {g.items.map((w) => (
                    <WorkflowCard
                      key={w.id}
                      orgId={orgId}
                      workflow={w}
                      canManage={canManage(w)}
                      onEdit={() => setEditing(w)}
                      onOpenWorkflow={onOpenWorkflow}
                    />
                  ))}
                </div>
              </section>
            ))}
          </div>
        )}
      </div>

      {editing && (
        <WorkflowEditor
          orgId={orgId}
          agents={agents}
          authorableTeams={authorableTeams}
          canAuthorGlobal={isPrivileged}
          workflow={editing === "new" ? null : editing}
          onClose={() => setEditing(null)}
        />
      )}
    </div>
  );
}

function WorkflowCard({
  orgId,
  workflow: w,
  canManage,
  onEdit,
  onOpenWorkflow,
}: {
  orgId: string;
  workflow: WorkflowDef;
  canManage: boolean;
  onEdit: () => void;
  onOpenWorkflow: (requestId: string) => void;
}) {
  const qc = useQueryClient();
  const toasts = useToasts();
  const [open, setOpen] = useState(false);

  const runsQ = useQuery({
    queryKey: ["workflow-runs", orgId, w.id],
    queryFn: () => api.listWorkflowRuns(orgId, w.id).then((r) => r.runs),
    enabled: open,
  });

  const runMut = useMutation({
    mutationFn: () => api.runWorkflow(orgId, w.id),
    onSuccess: (res) => {
      toasts.push({ kind: "success", title: `Started ${w.name}` });
      qc.invalidateQueries({ queryKey: ["workflow-runs", orgId, w.id] });
      onOpenWorkflow(res.request.id);
    },
    onError: (e: Error) => toasts.push({ kind: "error", title: e.message }),
  });

  const deleteMut = useMutation({
    mutationFn: () => api.deleteWorkflow(orgId, w.id),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["workflows", orgId] }),
    onError: (e: Error) => toasts.push({ kind: "error", title: e.message }),
  });

  return (
    <div className="rounded-xl border border-[var(--color-border)] bg-[var(--color-surface)]">
      <div className="flex items-start gap-3 p-4">
        <div className="size-9 rounded-lg flex items-center justify-center shrink-0 bg-[var(--color-surface-2)]">
          <WorkflowIcon size={17} className="text-[var(--color-fg-muted)]" />
        </div>
        <div className="min-w-0 flex-1">
          <h3 className="text-sm font-medium text-[var(--color-fg)] truncate">{w.name}</h3>
          {w.description && <p className="text-xs text-[var(--color-fg-muted)] mt-0.5 line-clamp-2">{w.description}</p>}
          <div className="flex flex-wrap items-center gap-1.5 mt-2">
            {w.nodes.map((n) => (
              <span key={n.key} className="inline-flex items-center gap-1 rounded bg-[var(--color-surface-2)] px-1.5 py-0.5 text-[10px] text-[var(--color-fg-label)]">
                <span className="size-1.5 rounded-full" style={{ background: departmentColor(n.department) }} />
                {n.department}
              </span>
            ))}
          </div>
        </div>
        <div className="flex items-center gap-1 shrink-0">
          <button
            onClick={() => runMut.mutate()}
            disabled={runMut.isPending}
            className="flex items-center gap-1 rounded-md bg-[var(--color-brand)] px-2.5 py-1.5 text-xs font-medium text-white hover:bg-[var(--color-brand-hover)] disabled:opacity-50 transition-colors"
          >
            {runMut.isPending ? <Loader2 size={13} className="animate-spin" /> : <Play size={13} />} Run
          </button>
          {canManage && (
            <>
              <button onClick={onEdit} className="size-7 flex items-center justify-center rounded-md text-[var(--color-fg-muted)] hover:bg-[var(--color-surface-2)] hover:text-[var(--color-fg)]" title="Edit">
                <Pencil size={13} />
              </button>
              <button onClick={() => deleteMut.mutate()} disabled={deleteMut.isPending} className="size-7 flex items-center justify-center rounded-md text-[var(--color-fg-muted)] hover:bg-red-100 hover:text-red-500 dark:hover:bg-red-950/40 disabled:opacity-40" title="Delete">
                {deleteMut.isPending ? <Loader2 size={13} className="animate-spin" /> : <Trash2 size={13} />}
              </button>
            </>
          )}
        </div>
      </div>
      <button
        onClick={() => setOpen((v) => !v)}
        className="w-full flex items-center gap-1.5 border-t border-[var(--color-border)] px-4 py-2 text-xs text-[var(--color-fg-muted)] hover:text-[var(--color-fg)] transition-colors"
      >
        {open ? <ChevronDown size={13} /> : <ChevronRight size={13} />} Run history
      </button>
      {open && (
        <div className="border-t border-[var(--color-border)] px-4 py-3">
          {runsQ.isLoading ? (
            <Loader2 size={14} className="animate-spin text-[var(--color-fg-muted)] mx-auto" />
          ) : (runsQ.data ?? []).length === 0 ? (
            <p className="text-xs text-[var(--color-fg-subtle)]">No runs yet.</p>
          ) : (
            <ul className="flex flex-col gap-1">
              {(runsQ.data ?? []).map((run) => (
                <li key={run.id}>
                  <button
                    onClick={() => onOpenWorkflow(run.id)}
                    className="w-full flex items-center justify-between gap-2 rounded px-2 py-1.5 text-left hover:bg-[var(--color-surface-2)] transition-colors"
                  >
                    <span className="text-xs text-[var(--color-fg-muted)]">{new Date(run.created_at).toLocaleString()}</span>
                    <span className={`shrink-0 rounded px-1.5 py-0.5 text-[10px] font-medium ${statusBadgeClass(run.status as RequestStatus)}`}>
                      {prettyLabel(run.status)}
                    </span>
                  </button>
                </li>
              ))}
            </ul>
          )}
        </div>
      )}
    </div>
  );
}

function WorkflowEditor({
  orgId,
  agents,
  authorableTeams,
  canAuthorGlobal,
  workflow,
  onClose,
}: {
  orgId: string;
  agents: AgentRosterEntry[];
  authorableTeams: { id: string; name: string }[];
  canAuthorGlobal: boolean;
  workflow: WorkflowDef | null;
  onClose: () => void;
}) {
  const qc = useQueryClient();
  const toasts = useToasts();
  const [name, setName] = useState(workflow?.name ?? "");
  const [description, setDescription] = useState(workflow?.description ?? "");
  const [scope, setScope] = useState<"global" | "team">(
    workflow?.scope ?? (canAuthorGlobal ? "global" : "team"),
  );
  const [teamId, setTeamId] = useState<string>(workflow?.team_id ?? authorableTeams[0]?.id ?? "");
  const [steps, setSteps] = useState<WorkflowStep[]>(workflow?.nodes ?? []);

  const save = useMutation({
    mutationFn: () => {
      const payload = {
        name: name.trim(),
        description: description.trim(),
        category: workflow?.category ?? "general",
        scope,
        team_id: scope === "team" ? teamId : null,
        nodes: steps,
        edges: linearEdges(steps),
      };
      return workflow ? api.updateWorkflow(orgId, workflow.id, payload) : api.createWorkflow(orgId, payload);
    },
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["workflows", orgId] });
      onClose();
    },
    onError: (e: Error) => toasts.push({ kind: "error", title: e.message }),
  });

  const canSave = name.trim().length > 0 && steps.length > 0 && (scope === "global" || teamId) && !save.isPending;

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center">
      <div className="absolute inset-0 bg-black/40" onClick={onClose} />
      <div className="relative bg-[var(--color-surface)] rounded-xl shadow-stripe-elevated w-full max-w-lg max-h-[88vh] overflow-auto border border-[var(--color-border)]">
        <div className="sticky top-0 bg-[var(--color-surface)] flex items-center justify-between px-5 py-3.5 border-b border-[var(--color-border)]">
          <h2 className="text-base font-medium text-[var(--color-fg)]">{workflow ? "Edit workflow" : "New workflow"}</h2>
          <button onClick={onClose} className="text-[var(--color-fg-muted)] hover:text-[var(--color-fg)]"><X size={18} /></button>
        </div>
        <div className="p-5 flex flex-col gap-4">
          <div>
            <label className="block text-xs font-medium text-[var(--color-fg-label)] mb-1">Name</label>
            <input
              value={name}
              onChange={(e) => setName(e.target.value)}
              placeholder="e.g. Hiring"
              className="w-full rounded-lg border border-[var(--color-border)] bg-[var(--color-bg)] px-3 py-2 text-sm text-[var(--color-fg)] focus:outline-none focus:ring-1 focus:ring-[var(--color-brand)]"
            />
          </div>
          <div>
            <label className="block text-xs font-medium text-[var(--color-fg-label)] mb-1">Description</label>
            <textarea
              value={description}
              onChange={(e) => setDescription(e.target.value)}
              rows={2}
              placeholder="What this process is for."
              className="w-full rounded-lg border border-[var(--color-border)] bg-[var(--color-bg)] px-3 py-2 text-sm text-[var(--color-fg)] focus:outline-none focus:ring-1 focus:ring-[var(--color-brand)] resize-none"
            />
          </div>
          <div className="flex gap-3">
            <div className="flex-1">
              <label className="block text-xs font-medium text-[var(--color-fg-label)] mb-1">Scope</label>
              <select
                value={scope}
                onChange={(e) => setScope(e.target.value as "global" | "team")}
                className="w-full rounded-lg border border-[var(--color-border)] bg-[var(--color-bg)] px-3 py-2 text-sm text-[var(--color-fg)] focus:outline-none focus:ring-1 focus:ring-[var(--color-brand)]"
              >
                {canAuthorGlobal && <option value="global">Global (whole company)</option>}
                {authorableTeams.length > 0 && <option value="team">A department</option>}
              </select>
            </div>
            {scope === "team" && (
              <div className="flex-1">
                <label className="block text-xs font-medium text-[var(--color-fg-label)] mb-1">Department</label>
                <select
                  value={teamId}
                  onChange={(e) => setTeamId(e.target.value)}
                  className="w-full rounded-lg border border-[var(--color-border)] bg-[var(--color-bg)] px-3 py-2 text-sm text-[var(--color-fg)] focus:outline-none focus:ring-1 focus:ring-[var(--color-brand)]"
                >
                  {authorableTeams.map((t) => (
                    <option key={t.id} value={t.id}>{t.name}</option>
                  ))}
                </select>
              </div>
            )}
          </div>
          <div>
            <label className="block text-xs font-medium text-[var(--color-fg-label)] mb-1.5">Steps</label>
            <PlanEditor steps={steps} agents={agents} onChange={setSteps} />
          </div>
        </div>
        <div className="sticky bottom-0 bg-[var(--color-surface)] flex justify-end gap-2 px-5 py-3.5 border-t border-[var(--color-border)]">
          <button onClick={onClose} className="px-3 py-1.5 text-sm rounded-lg text-[var(--color-fg-muted)] hover:bg-[var(--color-surface-2)]">Cancel</button>
          <button
            onClick={() => save.mutate()}
            disabled={!canSave}
            className="flex items-center gap-1.5 px-3 py-1.5 text-sm rounded-lg bg-[var(--color-brand)] text-white font-medium hover:bg-[var(--color-brand-hover)] disabled:opacity-40"
          >
            {save.isPending && <Loader2 size={13} className="animate-spin" />}
            {workflow ? "Save changes" : "Create workflow"}
          </button>
        </div>
      </div>
    </div>
  );
}
