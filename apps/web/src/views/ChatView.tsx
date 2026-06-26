import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { AnimatePresence, motion } from "framer-motion";
import {
  ArrowRight,
  Check,
  ChevronDown,
  ChevronLeft,
  ChevronRight,
  Download,
  FileText,
  GitBranch,
  HelpCircle,
  History,
  Image as ImageIcon,
  Loader2,
  MessageSquare,
  Mic,
  PanelLeftClose,
  PanelLeftOpen,
  Paperclip,
  Rocket,
  RotateCcw,
  Sparkles,
  Workflow as WorkflowIcon,
  X,
} from "lucide-react";

import { api } from "../lib/api";
import { IRCanvas } from "../components/IRCanvas";
import { collectLowConfidence } from "../lib/confidence";
import type { Attachment, Chat, ChatMessage, DeployTarget, EngineAdapter, Workflow, WorkflowVersionListItem } from "../lib/types";
import { useToasts } from "../components/Toasts";

// ChatView is the main work surface. Three regions:
//   - TopBar     (chat title, status chip, target selector, Compile, Deploy)
//   - Thread     (left column, bubble transcript with pinned composer)
//   - Canvas     (centre, renders the chat's latest IR)
//
// Round C — the composer now actually extracts. Round E's deploy
// targets are already read here so the Deploy button surfaces the
// project's configured engines.

interface Props {
  chatId: string;
}

// localStorage key for thread-panel collapsed state. Per-chat keys
// would be noisy; one shared preference matches operator
// expectations ("I like the thread on or off by default").
const THREAD_COLLAPSED_KEY = "aios.chatThreadCollapsed";

type Status = "empty" | "drafting" | "ready" | "approved" | "compiling" | "compiled" | "deploying" | "deployed" | "error";

