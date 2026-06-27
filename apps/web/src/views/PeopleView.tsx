import { useMemo, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Loader2, Plus, ShieldCheck, X } from "lucide-react";

import { api } from "../lib/api";
import { useToasts } from "../components/Toasts";
import { Avatar } from "../components/Avatar";
import { PageHeader, EmptyState } from "../components/ui";
import { departmentColor } from "../lib/department";

type TeamRoleEntry = { team: string; role: string };
type OrgMember = {
  id: number;
  name: string;
  email: string;
  role: string;
  team_roles?: TeamRoleEntry[];
  joined_at: string;
};

// Org-level roles, with a one-line meaning so an admin understands the grant.
const ORG_ROLES: { value: string; label: string; hint: string }[] = [
  { value: "admin", label: "Admin", hint: "The executive. Can verify any department and manage everyone." },
  { value: "executor", label: "Executor", hint: "Runs and manages workflows. Verifies only their own departments." },
  { value: "employee", label: "Employee", hint: "Verifies their own departments. No org administration." },
];

const TEAM_ROLES = ["lead", "member", "technician"] as const;

// PeopleView — a dedicated place to manage who can do what: each person's org
// role and which departments (teams) they belong to. Distinct from the Teams
// page, which is organized around team structure rather than per-person access.
export function PeopleView({ orgId, role }: { orgId: string; role: string }) {
  const isAdmin = role === "admin";

  const membersQ = useQuery({
    queryKey: ["org-members", orgId],
    queryFn: () => api.listOrgMembers(orgId).then((r) => r.members),
  });
  const teamsQ = useQuery({
    queryKey: ["teams", orgId],
    queryFn: () => api.listTeams(orgId).then((r) => r.teams),
  });

  const members = (membersQ.data ?? []) as OrgMember[];
  const teams = teamsQ.data ?? [];

  return (
    <div className="flex-1 flex flex-col overflow-hidden">
      <PageHeader
        title="People & Access"
        subtitle={
          isAdmin
            ? "Set each person's org role and the departments they belong to."
            : "The people in this organization and their access. Only an admin can make changes."
        }
      />

      <div className="flex-1 overflow-auto px-8 py-6">
        {membersQ.isLoading ? (
          <div className="flex justify-center py-16">
            <Loader2 size={20} className="animate-spin text-[var(--color-fg-muted)]" />
          </div>
        ) : members.length === 0 ? (
          <EmptyState icon={ShieldCheck} title="No people yet" hint="Invite members from the Teams page." />
        ) : (
          <div className="flex flex-col gap-3 w-full max-w-4xl">
            {members.map((m) => (
              <PersonRow key={m.id} orgId={orgId} member={m} teams={teams} isAdmin={isAdmin} />
            ))}
          </div>
        )}
      </div>
    </div>
  );
}

