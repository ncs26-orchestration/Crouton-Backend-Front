import { useEffect, useMemo, useRef, useState } from "react";
import { useMutation } from "@tanstack/react-query";
import { AnimatePresence, motion } from "framer-motion";
import {
  ArrowRight,
  Bot,
  Check,
  HelpCircle,
  Loader2,
  Sparkles,
  X,
  Undo2,
} from "lucide-react";

import { api } from "../lib/api";
import { collectLowConfidence, TIER_COLOR, TIER_TEXT } from "../lib/confidence";
import type { Workflow } from "../lib/types";
import { useToasts } from "./Toasts";

// The Copilot sidebar is the interactive face of Crouton. Two modes:
//   - Ask      → grounded Q&A over the current IR + IS + sources.
//   - Clarify  → auto-populated cards that propose JSON-Patches to
//                resolve low-confidence elements. Accepting a card
//                applies the patch and drops it into an "Accepted"
//                ribbon so the user can undo experimentally.

export interface PatchRecord {
  id: number;
  label: string;
  patch: unknown[];
  previousIR: Workflow;
}

interface Props {
  open: boolean;
  onClose: () => void;
  workflow: Workflow | null;
  onIRUpdate: (next: Workflow) => void;
  // When the Copilot wants to glow a specific task on the canvas.
  onHighlightTask?: (id: string | null) => void;
  // Drafting-gate integration. When `stage === "drafting"` the
  // Copilot becomes the primary stage for user action — shows
  // resolved/total progress + a Finalize CTA. Accepted and
  // reopened Clarify items are reported back so the parent can
  // keep its finalized set in sync.
  stage?: "empty" | "drafting" | "ready";
  interviewProgress?: { resolved: number; pending: number; total: number };
  onFinalize?: () => void;
  onItemResolved?: (key: string) => void;
  onItemReopened?: (key: string) => void;
}

type Mode = "ask" | "clarify";

interface ChatMessage {
  id: number;
  role: "user" | "assistant";
  text: string;
  evidence?: { ir_ref?: string; quote?: string }[];
  busy?: boolean;
  error?: string;
}

let msgCounter = 0;
let patchCounter = 0;

