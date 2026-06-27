import { AnimatePresence, motion } from "framer-motion";
import {
  X,
  FileText,
  Compass,
  Users,
  ShieldCheck,
  Hammer,
  ClipboardList,
  ArrowRight,
  type LucideIcon,
} from "lucide-react";

import type { ShellSection } from "./ShellRail";

// HowItWorks — an always-available explainer of the request pipeline and,
// crucially for operators, what *they* do at each step. Opened from the Home
// header and auto-shown once on first visit.

interface Props {
  open: boolean;
  onClose: () => void;
  onNavigate: (section: ShellSection) => void;
}

type Actor = "you" | "agents" | "auto";

interface Stage {
  icon: LucideIcon;
  title: string;
  actor: Actor;
  actorLabel: string;
  desc: string;
}

const STAGES: Stage[] = [
  {
    icon: FileText,
    title: "Submit a request",
    actor: "you",
    actorLabel: "You",
    desc: "Describe what you need — open an office, buy laptops, change a policy — and set a priority.",
  },
  {
    icon: Compass,
    title: "Intake & planning",
    actor: "agents",
    actorLabel: "Agent",
    desc: "An intake agent reads the request and plans which departments must weigh in, and in what order.",
  },
  {
    icon: Users,
    title: "Department reviews",
    actor: "agents",
    actorLabel: "Agents · in parallel",
    desc: "Finance, Legal and IT review at once — budget, compliance, feasibility. One can wait on another (e.g. Finance needs IT's cost estimate first).",
  },
  {
    icon: ShieldCheck,
    title: "Executive approval",
    actor: "you",
    actorLabel: "You",
    desc: "Once reviews clear, the request waits for your decision. Approve or reject with a written reason — from My Work.",
  },
  {
    icon: Hammer,
    title: "Execution",
    actor: "agents",
    actorLabel: "Agents · in parallel",
    desc: "After approval, HR and Operations plan the work, then Implementation carries it out.",
  },
  {
    icon: ClipboardList,
    title: "Report",
    actor: "auto",
    actorLabel: "Automatic",
    desc: "A final report summarizes what was decided, what was flagged, and what was done. Every step is in the audit trail.",
  },
];

const ACTOR_STYLE: Record<Actor, string> = {
  you: "bg-[var(--color-accent-bg)] text-[var(--color-brand)]",
  agents: "bg-[var(--color-surface-2)] text-[var(--color-fg-muted)]",
  auto: "bg-[var(--color-success)]/12 text-[var(--color-success)]",
};

const WHERE: { section: ShellSection; label: string; desc: string }[] = [
  { section: "requests", label: "Requests", desc: "submit & track" },
  { section: "workflows", label: "Workflows", desc: "watch it run live" },
  { section: "my-work", label: "My Work", desc: "approve at the gate" },
  { section: "reports", label: "Reports", desc: "read the outcome" },
];

