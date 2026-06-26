import { useMutation, useQueries, useQuery, useQueryClient } from "@tanstack/react-query";
import { FolderPlus, Loader2, MessageSquarePlus, Rocket, Sparkles, Workflow as WorkflowIcon } from "lucide-react";
import { useMemo, useState } from "react";

import { api } from "../lib/api";
import type { Chat, Project } from "../lib/types";
import { OnboardingWizard } from "../components/OnboardingWizard";
import { useToasts } from "../components/Toasts";
import { useOrg } from "../contexts/OrgContext";

// ProjectsHomeView is the landing surface when no chat is open. It
// renders the project grid. Clicking a card opens the project in
// the left rail (expanded, chats listed); clicking a chat inside a
// card opens that chat directly.

interface Props {
  onOpenChat: (chat: Chat) => void;
  onOpenProject: (id: string) => void;
}

export function ProjectsHomeView({ onOpenChat, onOpenProject }: Props) {
  const toasts = useToasts();
  const qc = useQueryClient();
  const { activeOrg } = useOrg();
  const projectsQuery = useQuery({
    queryKey: ["projects", activeOrg?.id],
    queryFn: () => api.listProjects(activeOrg!.id).then((r) => r.projects),
    enabled: !!activeOrg,
  });
  const [showOnboarding, setShowOnboarding] = useState(false);

  const projects = projectsQuery.data ?? [];

  // Fan-out one "list chats" query per project so the stats strip +
  // recent activity list can aggregate across all of them without
  // adding a new API endpoint. The queries are cached by project id,
  // so this is cheap once the user has browsed a project once.
  const chatQueries = useQueries({
    queries: projects.map((p) => ({
      queryKey: ["chats", p.id],
      queryFn: () => api.listChats(p.id).then((r) => r.chats),
      enabled: projects.length > 0,
    })),
  });

  const { totalChats, readyCount, recent } = useMemo(() => {
    type WithProject = Chat & { project_name: string };
    const all: WithProject[] = [];
    let ready = 0;
    chatQueries.forEach((q, i) => {
      const project = projects[i];
      if (!project || !q.data) return;
      for (const ch of q.data) {
        all.push({ ...ch, project_name: project.name });
        if (ch.latest_workflow_version_id) ready += 1;
      }
    });
    all.sort((a, b) => (b.updated_at > a.updated_at ? 1 : -1));
    return {
      totalChats: all.length,
      readyCount: ready,
      recent: all.slice(0, 4),
    };
  }, [chatQueries, projects]);

  return (
    <div className="flex-1 overflow-y-auto nice-scroll bg-[var(--color-bg)]">
      <div className="max-w-5xl mx-auto px-8 py-10">
        <header className="flex items-end justify-between gap-6 mb-8">
          <div>
            <div
              className="text-[11px] uppercase tracking-[0.16em] text-[var(--color-fg-muted)]"
              style={{ fontWeight: 500 }}
            >
              Operator workspace
            </div>
            <h1
              className="text-display text-[var(--color-fg)] mt-1"
              style={{ letterSpacing: "-0.02em" }}
            >
              Your projects
            </h1>
            <p className="text-[14px] text-[var(--color-fg-muted)] mt-1 max-w-xl leading-relaxed">
              Each project is a client company. Open one to see its chats — every chat is
              a persistent thread for designing one workflow.
            </p>
          </div>
          <button
            onClick={() => setShowOnboarding(true)}
            className="inline-flex items-center gap-1.5 text-[13px] px-3 py-2 rounded-md bg-[var(--color-brand)] text-white hover:brightness-110 shadow-stripe-ambient"
            style={{ fontWeight: 500 }}
          >
            <FolderPlus size={13} />
            New project
          </button>
        </header>

        {!projectsQuery.isLoading && projects.length > 0 && (
          <>
            <section className="mb-6 grid grid-cols-3 gap-3">
              <StatCard
                label="Projects"
                value={projects.length}
                hint={projects.length === 1 ? "1 client company" : `${projects.length} client companies`}
                icon={<FolderPlus size={14} />}
              />
              <StatCard
                label="Chats"
                value={totalChats}
                hint={totalChats === 0 ? "No workflows drafted yet" : "Ongoing workflow threads"}
                icon={<Sparkles size={14} />}
              />
              <StatCard
                label="Ready workflows"
                value={readyCount}
                hint={
                  readyCount === 0
                    ? "Send a prompt to draft one"
                    : readyCount === totalChats
                      ? "All chats have a workflow"
                      : `${totalChats - readyCount} still drafting`
                }
                icon={<Rocket size={14} />}
                accent={readyCount > 0}
              />
            </section>

            {recent.length > 0 && (
              <section className="mb-8 rounded-lg border border-[var(--color-border)] bg-[var(--color-surface)] p-4">
                <div
                  className="text-[10px] uppercase tracking-[0.14em] text-[var(--color-fg-subtle)] mb-2 flex items-center gap-1.5"
                  style={{ fontWeight: 500 }}
                >
                  <WorkflowIcon size={10} />
                  recently active
                </div>
                <ul className="divide-y divide-[var(--color-border)]">
                  {recent.map((ch) => (
                    <li key={ch.id}>
                      <button
                        onClick={() => onOpenChat(ch)}
                        className="w-full text-left flex items-center gap-2 py-1.5 hover:bg-[var(--color-surface-2)] rounded px-2 -mx-2 transition-colors"
                      >
                        <span
                          className={`size-[7px] rounded-full shrink-0 ${
                            ch.latest_workflow_version_id ? "bg-emerald-500" : "bg-[var(--color-border-strong)]"
                          }`}
                        />
                        <span className="text-[13px] text-[var(--color-fg)] truncate" style={{ fontWeight: 400 }}>
                          {ch.title}
                        </span>
                        <span className="text-[11px] text-[var(--color-fg-subtle)] font-mono shrink-0">
                          {(ch as Chat & { project_name: string }).project_name}
                        </span>
                        <span className="flex-1" />
                        <span className="text-[10.5px] text-[var(--color-fg-subtle)] font-mono shrink-0">
                          {relativeTime(ch.updated_at)}
                        </span>
                      </button>
                    </li>
                  ))}
                </ul>
              </section>
            )}
          </>
        )}

        {projectsQuery.isLoading && (
          <div className="text-[var(--color-fg-subtle)] flex items-center gap-2 text-[13px]">
            <Loader2 size={13} className="animate-spin" /> loading projects…
          </div>
        )}

        {!projectsQuery.isLoading && projects.length === 0 && !showOnboarding && (
          <div className="rounded-lg border border-dashed border-[var(--color-border-purple)] bg-[var(--color-accent-bg)] p-10 text-center">
            <div className="text-[15px] text-[var(--color-fg)]" style={{ fontWeight: 400 }}>
              No projects yet
            </div>
            <div className="text-[13px] text-[var(--color-fg-muted)] mt-2 max-w-md mx-auto leading-relaxed">
              Start by creating a project for your first client company. You'll add
              workflow-design chats inside it.
            </div>
            <button
              onClick={() => setShowOnboarding(true)}
              className="mt-4 inline-flex items-center gap-1.5 text-[13px] px-3 py-2 rounded-md bg-[var(--color-brand)] text-white hover:brightness-110"
              style={{ fontWeight: 500 }}
            >
              <FolderPlus size={13} />
              New project
            </button>
          </div>
        )}

        <div className="grid sm:grid-cols-2 lg:grid-cols-3 gap-4">
          {projects.map((p) => (
            <ProjectCard key={p.id} project={p} onOpenChat={onOpenChat} onOpenProject={onOpenProject} />
          ))}
        </div>

        {showOnboarding && (
          <OnboardingWizard
            onComplete={(overview, projectId) => {
              qc.invalidateQueries({ queryKey: ["projects", activeOrg?.id] });
              setShowOnboarding(false);
              if (projectId) {
                onOpenProject(projectId);
              }
            }}
            onSkip={() => {
              setShowOnboarding(false);
            }}
          />
        )}
      </div>
    </div>
  );
}