export function CopilotPanel({
  open,
  onClose,
  workflow,
  onIRUpdate,
  onHighlightTask,
  stage,
  interviewProgress,
  onFinalize,
  onItemResolved,
  onItemReopened,
}: Props) {
  const toasts = useToasts();
  const [mode, setMode] = useState<Mode>("clarify");
  const [messages, setMessages] = useState<ChatMessage[]>([]);
  const [input, setInput] = useState("");
  const [patches, setPatches] = useState<PatchRecord[]>([]);
  // Track which Clarify items the user has resolved. Keyed by
  // `${kind}:${id}` to match collectLowConfidence item identity.
  // Applied items collapse to a tight "✓ resolved" row so the user
  // sees progress and the active queue stays short.
  const [appliedItems, setAppliedItems] = useState<Set<string>>(new Set());
  const scrollRef = useRef<HTMLDivElement | null>(null);

  // Low-confidence queue — the Clarify mode works off this. Computed
  // from the workflow + any patches already applied.
  const lowConfidence = useMemo(() => collectLowConfidence(workflow), [workflow]);

  // When the panel opens and we have low-confidence items but no
  // messages yet, seed a welcome bubble explaining the two modes.
  useEffect(() => {
    if (!open) return;
    if (messages.length === 0) {
      setMessages([
        {
          id: ++msgCounter,
          role: "assistant",
          text:
            lowConfidence.length > 0
              ? `I spotted ${lowConfidence.length} element${lowConfidence.length === 1 ? "" : "s"} below 80% confidence. Pick any of them to clarify, or switch to Ask mode to query the workflow.`
              : "Everything looks confident. Switch to Ask to query the workflow, or come back when the extractor flags something.",
        },
      ]);
    }
  }, [open]); // eslint-disable-line react-hooks/exhaustive-deps

  useEffect(() => {
    if (scrollRef.current) scrollRef.current.scrollTop = scrollRef.current.scrollHeight;
  }, [messages, patches]);

  const askMut = useMutation({
    mutationFn: (question: string) =>
      api.copilotAsk({ ir: workflow!, question }),
    onMutate: (question) => {
      const id = ++msgCounter;
      setMessages((prev) => [
        ...prev,
        { id: id - 1, role: "user", text: question },
        { id, role: "assistant", text: "", busy: true },
      ]);
      return id;
    },
    onSuccess: (resp, _q, id) => {
      setMessages((prev) =>
        prev.map((m) =>
          m.id === id
            ? {
                ...m,
                busy: false,
                text: resp.answer || "(no answer)",
                evidence: resp.evidence,
                error: resp.error ?? undefined,
              }
            : m,
        ),
      );
    },
    onError: (err, _q, id) => {
      const message = err instanceof Error ? err.message : String(err);
      setMessages((prev) =>
        prev.map((m) => (m.id === id ? { ...m, busy: false, error: message, text: "" } : m)),
      );
    },
  });

  const clarifyMut = useMutation({
    mutationFn: (item: ReturnType<typeof collectLowConfidence>[number]) =>
      api.copilotClarify({
        ir: workflow!,
        kind: item.kind,
        element_id: item.id,
        evidence: item.evidence,
        confidence: item.confidence,
      }),
  });

  const applyMut = useMutation({
    mutationFn: (args: { patch: unknown[]; label: string }) => {
      if (!workflow) throw new Error("no workflow");
      return api.copilotApply({ ir: workflow, patch: args.patch }).then((r) => ({ ...r, label: args.label, patch: args.patch }));
    },
    onSuccess: (resp) => {
      if (resp.error) {
        toasts.push({
          kind: "error",
          title: "Couldn't apply that suggestion",
          body: humanizeApplyError(resp.error),
        });
        return;
      }
      if (!resp.ir || !workflow) return;
      setPatches((prev) => [
        ...prev,
        { id: ++patchCounter, label: resp.label, patch: resp.patch, previousIR: workflow },
      ]);
      onIRUpdate(resp.ir);
      toasts.push({
        kind: resp.normalized ? "info" : "success",
        title: resp.normalized ? "Applied (auto-repaired)" : "Applied",
        body: resp.normalized
          ? `${resp.label} — Crouton fixed a small patch mistake from the Copilot before applying.`
          : resp.label,
      });
    },
    onError: (err) => {
      const m = err instanceof Error ? err.message : String(err);
      toasts.push({
        kind: "error",
        title: "Couldn't apply that suggestion",
        body: humanizeApplyError(m),
      });
    },
  });

  const submit = () => {
    const trimmed = input.trim();
    if (!trimmed || !workflow) return;
    setInput("");
    if (mode === "ask") {
      askMut.mutate(trimmed);
    }
  };

  const undoPatch = (rec: PatchRecord) => {
    // Walk back to the IR snapshot we captured when the patch landed.
    onIRUpdate(rec.previousIR);
    setPatches((prev) => prev.filter((p) => p.id !== rec.id));
    toasts.push({ kind: "info", title: "Reverted", body: rec.label });
  };

  return (
    <AnimatePresence>
      {open && (
        <motion.aside
          initial={{ x: 360, opacity: 0 }}
          animate={{ x: 0, opacity: 1 }}
          exit={{ x: 360, opacity: 0 }}
          transition={{ type: "spring", stiffness: 320, damping: 32 }}
          className="w-[360px] shrink-0 bg-[var(--color-surface)] border-l border-[var(--color-border)] flex flex-col h-full"
        >
          <header className="px-4 py-3 border-b border-[var(--color-border)] flex items-center justify-between gap-2">
            <div className="flex items-center gap-2 min-w-0">
              <div className="size-6 rounded-md bg-[var(--color-accent-bg)] flex items-center justify-center">
                <Bot size={13} strokeWidth={2} className="text-[var(--color-brand)]" />
              </div>
              <div className="min-w-0">
                <div className="text-[10px] uppercase tracking-[0.14em] text-[var(--color-fg-subtle)]" style={{ fontWeight: 500 }}>
                  Copilot
                </div>
                <div className="text-[13px] text-[var(--color-fg)] truncate" style={{ fontWeight: 400 }}>
                  {mode === "ask" ? "Ask about this workflow" : "Clarify ambiguities"}
                </div>
              </div>
            </div>
            <button
              onClick={onClose}
              className="btn-sm text-[var(--color-fg-subtle)] hover:text-[var(--color-fg)]"
              aria-label="close copilot"
            >
              <X size={14} />
            </button>
          </header>

          {/* Drafting banner — the "missing piece before generation"
              gate. Appears only while the workflow is in drafting
              stage; shows progress + Finalize CTA so the user can
              proceed even with unresolved items. */}
          {stage === "drafting" && interviewProgress && interviewProgress.total > 0 && (
            <div className="px-4 py-3 border-b border-[var(--color-border)] bg-[var(--color-accent-bg)]">
              <div className="flex items-center justify-between text-[10.5px] uppercase tracking-[0.14em] text-[var(--color-brand)]" style={{ fontWeight: 500 }}>
                <span>clarify before generating</span>
                <span className="font-mono tnum">
                  {interviewProgress.resolved}/{interviewProgress.total}
                </span>
              </div>
              <div className="mt-1.5 h-[4px] rounded-full bg-[var(--color-surface)] overflow-hidden">
                <motion.div
                  className="h-full bg-[var(--color-brand)]"
                  initial={false}
                  animate={{
                    width: `${interviewProgress.total > 0 ? (interviewProgress.resolved / interviewProgress.total) * 100 : 0}%`,
                  }}
                  transition={{ type: "spring", stiffness: 180, damping: 24 }}
                />
              </div>
              <p className="mt-2 text-[11.5px] text-[var(--color-fg)] leading-snug" style={{ fontWeight: 400 }}>
                The extractor flagged {interviewProgress.pending}{" "}
                {interviewProgress.pending === 1 ? "element" : "elements"} it wasn't confident about.
                Resolve them below — or{" "}
                <button
                  onClick={onFinalize}
                  disabled={!onFinalize}
                  className="underline underline-offset-2 decoration-dotted text-[var(--color-brand)] hover:decoration-solid disabled:opacity-50"
                >
                  finalize as-is
                </button>{" "}
                and Crouton will proceed with the extractor's best guess.
              </p>
            </div>
          )}

          {/* Mode toggle */}
          <div className="px-3 pt-2 pb-2 border-b border-[var(--color-border)]">
            <nav className="flex gap-0.5 bg-[var(--color-surface-2)] rounded-md p-0.5">
              <ModeButton
                active={mode === "clarify"}
                onClick={() => setMode("clarify")}
                icon={<Sparkles size={11} strokeWidth={2} />}
                label="Clarify"
                count={lowConfidence.length}
              />
              <ModeButton
                active={mode === "ask"}
                onClick={() => setMode("ask")}
                icon={<HelpCircle size={11} strokeWidth={2} />}
                label="Ask"
              />
            </nav>
          </div>

          {/* Accepted patches ribbon */}
          {patches.length > 0 && (
            <div className="px-3 py-2 border-b border-[var(--color-border)] flex flex-wrap gap-1">
              {patches.map((p) => (
                <button
                  key={p.id}
                  onClick={() => undoPatch(p)}
                  title={`Click to undo: ${p.label}`}
                  className="group inline-flex items-center gap-1 text-[10px] px-2 py-0.5 rounded-full border border-emerald-500/30 bg-emerald-500/10 text-emerald-700 dark:text-emerald-400 hover:bg-rose-500/10 hover:border-rose-500/30 hover:text-rose-600 transition-colors"
                >
                  <Undo2 size={10} className="opacity-0 group-hover:opacity-100 -ml-0.5 transition-opacity" />
                  <span className="truncate max-w-[18ch]">{p.label}</span>
                </button>
              ))}
            </div>
          )}

          <div ref={scrollRef} className="flex-1 overflow-y-auto nice-scroll p-3 space-y-3 text-xs">
            {!workflow && (
              <div className="text-[11px] text-[var(--color-fg-subtle)] italic px-2">
                Extract a workflow first — the Copilot works on the current canvas.
              </div>
            )}

            {workflow && mode === "clarify" && (
              <>
                {lowConfidence.length === 0 && (
                  <div className="text-[11px] text-[var(--color-fg-subtle)] italic px-2">
                    No low-confidence elements. Switch to Ask to query the workflow.
                  </div>
                )}
                {lowConfidence.map((item) => {
                  const itemKey = `${item.kind}:${item.id}`;
                  return (
                    <ClarifyCard
                      key={itemKey}
                      item={item}
                      applied={appliedItems.has(itemKey)}
                      onFetchSuggestions={() => clarifyMut.mutateAsync(item)}
                      onApply={(suggestion) => {
                        applyMut.mutate(
                          { patch: suggestion.patch, label: suggestion.label },
                          {
                            // Mark this item done only when the patch
                            // actually lands server-side. Otherwise a
                            // rejected patch leaves the card ready for
                            // another try.
                            onSuccess: (resp) => {
                              if (!resp.error) {
                                setAppliedItems((prev) => {
                                  const next = new Set(prev);
                                  next.add(itemKey);
                                  return next;
                                });
                                onItemResolved?.(itemKey);
                              }
                            },
                          },
                        );
                      }}
                      onKeepAsIs={() => {
                        // User dismisses the question without a patch
                        // (the extractor's guess stays). Still counts
                        // toward progress so the draft can advance.
                        setAppliedItems((prev) => {
                          const next = new Set(prev);
                          next.add(itemKey);
                          return next;
                        });
                        onItemResolved?.(itemKey);
                      }}
                      onReopen={() => {
                        setAppliedItems((prev) => {
                          const next = new Set(prev);
                          next.delete(itemKey);
                          return next;
                        });
                        onItemReopened?.(itemKey);
                      }}
                      onHighlight={(id) => {
                        if (item.kind === "task" && onHighlightTask) onHighlightTask(id);
                      }}
                    />
                  );
                })}
              </>
            )}

            {workflow && mode === "ask" && (
              <>
                {messages.map((m) => (
                  <ChatBubble key={m.id} message={m} />
                ))}
              </>
            )}
          </div>

          {mode === "ask" && (
            <form
              className="border-t border-[var(--color-border)] p-3"
              onSubmit={(e) => {
                e.preventDefault();
                submit();
              }}
            >
              <div className="flex items-end gap-2">
                <textarea
                  value={input}
                  onChange={(e) => setInput(e.target.value)}
                  onKeyDown={(e) => {
                    if ((e.metaKey || e.ctrlKey) && e.key === "Enter") {
                      e.preventDefault();
                      submit();
                    }
                  }}
                  rows={1}
                  placeholder="Ask about actors, bindings, conditions…"
                  disabled={!workflow || askMut.isPending}
                  className="flex-1 min-w-0 resize-none bg-[var(--color-surface-2)] border border-[var(--color-border)] rounded-md px-2.5 py-1.5 text-[13px] text-[var(--color-fg)] placeholder:text-[var(--color-fg-subtle)] focus:outline-none focus:border-[var(--color-brand)] focus:bg-[var(--color-surface)] disabled:opacity-60"
                  style={{ fontWeight: 300, maxHeight: "140px" }}
                />
                <button
                  type="submit"
                  disabled={!workflow || !input.trim() || askMut.isPending}
                  className="btn-sm shrink-0 size-8 inline-flex items-center justify-center rounded-md bg-[var(--color-brand)] text-white hover:brightness-110 disabled:opacity-40"
                  aria-label="Send"
                >
                  {askMut.isPending ? <Loader2 size={14} className="animate-spin" /> : <ArrowRight size={14} />}
                </button>
              </div>
              <div className="mt-1 text-[10px] font-mono text-[var(--color-fg-subtle)]">⌘↵ to send</div>
            </form>
          )}
        </motion.aside>
      )}
    </AnimatePresence>
  );
}