export function ChatView({ chatId }: Props) {
  const toasts = useToasts();
  const qc = useQueryClient();

  // Thread panel collapse — default open, remembered in localStorage
  // so operators who prefer canvas-first stay that way between
  // sessions. Toggleable with an inline button or `⌘\`.
  const [threadCollapsed, setThreadCollapsed] = useState<boolean>(() => {
    if (typeof window === "undefined") return false;
    return window.localStorage.getItem(THREAD_COLLAPSED_KEY) === "1";
  });
  useEffect(() => {
    if (typeof window === "undefined") return;
    window.localStorage.setItem(THREAD_COLLAPSED_KEY, threadCollapsed ? "1" : "0");
  }, [threadCollapsed]);
  // ⌘\ toggles the thread (mirrors the VS Code sidebar shortcut).
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if ((e.metaKey || e.ctrlKey) && e.key === "\\") {
        e.preventDefault();
        setThreadCollapsed((v) => !v);
      }
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, []);

  const chatQuery = useQuery({
    queryKey: ["chat", chatId],
    queryFn: () => api.getChat(chatId),
  });
  const messagesQuery = useQuery({
    queryKey: ["chat-messages", chatId],
    queryFn: () => api.listMessages(chatId).then((r) => r.messages),
  });
  const chat: Chat | undefined = chatQuery.data?.chat;
  const workflow: Workflow | undefined = chatQuery.data?.workflow?.ir;
  const stage: "drafting" | "ready" | "approved" | undefined = chatQuery.data?.workflow?.stage;

  const deployTargetsQuery = useQuery({
    queryKey: ["deploy-targets", chat?.project_id],
    queryFn: () =>
      chat ? api.listDeployTargets(chat.project_id).then((r) => r.deploy_targets) : Promise.resolve([]),
    enabled: !!chat,
  });
  const deployTargets: DeployTarget[] = deployTargetsQuery.data ?? [];

  const adaptersQuery = useQuery({
    queryKey: ["engine-adapters"],
    queryFn: () => api.listAdapters().then((r) => r.adapters),
  });

  // Version history query (Phase 4)
  const versionsQuery = useQuery({
    queryKey: ["workflow-versions", chatId],
    queryFn: () => api.listWorkflowVersions(chatId).then((r) => r.versions),
    enabled: !!chatId,
  });
  const versions: WorkflowVersionListItem[] = versionsQuery.data ?? [];

  const [input, setInput] = useState("");
  const textareaRef = useRef<HTMLTextAreaElement | null>(null);
  const threadRef = useRef<HTMLDivElement | null>(null);
  const fileInputRef = useRef<HTMLInputElement | null>(null);

  // Staged attachments — files the user has dropped into the composer
  // but hasn't sent yet. Chips show each one with a remove button;
  // submitting the message binds these to the outgoing message on the
  // server. Order is preserved so the bubble's chips match drop order.
  const [pendingAttachments, setPendingAttachments] = useState<Attachment[]>([]);
  const [isDragging, setIsDragging] = useState(false);

  // Versioning UI state (Phase 4)
  const [versionMenuOpen, setVersionMenuOpen] = useState(false);
  const [forkModalOpen, setForkModalOpen] = useState(false);
  const [restoreModalOpen, setRestoreModalOpen] = useState(false);
  const [selectedVersionId, setSelectedVersionId] = useState<string | null>(null);
  const [diffData, setDiffData] = useState<{ added: string[]; removed: string[]; changed: string[] } | null>(null);

  const diffMut = useMutation({
    mutationFn: (otherVersionId: string) => {
      if (!chatQuery.data?.workflow?.id) throw new Error("no current version");
      return api.diffWorkflowVersions(chatQuery.data.workflow.id, otherVersionId);
    },
    onSuccess: (data) => {
      setDiffData(data);
    },
    onError: (e) => {
      toasts.push({
        kind: "error",
        title: "Diff failed",
        body: e instanceof Error ? e.message : String(e),
      });
    },
  });

  const forkMut = useMutation({
    mutationFn: async (payload: { version_id?: string; target_chat_id?: string; title?: string }) => {
      return api.forkWorkflow(chatId, payload);
    },
    onSuccess: (resp) => {
      toasts.push({
        kind: "success",
        title: "Workflow forked",
        body: `Created new chat "${resp.chat.title}"`,
        action: { label: "Open", href: `/chat/${resp.chat.id}` },
      });
      setForkModalOpen(false);
    },
    onError: (e) => {
      toasts.push({
        kind: "error",
        title: "Fork failed",
        body: e instanceof Error ? e.message : String(e),
      });
    },
  });

  const restoreMut = useMutation({
    mutationFn: async (versionId: string) => {
      return api.restoreWorkflowVersion(versionId);
    },
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["chat", chatId] });
      qc.invalidateQueries({ queryKey: ["workflow-versions", chatId] });
      setRestoreModalOpen(false);
      toasts.push({
        kind: "success",
        title: "Version restored",
        body: "Created new version from the restored snapshot",
      });
    },
    onError: (e) => {
      toasts.push({
        kind: "error",
        title: "Restore failed",
        body: e instanceof Error ? e.message : String(e),
      });
    },
  });

  const uploadMut = useMutation({
    mutationFn: async (file: File) => api.uploadAttachment(chatId, file),
    onSuccess: (att) => {
      setPendingAttachments((prev) => [...prev, att]);
    },
    onError: (e, file) => {
      toasts.push({
        kind: "error",
        title: "Upload failed",
        body: `${file.name}: ${e instanceof Error ? e.message : String(e)}`,
      });
    },
  });

  const handleFiles = useCallback(
    (files: FileList | File[]) => {
      for (const f of Array.from(files)) {
        uploadMut.mutate(f);
      }
    },
    [uploadMut],
  );

  useEffect(() => {
    const el = textareaRef.current;
    if (!el) return;
    el.style.height = "auto";
    el.style.height = Math.min(Math.max(el.scrollHeight, 44), 220) + "px";
  }, [input]);

  useEffect(() => {
    if (threadRef.current) threadRef.current.scrollTop = threadRef.current.scrollHeight;
  }, [messagesQuery.data?.length]);

  const sendMessage = useMutation({
    mutationFn: (args: { text: string; attachmentIds: string[] }) =>
      api.appendMessage(chatId, {
        role: "user",
        body: {
          text: args.text,
          ...(args.attachmentIds.length > 0 ? { attachment_ids: args.attachmentIds } : {}),
        },
      }),
    onSuccess: (resp) => {
      setInput("");
      setPendingAttachments([]);
      // Force a refresh of messages + chat (which carries the new
      // workflow IR) so the canvas and thread re-render together.
      qc.invalidateQueries({ queryKey: ["chat-messages", chatId] });
      qc.invalidateQueries({ queryKey: ["chat", chatId] });
      qc.invalidateQueries({ queryKey: ["attachments", chatId] });
      if (resp.error) {
        toasts.push({
          kind: "error",
          title: "Extraction failed",
          body: resp.error,
        });
        return;
      }
      if (resp.workflow) {
        toasts.push({
          kind: resp.workflow.stage === "ready" ? "success" : "info",
          title: resp.workflow.stage === "ready" ? "Workflow ready" : "Draft workflow",
          body: summarizeWorkflow(resp.workflow.ir as Workflow),
        });
      }
    },
    onError: (e) => {
      toasts.push({
        kind: "error",
        title: "Couldn't send message",
        body: e instanceof Error ? e.message : String(e),
      });
    },
  });

  const status: Status = useMemo(() => {
    if (!workflow) return sendMessage.isPending ? "compiling" : "empty";
    if (sendMessage.isPending) return "compiling";
    if (stage === "approved") return "approved";
    if (stage === "drafting") return "drafting";
    return "ready";
  }, [workflow, stage, sendMessage.isPending]);

  return (
    <div className="flex-1 flex flex-col min-h-0 bg-[var(--color-bg)]">
      <TopBar
        chat={chat}
        status={status}
        stage={stage}
        workflow={workflow ?? null}
        deployTargets={deployTargets}
        adapters={adaptersQuery.data ?? []}
        versions={versions}
        chatId={chatId}
        onForkClick={() => setForkModalOpen(true)}
        onVersionSelect={(vId) => restoreMut.mutate(vId)}
        onRestoreRequest={(vId) => {
          setSelectedVersionId(vId);
          setRestoreModalOpen(true);
        }}
        onDiffRequest={(vId) => diffMut.mutate(vId)}
      />

      {forkModalOpen && (
        <ForkModal
          isOpen={forkModalOpen}
          onClose={() => setForkModalOpen(false)}
          onConfirm={(title) => forkMut.mutate({ title })}
          isPending={forkMut.isPending}
        />
      )}

      {restoreModalOpen && selectedVersionId && (
        <RestoreModal
          isOpen={restoreModalOpen}
          versionId={selectedVersionId}
          versions={versions}
          onClose={() => {
            setRestoreModalOpen(false);
            setSelectedVersionId(null);
          }}
          onConfirm={() => restoreMut.mutate(selectedVersionId)}
          isPending={restoreMut.isPending}
        />
      )}

      {diffData && (
        <DiffOverlay
          data={diffData}
          onClose={() => setDiffData(null)}
        />
      )}

      <div className="flex-1 flex min-h-0">
        {/* Thread — animates between 360px (open) and 44px (rail).
            The `motion.aside` owns width; children fade in/out so the
            composer and messages never peek through while the rail is
            collapsing. */}
        <motion.aside
          initial={false}
          animate={{ width: threadCollapsed ? 44 : 360 }}
          transition={{ type: "spring", stiffness: 260, damping: 30 }}
          className="shrink-0 flex flex-col min-h-0 border-r border-[var(--color-border)] bg-[var(--color-surface)] overflow-hidden relative"
        >
          <AnimatePresence initial={false} mode="wait">
            {threadCollapsed ? (
              <motion.div
                key="rail"
                initial={{ opacity: 0 }}
                animate={{ opacity: 1 }}
                exit={{ opacity: 0 }}
                transition={{ duration: 0.12 }}
                className="flex-1 flex flex-col items-center justify-between py-3"
              >
                <button
                  onClick={() => setThreadCollapsed(false)}
                  title="Open thread (⌘\\)"
                  aria-label="Open thread"
                  className="size-8 flex items-center justify-center rounded-md text-[var(--color-fg-muted)] hover:text-[var(--color-fg)] hover:bg-[var(--color-surface-2)] transition-colors"
                >
                  <PanelLeftOpen size={15} />
                </button>
                <div className="flex flex-col items-center gap-2 text-[var(--color-fg-subtle)]">
                  <MessageSquare size={13} />
                  {messagesQuery.data && messagesQuery.data.length > 0 && (
                    <span className="text-[10px] font-mono">{messagesQuery.data.length}</span>
                  )}
                </div>
                <button
                  onClick={() => setThreadCollapsed(false)}
                  title="Expand to compose"
                  aria-label="Expand to compose"
                  className="size-8 flex items-center justify-center rounded-md text-[var(--color-brand)] hover:bg-[var(--color-accent-bg)] transition-colors"
                >
                  <ChevronRight size={15} />
                </button>
              </motion.div>
            ) : (
              <motion.div
                key="thread"
                initial={{ opacity: 0 }}
                animate={{ opacity: 1 }}
                exit={{ opacity: 0 }}
                transition={{ duration: 0.15 }}
                className="flex-1 min-h-0 flex flex-col min-w-[360px]"
              >
                <div className="h-9 px-3 flex items-center justify-between border-b border-[var(--color-border)]">
                  <div
                    className="text-[10px] uppercase tracking-[0.14em] text-[var(--color-fg-subtle)] flex items-center gap-1.5"
                    style={{ fontWeight: 500 }}
                  >
                    <MessageSquare size={10} />
                    Thread
                  </div>
                  <button
                    onClick={() => setThreadCollapsed(true)}
                    title="Collapse thread (⌘\\)"
                    aria-label="Collapse thread"
                    className="size-6 flex items-center justify-center rounded text-[var(--color-fg-subtle)] hover:text-[var(--color-fg)] hover:bg-[var(--color-surface-2)] transition-colors"
                  >
                    <PanelLeftClose size={13} />
                  </button>
                </div>

                <div
                  ref={threadRef}
                  className="flex-1 overflow-y-auto nice-scroll px-4 py-4 space-y-3"
                >
                  {messagesQuery.isLoading && (
                    <div className="text-[11px] text-[var(--color-fg-subtle)] flex items-center gap-1.5">
                      <Loader2 size={10} className="animate-spin" /> loading thread…
                    </div>
                  )}
                  {messagesQuery.data?.length === 0 && <EmptyThread />}
                  <AnimatePresence initial={false}>
                    {messagesQuery.data?.map((m) => <MessageBubble key={m.id} message={m} />)}
                    {sendMessage.isPending && (
                      <motion.div
                        key="thinking"
                        initial={{ opacity: 0, y: 4 }}
                        animate={{ opacity: 1, y: 0 }}
                        className="flex items-center gap-2 text-[11px] text-[var(--color-fg-muted)] italic pl-1"
                      >
                        <Loader2 size={11} className="animate-spin text-[var(--color-brand)]" />
                        Pablo is drafting the workflow…
                      </motion.div>
                    )}
                  </AnimatePresence>
                </div>

                <form
                  className="border-t border-[var(--color-border)] p-3"
                  onSubmit={(e) => {
                    e.preventDefault();
                    const t = input.trim();
                    const ids = pendingAttachments.map((a) => a.id);
                    if ((t || ids.length > 0) && !sendMessage.isPending) {
                      sendMessage.mutate({ text: t, attachmentIds: ids });
                    }
                  }}
                  onDragOver={(e) => {
                    e.preventDefault();
                    if (!isDragging) setIsDragging(true);
                  }}
                  onDragLeave={(e) => {
                    if (e.currentTarget === e.target) setIsDragging(false);
                  }}
                  onDrop={(e) => {
                    e.preventDefault();
                    setIsDragging(false);
                    if (e.dataTransfer.files && e.dataTransfer.files.length > 0) {
                      handleFiles(e.dataTransfer.files);
                    }
                  }}
                >
                  <div
                    className={`rounded-lg border bg-[var(--color-surface-2)] transition-colors ${
                      isDragging
                        ? "border-[var(--color-brand)] ring-2 ring-[var(--color-accent-border)]"
                        : "border-[var(--color-border)] focus-within:border-[var(--color-brand-light)]"
                    }`}
                  >
                    <label
                      className="flex items-center gap-1.5 px-3 pt-2 text-[10px] uppercase tracking-[0.14em] text-[var(--color-fg-subtle)]"
                      style={{ fontWeight: 400 }}
                    >
                      <Sparkles size={10} className="text-[var(--color-brand)]" />
                      {isDragging ? "drop to attach" : "describe the workflow"}
                    </label>

                    {/* Staged attachment chips */}
                    {(pendingAttachments.length > 0 || uploadMut.isPending) && (
                      <div className="px-3 pt-1.5 pb-0.5 flex flex-wrap gap-1.5">
                        {pendingAttachments.map((a) => (
                          <AttachmentChip
                            key={a.id}
                            att={a}
                            onRemove={() =>
                              setPendingAttachments((prev) => prev.filter((x) => x.id !== a.id))
                            }
                          />
                        ))}
                        {uploadMut.isPending && (
                          <span className="inline-flex items-center gap-1 text-[10.5px] px-1.5 py-0.5 rounded-md bg-[var(--color-accent-bg)] text-[var(--color-brand)] font-mono">
                            <Loader2 size={9} className="animate-spin" />
                            uploading…
                          </span>
                        )}
                      </div>
                    )}

                    <div className="flex items-end gap-2 px-3 pb-2.5 pt-1">
                      <button
                        type="button"
                        onClick={() => fileInputRef.current?.click()}
                        title="Attach a file (PDF, TXT)"
                        aria-label="Attach file"
                        className="shrink-0 size-7 flex items-center justify-center rounded-md text-[var(--color-fg-subtle)] hover:text-[var(--color-fg)] hover:bg-[var(--color-surface)] transition-colors"
                      >
                        <Paperclip size={13} />
                      </button>
                      <input
                        ref={fileInputRef}
                        type="file"
                        multiple
                        className="hidden"
                        accept=".pdf,.txt,.md,.csv,application/pdf,text/plain,text/markdown,text/csv"
                        onChange={(e) => {
                          if (e.target.files && e.target.files.length > 0) {
                            handleFiles(e.target.files);
                          }
                          e.target.value = "";
                        }}
                      />
                      <textarea
                        ref={textareaRef}
                        value={input}
                        onChange={(e) => setInput(e.target.value)}
                        onKeyDown={(e) => {
                          if ((e.metaKey || e.ctrlKey) && e.key === "Enter") {
                            e.preventDefault();
                            const t = input.trim();
                            const ids = pendingAttachments.map((a) => a.id);
                            if ((t || ids.length > 0) && !sendMessage.isPending) {
                              sendMessage.mutate({ text: t, attachmentIds: ids });
                            }
                          }
                        }}
                        placeholder={
                          workflow
                            ? "Ask for a change or refinement…"
                            : "Paste a process description or drop a PDF — Pablo will extract actors, tasks, decisions."
                        }
                        rows={1}
                        disabled={sendMessage.isPending}
                        className="flex-1 min-w-0 resize-none bg-transparent text-[13px] text-[var(--color-fg)] placeholder:text-[var(--color-fg-subtle)] focus:outline-none disabled:opacity-60 leading-relaxed"
                        style={{ fontWeight: 300 }}
                      />
                      <button
                        type="submit"
                        disabled={(!input.trim() && pendingAttachments.length === 0) || sendMessage.isPending || uploadMut.isPending}
                        className="shrink-0 inline-flex items-center gap-1 text-[12px] px-2.5 py-1.5 rounded-md bg-[var(--color-brand)] text-white hover:brightness-110 disabled:opacity-40 transition-colors"
                        style={{ fontWeight: 500 }}
                      >
                        {sendMessage.isPending ? (
                          <Loader2 size={11} className="animate-spin" />
                        ) : (
                          <ArrowRight size={11} />
                        )}
                        Send
                      </button>
                    </div>
                  </div>
                  <div className="text-[10px] text-[var(--color-fg-subtle)] font-mono mt-1 flex items-center justify-between">
                    <span>⌘\ to collapse · drop to attach</span>
                    <span>⌘↵ to send</span>
                  </div>
                </form>
              </motion.div>
            )}
          </AnimatePresence>
        </motion.aside>

        {/* Canvas */}
        <section className="flex-1 min-w-0 flex flex-col relative">
          {threadCollapsed && (
            <button
              onClick={() => setThreadCollapsed(false)}
              title="Open thread (⌘\\)"
              aria-label="Open thread"
              className="absolute top-3 left-3 z-10 flex items-center gap-1.5 text-[11px] px-2 py-1 rounded-md border border-[var(--color-border)] bg-[var(--color-surface)]/90 backdrop-blur text-[var(--color-fg-muted)] hover:text-[var(--color-fg)] hover:bg-[var(--color-surface)] transition-colors shadow-sm"
            >
              <ChevronLeft size={12} />
              <MessageSquare size={12} />
              <span style={{ fontWeight: 400 }}>Thread</span>
            </button>
          )}
          <div className="flex-1 min-h-0 relative">
            {!workflow ? (
              <EmptyCanvas hasChat={!!chat} />
            ) : (
              <IRCanvas workflow={workflow} />
            )}
          </div>
        </section>
      </div>
    </div>
  );
}

