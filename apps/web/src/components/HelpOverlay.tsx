import { AnimatePresence, motion } from "framer-motion";
import { Keyboard, X } from "lucide-react";

// HelpOverlay — triggered by the `?` key. A calm modal card listing
// every keyboard shortcut in the app, grouped by surface. Pure
// static content; no state beyond open/close.

interface Props {
  open: boolean;
  onClose: () => void;
}

const SHORTCUTS: { group: string; items: [keys: string[], description: string][] }[] = [
  {
    group: "Global",
    items: [
      [["⌘", "K"], "Open the command palette"],
      [["⌘", "/"], "Toggle Copilot sidebar"],
      [["?"], "Show this help"],
      [["Esc"], "Close any modal"],
    ],
  },
  {
    group: "Workflow",
    items: [
      [["⌘", "↵"], "Extract workflow from composer text"],
      [["⌘", "⇧", "C"], "Compile the current workflow"],
      [["⌘", "⇧", "D"], "Deploy to the selected engine"],
    ],
  },
  {
    group: "Navigation",
    items: [
      [["⌘", "1"], "Workspace"],
      [["⌘", "2"], "Runs"],
      [["⌘", "3"], "Settings"],
    ],
  },
  {
    group: "Canvas",
    items: [
      [["Drag"], "Move a node"],
      [["Scroll"], "Zoom in / out"],
      [["Click"], "Select a task (opens Inspector)"],
      [["Esc"], "Deselect"],
    ],
  },
];

export function HelpOverlay({ open, onClose }: Props) {
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
            className="w-full max-w-[560px] bg-[var(--color-surface)] border border-[var(--color-border)] rounded-xl shadow-stripe-deep overflow-hidden"
          >
            <header className="px-4 h-12 border-b border-[var(--color-border)] flex items-center justify-between">
              <div className="flex items-center gap-2">
                <div className="size-6 rounded-md bg-[var(--color-accent-bg)] flex items-center justify-center">
                  <Keyboard size={12} className="text-[var(--color-brand)]" />
                </div>
                <div>
                  <div className="text-[10px] uppercase tracking-[0.14em] text-[var(--color-fg-subtle)]" style={{ fontWeight: 500 }}>
                    Help
                  </div>
                  <div className="text-[13px] text-[var(--color-fg)]" style={{ fontWeight: 400 }}>
                    Keyboard shortcuts
                  </div>
                </div>
              </div>
              <button
                onClick={onClose}
                className="text-[var(--color-fg-subtle)] hover:text-[var(--color-fg)]"
                aria-label="close help"
              >
                <X size={14} />
              </button>
            </header>
            <div className="p-4 grid grid-cols-2 gap-x-6 gap-y-5 nice-scroll max-h-[70vh] overflow-y-auto">
              {SHORTCUTS.map((s) => (
                <section key={s.group}>
                  <h3 className="text-[10px] uppercase tracking-[0.14em] text-[var(--color-fg-subtle)] mb-2" style={{ fontWeight: 500 }}>
                    {s.group}
                  </h3>
                  <ul className="space-y-1.5">
                    {s.items.map(([keys, desc], i) => (
                      <li key={i} className="flex items-center justify-between gap-3 text-[12px]">
                        <span className="text-[var(--color-fg-muted)]">{desc}</span>
                        <span className="flex items-center gap-0.5 shrink-0">
                          {keys.map((k, j) => (
                            <kbd
                              key={j}
                              className="inline-flex items-center justify-center min-w-[18px] h-[18px] px-1 font-mono text-[10px] border border-[var(--color-border)] rounded-[4px] bg-[var(--color-surface-2)] text-[var(--color-fg)]"
                              style={{ fontWeight: 500 }}
                            >
                              {k}
                            </kbd>
                          ))}
                        </span>
                      </li>
                    ))}
                  </ul>
                </section>
              ))}
            </div>
          </motion.div>
        </motion.div>
      )}
    </AnimatePresence>
  );
}