function ModeButton({
  active,
  onClick,
  icon,
  label,
  count,
}: {
  active: boolean;
  onClick: () => void;
  icon: React.ReactNode;
  label: string;
  count?: number;
}) {
  return (
    <button
      onClick={onClick}
      className={`flex-1 flex items-center justify-center gap-1.5 rounded px-2 py-1 text-[11px] transition-colors ${
        active
          ? "bg-[var(--color-surface)] text-[var(--color-fg)] shadow-stripe-ambient"
          : "text-[var(--color-fg-muted)] hover:text-[var(--color-fg)]"
      }`}
      style={{ fontWeight: 400 }}
    >
      {icon}
      <span>{label}</span>
      {count !== undefined && count > 0 && (
        <span className="font-mono text-[10px] text-[var(--color-fg-subtle)] tnum">{count}</span>
      )}
    </button>
  );
}

function ChatBubble({ message }: { message: ChatMessage }) {
  if (message.role === "user") {
    return (
      <div className="flex justify-end">
        <div className="max-w-[85%] rounded-lg px-2.5 py-1.5 bg-[var(--color-brand)] text-white text-[12px]">
          {message.text}
        </div>
      </div>
    );
  }
  if (message.busy) {
    return (
      <div className="flex items-center gap-2 text-[var(--color-fg-subtle)]">
        <Loader2 size={12} className="animate-spin" />
        <span className="text-[11px] italic">thinking…</span>
      </div>
    );
  }
  if (message.error) {
    return (
      <div className="rounded-lg border border-rose-500/30 bg-rose-500/10 px-2.5 py-1.5 text-[12px] text-rose-600 dark:text-rose-300">
        {message.error}
      </div>
    );
  }
  return (
    <div className="rounded-lg border border-[var(--color-border)] bg-[var(--color-surface-2)] px-2.5 py-2 text-[12px] text-[var(--color-fg)] leading-snug space-y-1.5">
      <p>{message.text}</p>
      {message.evidence && message.evidence.length > 0 && (
        <div className="flex flex-wrap gap-1 pt-1">
          {message.evidence.map((e, i) =>
            e.ir_ref || e.quote ? (
              <span
                key={i}
                className="inline-flex items-center gap-1 text-[10px] px-1.5 py-0.5 rounded border border-[var(--color-border)] bg-[var(--color-surface)] text-[var(--color-fg-muted)] font-mono"
                title={e.quote}
              >
                {e.ir_ref ?? "?"}
                {e.quote && <span className="italic text-[var(--color-fg-subtle)]">“{e.quote.slice(0, 40)}{e.quote.length > 40 ? "…" : ""}”</span>}
              </span>
            ) : null,
          )}
        </div>
      )}
    </div>
  );
}