// --- TopBar ---

const STATUS_TONE: Record<Status, { dot: string; ring: string; text: string; label: string }> = {
  empty:     { dot: "bg-[var(--color-fg-subtle)]",      ring: "border-[var(--color-border)]",     text: "text-[var(--color-fg-subtle)]", label: "EMPTY" },
  drafting:  { dot: "bg-amber-500 animate-pulse",       ring: "border-amber-500/40",              text: "text-amber-600 dark:text-amber-400", label: "DRAFTING" },
  ready:     { dot: "bg-emerald-500",                   ring: "border-emerald-500/40",            text: "text-emerald-600 dark:text-emerald-400", label: "READY" },
  approved:  { dot: "bg-emerald-600",                   ring: "border-emerald-600/50 bg-emerald-500/5", text: "text-emerald-700 dark:text-emerald-300", label: "APPROVED" },
  compiling: { dot: "bg-[var(--color-brand)] animate-pulse", ring: "border-[var(--color-accent-border)]", text: "text-[var(--color-brand)]", label: "WORKING" },
  compiled:  { dot: "bg-emerald-500",                   ring: "border-emerald-500/40",            text: "text-emerald-600 dark:text-emerald-400", label: "COMPILED" },
  deploying: { dot: "bg-[var(--color-brand)] animate-pulse", ring: "border-[var(--color-accent-border)]", text: "text-[var(--color-brand)]", label: "DEPLOYING" },
  deployed:  { dot: "bg-emerald-500",                   ring: "border-emerald-500/50",            text: "text-emerald-700 dark:text-emerald-300", label: "DEPLOYED" },
  error:     { dot: "bg-rose-500",                      ring: "border-rose-500/40",               text: "text-rose-600 dark:text-rose-400", label: "ERROR" },
};