function PersonRow({
  orgId,
  member,
  teams,
  isAdmin,
}: {
  orgId: string;
  member: OrgMember;
  teams: { id: string; name: string }[];
  isAdmin: boolean;
}) {
  const qc = useQueryClient();
  const toasts = useToasts();
  const [addTeamId, setAddTeamId] = useState("");
  const [addRole, setAddRole] = useState<(typeof TEAM_ROLES)[number]>("member");

  const refreshPeople = () => {
    qc.invalidateQueries({ queryKey: ["org-members", orgId] });
    qc.invalidateQueries({ queryKey: ["teams", orgId] });
  };

  const roleMut = useMutation({
    mutationFn: (newRole: string) => api.updateOrgMemberRole(orgId, member.id, newRole),
    onSuccess: refreshPeople,
    onError: (e: Error) => toasts.push({ kind: "error", title: e.message }),
  });
  const addMut = useMutation({
    mutationFn: ({ teamId, role }: { teamId: string; role: string }) =>
      api.addTeamMember(orgId, teamId, { user_id: member.id, role }),
    onSuccess: () => {
      setAddTeamId("");
      refreshPeople();
    },
    onError: (e: Error) => toasts.push({ kind: "error", title: e.message }),
  });
  const removeMut = useMutation({
    mutationFn: (teamId: string) => api.removeTeamMember(orgId, teamId, member.id),
    onSuccess: refreshPeople,
    onError: (e: Error) => toasts.push({ kind: "error", title: e.message }),
  });

  const teamRoles = member.team_roles ?? [];
  // Map a team name (what team_roles carries) to its id, for remove calls.
  const teamIdByName = useMemo(() => {
    const map = new Map<string, string>();
    for (const t of teams) map.set(t.name.toLowerCase(), t.id);
    return map;
  }, [teams]);
  const joinedNames = new Set(teamRoles.map((tr) => tr.team.toLowerCase()));
  const availableTeams = teams.filter((t) => !joinedNames.has(t.name.toLowerCase()));

  const roleHint = ORG_ROLES.find((r) => r.value === member.role)?.hint;

  return (
    <div className="rounded-xl border border-[var(--color-border)] bg-[var(--color-surface)] p-4">
      <div className="flex items-start gap-3">
        <Avatar name={member.name || member.email} size={36} />
        <div className="min-w-0 flex-1">
          <p className="text-sm font-medium text-[var(--color-fg)] truncate">{member.name || "—"}</p>
          <p className="text-xs text-[var(--color-fg-muted)] truncate">{member.email}</p>
        </div>

        {/* Org role */}
        <div className="shrink-0 text-right">
          {isAdmin ? (
            <select
              value={member.role}
              onChange={(e) => roleMut.mutate(e.target.value)}
              disabled={roleMut.isPending}
              className="rounded-lg border border-[var(--color-border)] bg-[var(--color-bg)] px-2 py-1 text-xs text-[var(--color-fg)] outline-none focus:border-[var(--color-brand)] transition-colors disabled:opacity-50"
            >
              {ORG_ROLES.map((r) => (
                <option key={r.value} value={r.value}>
                  {r.label}
                </option>
              ))}
            </select>
          ) : (
            <span className="inline-block rounded-md border border-[var(--color-border)] px-2 py-0.5 text-xs text-[var(--color-fg-muted)] capitalize">
              {member.role}
            </span>
          )}
          {isAdmin && roleHint && (
            <p className="text-[10px] text-[var(--color-fg-subtle)] mt-1 max-w-[16rem] text-right leading-snug">
              {roleHint}
            </p>
          )}
        </div>
      </div>

      {/* Departments */}
      <div className="mt-3 pt-3 border-t border-[var(--color-border)]">
        <p className="text-[10px] uppercase tracking-wide text-[var(--color-fg-muted)] mb-2">Departments</p>
        {teamRoles.length === 0 ? (
          <p className="text-xs text-[var(--color-fg-subtle)]">Not in any department.</p>
        ) : (
          <div className="flex flex-wrap gap-1.5">
            {teamRoles.map((tr, i) => {
              const teamId = teamIdByName.get(tr.team.toLowerCase());
              return (
                <span
                  key={i}
                  className="inline-flex items-center gap-1.5 rounded-md border border-[var(--color-border)] bg-[var(--color-surface-2)] pl-1.5 pr-1 py-0.5 text-[11px] text-[var(--color-fg-label)]"
                >
                  <span className="size-1.5 rounded-full" style={{ background: departmentColor(tr.team) }} />
                  {tr.team}
                  <span className="text-[var(--color-fg-subtle)]">· {tr.role}</span>
                  {isAdmin && teamId && (
                    <button
                      onClick={() => removeMut.mutate(teamId)}
                      className="ml-0.5 text-[var(--color-fg-subtle)] hover:text-[var(--color-danger)]"
                      title={`Remove from ${tr.team}`}
                    >
                      <X size={11} />
                    </button>
                  )}
                </span>
              );
            })}
          </div>
        )}

        {isAdmin && availableTeams.length > 0 && (
          <div className="flex gap-1.5 mt-2.5 flex-wrap">
            <select
              value={addTeamId}
              onChange={(e) => setAddTeamId(e.target.value)}
              className="rounded-lg border border-[var(--color-border)] bg-[var(--color-bg)] px-2 py-1 text-xs text-[var(--color-fg)] outline-none focus:border-[var(--color-brand)] transition-colors"
            >
              <option value="">Add to department…</option>
              {availableTeams.map((t) => (
                <option key={t.id} value={t.id}>
                  {t.name}
                </option>
              ))}
            </select>
            <select
              value={addRole}
              onChange={(e) => setAddRole(e.target.value as (typeof TEAM_ROLES)[number])}
              className="rounded-lg border border-[var(--color-border)] bg-[var(--color-bg)] px-2 py-1 text-xs text-[var(--color-fg)] outline-none focus:border-[var(--color-brand)] transition-colors capitalize"
            >
              {TEAM_ROLES.map((r) => (
                <option key={r} value={r}>
                  {r}
                </option>
              ))}
            </select>
            <button
              disabled={!addTeamId || addMut.isPending}
              onClick={() => addTeamId && addMut.mutate({ teamId: addTeamId, role: addRole })}
              className="flex items-center gap-1 rounded-lg bg-[var(--color-brand)] px-2.5 py-1 text-xs font-medium text-white hover:bg-[var(--color-brand-hover)] disabled:opacity-40 transition-colors"
            >
              {addMut.isPending ? <Loader2 size={12} className="animate-spin" /> : <Plus size={12} />}
              Add
            </button>
          </div>
        )}
      </div>
    </div>
  );
}