function ClarifyCard({
  item,
  applied,
  onFetchSuggestions,
  onApply,
  onKeepAsIs,
  onReopen,
  onHighlight,
}: {
  item: ReturnType<typeof collectLowConfidence>[number];
  applied: boolean;
  onFetchSuggestions: () => Promise<{
    suggestions: { label: string; rationale?: string; patch: unknown[] }[];
    error?: string;
  }>;
  onApply: (s: { label: string; patch: unknown[] }) => void;
  onKeepAsIs: () => void;
  onReopen: () => void;
  onHighlight: (id: string | null) => void;
}) {
  const [opened, setOpened] = useState(false);
  const [loading, setLoading] = useState(false);
  const [suggestions, setSuggestions] = useState<
    { label: string; rationale?: string; patch: unknown[] }[]
  >([]);
  const [error, setError] = useState<string | null>(null);

  // Collapse the expansion automatically once the patch lands. The
  // user has acknowledged the item; keeping the suggestions expanded
  // would only add clutter.
  useEffect(() => {
    if (applied) setOpened(false);
  }, [applied]);

  const openAndFetch = async () => {
    if (opened) {
      setOpened(false);
      return;
    }
    setOpened(true);
    if (suggestions.length === 0 && !loading) {
      setLoading(true);
      try {
        const resp = await onFetchSuggestions();
        setSuggestions(resp.suggestions);
        setError(resp.error ?? null);
      } catch (e) {
        setError(e instanceof Error ? e.message : String(e));
      } finally {
        setLoading(false);
      }
    }
  };

  const pct = Math.round(item.confidence * 100);

  // Applied rows collapse to a single-line acknowledgement with a
  // "Reopen" affordance so the user can un-dismiss and try a
  // different suggestion if they picked the wrong one.
  if (applied) {
    return (
      <article
        className="rounded-md border border-emerald-500/30 bg-emerald-500/5 px-3 py-2 flex items-center gap-2"
        onMouseEnter={() => onHighlight(item.id)}
        onMouseLeave={() => onHighlight(null)}
      >
        <Check size={12} className="text-emerald-500 shrink-0" />
        <div className="flex-1 min-w-0">
          <div
            className="text-[10px] uppercase tracking-[0.12em] text-[var(--color-fg-subtle)]"
            style={{ fontWeight: 500 }}
          >
            {item.kind} · resolved
          </div>
          <div className="text-[12px] text-[var(--color-fg)] truncate" style={{ fontWeight: 400 }}>
            {item.label}
          </div>
        </div>
        <button
          onClick={onReopen}
          className="text-[10px] font-mono text-[var(--color-fg-muted)] hover:text-[var(--color-brand)] transition-colors shrink-0"
          style={{ fontWeight: 500 }}
        >
          reopen
        </button>
      </article>
    );
  }

  return (
    <article
      className="rounded-md border border-[var(--color-border)] bg-[var(--color-surface)] overflow-hidden"
      onMouseEnter={() => onHighlight(item.id)}
      onMouseLeave={() => onHighlight(null)}
    >
      <button
        onClick={openAndFetch}
        className="w-full text-left px-3 py-2 flex items-start gap-2 hover:bg-[var(--color-surface-2)] transition-colors"
      >
        <span className={`mt-1 size-[6px] rounded-full ${TIER_COLOR[item.tier]}`} />
        <div className="flex-1 min-w-0">
          <div className="flex items-center justify-between gap-2">
            <span className="text-[10px] uppercase tracking-[0.12em] text-[var(--color-fg-subtle)]">{item.kind}</span>
            <span className={`text-[10px] font-mono tnum ${TIER_TEXT[item.tier]}`}>{pct}%</span>
          </div>
          <div className="text-[13px] text-[var(--color-fg)] truncate" style={{ fontWeight: 400 }}>
            {item.label}
          </div>
          {item.evidence && (
            <div className="text-[11px] text-[var(--color-fg-muted)] italic truncate mt-0.5">
              “{item.evidence}”
            </div>
          )}
        </div>
      </button>
      <AnimatePresence>
        {opened && (
          <motion.div
            initial={{ height: 0, opacity: 0 }}
            animate={{ height: "auto", opacity: 1 }}
            exit={{ height: 0, opacity: 0 }}
            className="border-t border-[var(--color-border)] bg-[var(--color-surface-2)]"
          >
            <div className="p-2 space-y-1.5">
              {loading && (
                <div className="flex items-center gap-2 text-[11px] text-[var(--color-fg-subtle)]">
                  <Loader2 size={11} className="animate-spin" /> thinking…
                </div>
              )}
              {error && (
                <div className="text-[11px] text-rose-600 dark:text-rose-400">{error}</div>
              )}
              {!loading && suggestions.length === 0 && !error && (
                <div className="text-[11px] text-[var(--color-fg-subtle)] italic">No suggestions available.</div>
              )}
              {suggestions.map((s, i) => (
                <button
                  key={i}
                  onClick={() => onApply(s)}
                  disabled={!s.patch || s.patch.length === 0}
                  className="w-full text-left rounded border border-[var(--color-border)] bg-[var(--color-surface)] px-2 py-1.5 hover:border-[var(--color-brand)] hover:bg-[var(--color-accent-bg)] transition-colors disabled:opacity-50 disabled:hover:border-[var(--color-border)] disabled:hover:bg-[var(--color-surface)]"
                >
                  <div className="text-[12px] text-[var(--color-fg)] flex items-center gap-1.5" style={{ fontWeight: 400 }}>
                    <ArrowRight size={11} className="text-[var(--color-brand)]" />
                    <span className="flex-1">{s.label}</span>
                  </div>
                  {s.rationale && (
                    <div className="mt-0.5 text-[11px] text-[var(--color-fg-muted)]">{s.rationale}</div>
                  )}
                </button>
              ))}
              {!loading && (
                <button
                  onClick={onKeepAsIs}
                  className="w-full text-left text-[11px] text-[var(--color-fg-muted)] italic px-2 py-1 hover:text-[var(--color-fg)] transition-colors"
                >
                  ↳ keep the extractor's guess as-is
                </button>
              )}
            </div>
          </motion.div>
        )}
      </AnimatePresence>
    </article>
  );
}

// humanizeApplyError translates the agent/validator's raw
// JSON-Pointer / JSON-Patch language into a message the user can
// act on. We keep the original at the end so it stays debuggable.
function humanizeApplyError(raw: string): string {
  const msg = raw.toLowerCase();
  if (msg.includes("can't replace a non-existent")) {
    return "The suggestion targeted a field that isn't set yet. Try the Clarify suggestion again — Crouton now auto-corrects this kind of mistake.";
  }
  if (msg.includes("patched_ir_schema_violation")) {
    return "The suggestion would leave the workflow in an invalid shape. Pick a different Clarify option or refine the text in the composer.";
  }
  if (msg.includes("patched_ir_invalid_json")) {
    return "Crouton couldn't parse the Copilot's patched workflow. Try re-running Clarify for this item.";
  }
  if (msg.includes("unresolved_references") || msg.includes("not in the is registry")) {
    return "The suggestion used an id that isn't in your Information System. Connect the relevant engine or declare the system first.";
  }
  // Fallback — show the raw message but strip any stack-trace noise.
  return (raw.split("\n")[0] ?? raw).slice(0, 200);
}
