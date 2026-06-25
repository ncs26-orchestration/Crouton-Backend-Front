import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { AnimatePresence, motion } from "framer-motion";
import {
  ArrowLeft,
  Building2,
  FolderPlus,
  Loader2,
  MessageSquarePlus,
  MoreHorizontal,
  Pencil,
  Plus,
  Search,
  Trash2,
} from "lucide-react";

import { api } from "../lib/api";
import type { Chat, Project } from "../lib/types";
import { useToasts } from "./Toasts";
import { DeleteChatModal, DeleteProjectModal, EditProjectModal } from "./ProjectModals";

// ProjectTree renders two modes depending on whether a project is
// scoped:
//
//   • Unscoped (selectedProjectId == null)
//     — flat list of every project, click to open.
//
//   • Scoped (selectedProjectId != null)
//     — header with the project name + back arrow, then ONLY that
//       project's chats. No sibling projects visible so the operator
//       stays focused on one client at a time. ChatGPT-style.
//
// Animation transitions keep the layout calm when switching modes.

interface Props {
  selectedProjectId: string | null;
  selectedChatId: string | null;
  onOpenProject: (projectId: string) => void;
  onOpenChat: (chat: Chat) => void;
  onBackToProjects: () => void;
  onOrganisationSettings?: () => void;
}

export function ProjectTree({
  selectedProjectId,
  selectedChatId,
  onOpenProject,
  onOpenChat,
  onBackToProjects,
  onOrganisationSettings,
}: Props) {
  if (selectedProjectId) {
    return (
      <ScopedToProject
        projectId={selectedProjectId}
        selectedChatId={selectedChatId}
        onOpenChat={onOpenChat}
        onBackToProjects={onBackToProjects}
        onOrganisationSettings={onOrganisationSettings}
      />
    );
  }
  return (
    <AllProjects onOpenProject={onOpenProject} />
  );
}

// --- unscoped: all projects ---

function AllProjects({ onOpenProject }: { onOpenProject: (id: string) => void }) {
  const toasts = useToasts();
  const qc = useQueryClient();
  const projectsQuery = useQuery({
    queryKey: ["projects"],
    queryFn: () => api.listProjects().then((r) => r.projects),
  });
  const [showForm, setShowForm] = useState(false);
  const [newName, setNewName] = useState("");
  const [filter, setFilter] = useState("");

  const createProject = useMutation({
    mutationFn: (name: string) => api.createProject({ name }),
    onSuccess: (p) => {
      qc.invalidateQueries({ queryKey: ["projects"] });
      setShowForm(false);
      setNewName("");
      onOpenProject(p.id);
    },
    onError: (e) => {
      toasts.push({
        kind: "error",
        title: "Couldn't create project",
        body: e instanceof Error ? e.message : String(e),
      });
    },
  });

  const projects = (projectsQuery.data ?? []).filter(
    (p) => !filter.trim() || p.name.toLowerCase().includes(filter.toLowerCase()),
  );

  return (
    <aside className="w-[260px] shrink-0 bg-[var(--color-surface)] border-r border-[var(--color-border)] flex flex-col h-full">
      <header className="px-4 pt-4 pb-2 flex items-start justify-between gap-2">
        <div>
          <div
            className="text-[10px] uppercase tracking-[0.14em] text-[var(--color-fg-muted)]"
            style={{ fontWeight: 500 }}
          >
            Projects
          </div>
          <div
            className="text-[16px] text-[var(--color-fg)]"
            style={{ fontWeight: 300, letterSpacing: "-0.01em" }}
          >
            Workspace
          </div>
        </div>
        <motion.button
          whileHover={{ scale: 1.05 }}
          whileTap={{ scale: 0.92 }}
          onClick={() => setShowForm((v) => !v)}
          className="size-7 rounded-md flex items-center justify-center text-[var(--color-fg-muted)] hover:text-[var(--color-fg)] hover:bg-[var(--color-surface-2)] transition-colors"
          title="New project"
        >
          <FolderPlus size={13} />
        </motion.button>
      </header>

      <div className="px-3 pb-2 relative">
        <Search
          size={11}
          className="absolute left-[18px] top-1/2 -translate-y-1/2 text-[var(--color-fg-subtle)]"
        />
        <input
          value={filter}
          onChange={(e) => setFilter(e.target.value)}
          placeholder="Search projects…"
          className="w-full bg-[var(--color-surface-2)] border border-[var(--color-border)] rounded-md pl-7 pr-2 py-1 text-[12px] text-[var(--color-fg)] placeholder:text-[var(--color-fg-subtle)] focus:outline-none focus:border-[var(--color-brand)]"
        />
      </div>

      {showForm && (
        <motion.form
          initial={{ opacity: 0, y: -4 }}
          animate={{ opacity: 1, y: 0 }}
          onSubmit={(e) => {
            e.preventDefault();
            const n = newName.trim();
            if (n) createProject.mutate(n);
          }}
          className="mx-3 mb-2 border border-[var(--color-border)] rounded-md bg-[var(--color-surface-2)] p-2"
        >
          <input
            autoFocus
            value={newName}
            onChange={(e) => setNewName(e.target.value)}
            placeholder="Client company name"
            className="w-full bg-transparent text-[13px] text-[var(--color-fg)] placeholder:text-[var(--color-fg-subtle)] focus:outline-none"
            style={{ fontWeight: 400 }}
          />
          <div className="flex items-center justify-end gap-1 mt-1">
            <button
              type="button"
              onClick={() => {
                setShowForm(false);
                setNewName("");
              }}
              className="text-[11px] text-[var(--color-fg-muted)] hover:text-[var(--color-fg)] px-2 py-0.5"
            >
              cancel
            </button>
            <button
              type="submit"
              disabled={!newName.trim() || createProject.isPending}
              className="text-[11px] px-2 py-0.5 rounded bg-[var(--color-brand)] text-white hover:brightness-110 disabled:opacity-40"
            >
              {createProject.isPending ? "…" : "create"}
            </button>
          </div>
        </motion.form>
      )}

      <div className="flex-1 overflow-y-auto nice-scroll py-1">
        {projectsQuery.isLoading && (
          <div className="px-4 py-2 text-[11px] text-[var(--color-fg-subtle)]">loading…</div>
        )}
        {projects.length === 0 && !projectsQuery.isLoading && !showForm && (
          <EmptyProjectsHint onNew={() => setShowForm(true)} />
        )}
        {projects.map((p) => (
          <ProjectRowSummary key={p.id} project={p} onOpen={() => onOpenProject(p.id)} />
        ))}
      </div>
    </aside>
  );
}