export function HowItWorks({ open, onClose, onNavigate }: Props) {
  return (
    <AnimatePresence>
      {open && (
        <motion.div
          initial={{ opacity: 0 }}
          animate={{ opacity: 1 }}
          exit={{ opacity: 0 }}
          transition={{ duration: 0.12 }}
          onMouseDown={onClose}
          className="fixed inset-0 z-50 bg-black/45 backdrop-blur-sm flex items-start justify-center px-4 py-8 overflow-auto"
        >
          <motion.div
            initial={{ y: 14, opacity: 0, scale: 0.985 }}
            animate={{ y: 0, opacity: 1, scale: 1 }}
            exit={{ y: 8, opacity: 0 }}
            transition={{ type: "spring", stiffness: 320, damping: 30 }}
            onMouseDown={(e) => e.stopPropagation()}
            className="w-full max-w-[720px] bg-[var(--color-surface)] border border-[var(--color-border)] rounded-xl shadow-stripe-deep overflow-hidden"
          >
            <header className="flex items-center justify-between px-6 h-14 border-b border-[var(--color-border)]">
              <div>
                <div className="text-[10px] uppercase tracking-[0.14em] text-[var(--color-fg-subtle)] font-medium">
                  AI Organization OS
                </div>
                <div className="text-sm font-medium text-[var(--color-fg)]">How it works</div>
              </div>
              <button
                onClick={onClose}
                aria-label="Close"
                className="size-7 flex items-center justify-center rounded-md text-[var(--color-fg-muted)] hover:bg-[var(--color-surface-2)] hover:text-[var(--color-fg)]"
              >
                <X size={15} />
              </button>
            </header>

            <div className="px-6 py-5">
              <p className="text-sm text-[var(--color-fg-muted)] leading-relaxed max-w-[60ch]">
                You submit a business request; a team of department agents reviews it, an executive
                approves it, and it gets executed — with every decision logged. Here's the flow and
                what you do at each step.
              </p>

              {/* Pipeline */}
              <ol className="mt-5 relative flex flex-col gap-3 before:absolute before:left-[18px] before:top-3 before:bottom-3 before:w-px before:bg-[var(--color-border)]">
                {STAGES.map((s, i) => {
                  const Icon = s.icon;
                  return (
                    <li key={s.title} className="relative flex gap-3.5">
                      <span className="relative z-10 size-9 shrink-0 rounded-lg bg-[var(--color-surface)] border border-[var(--color-border)] flex items-center justify-center">
                        <Icon size={16} className="text-[var(--color-brand)]" strokeWidth={1.75} />
                      </span>
                      <div className="min-w-0 pt-0.5">
                        <div className="flex items-center gap-2 flex-wrap">
                          <span className="text-[10px] tnum text-[var(--color-fg-subtle)]">{i + 1}</span>
                          <h3 className="text-sm font-medium text-[var(--color-fg)]">{s.title}</h3>
                          <span className={`rounded-full px-2 py-0.5 text-[10px] font-medium ${ACTOR_STYLE[s.actor]}`}>
                            {s.actorLabel}
                          </span>
                        </div>
                        <p className="text-xs text-[var(--color-fg-muted)] leading-snug mt-0.5 max-w-[58ch]">
                          {s.desc}
                        </p>
                      </div>
                    </li>
                  );
                })}
              </ol>

              {/* Where you act */}
              <div className="mt-6 rounded-lg border border-[var(--color-border)] bg-[var(--color-surface-2)] p-3">
                <p className="text-[10px] uppercase tracking-wide text-[var(--color-fg-muted)] mb-2 px-1">
                  Where you do each part
                </p>
                <div className="grid grid-cols-2 sm:grid-cols-4 gap-2">
                  {WHERE.map((w) => (
                    <button
                      key={w.section}
                      onClick={() => {
                        onNavigate(w.section);
                        onClose();
                      }}
                      className="group rounded-md bg-[var(--color-surface)] border border-[var(--color-border)] px-2.5 py-2 text-left transition-colors hover:border-[var(--color-border-strong)]"
                    >
                      <span className="block text-xs font-medium text-[var(--color-fg)] group-hover:text-[var(--color-brand)]">
                        {w.label}
                      </span>
                      <span className="block text-[11px] text-[var(--color-fg-muted)]">{w.desc}</span>
                    </button>
                  ))}
                </div>
              </div>
            </div>

            <footer className="flex items-center justify-end gap-2 px-6 py-4 border-t border-[var(--color-border)]">
              <button
                onClick={onClose}
                className="rounded-md px-3 py-2 text-sm text-[var(--color-fg-muted)] hover:bg-[var(--color-surface-2)] transition-colors"
              >
                Got it
              </button>
              <button
                onClick={() => {
                  onNavigate("requests");
                  onClose();
                }}
                className="flex items-center gap-1.5 rounded-md bg-[var(--color-brand)] px-3 py-2 text-sm font-medium text-white transition-colors hover:bg-[var(--color-brand-hover)]"
              >
                Submit a request <ArrowRight size={14} />
              </button>
            </footer>
          </motion.div>
        </motion.div>
      )}
    </AnimatePresence>
  );
}
