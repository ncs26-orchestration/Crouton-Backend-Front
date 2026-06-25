import { ChevronDown, Download, Loader2, Rocket, Workflow } from "lucide-react";

import type { EngineAdapter, Workflow as WorkflowType } from "../lib/types";
import { PipelineChips, type PipelineStage } from "./PipelineChips";

type Status =
  | "empty"
  | "draft"
  | "ready"
  | "compiling"
  | "compiled"
  | "deploying"
  | "deployed"
  | "error";

interface Props {
  workflow: WorkflowType | null;
  status: Status;
  target: string;
  adapters: EngineAdapter[];
  onTargetChange: (t: string) => void;
  onCompile: () => void;
  onDeploy?: () => void;
  deployBusy?: boolean;
  compileDisabled: boolean;
  compileDisabledReason?: string;
  lastCompiledBytes?: number;
  taskCount?: number;
  boundCount?: number;
  // Drafting gate — when the extractor flagged ambiguities, Compile
  // is disabled and the TopBar surfaces a banner with a jump-to-
  // Copilot action so the user sees why.
  extractionStage?: "empty" | "drafting" | "ready";
  interviewProgress?: { resolved: number; pending: number; total: number };
  onOpenCopilot?: () => void;
}

// Map compile/deploy status to the pipeline stages we visualize.
// "process"    — the workflow has been extracted but not compiled
// "executable" — compiled, artifact downloaded, awaiting (or past) deploy
// "deployed"   — live in a running engine
function stageForStatus(status: Status): { active: PipelineStage; furthest: PipelineStage } {
  switch (status) {
    case "empty":
    case "draft":
    case "ready":
    case "error":
      return { active: "process", furthest: "process" };
    case "compiling":
    case "compiled":
      return { active: "executable", furthest: "executable" };
    case "deploying":
    case "deployed":
      return { active: "deployed", furthest: "deployed" };
  }
}

const STATUS_TONE: Record<Status, { ring: string; dot: string; label: string; text: string }> = {
  empty:     { ring: "border-[var(--color-border)]",     dot: "bg-[var(--color-fg-subtle)]", label: "EMPTY",       text: "text-[var(--color-fg-subtle)]" },
  draft:     { ring: "border-[var(--color-border)]",     dot: "bg-[var(--color-fg-muted)]",  label: "DRAFT",       text: "text-[var(--color-fg-muted)]" },
  ready:     { ring: "border-emerald-500/30",            dot: "bg-emerald-500",              label: "READY",       text: "text-emerald-600 dark:text-emerald-400" },
  compiling: { ring: "border-[var(--color-accent-border)]", dot: "bg-[var(--color-accent)] animate-pulse", label: "COMPILING", text: "text-[var(--color-accent)]" },
  compiled:  { ring: "border-emerald-500/30",            dot: "bg-emerald-500",              label: "COMPILED",    text: "text-emerald-600 dark:text-emerald-400" },
  deploying: { ring: "border-[var(--color-accent-border)]", dot: "bg-[var(--color-accent)] animate-pulse", label: "DEPLOYING", text: "text-[var(--color-accent)]" },
  deployed:  { ring: "border-emerald-500/50",            dot: "bg-emerald-500",              label: "DEPLOYED",    text: "text-emerald-600 dark:text-emerald-400" },
  error:     { ring: "border-rose-500/40",               dot: "bg-rose-500",                 label: "ERROR",       text: "text-rose-600 dark:text-rose-400" },
};