function StatCard({
  label,
  value,
  hint,
  icon,
  accent,
}: {
  label: string;
  value: number;
  hint: string;
  icon: React.ReactNode;
  accent?: boolean;
}) {
  return (
    <div
      className={`rounded-lg border px-4 py-3 ${
        accent
          ? "border-[var(--color-border-purple)] bg-[var(--color-accent-bg)]"
          : "border-[var(--color-border)] bg-[var(--color-surface)]"
      }`}
    >
      <div className="flex items-center gap-1.5 text-[10px] uppercase tracking-[0.14em] text-[var(--color-fg-subtle)]" style={{ fontWeight: 500 }}>
        <span className={accent ? "text-[var(--color-brand)]" : "text-[var(--color-fg-subtle)]"}>{icon}</span>
        {label}
      </div>
      <div
        className="text-[26px] text-[var(--color-fg)] tnum mt-0.5"
        style={{ fontWeight: 300, letterSpacing: "-0.02em" }}
      >
        {value}
      </div>
      <div className="text-[11px] text-[var(--color-fg-muted)] mt-0.5 leading-snug">{hint}</div>
    </div>
  );
}

// relativeTime formats an ISO timestamp into a short label ("3m",
// "2h", "5d"). We don't need a heavyweight date lib for this — the
// stamps are always recent-ish (the project is new) and anything
// older than a week can fall back to the date.
function relativeTime(iso: string): string {
  const then = new Date(iso).getTime();
  if (!Number.isFinite(then)) return "";
  const diff = Date.now() - then;
  const sec = Math.max(0, Math.floor(diff / 1000));
  if (sec < 60) return "just now";
  const min = Math.floor(sec / 60);
  if (min < 60) return `${min}m ago`;
  const hr = Math.floor(min / 60);
  if (hr < 24) return `${hr}h ago`;
  const day = Math.floor(hr / 24);
  if (day < 7) return `${day}d ago`;
  return new Date(iso).toLocaleDateString();
}