function TopBar({
  chat,
  status,
  stage,
  workflow,
  deployTargets,
  adapters,
  versions,
  chatId,
  onForkClick,
  onVersionSelect,
  onRestoreRequest,
  onDiffRequest,
}: {
  chat: Chat | undefined;
  status: Status;
  stage: "drafting" | "ready" | "approved" | undefined;
  workflow: Workflow | null;
  deployTargets: DeployTarget[];
  adapters: EngineAdapter[];
  versions: WorkflowVersionListItem[];
  chatId: string;
  onForkClick: () => void;
  onVersionSelect: (versionId: string) => void;
  onRestoreRequest: (versionId: string) => void;
  onDiffRequest: (versionId: string) => void;
}) {
  const toasts = useToasts();
  const qc = useQueryClient();
  const tone = STATUS_TONE[status];
  const [target, setTarget] = useState<string>("camunda7");
  const [deployMenuOpen, setDeployMenuOpen] = useState(false);
  const [versionMenuOpen, setVersionMenuOpen] = useState(false);
  const selectedAdapter = adapters.find((a) => a.kind === target);
  const compileTargets = adapters.length > 0
    ? adapters
    : [
        {
          kind: "camunda7",
          name: "Camunda 7",
          capabilities: { can_discover: true, can_deploy: true, artifact_mime: "application/xml", artifact_ext: "bpmn" },
        },
        {
          kind: "elsa3",
          name: "Elsa 3",
          capabilities: { can_discover: false, can_deploy: true, artifact_mime: "application/json", artifact_ext: "json" },
        },
      ];

  useEffect(() => {
    if (adapters.length > 0 && !adapters.some((a) => a.kind === target)) {
      const first = adapters[0];
      if (first) setTarget(first.kind);
    }
  }, [adapters, target]);

  const handleRestoreClick = (versionId: string) => {
    setVersionMenuOpen(false);
    onRestoreRequest(versionId);
  };

  // Approve gate — US-013. "Fully resolved" is derived client-side
  // (US-011) from collectLowConfidence so canvas + Approve stay in
  // lockstep: if the Diagnostics chip sees ambiguity, the button
  // disables and explains why via the title tooltip.
  const lowConfidence = useMemo(
    () => (workflow ? collectLowConfidence(workflow) : []),
    [workflow],
  );
  const alreadyApproved = stage === "approved";
  const canApprove =
    !!workflow && stage === "ready" && lowConfidence.length === 0 && !alreadyApproved;
  const approveBlockReason = !workflow
    ? "Describe a workflow first."
    : alreadyApproved
      ? "Already approved — further edits will supersede."
      : stage === "drafting"
        ? "Resolve the drafting issues before approving."
        : lowConfidence.length > 0
          ? `${lowConfidence.length} low-confidence element${lowConfidence.length === 1 ? "" : "s"} remain — answer the open questions first.`
          : "";

  const approveMut = useMutation({
    mutationFn: async () => {
      if (!chat) throw new Error("no chat");
      return api.approveChat(chat.id);
    },
    onSuccess: () => {
      // Refresh chat so the status chip reads the new stage.
      qc.invalidateQueries({ queryKey: ["chat", chat?.id] });
      toasts.push({
        kind: "success",
        title: "Workflow approved",
        body: "Locked as the operator-sanctioned version. New edits will create a fresh draft.",
      });
    },
    onError: (e) => {
      toasts.push({
        kind: "error",
        title: "Couldn't approve",
        body: e instanceof Error ? e.message : String(e),
      });
    },
  });

  const deployMut = useMutation({
    mutationFn: async (targetId: string) => {
      if (!chat) throw new Error("no chat");
      return api.deployChat(chat.id, targetId);
    },
    onSuccess: (resp) => {
      const ref = resp.process_key || resp.process_definition_id || resp.deployment_id || "(no id)";
      const warnings = resp.diagnostics?.filter((d) => d.severity === "warning").length ?? 0;
      toasts.push({
        kind: "success",
        title: `Deployed to ${resp.kind === "camunda7" ? "Camunda 7" : "Elsa 3"}`,
        body: `${ref} · ${resp.artifact_bytes.toLocaleString()} bytes pushed${warnings > 0 ? ` · ${warnings} warning${warnings === 1 ? "" : "s"}` : ""}`,
        action: resp.studio_url
          ? {
              label: resp.kind === "elsa3" ? "Open in Elsa Studio" : "Open in Cockpit",
              href: resp.studio_url,
            }
          : undefined,
        // Toasts with a link stay visible longer — the user needs
        // time to read, decide, and click. 5s default is too short.
        ttlMs: resp.studio_url ? 10000 : 5000,
      });
    },
    onError: (e) => {
      toasts.push({
        kind: "error",
        title: "Deploy failed",
        body: e instanceof Error ? e.message : String(e),
      });
    },
  });

  const compileMut = useMutation({
    mutationFn: async () => {
      if (!workflow) throw new Error("no workflow");
      return api.compile(target, workflow);
    },
    onSuccess: (resp) => {
      const mime = resp.mime ?? selectedAdapter?.capabilities.artifact_mime ?? "application/octet-stream";
      const ext = selectedAdapter?.capabilities.artifact_ext ?? (target === "camunda7" ? "bpmn" : "json");
      downloadArtifact(chat?.title ?? "workflow", resp.artifact, ext, mime);
      const warnings = resp.diagnostics?.filter((d) => d.severity === "warning").length ?? 0;
      toasts.push({
        kind: "success",
        title: "Artifact compiled",
        body: `${new Blob([resp.artifact]).size.toLocaleString()} bytes downloaded as .${ext}${warnings > 0 ? ` · ${warnings} warning${warnings === 1 ? "" : "s"}` : ""}`,
      });
    },
    onError: (e) => {
      toasts.push({
        kind: "error",
        title: "Compile failed",
        body: e instanceof Error ? e.message : String(e),
      });
    },
  });

  const canCompile = !!workflow && status !== "drafting";

  const currentVersion = versions[0];
  const hasMultipleVersions = versions.length > 1;

  return (
    <header className="h-14 shrink-0 bg-[var(--color-surface)] border-b border-[var(--color-border)] flex items-center px-5 gap-3">
      <div className="flex items-center gap-2 min-w-0">
        <WorkflowIcon size={14} className="text-[var(--color-fg-muted)] shrink-0" />
        <div className="min-w-0">
          <div
            className="text-[10px] uppercase tracking-[0.14em] text-[var(--color-fg-subtle)]"
            style={{ fontWeight: 400 }}
          >
            Chat
          </div>
          <div
            className="text-[14px] text-[var(--color-fg)] truncate max-w-[32ch]"
            style={{ fontWeight: 400 }}
          >
            {chat?.title ?? "…"}
          </div>
        </div>
      </div>

      <span
        className={`ml-1 flex items-center gap-1.5 text-[10px] font-mono px-2 py-0.5 border rounded-full ${tone.ring} ${tone.text}`}
      >
        <span className={`size-1.5 rounded-full ${tone.dot}`} />
        {tone.label}
      </span>

      {hasMultipleVersions && (
        <div className="relative">
          <button
            onClick={() => setVersionMenuOpen((v) => !v)}
            className="flex items-center gap-1.5 text-xs px-2 py-1 rounded-md border border-[var(--color-border)] bg-[var(--color-surface)] text-[var(--color-fg)] hover:bg-[var(--color-surface-2)]"
            title="Version history"
          >
            <History size={13} />
            <span className="text-[10px] uppercase tracking-widest">v{versions.length}</span>
            <ChevronDown size={11} />
          </button>
          <AnimatePresence>
            {versionMenuOpen && (
              <motion.div
                initial={{ opacity: 0, y: -4 }}
                animate={{ opacity: 1, y: 0 }}
                exit={{ opacity: 0, y: -2 }}
                className="absolute right-0 top-full mt-1 w-[320px] bg-[var(--color-surface)] border border-[var(--color-border)] rounded-md shadow-stripe-deep z-20 overflow-hidden"
                onMouseLeave={() => setVersionMenuOpen(false)}
              >
                <div className="px-3 pt-2 pb-1 text-[10px] uppercase tracking-[0.14em] text-[var(--color-fg-subtle)]" style={{ fontWeight: 500 }}>
                  Version history
                </div>
                {versions.map((v, idx) => (
                  <div
                    key={v.id}
                    className="w-full text-left px-3 py-2 flex items-center justify-between hover:bg-[var(--color-surface-2)] transition-colors group"
                  >
                    <div className="flex items-center gap-2 min-w-0">
                      <span className={`size-1.5 shrink-0 rounded-full ${idx === 0 ? "bg-emerald-500" : "bg-[var(--color-fg-subtle)]"}`} />
                      <div className="min-w-0">
                        <div className="text-[12px] text-[var(--color-fg)] truncate">
                          {idx === 0 ? "Current version" : `v${versions.length - idx}`}
                        </div>
                        <div className="text-[9px] text-[var(--color-fg-subtle)] font-mono">
                          {new Date(v.created_at).toLocaleDateString(undefined, { month: "short", day: "numeric", hour: "2-digit", minute: "2-digit" })}
                        </div>
                      </div>
                    </div>
                    <div className="flex items-center gap-1 opacity-0 group-hover:opacity-100 transition-opacity">
                      {idx !== 0 && (
                        <button
                          onClick={() => onDiffRequest(v.id)}
                          className="px-1.5 py-0.5 text-[10px] bg-[var(--color-surface)] border border-[var(--color-border)] rounded hover:bg-[var(--color-surface-2)]"
                        >
                          Diff
                        </button>
                      )}
                      <button
                        onClick={() => handleRestoreClick(v.id)}
                        disabled={idx === 0}
                        className="px-1.5 py-0.5 text-[10px] bg-[var(--color-brand)] text-white rounded hover:brightness-110 disabled:opacity-50"
                      >
                        {idx === 0 ? "Active" : "Restore"}
                      </button>
                    </div>
                  </div>
                ))}
              </motion.div>
            )}
          </AnimatePresence>
        </div>
      )}

      <button
        onClick={onForkClick}
        disabled={!workflow}
        title={!workflow ? "Describe a workflow first to fork it" : "Create a copy of this workflow"}
        className="flex items-center gap-1.5 text-xs font-medium px-3 py-1.5 rounded-md border border-[var(--color-border)] bg-[var(--color-surface)] text-[var(--color-fg)] hover:bg-[var(--color-surface-2)] disabled:opacity-40 disabled:cursor-not-allowed"
      >
        <GitBranch size={13} />
        Fork
      </button>

      <div className="flex-1" />

      <label className="flex items-center gap-1.5 text-xs text-[var(--color-fg-muted)]">
        <span className="text-[10px] uppercase tracking-widest">target</span>
        <div className="relative">
          <select
            value={target}
            onChange={(e) => setTarget(e.target.value)}
            className="appearance-none bg-[var(--color-surface)] border border-[var(--color-border)] rounded-md pl-2.5 pr-7 py-1 text-xs text-[var(--color-fg)] hover:border-[var(--color-border-strong)] focus:outline-none focus:ring-2 focus:ring-[var(--color-accent-border)]"
          >
            {compileTargets.map((a) => (
              <option key={a.kind} value={a.kind}>
                {a.name}
              </option>
            ))}
          </select>
          <ChevronDown size={12} className="absolute right-2 top-1/2 -translate-y-1/2 text-[var(--color-fg-subtle)] pointer-events-none" />
        </div>
      </label>

      <button
        onClick={() => compileMut.mutate()}
        disabled={!canCompile || compileMut.isPending}
        title={!canCompile ? "Describe a workflow first, then come back to compile." : ""}
        className="flex items-center gap-1.5 text-xs font-medium px-3 py-1.5 rounded-md border border-[var(--color-border)] bg-[var(--color-surface)] text-[var(--color-fg)] hover:bg-[var(--color-surface-2)] disabled:opacity-40 disabled:cursor-not-allowed"
      >
        {compileMut.isPending ? (
          <Loader2 size={13} className="animate-spin" />
        ) : (
          <Download size={13} />
        )}
        Compile
      </button>

      <button
        onClick={() => approveMut.mutate()}
        disabled={!canApprove || approveMut.isPending}
        title={canApprove ? "Lock this workflow as the approved version" : approveBlockReason}
        className={
          alreadyApproved
            ? "flex items-center gap-1.5 text-xs font-medium px-3 py-1.5 rounded-md border border-emerald-500/50 bg-emerald-500/10 text-emerald-700 dark:text-emerald-300 cursor-default"
            : "flex items-center gap-1.5 text-xs font-medium px-3 py-1.5 rounded-md border border-emerald-500/40 text-emerald-700 dark:text-emerald-300 hover:bg-emerald-500/10 disabled:opacity-40 disabled:cursor-not-allowed disabled:hover:bg-transparent"
        }
      >
        {approveMut.isPending ? (
          <Loader2 size={13} className="animate-spin" />
        ) : (
          <Check size={13} />
        )}
        {alreadyApproved ? "Approved" : "Approve"}
      </button>

      <div className="relative">
        <button
          onClick={() => setDeployMenuOpen((v) => !v)}
          disabled={!canCompile || deployMut.isPending}
          className="flex items-center gap-1.5 text-xs font-medium px-3 py-1.5 rounded-md text-white bg-[var(--color-brand)] hover:brightness-110 disabled:opacity-40 disabled:cursor-not-allowed shadow-sm"
        >
          {deployMut.isPending ? <Loader2 size={13} className="animate-spin" /> : <Rocket size={13} />}
          Deploy
          <ChevronDown size={11} />
        </button>
        <AnimatePresence>
          {deployMenuOpen && (
            <motion.div
              initial={{ opacity: 0, y: -4 }}
              animate={{ opacity: 1, y: 0 }}
              exit={{ opacity: 0, y: -2 }}
              className="absolute right-0 top-full mt-1 w-[240px] bg-[var(--color-surface)] border border-[var(--color-border)] rounded-md shadow-stripe-deep z-20 overflow-hidden"
              onMouseLeave={() => setDeployMenuOpen(false)}
            >
              <div
                className="px-3 pt-2 pb-1 text-[10px] uppercase tracking-[0.14em] text-[var(--color-fg-subtle)]"
                style={{ fontWeight: 500 }}
              >
                Project deploy targets
              </div>
              {deployTargets.length === 0 && (
                <div className="px-3 py-2 text-[11px] text-[var(--color-fg-muted)] leading-snug">
                  None configured. Open Settings → Deploy targets to add a Camunda or Elsa endpoint for this project.
                </div>
              )}
              {deployTargets.map((t) => (
                <button
                  key={t.id}
                  onClick={() => {
                    setDeployMenuOpen(false);
                    deployMut.mutate(t.id);
                  }}
                  disabled={deployMut.isPending}
                  className="w-full text-left px-3 py-2 flex items-start gap-2 hover:bg-[var(--color-surface-2)] disabled:opacity-50 transition-colors"
                >
                  <span
                    className={`mt-1 size-[6px] rounded-full ${
                      t.kind === "camunda7" ? "bg-emerald-500" : "bg-violet-500"
                    }`}
                  />
                  <div className="min-w-0">
                    <div className="text-[12.5px] text-[var(--color-fg)] truncate" style={{ fontWeight: 400 }}>
                      {t.name}
                    </div>
                    <div className="text-[10.5px] text-[var(--color-fg-subtle)] font-mono truncate">
                      {t.kind} · {t.endpoint}
                    </div>
                  </div>
                </button>
              ))}
            </motion.div>
          )}
        </AnimatePresence>
      </div>
    </header>
  );
}

