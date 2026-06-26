import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { motion } from "framer-motion";
import { Building2, Cog, Loader2, Moon, Plug, Plus, Sun, Trash2 } from "lucide-react";
import { api } from "../lib/api";
import { useTheme } from "../lib/theme";
import { OrgSettingsModal } from "../components/OrgSettingsModal";
import { useToasts } from "../components/Toasts";
import { useOrg } from "../contexts/OrgContext";
// SettingsView is the operator's control panel. Top section is
// Projects + their Deploy targets (Camunda, Elsa); bottom is global
// preferences (theme). Per-project sub-sections make it obvious
// which connectors belong to which client.
const LOCAL_DEPLOY_TARGETS = [
    {
        kind: "camunda7",
        name: "Local Camunda 7",
        endpoint: "http://camunda7:8080/engine-rest",
        auth_kind: "none",
        auth_user: "",
        auth_secret: "",
    },
    {
        kind: "elsa3",
        name: "Local Elsa 3",
        endpoint: "http://elsa3:8080",
        auth_kind: "credentials",
        auth_user: "admin",
        auth_secret: "password",
    },
];
export function SettingsView({ scopedProjectId, }) {
    const { theme, toggle } = useTheme();
    const { activeOrg } = useOrg();
    const projectsQuery = useQuery({
        queryKey: ["projects", activeOrg?.id],
        queryFn: () => api.listProjects(activeOrg.id).then((r) => r.projects),
        enabled: !!activeOrg,
    });
    const projects = projectsQuery.data ?? [];
    const [orgModalProject, setOrgModalProject] = useState(null);
    // If a project is scoped (the rail has one open), surface that
    // project first so deploy-target config feels targeted.
    const sortedProjects = [...projects].sort((a, b) => {
        if (scopedProjectId && a.id === scopedProjectId)
            return -1;
        if (scopedProjectId && b.id === scopedProjectId)
            return 1;
        return 0;
    });
    return (<div className="flex-1 overflow-y-auto nice-scroll bg-[var(--color-bg)]">
      <div className="max-w-3xl mx-auto px-8 py-10">
        <header className="mb-8">
          <div className="text-[11px] uppercase tracking-[0.16em] text-[var(--color-fg-muted)]" style={{ fontWeight: 500 }}>
            Operator workspace
          </div>
          <h1 className="text-display text-[var(--color-fg)] mt-1" style={{ letterSpacing: "-0.02em" }}>
            Settings
          </h1>
          <p className="text-[13px] text-[var(--color-fg-muted)] mt-1 max-w-xl">
            Manage deploy-target connectors per project, plus your global preferences.
          </p>
        </header>

        <section className="mb-8">
          <h2 className="text-[13px] text-[var(--color-fg)] flex items-center gap-2" style={{ fontWeight: 500 }}>
            <Plug size={14} className="text-[var(--color-brand)]"/>
            Deploy targets
          </h2>
          <p className="text-[12px] text-[var(--color-fg-muted)] mt-1 max-w-2xl leading-relaxed">
            Each project can deploy workflows to a Camunda 7 or Elsa 3 instance.
            Credentials are scoped per project so different client engagements stay isolated.
          </p>
          <div className="mt-4 space-y-4">
            {projectsQuery.isLoading && (<div className="text-[12px] text-[var(--color-fg-subtle)] flex items-center gap-1.5">
                <Loader2 size={12} className="animate-spin"/> loading projects…
              </div>)}
            {!projectsQuery.isLoading && projects.length === 0 && (<div className="rounded-lg border border-dashed border-[var(--color-border-purple)] bg-[var(--color-accent-bg)] p-6 text-center">
                <div className="text-[13px] text-[var(--color-fg)]" style={{ fontWeight: 400 }}>
                  No projects yet
                </div>
                <div className="text-[12px] text-[var(--color-fg-muted)] mt-1">
                  Create a project first — deploy targets are scoped per project.
                </div>
              </div>)}
            {sortedProjects.map((p) => (<ProjectTargetsCard key={p.id} project={p} initiallyOpen={p.id === scopedProjectId} onOpenOrgSettings={() => setOrgModalProject({ id: p.id, name: p.name })}/>))}
          </div>
        </section>

        {scopedProjectId && (<section className="mb-8">
            <h2 className="text-[13px] text-[var(--color-fg)] flex items-center gap-2" style={{ fontWeight: 500 }}>
              <Building2 size={14} className="text-[var(--color-brand)]"/>
              Organisation
            </h2>
            <p className="text-[12px] text-[var(--color-fg-muted)] mt-1 max-w-2xl leading-relaxed">
              View and edit the organisation overview collected during onboarding.
            </p>
            <div className="mt-4">
              <button onClick={() => {
                const proj = projects.find((p) => p.id === scopedProjectId);
                if (proj)
                    setOrgModalProject({ id: proj.id, name: proj.name });
            }} className="inline-flex items-center gap-2 text-[12px] px-4 py-2 rounded-md border border-[var(--color-border)] bg-[var(--color-surface)] text-[var(--color-fg)] hover:border-[var(--color-brand)]">
                <Building2 size={14}/>
                Edit Organisation Overview
              </button>
            </div>
          </section>)}

        <section>
          <h2 className="text-[13px] text-[var(--color-fg)] flex items-center gap-2" style={{ fontWeight: 500 }}>
            <Cog size={14} className="text-[var(--color-brand)]"/>
            Preferences
          </h2>
          <div className="mt-3 rounded-lg border border-[var(--color-border)] bg-[var(--color-surface)] p-4 flex items-center justify-between">
            <div>
              <div className="text-[13px] text-[var(--color-fg)]" style={{ fontWeight: 400 }}>
                Theme
              </div>
              <div className="text-[11.5px] text-[var(--color-fg-muted)] mt-0.5">
                Currently using {theme} mode.
              </div>
            </div>
            <button onClick={toggle} className="inline-flex items-center gap-1.5 text-[12px] px-3 py-1.5 rounded-md border border-[var(--color-border)] bg-[var(--color-surface-2)] text-[var(--color-fg)] hover:border-[var(--color-border-strong)]" style={{ fontWeight: 500 }}>
              {theme === "dark" ? <Sun size={12}/> : <Moon size={12}/>}
              Switch to {theme === "dark" ? "light" : "dark"}
            </button>
          </div>
        </section>

        {orgModalProject && (<OrgSettingsModalWrapper projectId={orgModalProject.id} projectName={orgModalProject.name} onClose={() => setOrgModalProject(null)}/>)}
      </div>
    </div>);
}
function ProjectTargetsCard({ project, initiallyOpen, onOpenOrgSettings, }) {
    const toasts = useToasts();
    const qc = useQueryClient();
    const targetsQuery = useQuery({
        queryKey: ["deploy-targets", project.id],
        queryFn: () => api.listDeployTargets(project.id).then((r) => r.deploy_targets),
    });
    const targets = targetsQuery.data ?? [];
    const [showForm, setShowForm] = useState(!!initiallyOpen && targets.length === 0);
    const [kind, setKind] = useState("camunda7");
    const [name, setName] = useState("");
    const [endpoint, setEndpoint] = useState("");
    const [authUser, setAuthUser] = useState("");
    const [authSecret, setAuthSecret] = useState("");
    const createTarget = useMutation({
        mutationFn: (override) => api.createDeployTarget(project.id, override ?? {
            kind,
            name: name.trim() || kind,
            endpoint: endpoint.trim(),
            auth_kind: authUser ? "basic" : "none",
            auth_user: authUser,
            auth_secret: authSecret,
        }),
        onSuccess: (created) => {
            qc.invalidateQueries({ queryKey: ["deploy-targets", project.id] });
            setShowForm(false);
            setName("");
            setEndpoint("");
            setAuthUser("");
            setAuthSecret("");
            toasts.push({
                kind: "success",
                title: "Deploy target added",
                body: `${created.kind} endpoint registered for ${project.name}.`,
            });
        },
        onError: (e) => {
            toasts.push({
                kind: "error",
                title: "Couldn't register target",
                body: e instanceof Error ? e.message : String(e),
            });
        },
    });
    const deleteTarget = useMutation({
        mutationFn: (id) => api.deleteDeployTarget(id),
        onSuccess: () => qc.invalidateQueries({ queryKey: ["deploy-targets", project.id] }),
    });
    return (<motion.article initial={{ opacity: 0, y: 4 }} animate={{ opacity: 1, y: 0 }} className="rounded-lg border border-[var(--color-border)] bg-[var(--color-surface)] overflow-hidden">
      <header className="px-4 py-3 flex items-center justify-between border-b border-[var(--color-border)]">
        <div className="min-w-0">
          <div className="text-[13px] text-[var(--color-fg)] truncate" style={{ fontWeight: 500 }}>
            {project.name}
          </div>
          <div className="text-[10.5px] text-[var(--color-fg-subtle)] font-mono tnum mt-0.5">
            {targets.length} {targets.length === 1 ? "target" : "targets"}
          </div>
        </div>
        <div className="flex items-center gap-2">
          {onOpenOrgSettings && (<button onClick={onOpenOrgSettings} title="Organisation settings" className="text-[var(--color-fg-muted)] hover:text-[var(--color-brand)] p-1.5 rounded hover:bg-[var(--color-surface-2)]">
              <Building2 size={14}/>
            </button>)}
          <button onClick={() => setShowForm((v) => !v)} className="inline-flex items-center gap-1 text-[11.5px] px-2.5 py-1 rounded-md border border-[var(--color-border)] bg-[var(--color-surface-2)] hover:border-[var(--color-border-strong)] transition-colors" style={{ fontWeight: 500 }}>
            <Plus size={11}/> Add connector
          </button>
        </div>
      </header>

      {targets.map((t) => (<TargetRow key={t.id} target={t} onDelete={() => deleteTarget.mutate(t.id)}/>))}

      <div className="px-4 py-3 border-b border-[var(--color-border)] bg-[var(--color-surface)] flex flex-wrap items-center gap-2">
        <span className="text-[10px] uppercase tracking-[0.12em] text-[var(--color-fg-subtle)]" style={{ fontWeight: 500 }}>
          Local compose
        </span>
        {LOCAL_DEPLOY_TARGETS.map((preset) => {
            const exists = targets.some((t) => t.kind === preset.kind && t.endpoint === preset.endpoint);
            return (<button key={preset.kind} type="button" onClick={() => createTarget.mutate(preset)} disabled={exists || createTarget.isPending} className="inline-flex items-center gap-1 text-[11px] px-2 py-1 rounded-md border border-[var(--color-border)] bg-[var(--color-surface-2)] text-[var(--color-fg)] hover:border-[var(--color-brand)] disabled:opacity-45 disabled:cursor-not-allowed" title={exists ? "Already registered for this project" : `Register ${preset.name} from docker compose`}>
              <Plus size={10}/>
              {exists ? `${preset.name} added` : `Add ${preset.name}`}
            </button>);
        })}
      </div>

      {showForm && (<motion.form initial={{ height: 0, opacity: 0 }} animate={{ height: "auto", opacity: 1 }} onSubmit={(e) => {
                e.preventDefault();
                if (endpoint.trim())
                    createTarget.mutate(undefined);
            }} className="border-t border-[var(--color-border)] bg-[var(--color-surface-2)] p-3 space-y-2">
          <div className="grid grid-cols-2 gap-2">
            <Field label="Kind">
              <select value={kind} onChange={(e) => setKind(e.target.value)} className="bg-[var(--color-surface)] border border-[var(--color-border)] rounded-md px-2 py-1 text-[12px] text-[var(--color-fg)]">
                <option value="camunda7">Camunda 7</option>
                <option value="elsa3">Elsa 3</option>
              </select>
            </Field>
            <Field label="Name">
              <input value={name} onChange={(e) => setName(e.target.value)} placeholder={kind === "camunda7" ? "Dev Camunda" : "Staging Elsa"} className="bg-[var(--color-surface)] border border-[var(--color-border)] rounded-md px-2 py-1 text-[12px] text-[var(--color-fg)] focus:outline-none focus:border-[var(--color-brand)]"/>
            </Field>
          </div>
          <Field label="Endpoint">
            <input value={endpoint} onChange={(e) => setEndpoint(e.target.value)} placeholder={kind === "camunda7"
                ? "http://camunda7:8080/engine-rest"
                : "http://elsa3:8080"} className="w-full bg-[var(--color-surface)] border border-[var(--color-border)] rounded-md px-2 py-1 text-[12px] font-mono text-[var(--color-fg)] focus:outline-none focus:border-[var(--color-brand)]"/>
          </Field>
          <div className="grid grid-cols-2 gap-2">
            <Field label="User (optional)">
              <input value={authUser} onChange={(e) => setAuthUser(e.target.value)} placeholder="demo" className="bg-[var(--color-surface)] border border-[var(--color-border)] rounded-md px-2 py-1 text-[12px] text-[var(--color-fg)]"/>
            </Field>
            <Field label="Secret">
              <input type="password" value={authSecret} onChange={(e) => setAuthSecret(e.target.value)} placeholder="••••" className="bg-[var(--color-surface)] border border-[var(--color-border)] rounded-md px-2 py-1 text-[12px] text-[var(--color-fg)]"/>
            </Field>
          </div>
          <div className="flex items-center justify-end gap-2 pt-1">
            <button type="button" onClick={() => setShowForm(false)} className="text-[11px] text-[var(--color-fg-muted)] hover:text-[var(--color-fg)] px-2 py-1">
              cancel
            </button>
            <button type="submit" disabled={!endpoint.trim() || createTarget.isPending} className="inline-flex items-center gap-1 text-[11.5px] px-3 py-1.5 rounded-md bg-[var(--color-brand)] text-white hover:brightness-110 disabled:opacity-40">
              {createTarget.isPending && <Loader2 size={10} className="animate-spin"/>}
              Register connector
            </button>
          </div>
        </motion.form>)}

      {!showForm && targets.length === 0 && (<div className="px-4 py-4 text-[11.5px] text-[var(--color-fg-muted)] italic">
          No connectors yet for this project. Add Camunda or Elsa to enable Deploy from chats.
        </div>)}
    </motion.article>);
}
function TargetRow({ target, onDelete }) {
    const kindTone = target.kind === "camunda7"
        ? "bg-emerald-500/15 text-emerald-600 dark:text-emerald-400 border-emerald-500/30"
        : "bg-violet-500/15 text-violet-600 dark:text-violet-400 border-violet-500/30";
    return (<div className="group flex items-start gap-3 px-4 py-3 border-b border-[var(--color-border)] last:border-b-0 hover:bg-[var(--color-surface-2)] transition-colors">
      <span className={`inline-flex items-center gap-1 text-[10px] uppercase tracking-[0.12em] px-1.5 py-0.5 rounded border ${kindTone}`} style={{ fontWeight: 500 }}>
        {target.kind}
      </span>
      <div className="flex-1 min-w-0">
        <div className="text-[13px] text-[var(--color-fg)] truncate" style={{ fontWeight: 400 }}>
          {target.name}
        </div>
        <div className="text-[11px] font-mono text-[var(--color-fg-muted)] truncate mt-0.5">
          {target.endpoint}
        </div>
      </div>
      <button onClick={onDelete} title="Remove connector" className="opacity-0 group-hover:opacity-100 text-[var(--color-fg-subtle)] hover:text-rose-500 transition-opacity">
        <Trash2 size={12}/>
      </button>
    </div>);
}
function Field({ label, children }) {
    return (<label className="flex flex-col gap-1">
      <span className="text-[10px] uppercase tracking-[0.12em] text-[var(--color-fg-subtle)]" style={{ fontWeight: 500 }}>
        {label}
      </span>
      {children}
    </label>);
}
function OrgSettingsModalWrapper({ projectId, projectName, onClose, }) {
    const qc = useQueryClient();
    const projectQuery = useQuery({
        queryKey: ["project", projectId],
        queryFn: () => api.getProject(projectId),
    });
    const overview = projectQuery.data?.project?.overview_json;
    return (<OrgSettingsModal projectId={projectId} projectName={projectName} overview={overview ?? null} onClose={onClose} onSaved={() => {
            qc.invalidateQueries({ queryKey: ["project", projectId] });
        }}/>);
}