function ProjectCard({
  project,
  onOpenChat,
  onOpenProject,
}: {
  project: Project;
  onOpenChat: (c: Chat) => void;
  onOpenProject: (id: string) => void;
}) {
  const qc = useQueryClient();
  const toasts = useToasts();
  const chatsQuery = useQuery({
    queryKey: ["chats", project.id],
    queryFn: () => api.listChats(project.id).then((r) => r.chats),
  });
  const createChat = useMutation({
    mutationFn: () => api.createChat(project.id, { title: "New chat" }),
    onSuccess: (c) => {
      qc.invalidateQueries({ queryKey: ["chats", project.id] });
      onOpenChat(c);
    },
    onError: (e) => {
      toasts.push({
        kind: "error",
        title: "Couldn't create chat",
        body: e instanceof Error ? e.message : String(e),
      });
    },
  });

  const chats = chatsQuery.data ?? [];
  const preview = chats.slice(0, 3);
  const more = Math.max(0, chats.length - preview.length);

  return (
    <article
      className="group rounded-xl border border-[var(--color-border)] bg-[var(--color-surface)] p-4 hover:shadow-stripe-ambient transition-shadow"
    >
      <header className="flex items-start justify-between gap-2">
        <button
          onClick={() => onOpenProject(project.id)}
          className="text-left min-w-0 flex-1"
        >
          <div className="text-[15px] text-[var(--color-fg)] truncate" style={{ fontWeight: 400 }}>
            {project.name}
          </div>
          <div className="text-[11px] text-[var(--color-fg-subtle)] mt-0.5 tnum font-mono">
            {chats.length} {chats.length === 1 ? "chat" : "chats"}
          </div>
        </button>
        <button
          onClick={() => createChat.mutate()}
          disabled={createChat.isPending}
          title="New chat in this project"
          className="size-7 rounded-md flex items-center justify-center text-[var(--color-fg-muted)] hover:text-[var(--color-brand)] hover:bg-[var(--color-accent-bg)] opacity-0 group-hover:opacity-100 transition-all disabled:opacity-40"
        >
          {createChat.isPending ? (
            <Loader2 size={12} className="animate-spin" />
          ) : (
            <MessageSquarePlus size={12} />
          )}
        </button>
      </header>
      {project.description && (
        <p className="text-[12px] text-[var(--color-fg-muted)] mt-2 line-clamp-2 leading-snug">
          {project.description}
        </p>
      )}
      <ul className="mt-3 space-y-0.5">
        {preview.map((ch) => (
          <li key={ch.id}>
            <button
              onClick={() => onOpenChat(ch)}
              className="w-full text-left flex items-center gap-1.5 text-[12px] text-[var(--color-fg-muted)] hover:text-[var(--color-fg)] hover:bg-[var(--color-surface-2)] rounded px-2 py-1 transition-colors"
            >
              <span
                className={`size-[6px] rounded-full shrink-0 ${
                  ch.latest_workflow_version_id
                    ? "bg-emerald-500"
                    : "bg-[var(--color-border-strong)]"
                }`}
              />
              <span className="truncate">{ch.title}</span>
            </button>
          </li>
        ))}
        {more > 0 && (
          <li className="text-[11px] text-[var(--color-fg-subtle)] italic px-2">
            +{more} more
          </li>
        )}
        {chats.length === 0 && !chatsQuery.isLoading && (
          <li>
            <button
              onClick={() => createChat.mutate()}
              className="w-full text-left text-[12px] text-[var(--color-brand)] hover:underline px-2 py-1"
            >
              Start your first chat →
            </button>
          </li>
        )}
      </ul>
    </article>
  );
}