// --- supporting pieces ---

function EmptyThread() {
  return (
    <div className="rounded-lg border border-dashed border-[var(--color-border-purple)] bg-[var(--color-accent-bg)] p-4">
      <div className="text-[13px] text-[var(--color-fg)]" style={{ fontWeight: 400 }}>
        Start the conversation
      </div>
      <div className="text-[11.5px] text-[var(--color-fg-muted)] mt-1 leading-relaxed">
        Describe a workflow below — in any language. You can also paste a process transcript,
        procedure, or instructions. Pablo extracts actors, tasks, and decisions, and you refine
        by chatting.
      </div>
    </div>
  );
}

function EmptyCanvas({ hasChat }: { hasChat: boolean }) {
  return (
    <div className="w-full h-full flex items-center justify-center canvas-dots">
      <div className="max-w-md text-center px-6">
        <div className="mx-auto mb-4 size-12 rounded-2xl bg-[var(--color-accent-bg)] flex items-center justify-center">
          <WorkflowIcon size={20} className="text-[var(--color-brand)]" strokeWidth={1.6} />
        </div>
        <div
          className="text-[11px] tracking-[0.14em] uppercase text-[var(--color-fg-subtle)]"
          style={{ fontWeight: 500 }}
        >
          Empty canvas
        </div>
        <h2
          className="text-[17px] text-[var(--color-fg)] mt-1"
          style={{ fontWeight: 300, letterSpacing: "-0.01em" }}
        >
          {hasChat ? "Describe a workflow to begin" : "Open a chat to begin"}
        </h2>
        <p className="text-[12.5px] text-[var(--color-fg-muted)] leading-relaxed mt-2">
          Use the composer on the left — type a description, paste a process, or drop a document.
          Pablo will extract the workflow and render it here. Refine iteratively by chatting.
        </p>
      </div>
    </div>
  );
}

