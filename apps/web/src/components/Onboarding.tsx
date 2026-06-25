import { useState } from "react";
import { motion } from "framer-motion";
import { ArrowRight, CheckCircle2, Circle, Database, FileText, Plug, Sparkles, X } from "lucide-react";

import type { ISRegistry } from "../lib/types";
import { BrandMark } from "./Brand";
import { ConnectEngineForm } from "./ConnectEngineForm";
import { DeclareSystemForm } from "./DeclareSystemForm";

interface Props {
  is?: ISRegistry;
  onDismiss: () => void;
  onRefresh: () => void;
  onLoadSample: () => void;
}

export function Onboarding({ is, onDismiss, onRefresh, onLoadSample }: Props) {
  const [step, setStep] = useState<"engine" | "system" | "sample" | null>(null);

  const hasEngine = (is?.engine_connections?.length ?? 0) > 0;
  const hasSystem = (is?.systems?.length ?? 0) > 0;
  const steps = [
    { id: 1, label: "Connect a workflow engine", done: hasEngine, action: () => setStep("engine") },
    { id: 2, label: "Declare the systems you talk to", done: hasSystem, action: () => setStep("system") },
    { id: 3, label: "Describe a process and watch it compile", done: false, action: onLoadSample },
  ];
  const progress = steps.filter((s) => s.done).length;

  return (
    <motion.div
      initial={{ opacity: 0 }}
      animate={{ opacity: 1 }}
      exit={{ opacity: 0 }}
      transition={{ duration: 0.15 }}
      className="fixed inset-0 z-40 bg-black/40 backdrop-blur-sm flex items-center justify-center p-6"
    >
      <motion.div
        initial={{ opacity: 0, y: 16, scale: 0.97 }}
        animate={{ opacity: 1, y: 0, scale: 1 }}
        exit={{ opacity: 0, y: 8, scale: 0.98 }}
        transition={{ type: "spring", stiffness: 320, damping: 28 }}
        className="relative max-w-xl w-full bg-[var(--color-surface)] border border-[var(--color-border)] rounded-xl shadow-stripe-deep overflow-hidden">
        <button
          onClick={onDismiss}
          className="absolute top-4 right-4 text-[var(--color-fg-subtle)] hover:text-[var(--color-fg)]"
          aria-label="Skip onboarding"
        >
          <X size={16} />
        </button>

        <div className="px-7 pt-7 pb-5 border-b border-[var(--color-border)]">
          <div className="flex items-center gap-3 mb-3">
            <BrandMark size={32} />
            <div>
              <div className="text-[10px] uppercase tracking-widest text-[var(--color-fg-subtle)]">
                Welcome to
              </div>
              <div className="text-xl font-semibold text-[var(--color-fg)] flex items-center gap-2">
                Pablo
                <span className="text-xs font-normal px-1.5 py-0.5 rounded bg-[var(--color-accent-bg)] text-[var(--color-accent)]">
                  workspace
                </span>
              </div>
            </div>
          </div>
          <p className="text-sm text-[var(--color-fg-muted)] leading-relaxed">
            Pablo sits <span className="text-[var(--color-fg)] font-medium">above your existing engine</span>{" "}
            — it never creates users or replaces anything. Describe a process in plain language;
            Pablo grounds it in your IS and compiles a deployable workflow your engine runs.
          </p>
        </div>

        <div className="px-7 py-5 space-y-3">
          <div className="flex items-center justify-between">
            <div className="text-[10px] uppercase tracking-widest text-[var(--color-fg-subtle)]">
              Get started
            </div>
            <div className="text-[11px] font-mono text-[var(--color-fg-subtle)]">
              {progress} / {steps.length}
            </div>
          </div>

          <ol className="space-y-2">
            {steps.map((s) => (
              <li key={s.id}>
                <button
                  onClick={s.action}
                  className="w-full flex items-center gap-3 text-left px-3 py-2.5 rounded-lg border border-[var(--color-border)] hover:border-[var(--color-accent-border)] hover:bg-[var(--color-accent-bg)] transition-colors group"
                >
                  {s.done ? (
                    <CheckCircle2 size={18} className="text-emerald-500 shrink-0" />
                  ) : (
                    <Circle size={18} className="text-[var(--color-fg-subtle)] shrink-0" />
                  )}
                  <div className="flex-1 min-w-0">
                    <div className={`text-sm ${s.done ? "text-[var(--color-fg-muted)] line-through" : "text-[var(--color-fg)] font-medium"}`}>
                      {s.label}
                    </div>
                  </div>
                  <ArrowRight
                    size={15}
                    className="text-[var(--color-fg-subtle)] group-hover:text-[var(--color-accent)] group-hover:translate-x-0.5 transition-all"
                  />
                </button>
              </li>
            ))}
          </ol>

          {step === "engine" && (
            <div className="pt-2">
              <ConnectEngineForm
                onClose={() => setStep(null)}
                onConnected={() => {
                  setStep(null);
                  onRefresh();
                }}
              />
            </div>
          )}
          {step === "system" && (
            <div className="pt-2">
              <DeclareSystemForm
                onClose={() => setStep(null)}
                onDeclared={() => {
                  setStep(null);
                  onRefresh();
                }}
              />
            </div>
          )}
        </div>

        <div className="px-7 py-4 bg-[var(--color-surface-2)] border-t border-[var(--color-border)] flex items-center justify-between">
          <div className="flex items-center gap-2 text-xs text-[var(--color-fg-muted)]">
            <Sparkles size={13} className="text-[var(--color-accent)]" />
            Grounded in your IS · Deploys to Camunda 7
          </div>
          <button
            onClick={onDismiss}
            className="text-xs font-medium text-[var(--color-fg-muted)] hover:text-[var(--color-fg)]"
          >
            Skip for now
          </button>
        </div>
      </motion.div>
    </motion.div>
  );
}

// Compact inline checklist for the top of the workspace after dismissal.
interface InlineChecklistProps {
  is?: ISRegistry;
  onStep: (s: "engine" | "system" | "sample") => void;
}

export function InlineChecklist({ is, onStep }: InlineChecklistProps) {
  const hasEngine = (is?.engine_connections?.length ?? 0) > 0;
  const hasSystem = (is?.systems?.length ?? 0) > 0;
  if (hasEngine && hasSystem) return null;

  return (
    <div className="mx-4 mt-3 border border-[var(--color-accent-border)] bg-[var(--color-accent-bg)] rounded-lg px-3 py-2 flex items-center gap-3 text-xs">
      <Sparkles size={14} className="text-[var(--color-accent)] shrink-0" />
      <span className="text-[var(--color-fg)] font-medium">Finish setup:</span>
      {!hasEngine && (
        <button
          onClick={() => onStep("engine")}
          className="flex items-center gap-1 text-[var(--color-accent)] hover:underline"
        >
          <Plug size={12} /> connect engine
        </button>
      )}
      {!hasSystem && (
        <button
          onClick={() => onStep("system")}
          className="flex items-center gap-1 text-[var(--color-accent)] hover:underline"
        >
          <Database size={12} /> declare a system
        </button>
      )}
      <button
        onClick={() => onStep("sample")}
        className="flex items-center gap-1 text-[var(--color-accent)] hover:underline ml-auto"
      >
        <FileText size={12} /> try sample
      </button>
    </div>
  );
}