function ProjectRowSummary({ project, onOpen }: { project: { id: string; name: string; description: string }; onOpen: () => void }) {
  const chatsQuery = useQuery({
    queryKey: ["chats", project.id],
    queryFn: () => api.listChats(project.id).then((r) => r.chats),
  });
  const chatCount = chatsQuery.data?.length ?? 0;

  return (
    <motion.button
      whileHover={{ x: 2 }}
      transition={{ type: "spring", stiffness: 500, damping: 30 }}
      onClick={onOpen}
      className="w-full text-left mx-0 px-4 py-2 hover:bg-[var(--color-surface-2)] transition-colors"
    >
      <div className="text-[13px] text-[var(--color-fg)] truncate" style={{ fontWeight: 400 }}>
        {project.name}
      </div>
      <div className="text-[10.5px] text-[var(--color-fg-subtle)] font-mono tnum mt-0.5">
        {chatCount} {chatCount === 1 ? "chat" : "chats"}
      </div>
    </motion.button>
  );
}

function EmptyProjectsHint({ onNew }: { onNew: () => void }) {
  return (
    <div className="mx-3 my-3 rounded-lg border border-dashed border-[var(--color-border-purple)] bg-[var(--color-accent-bg)] px-3 py-4 text-center">
      <div className="text-[12px] text-[var(--color-fg)]" style={{ fontWeight: 400 }}>
        No projects yet
      </div>
      <div className="text-[11px] text-[var(--color-fg-muted)] mt-1 leading-snug">
        Start with your first client company.
      </div>
      <button
        onClick={onNew}
        className="mt-2 inline-flex items-center gap-1 text-[12px] px-2.5 py-1 rounded-md bg-[var(--color-brand)] text-white hover:brightness-110"
        style={{ fontWeight: 500 }}
      >
        <Plus size={11} />
        New project
      </button>
    </div>
  );
}

// --- scoped: one project ---