function MessageBubble({ message }: { message: ChatMessage }) {
  const role = message.role;
  const body = message.body as {
    text?: string;
    workflow_version_id?: string;
    error?: string;
    attachment_ids?: string[];
    questions?: { id: string; text: string; ir_ref?: string }[];
  };
  const text = body.text ?? body.error ?? JSON.stringify(body);
  const questions = body.questions ?? [];

  if (role === "user") {
    const attIds = body.attachment_ids ?? [];
    return (
      <motion.div initial={{ opacity: 0, y: 4 }} animate={{ opacity: 1, y: 0 }} className="flex justify-end">
        <div className="max-w-[90%] flex flex-col items-end gap-1.5">
          {attIds.length > 0 && <BoundAttachmentChips chatId={message.chat_id} ids={attIds} />}
          {text && (
            <div
              className="text-[13px] rounded-lg px-3 py-2 bg-[var(--color-brand)] text-white"
              style={{ fontWeight: 300 }}
            >
              {text}
            </div>
          )}
        </div>
      </motion.div>
    );
  }
  if (role === "system") {
    return (
      <motion.div initial={{ opacity: 0 }} animate={{ opacity: 1 }}>
        <div
          className="text-[11px] rounded-md border border-rose-500/20 bg-rose-500/5 text-rose-600 dark:text-rose-300 px-2 py-1 italic"
          style={{ fontWeight: 400 }}
        >
          {text}
        </div>
      </motion.div>
    );
  }
  return (
    <motion.div initial={{ opacity: 0, y: 4 }} animate={{ opacity: 1, y: 0 }}>
      <div className="flex items-start gap-2">
        <div className="size-5 rounded-full bg-[var(--color-accent-bg)] flex items-center justify-center shrink-0 mt-0.5">
          <Sparkles size={10} className="text-[var(--color-brand)]" />
        </div>
        <div
          className="flex-1 min-w-0 rounded-lg border border-[var(--color-border)] bg-[var(--color-surface-2)] px-3 py-2 text-[13px] text-[var(--color-fg)] leading-relaxed"
          style={{ fontWeight: 300 }}
        >
          {text}
          {questions.length > 0 && (
            <ol className="mt-2.5 space-y-1.5 pl-0 list-none">
              {questions.map((q, i) => (
                <li
                  key={q.id}
                  className="flex items-start gap-2 rounded-md border border-[var(--color-border-purple)] bg-[var(--color-accent-bg)] px-2.5 py-1.5"
                >
                  <HelpCircle
                    size={11}
                    className="text-[var(--color-brand)] shrink-0 mt-[3px]"
                  />
                  <span className="text-[12.5px] text-[var(--color-fg)] leading-snug">
                    <span
                      className="text-[var(--color-fg-subtle)] font-mono mr-1.5"
                      style={{ fontWeight: 500 }}
                    >
                      {i + 1}.
                    </span>
                    {q.text}
                  </span>
                </li>
              ))}
            </ol>
          )}
        </div>
      </div>
    </motion.div>
  );
}

