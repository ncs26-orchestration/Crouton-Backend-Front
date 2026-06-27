import { useMemo, useState } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { ScrollText, Plus, Pencil, Trash2, X } from "lucide-react";

import { api } from "../lib/api";
import { useToasts } from "../components/Toasts";
import type { DepartmentPolicy, PolicyRule } from "../lib/types";

interface PolicyGroup {
  department: string;
  policies: DepartmentPolicy[];
}

const OPS: PolicyRule["op"][] = ["lte", "gte", "lt", "gt", "eq", "ne", "exists"];
const OP_LABEL: Record<PolicyRule["op"], string> = {
  lte: "≤", gte: "≥", lt: "<", gt: ">", eq: "=", ne: "≠", exists: "exists",
};

export function PoliciesView({ orgId, role }: { orgId: string; role: string }) {
  const canEdit = role === "admin" || role === "executor";
  const { data, isLoading, isError, error } = useQuery({
    queryKey: ["policies", orgId],
    queryFn: () => api.listPolicies(orgId),
  });
  const groups = useMemo(() => groupByDepartment(data?.policies ?? []), [data]);
  const [editing, setEditing] = useState<DepartmentPolicy | "new" | null>(null);

  return (
    <div className="flex-1 flex flex-col bg-[var(--color-bg)] text-[var(--color-fg)] overflow-auto nice-scroll">
      <div className="border-b border-[var(--color-border)] px-8 py-5 flex items-start justify-between">
        <div>
          <h1 className="text-xl font-medium" style={{ fontFeatureSettings: '"ss01"' }}>
            Policies
          </h1>
          <p className="text-sm text-[var(--color-fg-muted)] mt-0.5">
            The rules each department agent checks a request against. Rules are evaluated automatically.
          </p>
        </div>
        {canEdit && (
          <button
            onClick={() => setEditing("new")}
            className="flex items-center gap-1.5 rounded-md bg-[var(--color-brand)] px-3 py-2 text-sm font-medium text-white hover:bg-[var(--color-brand-hover)]"
          >
            <Plus size={15} /> New policy
          </button>
        )}
      </div>

      <div className="px-8 py-6 w-full max-w-[860px]">
        {isLoading ? (
          <div className="flex flex-col gap-3">
            {Array.from({ length: 4 }).map((_, i) => (
              <div key={i} className="h-24 rounded-lg bg-[var(--color-surface-2)] animate-pulse" />
            ))}
          </div>
        ) : isError ? (
          <p className="text-sm text-[var(--color-danger)]">
            Could not load policies. {(error as Error)?.message}
          </p>
        ) : groups.length === 0 ? (
          <EmptyState />
        ) : (
          <div className="flex flex-col gap-8">
            {groups.map((g) => (
              <section key={g.department}>
                <h2 className="text-[11px] font-semibold uppercase tracking-wide text-[var(--color-fg-muted)] mb-3">
                  {g.department}
                </h2>
                <div className="flex flex-col gap-3">
                  {g.policies.map((p) => (
                    <PolicyCard key={p.id} policy={p} canEdit={canEdit} onEdit={() => setEditing(p)} orgId={orgId} />
                  ))}
                </div>
              </section>
            ))}
          </div>
        )}
      </div>

      {editing && (
        <PolicyEditor
          orgId={orgId}
          policy={editing === "new" ? null : editing}
          onClose={() => setEditing(null)}
        />
      )}
    </div>
  );
}

function groupByDepartment(policies: DepartmentPolicy[]): PolicyGroup[] {
  const byDept = new Map<string, DepartmentPolicy[]>();
  for (const p of policies) {
    const dept = p.team_name || "General";
    const list = byDept.get(dept) ?? [];
    list.push(p);
    byDept.set(dept, list);
  }
  return [...byDept.entries()]
    .map(([department, list]) => ({ department, policies: list }))
    .sort((a, b) => a.department.localeCompare(b.department));
}