function ScopedToProject({
  projectId,
  selectedChatId,
  onOpenChat,
  onBackToProjects,
  onOrganisationSettings,
}: {
  projectId: string;
  selectedChatId: string | null;
  onOpenChat: (c: Chat) => void;
  onBackToProjects: () => void;
  onOrganisationSettings?: () => void;
}) {
  const toasts = useToasts();
  const qc = useQueryClient();
  const projectQuery = useQuery({
    queryKey: ["project", projectId],
    queryFn: () => api.getProject(projectId),
  });
  const project = projectQuery.data?.project;
  const chats = projectQuery.data?.chats ?? [];

  const [showNewChat, setShowNewChat] = useState(false);
  const [newTitle, setNewTitle] = useState("");
  const [projectMenuOpen, setProjectMenuOpen] = useState(false);
  const [editModalOpen, setEditModalOpen] = useState(false);
  const [deleteModalOpen, setDeleteModalOpen] = useState(false);
  const [deleteChatTarget, setDeleteChatTarget] = useState<Chat | null>(null);
  const [editingChatId, setEditingChatId] = useState<string | null>(null);
  const [editingChatTitle, setEditingChatTitle] = useState("");

  const updateProject = useMutation({
    mutationFn: (payload: { name: string; description: string }) =>
      api.updateProject(projectId, payload),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["project", projectId] });
      qc.invalidateQueries({ queryKey: ["projects"] });
      setEditModalOpen(false);
      toasts.push({ kind: "success", title: "Project updated" });
    },
    onError: (e) => {
      toasts.push({ kind: "error", title: "Update failed", body: e instanceof Error ? e.message : String(e) });
    },
  });

  const archiveProject = useMutation({
    mutationFn: () => api.archiveProject(projectId),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["projects"] });
      setDeleteModalOpen(false);
      onBackToProjects();
      toasts.push({ kind: "success", title: "Project archived" });
    },
    onError: (e) => {
      toasts.push({ kind: "error", title: "Archive failed", body: e instanceof Error ? e.message : String(e) });
    },
  });

  const deleteChat = useMutation({
    mutationFn: (chatId: string) => api.deleteChat(chatId),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["project", projectId] });
      qc.invalidateQueries({ queryKey: ["chats", projectId] });
      setDeleteChatTarget(null);
      toasts.push({ kind: "success", title: "Chat deleted" });
    },
    onError: (e) => {
      toasts.push({ kind: "error", title: "Delete failed", body: e instanceof Error ? e.message : String(e) });
    },
  });

  const renameChat = useMutation({
    mutationFn: ({ id, title }: { id: string; title: string }) =>
      api.renameChat(id, title),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["project", projectId] });
      setEditingChatId(null);
      setEditingChatTitle("");
      toasts.push({ kind: "success", title: "Chat renamed" });
    },
    onError: (e) => {
      toasts.push({ kind: "error", title: "Rename failed", body: e instanceof Error ? e.message : String(e) });
    },
  });

  const createChat = useMutation({
    mutationFn: (title: string) => api.createChat(projectId, { title }),
    onSuccess: (c) => {
      qc.invalidateQueries({ queryKey: ["project", projectId] });
      qc.invalidateQueries({ queryKey: ["chats", projectId] });
      setShowNewChat(false);
      setNewTitle("");
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

  return (
    <motion.aside
      initial={{ x: -8, opacity: 0 }}
      animate={{ x: 0, opacity: 1 }}
      transition={{ type: "spring", stiffness: 320, damping: 28 }}
      className="w-[260px] shrink-0 bg-[var(--color-surface)] border-r border-[var(--color-border)] flex flex-col h-full"
    >
      <header className="px-4 pt-4 pb-3 border-b border-[var(--color-border)]">
        <div className="flex items-center justify-between">
          <button
            onClick={onBackToProjects}
            className="flex items-center gap-1 text-[10.5px] text-[var(--color-fg-muted)] hover:text-[var(--color-fg)] transition-colors"
            style={{ fontWeight: 500 }}
          >
            <ArrowLeft size={10} />
            All projects
          </button>
          <div className="flex items-center gap-1">
            {onOrganisationSettings && (
              <motion.button
                whileHover={{ scale: 1.05 }}
                whileTap={{ scale: 0.92 }}
                onClick={onOrganisationSettings}
                className="size-6 rounded-md flex items-center justify-center text-[var(--color-fg-muted)] hover:text-[var(--color-brand)] hover:bg-[var(--color-accent-bg)] transition-colors"
                title="Organisation settings"
              >
                <Building2 size={12} />
              </motion.button>
            )}
            <div className="relative">
              <motion.button
                whileHover={{ scale: 1.05 }}
                whileTap={{ scale: 0.92 }}
                onClick={() => setProjectMenuOpen((v) => !v)}
                className="size-6 rounded-md flex items-center justify-center text-[var(--color-fg-muted)] hover:text-[var(--color-fg)] hover:bg-[var(--color-surface-2)] transition-colors"
                title="Project options"
              >
                <MoreHorizontal size={13} />
              </motion.button>
              <AnimatePresence>
                {projectMenuOpen && (
                  <motion.div
                    initial={{ opacity: 0, y: -4 }}
                    animate={{ opacity: 1, y: 0 }}
                    exit={{ opacity: 0, y: -2 }}
                    className="absolute right-0 top-full mt-1 w-[160px] bg-[var(--color-surface)] border border-[var(--color-border)] rounded-md shadow-stripe-deep z-20 overflow-hidden"
                    onMouseLeave={() => setProjectMenuOpen(false)}
                  >
                    <button
                      onClick={() => {
                        setProjectMenuOpen(false);
                        setEditModalOpen(true);
                      }}
                      className="w-full text-left px-3 py-2 flex items-center gap-2 text-[12px] text-[var(--color-fg)] hover:bg-[var(--color-surface-2)] transition-colors"
                    >
                      <Pencil size={12} />
                      Edit project
                    </button>
                    <button
                      onClick={() => {
                        setProjectMenuOpen(false);
                        setDeleteModalOpen(true);
                      }}
                      className="w-full text-left px-3 py-2 flex items-center gap-2 text-[12px] text-rose-500 hover:bg-rose-500/5 transition-colors"
                    >
                      <Trash2 size={12} />
                      Delete project
                    </button>
                  </motion.div>
                )}
              </AnimatePresence>
            </div>
          </div>
        </div>
        <div
          className="mt-1.5 text-[10px] uppercase tracking-[0.14em] text-[var(--color-fg-muted)]"
          style={{ fontWeight: 500 }}
        >
          Project
        </div>
        <div
          className="text-[16px] text-[var(--color-fg)] truncate"
          style={{ fontWeight: 300, letterSpacing: "-0.01em" }}
          title={project?.description || project?.name}
        >
          {project?.name ?? "…"}
        </div>
        {project?.description && (
          <div className="text-[11px] text-[var(--color-fg-muted)] mt-0.5 line-clamp-2 leading-snug">
            {project.description}
          </div>
        )}
      </header>

      <div className="px-3 pt-2 flex items-center justify-between">
        <div
          className="text-[10px] uppercase tracking-[0.14em] text-[var(--color-fg-subtle)]"
          style={{ fontWeight: 500 }}
        >
          Chats
        </div>
        <motion.button
          whileHover={{ scale: 1.05 }}
          whileTap={{ scale: 0.92 }}
          onClick={() => setShowNewChat((v) => !v)}
          className="size-6 rounded-md flex items-center justify-center text-[var(--color-fg-muted)] hover:text-[var(--color-brand)] hover:bg-[var(--color-accent-bg)] transition-colors"
          title="New chat"
        >
          <MessageSquarePlus size={11} />
        </motion.button>
      </div>

      {showNewChat && (
        <motion.form
          initial={{ opacity: 0, y: -4 }}
          animate={{ opacity: 1, y: 0 }}
          onSubmit={(e) => {
            e.preventDefault();
            const t = newTitle.trim() || "New chat";
            createChat.mutate(t);
          }}
          className="mx-3 my-1 border border-[var(--color-border)] rounded-md bg-[var(--color-surface-2)] p-2"
        >
          <input
            autoFocus
            value={newTitle}
            onChange={(e) => setNewTitle(e.target.value)}
            onBlur={() => {
              if (!newTitle.trim()) setShowNewChat(false);
            }}
            placeholder="Chat title — e.g. Expense approval"
            className="w-full bg-transparent text-[12px] text-[var(--color-fg)] placeholder:text-[var(--color-fg-subtle)] focus:outline-none"
          />
        </motion.form>
      )}

      <div className="flex-1 overflow-y-auto nice-scroll py-1 px-1.5">
        {projectQuery.isLoading && (
          <div className="flex items-center gap-1.5 text-[11px] text-[var(--color-fg-subtle)] px-3 py-2">
            <Loader2 size={11} className="animate-spin" /> loading chats…
          </div>
        )}
        {!projectQuery.isLoading && chats.length === 0 && !showNewChat && (
          <button
            onClick={() => setShowNewChat(true)}
            className="w-full text-left text-[12px] text-[var(--color-brand)] hover:underline px-3 py-2"
          >
            + Start your first chat
          </button>
        )}
        {chats.map((ch) => (
          <div
            key={ch.id}
            className={`group relative flex items-center gap-2 px-2.5 py-1.5 rounded cursor-pointer transition-colors ${
              ch.id === selectedChatId
                ? "bg-[var(--color-accent-bg)]"
                : "hover:bg-[var(--color-surface-2)]"
            }`}
            onClick={() => editingChatId !== ch.id && onOpenChat(ch)}
          >
            <span
              className={`size-[6px] rounded-full shrink-0 ${
                ch.latest_workflow_version_id
                  ? "bg-emerald-500"
                  : "bg-[var(--color-border-strong)]"
              }`}
            />
            {editingChatId === ch.id ? (
              <input
                autoFocus
                value={editingChatTitle}
                onChange={(e) => setEditingChatTitle(e.target.value)}
                onBlur={() => {
                  if (editingChatTitle.trim() && editingChatTitle !== ch.title) {
                    renameChat.mutate({ id: ch.id, title: editingChatTitle.trim() });
                  } else {
                    setEditingChatId(null);
                  }
                }}
                onKeyDown={(e) => {
                  if (e.key === "Enter") {
                    if (editingChatTitle.trim() && editingChatTitle !== ch.title) {
                      renameChat.mutate({ id: ch.id, title: editingChatTitle.trim() });
                    } else {
                      setEditingChatId(null);
                    }
                  }
                  if (e.key === "Escape") {
                    setEditingChatId(null);
                  }
                }}
                className="flex-1 bg-transparent text-[12.5px] text-[var(--color-fg)] focus:outline-none border-b border-[var(--color-brand)]"
              />
            ) : (
              <span
                className={`flex-1 truncate text-[12.5px] ${
                  ch.id === selectedChatId ? "text-[var(--color-brand)]" : "text-[var(--color-fg-muted)]"
                }`}
                style={{ fontWeight: ch.id === selectedChatId ? 500 : 400 }}
              >
                {ch.title}
              </span>
            )}
            <div className="flex items-center gap-0.5 opacity-0 group-hover:opacity-100 transition-opacity">
              <button
                onClick={(e) => {
                  e.stopPropagation();
                  setEditingChatId(ch.id);
                  setEditingChatTitle(ch.title);
                }}
                className="size-5 rounded flex items-center justify-center text-[var(--color-fg-subtle)] hover:text-[var(--color-fg)] hover:bg-[var(--color-surface)]"
                title="Rename chat"
              >
                <Pencil size={10} />
              </button>
              <button
                onClick={(e) => {
                  e.stopPropagation();
                  setDeleteChatTarget(ch);
                }}
                className="size-5 rounded flex items-center justify-center text-[var(--color-fg-subtle)] hover:text-rose-500 hover:bg-rose-500/10"
                title="Delete chat"
              >
                <Trash2 size={10} />
              </button>
            </div>
          </div>
        ))}
      </div>

      <EditProjectModal
        isOpen={editModalOpen}
        project={{ id: projectId, name: project?.name ?? "", description: project?.description ?? "" }}
        onClose={() => setEditModalOpen(false)}
        onConfirm={(name, description) => updateProject.mutate({ name, description })}
        isPending={updateProject.isPending}
      />

      <DeleteProjectModal
        isOpen={deleteModalOpen}
        project={{ id: projectId, name: project?.name ?? "" }}
        chatCount={chats.length}
        onClose={() => setDeleteModalOpen(false)}
        onConfirm={() => archiveProject.mutate()}
        isPending={archiveProject.isPending}
      />

      <DeleteChatModal
        isOpen={!!deleteChatTarget}
        chat={deleteChatTarget ?? { id: "", title: "" }}
        onClose={() => setDeleteChatTarget(null)}
        onConfirm={() => deleteChatTarget && deleteChat.mutate(deleteChatTarget.id)}
        isPending={deleteChat.isPending}
      />
    </motion.aside>
  );
}