// --- attachments ---

function kindIcon(kind: Attachment["kind"], size = 11) {
  if (kind === "voice") return <Mic size={size} />;
  if (kind === "image") return <ImageIcon size={size} />;
  return <FileText size={size} />;
}

function prettySize(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(0)} KB`;
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
}

function AttachmentChip({
  att,
  onRemove,
}: {
  att: Attachment;
  onRemove?: () => void;
}) {
  return (
    <motion.span
      initial={{ opacity: 0, scale: 0.96 }}
      animate={{ opacity: 1, scale: 1 }}
      exit={{ opacity: 0, scale: 0.96 }}
      title={`${att.filename} · ${prettySize(att.size_bytes)}\n\n${att.text_preview || "(no extracted text)"}`}
      className="inline-flex items-center gap-1 text-[11px] px-1.5 py-0.5 rounded-md border border-[var(--color-border)] bg-[var(--color-surface)] text-[var(--color-fg-muted)] max-w-[220px]"
    >
      <span className="text-[var(--color-brand)] shrink-0">{kindIcon(att.kind, 10)}</span>
      <span className="truncate" style={{ fontWeight: 400 }}>
        {att.filename}
      </span>
      <span className="text-[9.5px] text-[var(--color-fg-subtle)] font-mono shrink-0">
        {prettySize(att.size_bytes)}
      </span>
      {onRemove && (
        <button
          type="button"
          onClick={(e) => {
            e.stopPropagation();
            onRemove();
          }}
          aria-label={`Remove ${att.filename}`}
          className="ml-0.5 shrink-0 size-3.5 flex items-center justify-center rounded hover:bg-[var(--color-surface-2)] text-[var(--color-fg-subtle)] hover:text-[var(--color-fg)]"
        >
          <X size={9} />
        </button>
      )}
    </motion.span>
  );
}

// BoundAttachmentChips renders chips for the attachments bound to a
// past user message. It hits the chat-scoped attachments list and
// filters by id — cheap, cached, and matches the composer's preview.
function BoundAttachmentChips({ chatId, ids }: { chatId: string; ids: string[] }) {
  const q = useQuery({
    queryKey: ["attachments", chatId],
    queryFn: () => api.listAttachments(chatId).then((r) => r.attachments),
  });
  const byId = new Map((q.data ?? []).map((a) => [a.id, a]));
  const resolved = ids.map((id) => byId.get(id)).filter(Boolean) as Attachment[];
  if (resolved.length === 0) return null;
  return (
    <div className="flex flex-wrap gap-1 justify-end">
      {resolved.map((a) => (
        <AttachmentChip key={a.id} att={a} />
      ))}
    </div>
  );
}

function summarizeWorkflow(ir: Workflow): string {
  const tasks = ir.tasks?.length ?? 0;
  const gateways = ir.gateways?.length ?? 0;
  return `${tasks} task${tasks === 1 ? "" : "s"} · ${gateways} gateway${gateways === 1 ? "" : "s"}`;
}

function downloadArtifact(name: string, bytes: string, ext: string, mime: string) {
  const safe = name.replace(/[^a-zA-Z0-9_-]+/g, "_").toLowerCase() || "workflow";
  const blob = new Blob([bytes], { type: mime });
  const url = URL.createObjectURL(blob);
  const a = document.createElement("a");
  a.href = url;
  a.download = `${safe}.${ext}`;
  a.click();
  URL.revokeObjectURL(url);
}

// --- Versioning / Forking Modals (Phase 4) ---

function ForkModal({
  isOpen,
  onClose,
  onConfirm,
  isPending,
}: {
  isOpen: boolean;
  onClose: () => void;
  onConfirm: (title: string) => void;
  isPending: boolean;
}) {
  const [title, setTitle] = useState("");

  if (!isOpen) return null;

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/40 backdrop-blur-sm">
      <motion.div
        initial={{ opacity: 0, scale: 0.95 }}
        animate={{ opacity: 1, scale: 1 }}
        className="w-full max-w-sm bg-[var(--color-surface)] border border-[var(--color-border)] rounded-xl shadow-stripe-deep overflow-hidden"
      >
        <div className="p-5">
          <div className="flex items-center gap-2 text-[var(--color-brand)] mb-1">
            <GitBranch size={16} />
            <h3 className="text-sm font-semibold">Fork Workflow</h3>
          </div>
          <p className="text-[12.5px] text-[var(--color-fg-muted)] mb-4 leading-relaxed">
            Create a copy of this workflow in a new chat to explore variants without losing the original.
          </p>

          <label className="block text-[11px] uppercase tracking-widest text-[var(--color-fg-subtle)] mb-1.5 ml-1">
            New chat title
          </label>
          <input
            autoFocus
            type="text"
            value={title}
            onChange={(e) => setTitle(e.target.value)}
            placeholder="e.g. Variant with parallel approval"
            className="w-full bg-[var(--color-surface-2)] border border-[var(--color-border)] rounded-md px-3 py-2 text-[13px] text-[var(--color-fg)] focus:outline-none focus:border-[var(--color-brand)]"
            onKeyDown={(e) => {
              if (e.key === "Enter" && title.trim()) onConfirm(title.trim());
              if (e.key === "Escape") onClose();
            }}
          />
        </div>

        <div className="bg-[var(--color-surface-2)] px-5 py-3 flex justify-end gap-2 border-t border-[var(--color-border)]">
          <button
            onClick={onClose}
            className="px-3 py-1.5 text-xs font-medium text-[var(--color-fg-muted)] hover:text-[var(--color-fg)]"
          >
            Cancel
          </button>
          <button
            onClick={() => onConfirm(title.trim())}
            disabled={!title.trim() || isPending}
            className="px-4 py-1.5 text-xs font-medium bg-[var(--color-brand)] text-white rounded-md hover:brightness-110 disabled:opacity-40"
          >
            {isPending ? <Loader2 size={13} className="animate-spin" /> : "Create Fork"}
          </button>
        </div>
      </motion.div>
    </div>
  );
}

function RestoreModal({
  isOpen,
  versionId,
  versions,
  onClose,
  onConfirm,
  isPending,
}: {
  isOpen: boolean;
  versionId: string;
  versions: WorkflowVersionListItem[];
  onClose: () => void;
  onConfirm: () => void;
  isPending: boolean;
}) {
  if (!isOpen) return null;

  const versionIndex = versions.findIndex((v) => v.id === versionId);
  const versionLabel = versionIndex >= 0 ? `v${versions.length - versionIndex}` : versionId.slice(0, 8);

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/40 backdrop-blur-sm">
      <motion.div
        initial={{ opacity: 0, scale: 0.95 }}
        animate={{ opacity: 1, scale: 1 }}
        className="w-full max-w-sm bg-[var(--color-surface)] border border-[var(--color-border)] rounded-xl shadow-stripe-deep overflow-hidden"
      >
        <div className="p-5">
          <div className="flex items-center gap-2 text-[var(--color-brand)] mb-1">
            <RotateCcw size={16} />
            <h3 className="text-sm font-semibold">Restore Version</h3>
          </div>
          <p className="text-[12.5px] text-[var(--color-fg-muted)] mb-4 leading-relaxed">
            This will create a new version from the selected snapshot. Your current workflow will be preserved in history.
          </p>
          <div className="bg-[var(--color-surface-2)] border border-[var(--color-border)] rounded-md px-3 py-2">
            <div className="text-[10px] uppercase tracking-widest text-[var(--color-fg-subtle)] mb-0.5">Restoring version</div>
            <div className="text-[13px] text-[var(--color-fg)]">{versionLabel}</div>
          </div>
        </div>

        <div className="bg-[var(--color-surface-2)] px-5 py-3 flex justify-end gap-2 border-t border-[var(--color-border)]">
          <button
            onClick={onClose}
            className="px-3 py-1.5 text-xs font-medium text-[var(--color-fg-muted)] hover:text-[var(--color-fg)]"
          >
            Cancel
          </button>
          <button
            onClick={onConfirm}
            disabled={isPending}
            className="px-4 py-1.5 text-xs font-medium bg-[var(--color-brand)] text-white rounded-md hover:brightness-110 disabled:opacity-40"
          >
            {isPending ? <Loader2 size={13} className="animate-spin" /> : "Restore Version"}
          </button>
        </div>
      </motion.div>
    </div>
  );
}

function DiffOverlay({
  data,
  onClose,
}: {
  data: { added: string[]; removed: string[]; changed: string[] };
  onClose: () => void;
}) {
  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/20 backdrop-blur-sm">
      <motion.div
        initial={{ opacity: 0, y: 10 }}
        animate={{ opacity: 1, y: 0 }}
        className="w-full max-w-md bg-[var(--color-surface)] border border-[var(--color-border)] rounded-xl shadow-stripe-deep overflow-hidden flex flex-col max-h-[80vh]"
      >
        <div className="p-4 border-b border-[var(--color-border)] flex items-center justify-between">
          <div className="flex items-center gap-2">
            <History size={16} className="text-[var(--color-brand)]" />
            <h3 className="text-sm font-semibold">Workflow Diff</h3>
          </div>
          <button
            onClick={onClose}
            className="p-1 rounded-md hover:bg-[var(--color-surface-2)] text-[var(--color-fg-subtle)]"
          >
            <X size={16} />
          </button>
        </div>

        <div className="flex-1 overflow-y-auto p-5 space-y-6">
          <section>
            <h4 className="text-[10px] uppercase tracking-widest text-emerald-600 dark:text-emerald-400 font-semibold mb-2 flex items-center gap-1.5">
              <span className="size-1.5 rounded-full bg-emerald-500" />
              Added
            </h4>
            {data.added.length === 0 ? (
              <p className="text-[12px] text-[var(--color-fg-subtle)] italic">No elements added</p>
            ) : (
              <div className="space-y-1">
                {data.added.map((item) => (
                  <div key={item} className="text-[12px] font-mono bg-emerald-500/5 text-emerald-700 dark:text-emerald-300 px-2 py-1 rounded border border-emerald-500/10">
                    + {item}
                  </div>
                ))}
              </div>
            )}
          </section>

          <section>
            <h4 className="text-[10px] uppercase tracking-widest text-rose-600 dark:text-rose-400 font-semibold mb-2 flex items-center gap-1.5">
              <span className="size-1.5 rounded-full bg-rose-500" />
              Removed
            </h4>
            {data.removed.length === 0 ? (
              <p className="text-[12px] text-[var(--color-fg-subtle)] italic">No elements removed</p>
            ) : (
              <div className="space-y-1">
                {data.removed.map((item) => (
                  <div key={item} className="text-[12px] font-mono bg-rose-500/5 text-rose-700 dark:text-rose-300 px-2 py-1 rounded border border-rose-500/10">
                    - {item}
                  </div>
                ))}
              </div>
            )}
          </section>

          <section>
            <h4 className="text-[10px] uppercase tracking-widest text-amber-600 dark:text-amber-400 font-semibold mb-2 flex items-center gap-1.5">
              <span className="size-1.5 rounded-full bg-amber-500" />
              Modified
            </h4>
            {data.changed.length === 0 ? (
              <p className="text-[12px] text-[var(--color-fg-subtle)] italic">No elements modified</p>
            ) : (
              <div className="space-y-1">
                {data.changed.map((item) => (
                  <div key={item} className="text-[12px] font-mono bg-amber-500/5 text-amber-700 dark:text-amber-300 px-2 py-1 rounded border border-amber-500/10">
                    ~ {item}
                  </div>
                ))}
              </div>
            )}
          </section>
        </div>

        <div className="p-4 bg-[var(--color-surface-2)] border-t border-[var(--color-border)] text-center">
          <p className="text-[11px] text-[var(--color-fg-subtle)] leading-relaxed">
            Comparing selected version against the active workspace.
            <br />
            Restoring a version will create a new commit from its snapshot.
          </p>
        </div>
      </motion.div>
    </div>
  );
}