function PolicyCard({
  policy,
  canEdit,
  onEdit,
  orgId,
}: {
  policy: DepartmentPolicy;
  canEdit: boolean;
  onEdit: () => void;
  orgId: string;
}) {
  const qc = useQueryClient();
  const toasts = useToasts();
  const del = useMutation({
    mutationFn: () => api.deletePolicy(orgId, policy.id),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["policies", orgId] }),
    onError: (e: Error) => toasts.push({ kind: "error", title: e.message }),
  });
  const rules = policy.rules ?? [];

  return (
    <article className="rounded-lg border border-[var(--color-border)] bg-[var(--color-surface)] p-4 shadow-stripe-ambient">
      <div className="flex items-start gap-3">
        <div className="size-9 rounded-md bg-[var(--color-accent-bg)] border border-[var(--color-accent-border)] flex items-center justify-center shrink-0">
          <ScrollText size={16} className="text-[var(--color-brand)]" strokeWidth={1.75} />
        </div>
        <div className="min-w-0 flex-1">
          <h3 className="text-sm font-medium text-[var(--color-fg)]">{policy.title}</h3>
          <p className="text-xs text-[var(--color-fg-muted)] leading-relaxed mt-1">{policy.body}</p>
          {rules.length > 0 && (
            <ul className="flex flex-col gap-1 mt-2">
              {rules.map((r, i) => (
                <li key={i} className="text-[11px] text-[var(--color-fg-label)] flex items-center gap-1.5">
                  <span className="rounded bg-[var(--color-surface-2)] px-1.5 py-0.5 font-mono">
                    {r.field} {OP_LABEL[r.op]} {String(r.value)}
                  </span>
                  <span className="text-[var(--color-fg-muted)] truncate">{r.label || r.message}</span>
                </li>
              ))}
            </ul>
          )}
        </div>
        {canEdit && (
          <div className="flex gap-1 shrink-0">
            <button onClick={onEdit} className="size-7 flex items-center justify-center rounded text-[var(--color-fg-muted)] hover:text-[var(--color-fg)] hover:bg-[var(--color-surface-2)]" title="Edit">
              <Pencil size={13} />
            </button>
            <button onClick={() => del.mutate()} className="size-7 flex items-center justify-center rounded text-[var(--color-fg-muted)] hover:text-[var(--color-danger)] hover:bg-[var(--color-surface-2)]" title="Delete">
              <Trash2 size={13} />
            </button>
          </div>
        )}
      </div>
    </article>
  );
}

function blankRule(): PolicyRule {
  return { label: "", field: "", op: "lte", value: "", severity: "warning", message: "" };
}