export function TopBar({
  workflow,
  status,
  target,
  adapters,
  onTargetChange,
  onCompile,
  onDeploy,
  deployBusy,
  compileDisabled,
  compileDisabledReason,
  lastCompiledBytes,
  taskCount,
  boundCount,
  extractionStage,
  interviewProgress,
  onOpenCopilot,
}: Props) {
  // Derive the current adapter's capabilities so Deploy is disabled
  // when the selected target doesn't support it (Elsa 3 today).
  const currentAdapter = adapters.find((a) => a.kind === target);
  const canDeploy = currentAdapter?.capabilities.can_deploy ?? false;
  // Fallback to the static list when the registry hasn't loaded yet
  // (first render, before listAdapters() resolves).
  const selectorAdapters = adapters.length > 0 ? adapters : [{ kind: "camunda7", name: "Camunda 7" } as EngineAdapter];
  const tone = STATUS_TONE[status];
  const { active: pipelineActive, furthest: pipelineFurthest } = stageForStatus(status);
  return (
    <header className="h-14 shrink-0 bg-[var(--color-surface)] border-b border-[var(--color-border)] flex items-center px-5 gap-4">
      <div className="flex items-center gap-2 min-w-0">
        <span className="text-[10px] uppercase tracking-widest text-[var(--color-fg-subtle)]">
          Process
        </span>
        <div className="flex items-center gap-2 min-w-0">
          <Workflow size={14} className="text-[var(--color-fg-muted)] shrink-0" />
          <span className="text-sm font-medium text-[var(--color-fg)] truncate max-w-[22ch]">
            {workflow?.metadata.name ?? "untitled"}
          </span>
        </div>

        {/* Drafting banner replaces the static status chip while the
            workflow has open questions. Clicking it jumps to the
            Copilot — the sidebar's where the interview happens. */}
        {extractionStage === "drafting" && interviewProgress ? (
          <button
            onClick={onOpenCopilot}
            className="group flex items-center gap-2 text-[10.5px] font-mono px-2 py-0.5 border rounded-full border-[var(--color-accent-border)] bg-[var(--color-accent-bg)] text-[var(--color-brand)] hover:brightness-110 transition-all"
            style={{ fontWeight: 500 }}
            title="Open Copilot to resolve the remaining questions"
          >
            <span className="size-1.5 rounded-full bg-[var(--color-brand)] animate-pulse" />
            DRAFTING · {interviewProgress.resolved}/{interviewProgress.total}
            <span className="group-hover:text-[var(--color-fg)] text-[var(--color-fg-subtle)]">→ resolve</span>
          </button>
        ) : (
          <span
            className={`flex items-center gap-1.5 text-[10px] font-mono px-2 py-0.5 border rounded-full ${tone.ring} ${tone.text}`}
          >
            <span className={`size-1.5 rounded-full ${tone.dot}`} />
            {tone.label}
          </span>
        )}

        {taskCount !== undefined && taskCount > 0 && (
          <span className="text-[11px] text-[var(--color-fg-muted)] font-mono">
            {boundCount ?? 0}/{taskCount} bound
          </span>
        )}
      </div>

      {/* Pipeline chips sit in the middle — they're the visible face
          of the Process → Executable → Deployed compilation pipeline. */}
      {workflow && (
        <div className="hidden md:flex items-center ml-2">
          <PipelineChips active={pipelineActive} furthest={pipelineFurthest} />
        </div>
      )}

      <div className="flex-1" />

      {lastCompiledBytes ? (
        <span className="text-[11px] font-mono text-[var(--color-fg-subtle)]">
          {lastCompiledBytes.toLocaleString()} bytes
        </span>
      ) : null}

      <label className="flex items-center gap-1.5 text-xs text-[var(--color-fg-muted)]">
        <span className="text-[10px] uppercase tracking-widest">target</span>
        <div className="relative">
          <select
            value={target}
            onChange={(e) => onTargetChange(e.target.value)}
            className="appearance-none bg-[var(--color-surface)] border border-[var(--color-border)] rounded-md pl-2.5 pr-7 py-1 text-xs text-[var(--color-fg)] hover:border-[var(--color-border-strong)] focus:outline-none focus:ring-2 focus:ring-[var(--color-accent-border)]"
          >
            {selectorAdapters.map((a) => (
              <option key={a.kind} value={a.kind}>
                {a.name}
              </option>
            ))}
          </select>
          <ChevronDown size={12} className="absolute right-2 top-1/2 -translate-y-1/2 text-[var(--color-fg-subtle)] pointer-events-none" />
        </div>
      </label>

      <button
        onClick={onCompile}
        disabled={compileDisabled}
        title={compileDisabledReason ?? ""}
        className="flex items-center gap-1.5 text-xs font-medium px-3 py-1.5 rounded-md border border-[var(--color-border)] bg-[var(--color-surface)] text-[var(--color-fg)] hover:bg-[var(--color-surface-2)] disabled:opacity-40 disabled:cursor-not-allowed"
      >
        <Download size={13} />
        Compile
      </button>

      {onDeploy && (
        <button
          onClick={onDeploy}
          disabled={compileDisabled || deployBusy || !canDeploy}
          title={
            compileDisabledReason ??
            (canDeploy ? "" : `Deploy is not supported by the ${currentAdapter?.name ?? "selected"} adapter yet — use Compile to download the artifact.`)
          }
          className="relative flex items-center gap-1.5 text-xs font-medium px-3 py-1.5 rounded-md text-white bg-[var(--color-accent)] hover:brightness-110 disabled:opacity-40 disabled:cursor-not-allowed shadow-sm shadow-indigo-900/20"
        >
          {deployBusy ? <Loader2 size={13} className="animate-spin" /> : <Rocket size={13} />}
          {deployBusy ? "Deploying…" : "Deploy"}
        </button>
      )}
    </header>
  );
}
