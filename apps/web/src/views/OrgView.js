import { useState } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { ChevronDown, ChevronRight, Loader2, Plus, Trash2, Users, X, } from "lucide-react";
import { api } from "../lib/api";
import { useOrg } from "../contexts/OrgContext";
import { useToasts } from "../components/Toasts";
// ── OrgView ────────────────────────────────────────────────────────────────
export function OrgView() {
    const { activeOrg } = useOrg();
    const [tab, setTab] = useState("teams");
    if (!activeOrg)
        return null;
    return (<div className="flex-1 flex flex-col overflow-hidden bg-[var(--color-bg)]">
      {/* Header */}
      <div className="px-8 pt-8 pb-4 border-b border-[var(--color-border)] shrink-0">
        <h1 className="text-xl font-semibold text-[var(--color-fg)]">{activeOrg.name}</h1>
        <p className="text-sm text-[var(--color-fg-muted)] mt-0.5">
          Manage your organization's teams and members
        </p>

        {/* Tabs */}
        <div className="flex gap-1 mt-5">
          {["teams", "members"].map((t) => (<button key={t} onClick={() => setTab(t)} className={`px-4 py-1.5 rounded-lg text-sm font-medium transition-colors capitalize ${tab === t
                ? "bg-[var(--color-accent-bg)] text-[var(--color-brand)]"
                : "text-[var(--color-fg-muted)] hover:text-[var(--color-fg)] hover:bg-[var(--color-surface-2)]"}`}>
              {t}
            </button>))}
        </div>
      </div>

      {/* Content */}
      <div className="flex-1 overflow-y-auto px-8 py-6">
        {tab === "teams" && <TeamsTab orgId={activeOrg.id}/>}
        {tab === "members" && <MembersTab orgId={activeOrg.id}/>}
      </div>
    </div>);
}
// ── Teams tab ──────────────────────────────────────────────────────────────
function TeamsTab({ orgId }) {
    const qc = useQueryClient();
    const toasts = useToasts();
    const [showCreate, setShowCreate] = useState(false);
    const [newName, setNewName] = useState("");
    const [newDesc, setNewDesc] = useState("");
    const [expandedId, setExpandedId] = useState(null);
    const teamsQ = useQuery({
        queryKey: ["teams", orgId],
        queryFn: () => api.listTeams(orgId).then((r) => r.teams),
    });
    const membersQ = useQuery({
        queryKey: ["org-members", orgId],
        queryFn: () => api.listOrgMembers(orgId).then((r) => r.members),
    });
    const createMut = useMutation({
        mutationFn: () => api.createTeam(orgId, { name: newName.trim(), description: newDesc.trim() }),
        onSuccess: () => {
            qc.invalidateQueries({ queryKey: ["teams", orgId] });
            setShowCreate(false);
            setNewName("");
            setNewDesc("");
        },
        onError: (e) => toasts.push({ kind: "error", title: e.message }),
    });
    const deleteMut = useMutation({
        mutationFn: (teamId) => api.deleteTeam(orgId, teamId),
        onSuccess: () => qc.invalidateQueries({ queryKey: ["teams", orgId] }),
        onError: (e) => toasts.push({ kind: "error", title: e.message }),
    });
    const teams = teamsQ.data ?? [];
    const members = membersQ.data ?? [];
    return (<div className="flex flex-col gap-4 max-w-2xl">
      {/* Create button */}
      <div className="flex justify-end">
        <button onClick={() => setShowCreate((v) => !v)} className="flex items-center gap-2 px-3 py-1.5 rounded-lg bg-[var(--color-brand)] text-white text-sm font-medium hover:opacity-90 transition-opacity">
          <Plus size={14}/> New team
        </button>
      </div>

      {/* Inline create form */}
      {showCreate && (<div className="rounded-xl border border-[var(--color-brand)] bg-[var(--color-accent-bg)] p-4 flex flex-col gap-3">
          <input autoFocus value={newName} onChange={(e) => setNewName(e.target.value)} placeholder="Team name" className="rounded-lg border border-[var(--color-border)] bg-[var(--color-bg)] px-3 py-2 text-sm text-[var(--color-fg)] placeholder-[var(--color-fg-muted)] outline-none focus:border-[var(--color-brand)] transition-colors"/>
          <input value={newDesc} onChange={(e) => setNewDesc(e.target.value)} placeholder="Description (optional)" className="rounded-lg border border-[var(--color-border)] bg-[var(--color-bg)] px-3 py-2 text-sm text-[var(--color-fg)] placeholder-[var(--color-fg-muted)] outline-none focus:border-[var(--color-brand)] transition-colors"/>
          <div className="flex gap-2 justify-end">
            <button onClick={() => setShowCreate(false)} className="px-3 py-1.5 rounded-lg text-sm text-[var(--color-fg-muted)] hover:bg-[var(--color-surface-2)] transition-colors">
              Cancel
            </button>
            <button disabled={!newName.trim() || createMut.isPending} onClick={() => createMut.mutate()} className="flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-[var(--color-brand)] text-white text-sm font-medium hover:opacity-90 disabled:opacity-40 transition-opacity">
              {createMut.isPending && <Loader2 size={13} className="animate-spin"/>}
              Create
            </button>
          </div>
        </div>)}

      {/* Loading */}
      {teamsQ.isLoading && (<div className="flex justify-center py-12">
          <Loader2 size={20} className="animate-spin text-[var(--color-fg-muted)]"/>
        </div>)}

      {/* Empty */}
      {!teamsQ.isLoading && teams.length === 0 && (<div className="flex flex-col items-center gap-3 py-16 text-center">
          <div className="size-12 rounded-xl bg-[var(--color-surface-2)] flex items-center justify-center">
            <Users size={22} className="text-[var(--color-fg-muted)]" strokeWidth={1.5}/>
          </div>
          <p className="text-sm text-[var(--color-fg-muted)]">No teams yet. Create one to get started.</p>
        </div>)}

      {/* Teams list */}
      {teams.map((team) => (<TeamCard key={team.id} team={team} orgId={orgId} orgMembers={members} expanded={expandedId === team.id} onToggle={() => setExpandedId(expandedId === team.id ? null : team.id)} onDelete={() => deleteMut.mutate(team.id)} deleting={deleteMut.isPending && deleteMut.variables === team.id}/>))}
    </div>);
}
function TeamCard({ team, orgId, orgMembers, expanded, onToggle, onDelete, deleting }) {
    const qc = useQueryClient();
    const toasts = useToasts();
    const [selectedUserId, setSelectedUserId] = useState("");
    const [memberRole, setMemberRole] = useState("member");
    const [addError, setAddError] = useState(null);
    const teamMembersQ = useQuery({
        queryKey: ["team-members", orgId, team.id],
        queryFn: () => api.getTeam(orgId, team.id).then((r) => r.members ?? []),
        enabled: expanded,
    });
    const addMut = useMutation({
        mutationFn: () => api.addTeamMember(orgId, team.id, { user_id: selectedUserId, role: memberRole }),
        onSuccess: () => {
            qc.invalidateQueries({ queryKey: ["team-members", orgId, team.id] });
            setSelectedUserId("");
            setAddError(null);
        },
        onError: (e) => setAddError(e.message),
    });
    const removeMut = useMutation({
        mutationFn: (userId) => api.removeTeamMember(orgId, team.id, userId),
        onSuccess: () => qc.invalidateQueries({ queryKey: ["team-members", orgId, team.id] }),
        onError: (e) => toasts.push({ kind: "error", title: e.message }),
    });
    const teamMembers = teamMembersQ.data ?? [];
    // Org members not yet in this team
    const available = orgMembers.filter((m) => !teamMembers.some((tm) => tm.id === m.user_id));
    return (<div className="rounded-xl border border-[var(--color-border)] bg-[var(--color-surface)] overflow-hidden">
      {/* Team header row */}
      <div className="flex items-center gap-3 px-4 py-3">
        <button onClick={onToggle} className="flex items-center gap-3 flex-1 text-left min-w-0">
          <span className="shrink-0 text-[var(--color-fg-muted)]">
            {expanded ? <ChevronDown size={15}/> : <ChevronRight size={15}/>}
          </span>
          <span className="font-medium text-sm text-[var(--color-fg)] truncate">{team.name}</span>
          {team.description && (<span className="text-xs text-[var(--color-fg-muted)] truncate hidden sm:block">
              — {team.description}
            </span>)}
        </button>
        <button onClick={onDelete} disabled={deleting} className="shrink-0 size-7 flex items-center justify-center rounded-lg text-[var(--color-fg-muted)] hover:bg-red-100 hover:text-red-500 dark:hover:bg-red-950/40 disabled:opacity-40 transition-colors">
          {deleting ? <Loader2 size={13} className="animate-spin"/> : <Trash2 size={13}/>}
        </button>
      </div>

      {/* Expanded: members list + add member */}
      {expanded && (<div className="border-t border-[var(--color-border)] bg-[var(--color-bg)] px-4 py-4 flex flex-col gap-4">

          {/* Current members */}
          {teamMembersQ.isLoading ? (<Loader2 size={16} className="animate-spin text-[var(--color-fg-muted)] mx-auto"/>) : teamMembers.length === 0 ? (<p className="text-xs text-[var(--color-fg-muted)] text-center py-2">No members yet</p>) : (<ul className="flex flex-col gap-2">
              {teamMembers.map((m) => (<li key={m.id} className="flex items-center gap-3">
                  <div className="size-7 rounded-full bg-[var(--color-accent-bg)] flex items-center justify-center text-[10px] font-bold text-[var(--color-brand)] shrink-0">
                    {m.name?.[0]?.toUpperCase() ?? "?"}
                  </div>
                  <div className="flex-1 min-w-0">
                    <p className="text-sm text-[var(--color-fg)] truncate">{m.name}</p>
                    <p className="text-xs text-[var(--color-fg-muted)] truncate">{m.email}</p>
                  </div>
                  <span className="text-[10px] px-1.5 py-0.5 rounded border border-[var(--color-border)] text-[var(--color-fg-muted)] capitalize shrink-0">
                    {m.role}
                  </span>
                  <button onClick={() => removeMut.mutate(m.id)} className="shrink-0 text-[var(--color-fg-muted)] hover:text-red-500 transition-colors">
                    <X size={13}/>
                  </button>
                </li>))}
            </ul>)}

          {/* Add member row */}
          {available.length > 0 && (<div className="flex gap-2 flex-wrap">
              <select value={selectedUserId} onChange={(e) => setSelectedUserId(e.target.value === "" ? "" : Number(e.target.value))} className="flex-1 min-w-0 rounded-lg border border-[var(--color-border)] bg-[var(--color-bg)] px-3 py-1.5 text-sm text-[var(--color-fg)] outline-none focus:border-[var(--color-brand)] transition-colors">
                <option value="">Select member…</option>
                {available.map((m) => (<option key={m.user_id} value={m.user_id}>
                    {m.name} ({m.email})
                  </option>))}
              </select>
              <select value={memberRole} onChange={(e) => setMemberRole(e.target.value)} className="rounded-lg border border-[var(--color-border)] bg-[var(--color-bg)] px-3 py-1.5 text-sm text-[var(--color-fg)] outline-none focus:border-[var(--color-brand)] transition-colors">
                <option value="member">Member</option>
                <option value="lead">Lead</option>
              </select>
              <button disabled={selectedUserId === "" || addMut.isPending} onClick={() => addMut.mutate()} className="flex items-center gap-1 px-3 py-1.5 rounded-lg bg-[var(--color-brand)] text-white text-sm font-medium hover:opacity-90 disabled:opacity-40 transition-opacity shrink-0">
                {addMut.isPending ? <Loader2 size={13} className="animate-spin"/> : <Plus size={13}/>}
                Add
              </button>
            </div>)}

          {addError && (<p className="text-xs text-red-500">{addError}</p>)}

          {available.length === 0 && teamMembers.length > 0 && (<p className="text-xs text-[var(--color-fg-muted)]">
              All org members are already in this team.
            </p>)}
        </div>)}
    </div>);
}
// ── Members tab ────────────────────────────────────────────────────────────
function MembersTab({ orgId }) {
    const qc = useQueryClient();
    const toasts = useToasts();
    const membersQ = useQuery({
        queryKey: ["org-members", orgId],
        queryFn: () => api.listOrgMembers(orgId).then((r) => r.members),
    });
    const removeMut = useMutation({
        mutationFn: (userId) => api.removeOrgMember(orgId, userId),
        onSuccess: () => qc.invalidateQueries({ queryKey: ["org-members", orgId] }),
        onError: (e) => toasts.push({ kind: "error", title: e.message }),
    });
    const roleMut = useMutation({
        mutationFn: ({ userId, role }) => api.updateOrgMemberRole(orgId, userId, role),
        onSuccess: () => qc.invalidateQueries({ queryKey: ["org-members", orgId] }),
        onError: (e) => toasts.push({ kind: "error", title: e.message }),
    });
    const members = membersQ.data ?? [];
    if (membersQ.isLoading) {
        return (<div className="flex justify-center py-12">
        <Loader2 size={20} className="animate-spin text-[var(--color-fg-muted)]"/>
      </div>);
    }
    return (<div className="flex flex-col gap-3 max-w-2xl">
      <p className="text-xs text-[var(--color-fg-muted)]">
        {members.length} member{members.length !== 1 ? "s" : ""} in this organization
      </p>

      {members.map((m) => (<div key={m.user_id} className="flex items-center gap-3 px-4 py-3 rounded-xl border border-[var(--color-border)] bg-[var(--color-surface)]">
          <div className="size-8 rounded-full bg-[var(--color-accent-bg)] flex items-center justify-center text-xs font-bold text-[var(--color-brand)] shrink-0">
            {m.name?.[0]?.toUpperCase() ?? "?"}
          </div>
          <div className="flex-1 min-w-0">
            <p className="text-sm font-medium text-[var(--color-fg)] truncate">{m.name}</p>
            <p className="text-xs text-[var(--color-fg-muted)] truncate">{m.email}</p>
          </div>
          <select value={m.role} onChange={(e) => roleMut.mutate({ userId: m.user_id, role: e.target.value })} className="rounded-lg border border-[var(--color-border)] bg-[var(--color-bg)] px-2 py-1 text-xs text-[var(--color-fg)] outline-none focus:border-[var(--color-brand)] transition-colors">
            <option value="admin">Admin</option>
            <option value="executor">Executor</option>
            <option value="employee">Employee</option>
          </select>
          <button onClick={() => removeMut.mutate(m.user_id)} className="shrink-0 text-[var(--color-fg-muted)] hover:text-red-500 transition-colors">
            <X size={14}/>
          </button>
        </div>))}
    </div>);
}
