import { AnimatePresence, motion } from "framer-motion";
import {
  HelpCircle,
  X,
  Home,
  Inbox,
  FileText,
  Workflow,
  Bot,
  BarChart3,
  ScrollText,
  Link2,
  Users,
  type LucideIcon,
} from "lucide-react";

// HelpOverlay — a short guide to the app. Most of the old keyboard-shortcut
// list referred to a different product; this is an honest walkthrough of the
// sections plus the handful of interactions that exist.

interface Props {
  open: boolean;
  onClose: () => void;
  onHowItWorks?: () => void;
}

const SECTIONS: { icon: LucideIcon; label: string; desc: string }[] = [
  { icon: Home, label: "Home", desc: "Org dashboard: stats, recent requests, live activity." },
  { icon: Inbox, label: "My Work", desc: "Your approvals, work in flight, and recent decisions." },
  { icon: FileText, label: "Requests", desc: "Submit a new request and track every one in the org." },
  { icon: Workflow, label: "Workflows", desc: "The live canvas. Click a node for its tasks and activity." },
  { icon: Bot, label: "Agents", desc: "The department agents and what they're working on." },
  { icon: BarChart3, label: "Reports", desc: "Completed reports (printable) and the full audit trail." },
  { icon: ScrollText, label: "Policies", desc: "The rules each department agent checks against." },
  { icon: Link2, label: "Integrations", desc: "The systems and data sources agents draw on." },
  { icon: Users, label: "Teams", desc: "Departments and their members." },
];

const TIPS: string[] = [
  "Click any node on the workflow canvas to open its details, tasks, and activity.",
  "Open a completed report from Reports to read and print or export it as PDF.",
  "Collapse the sidebar with the toggle at the top to give the canvas more room.",
  "Press Esc to close any dialog.",
];

export function HelpOverlay({ open, onClose, onHowItWorks }: Props) {
  return (
    <AnimatePresence>
      {open && (
        <motion.div
          initial={{ opacity: 0 }}
          animate={{ opacity: 1 }}
          exit={{ opacity: 0 }}
          transition={{ duration: 0.12 }}
          onMouseDown={onClose}
          className="fixed inset-0 z-40 bg-black/45 backdrop-blur-sm flex items-center justify-center px-4"
        >
          <motion.div
            initial={{ y: 12, opacity: 0, scale: 0.98 }}
            animate={{ y: 0, opacity: 1, scale: 1 }}
            exit={{ y: 6, opacity: 0 }}
            transition={{ type: "spring", stiffness: 360, damping: 30 }}
            onMouseDown={(e) => e.stopPropagation()}
            className="w-full max-w-[640px] bg-[var(--color-surface)] border border-[var(--color-border)] rounded-xl shadow-stripe-deep overflow-hidden mx-4"
          >
            <header className="px-5 h-14 border-b border-[var(--color-border)] flex items-center justify-between">
              <div className="flex items-center gap-2.5">
                <div className="size-7 rounded-md bg-[var(--color-accent-bg)] flex items-center justify-center">
                  <HelpCircle size={15} className="text-[var(--color-brand)]" />
                </div>
                <div>
                  <div className="text-[10px] uppercase tracking-[0.14em] text-[var(--color-fg-subtle)] font-medium">
                    Help
                  </div>
                  <div className="text-sm text-[var(--color-fg)]">Getting around</div>
                </div>
              </div>
              <button
                onClick={onClose}
                className="size-7 flex items-center justify-center rounded-md text-[var(--color-fg-subtle)] hover:text-[var(--color-fg)] hover:bg-[var(--color-surface-2)]"
                aria-label="close help"
              >
                <X size={15} />
              </button>
            </header>

            <div className="p-5 max-h-[72vh] overflow-y-auto nice-scroll">
              {onHowItWorks && (
                <button
                  onClick={onHowItWorks}
                  className="w-full mb-5 flex items-center justify-between gap-3 rounded-lg border border-[var(--color-accent-border)] bg-[var(--color-accent-bg)] px-4 py-3 text-left transition-colors hover:bg-[var(--color-accent-bg)]/70"
                >
                  <span>
                    <span className="block text-sm font-medium text-[var(--color-fg)]">New here? See how it works</span>
                    <span className="block text-xs text-[var(--color-fg-muted)]">The request pipeline and what you do at each step.</span>
                  </span>
                  <span className="text-[var(--color-brand)] text-sm font-medium shrink-0">Open →</span>
                </button>
              )}
              <h3 className="text-[10px] uppercase tracking-[0.14em] text-[var(--color-fg-subtle)] font-medium mb-2.5">
                The sections
              </h3>
              <ul className="grid grid-cols-1 sm:grid-cols-2 gap-x-5 gap-y-3 mb-5">
                {SECTIONS.map((s) => {
                  const Icon = s.icon;
                  return (
                    <li key={s.label} className="flex gap-2.5">
                      <span className="size-7 shrink-0 rounded-md bg-[var(--color-surface-2)] flex items-center justify-center">
                        <Icon size={14} className="text-[var(--color-brand)]" strokeWidth={1.75} />
                      </span>
                      <div className="min-w-0">
                        <p className="text-xs font-medium text-[var(--color-fg)]">{s.label}</p>
                        <p className="text-xs text-[var(--color-fg-muted)] leading-snug">{s.desc}</p>
                      </div>
                    </li>
                  );
                })}
              </ul>

              <h3 className="text-[10px] uppercase tracking-[0.14em] text-[var(--color-fg-subtle)] font-medium mb-2.5">
                Tips
              </h3>
              <ul className="flex flex-col gap-2">
                {TIPS.map((t) => (
                  <li key={t} className="flex items-start gap-2 text-xs text-[var(--color-fg-muted)] leading-snug">
                    <span className="mt-1.5 size-1 rounded-full bg-[var(--color-brand)] shrink-0" />
                    {t}
                  </li>
                ))}
              </ul>
            </div>
          </motion.div>
        </motion.div>
      )}
    </AnimatePresence>
  );
}