function PolicyEditor({
  orgId,
  policy,
  onClose,
}: {
  orgId: string;
  policy: DepartmentPolicy | null;
  onClose: () => void;
}) {
  const qc = useQueryClient();
  const toasts = useToasts();
  const isNew = policy === null;
  const teamsQuery = useQuery({ queryKey: ["teams", orgId], queryFn: () => api.listTeams(orgId), enabled: isNew });

  const [teamId, setTeamId] = useState(policy?.team_id ?? "");
  const [title, setTitle] = useState(policy?.title ?? "");
  const [body, setBody] = useState(policy?.body ?? "");
  const [rules, setRules] = useState<PolicyRule[]>(policy?.rules ?? []);

  const save = useMutation({
    mutationFn: () => {
      // Coerce numeric rule values where they parse as numbers.
      const cleaned = rules
        .filter((r) => r.field.trim())
        .map((r) => ({ ...r, value: r.value !== "" && !Number.isNaN(Number(r.value)) ? Number(r.value) : r.value }));
      return isNew
        ? api.createPolicy(orgId, { team_id: teamId, title: title.trim(), body: body.trim(), rules: cleaned })
        : api.updatePolicy(orgId, policy!.id, { title: title.trim(), body: body.trim(), rules: cleaned });
    },
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["policies", orgId] });
      onClose();
    },
    onError: (e: Error) => toasts.push({ kind: "error", title: e.message }),
  });

  const valid = title.trim() && (isNew ? teamId : true);

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center px-4">
      <div className="absolute inset-0 bg-black/40" onClick={onClose} />
      <div className="relative bg-[var(--color-surface)] rounded-lg shadow-stripe-elevated w-full max-w-xl p-6 border border-[var(--color-border)] max-h-[88vh] overflow-y-auto nice-scroll">
        <div className="flex items-start justify-between mb-4">
          <h2 className="text-base font-medium text-[var(--color-fg)]">{isNew ? "New policy" : "Edit policy"}</h2>
          <button onClick={onClose} className="text-[var(--color-fg-muted)] hover:text-[var(--color-fg)]"><X size={18} /></button>
        </div>

        <div className="flex flex-col gap-3">
          {isNew && (
            <div>
              <label className="block text-xs font-medium text-[var(--color-fg-label)] mb-1">Department</label>
              <select
                value={teamId}
                onChange={(e) => setTeamId(e.target.value)}
                className="w-full px-3 py-2 text-sm rounded border border-[var(--color-border)] bg-[var(--color-bg)] text-[var(--color-fg)] focus:outline-none focus:ring-2 focus:ring-[var(--color-brand)]"
              >
                <option value="">Select a department…</option>
                {(teamsQuery.data?.teams ?? []).map((t) => (
                  <option key={t.id} value={t.id}>{t.name}</option>
                ))}
              </select>
            </div>
          )}
          <div>
            <label className="block text-xs font-medium text-[var(--color-fg-label)] mb-1">Title</label>
            <input
              value={title}
              onChange={(e) => setTitle(e.target.value)}
              placeholder="Finance Policy"
              className="w-full px-3 py-2 text-sm rounded border border-[var(--color-border)] bg-[var(--color-bg)] text-[var(--color-fg)] focus:outline-none focus:ring-2 focus:ring-[var(--color-brand)]"
            />
          </div>
          <div>
            <label className="block text-xs font-medium text-[var(--color-fg-label)] mb-1">Description</label>
            <textarea
              value={body}
              onChange={(e) => setBody(e.target.value)}
              rows={2}
              placeholder="What this policy covers…"
              className="w-full px-3 py-2 text-sm rounded border border-[var(--color-border)] bg-[var(--color-bg)] text-[var(--color-fg)] focus:outline-none focus:ring-2 focus:ring-[var(--color-brand)] resize-none"
            />
          </div>

          {/* Rules */}
          <div>
            <div className="flex items-center justify-between mb-1.5">
              <label className="text-xs font-medium text-[var(--color-fg-label)]">Rules (checked against request details)</label>
              <button
                onClick={() => setRules((r) => [...r, blankRule()])}
                className="text-xs text-[var(--color-brand)] hover:underline flex items-center gap-1"
              >
                <Plus size={12} /> Add rule
              </button>
            </div>
            {rules.length === 0 ? (
              <p className="text-[11px] text-[var(--color-fg-subtle)]">No rules. Add one to check a field like total_cost or headcount.</p>
            ) : (
              <div className="flex flex-col gap-2">
                {rules.map((r, i) => (
                  <div key={i} className="rounded-md border border-[var(--color-border)] bg-[var(--color-surface-2)] p-2 flex flex-col gap-1.5">
                    <div className="flex gap-1.5">
                      <input
                        value={r.field}
                        onChange={(e) => updateRule(setRules, i, { field: e.target.value })}
                        placeholder="field (e.g. total_cost)"
                        className="flex-1 min-w-0 px-2 py-1 text-xs rounded border border-[var(--color-border)] bg-[var(--color-bg)] text-[var(--color-fg)] font-mono"
                      />
                      <select
                        value={r.op}
                        onChange={(e) => updateRule(setRules, i, { op: e.target.value as PolicyRule["op"] })}
                        className="px-1.5 py-1 text-xs rounded border border-[var(--color-border)] bg-[var(--color-bg)] text-[var(--color-fg)]"
                      >
                        {OPS.map((o) => <option key={o} value={o}>{OP_LABEL[o]}</option>)}
                      </select>
                      <input
                        value={String(r.value)}
                        onChange={(e) => updateRule(setRules, i, { value: e.target.value })}
                        placeholder="value"
                        disabled={r.op === "exists"}
                        className="w-20 px-2 py-1 text-xs rounded border border-[var(--color-border)] bg-[var(--color-bg)] text-[var(--color-fg)] disabled:opacity-40"
                      />
                      <select
                        value={r.severity}
                        onChange={(e) => updateRule(setRules, i, { severity: e.target.value as PolicyRule["severity"] })}
                        className="px-1.5 py-1 text-xs rounded border border-[var(--color-border)] bg-[var(--color-bg)] text-[var(--color-fg)]"
                      >
                        <option value="info">info</option>
                        <option value="warning">warning</option>
                        <option value="critical">critical</option>
                      </select>
                      <button onClick={() => setRules((rs) => rs.filter((_, j) => j !== i))} className="text-[var(--color-fg-muted)] hover:text-[var(--color-danger)] shrink-0"><X size={13} /></button>
                    </div>
                    <input
                      value={r.message}
                      onChange={(e) => updateRule(setRules, i, { message: e.target.value })}
                      placeholder="Message shown when this rule fails"
                      className="w-full px-2 py-1 text-xs rounded border border-[var(--color-border)] bg-[var(--color-bg)] text-[var(--color-fg)]"
                    />
                  </div>
                ))}
              </div>
            )}
          </div>
        </div>

        <div className="flex justify-end gap-2 mt-5">
          <button onClick={onClose} className="px-3 py-2 text-sm rounded border border-[var(--color-border)] text-[var(--color-fg-muted)] hover:bg-[var(--color-surface-2)]">
            Cancel
          </button>
          <button
            onClick={() => save.mutate()}
            disabled={!valid || save.isPending}
            className="px-3 py-2 text-sm rounded bg-[var(--color-brand)] font-medium text-white hover:bg-[var(--color-brand-hover)] disabled:opacity-50"
          >
            {save.isPending ? "Saving…" : "Save policy"}
          </button>
        </div>
      </div>
    </div>
  );
}

function updateRule(
  setRules: React.Dispatch<React.SetStateAction<PolicyRule[]>>,
  index: number,
  patch: Partial<PolicyRule>,
) {
  setRules((rs) => rs.map((r, i) => (i === index ? { ...r, ...patch } : r)));
}

function EmptyState() {
  return (
    <div className="flex flex-col items-center justify-center gap-4 py-20">
      <div className="size-14 rounded-2xl bg-[var(--color-accent-bg)] border border-[var(--color-accent-border)] flex items-center justify-center">
        <ScrollText size={24} className="text-[var(--color-brand)]" strokeWidth={1.5} />
      </div>
      <div className="text-center">
        <p className="text-sm font-medium text-[var(--color-fg)]">No policies yet</p>
        <p className="text-xs text-[var(--color-fg-muted)] mt-1">Add one to start checking requests against your rules.</p>
      </div>
    </div>
  );
}
