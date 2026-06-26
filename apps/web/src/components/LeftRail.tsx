import { Activity, Bot, Command, HelpCircle, Moon, Network, Settings, Sun, Workflow } from "lucide-react";
import { motion } from "framer-motion";

import { BrandMark } from "./Brand";
import { useTheme } from "../lib/theme";

export type ViewSection = "workspace" | "runs" | "settings";

interface Props {
  view: ViewSection;
  onSelectView: (v: ViewSection) => void;
  panelOpen: boolean;
  onTogglePanel: () => void;
  copilotOpen: boolean;
  onToggleCopilot: () => void;
  copilotBadge?: number; // count of open clarifications — small pip on the icon
  onOpenCommand: () => void;
  onOpenHelp: () => void;
}

interface ViewItem {
  id: ViewSection;
  icon: React.ComponentType<{ size?: number; strokeWidth?: number }>;
  label: string;
  hint?: string;
}

const VIEW_ITEMS: ViewItem[] = [
  { id: "workspace", icon: Workflow, label: "Workspace", hint: "Canvas + composer" },
  { id: "runs", icon: Activity, label: "Runs", hint: "Deployed workflows & instances" },
  { id: "settings", icon: Settings, label: "Settings", hint: "Engines, systems, theme" },
];

export function LeftRail({
  view,
  onSelectView,
  panelOpen,
  onTogglePanel,
  copilotOpen,
  onToggleCopilot,
  copilotBadge,
  onOpenCommand,
  onOpenHelp,
}: Props) {
  const { theme, toggle } = useTheme();
  return (
    <nav className="w-14 shrink-0 flex flex-col items-center py-3 bg-[var(--color-surface)] border-r border-[var(--color-border)]">
      <motion.div
        className="mb-3 relative group cursor-default"
        whileHover={{ scale: 1.04 }}
        whileTap={{ scale: 0.96 }}
        transition={{ type: "spring", stiffness: 340, damping: 20 }}
      >
        <BrandMark size={28} />
        <Tooltip label="Crouton · workspace" />
      </motion.div>

      <div className="h-px w-6 bg-[var(--color-border)] mb-3" />

      {/* Section 1 — Views. Mutually exclusive. */}
      <ul className="flex flex-col gap-1">
        {VIEW_ITEMS.map((it) => {
          const Icon = it.icon;
          const isActive = it.id === view;
          return (
            <li key={it.id} className="relative group">
              <motion.button
                onClick={() => onSelectView(it.id)}
                aria-label={it.label}
                aria-current={isActive ? "page" : undefined}
                whileHover={{ scale: 1.05 }}
                whileTap={{ scale: 0.92 }}
                transition={{ type: "spring", stiffness: 400, damping: 20 }}
                className={`size-9 flex items-center justify-center rounded-lg transition-colors ${
                  isActive
                    ? "bg-[var(--color-accent-bg)] text-[var(--color-brand)]"
                    : "text-[var(--color-fg-muted)] hover:bg-[var(--color-surface-2)] hover:text-[var(--color-fg)]"
                }`}
              >
                <Icon size={17} strokeWidth={isActive ? 2.25 : 1.75} />
              </motion.button>
              {isActive && (
                <motion.span
                  layoutId="rail-indicator"
                  className="absolute left-0 top-1/2 -translate-y-1/2 h-5 w-0.5 bg-[var(--color-brand)] rounded-r"
                  transition={{ type: "spring", stiffness: 500, damping: 28 }}
                />
              )}
              <Tooltip label={it.label} hint={it.hint} />
            </li>
          );
        })}
      </ul>

      {/* Divider — separates views from the IS toggle so the user sees
          the semantic difference. */}
      <div className="my-3 h-px w-6 bg-[var(--color-border)]" />

      {/* Section 2 — Side panel toggles. Not views; overlay any view. */}
      <div className="relative group">
        <motion.button
          onClick={onTogglePanel}
          aria-label={panelOpen ? "Hide information system panel" : "Show information system panel"}
          aria-pressed={panelOpen}
          whileHover={{ scale: 1.05 }}
          whileTap={{ scale: 0.92 }}
          transition={{ type: "spring", stiffness: 400, damping: 20 }}
          className={`size-9 flex items-center justify-center rounded-lg transition-colors ${
            panelOpen
              ? "bg-[var(--color-accent-bg)] text-[var(--color-brand)]"
              : "text-[var(--color-fg-muted)] hover:bg-[var(--color-surface-2)] hover:text-[var(--color-fg)]"
          }`}
        >
          <Network size={17} strokeWidth={panelOpen ? 2.25 : 1.75} />
        </motion.button>
        <Tooltip
          label={panelOpen ? "Hide IS panel" : "Show IS panel"}
          hint="Side panel · toggle"
        />
      </div>

      <div className="relative group mt-1">
        <motion.button
          onClick={onToggleCopilot}
          aria-label={copilotOpen ? "Hide copilot" : "Show copilot"}
          aria-pressed={copilotOpen}
          whileHover={{ scale: 1.05 }}
          whileTap={{ scale: 0.92 }}
          transition={{ type: "spring", stiffness: 400, damping: 20 }}
          className={`size-9 flex items-center justify-center rounded-lg transition-colors ${
            copilotOpen
              ? "bg-[var(--color-accent-bg)] text-[var(--color-brand)]"
              : "text-[var(--color-fg-muted)] hover:bg-[var(--color-surface-2)] hover:text-[var(--color-fg)]"
          }`}
        >
          <Bot size={17} strokeWidth={copilotOpen ? 2.25 : 1.75} />
          {/* Badge — count of pending clarifications from the extractor */}
          {copilotBadge != null && copilotBadge > 0 && !copilotOpen && (
            <span className="absolute top-0.5 right-0.5 size-2 rounded-full bg-amber-500 border-[1.5px] border-[var(--color-surface)]" />
          )}
        </motion.button>
        <Tooltip
          label={copilotOpen ? "Hide Copilot" : "Show Copilot"}
          hint="Ask + Clarify · ⌘/"
        />
      </div>

      <div className="mt-auto flex flex-col gap-1">
        <div className="relative group">
          <motion.button
            onClick={onOpenCommand}
            aria-label="Command palette"
            whileHover={{ scale: 1.05 }}
            whileTap={{ scale: 0.92 }}
            className="size-9 flex items-center justify-center rounded-lg text-[var(--color-fg-muted)] hover:bg-[var(--color-surface-2)] hover:text-[var(--color-fg)] transition-colors"
          >
            <Command size={16} strokeWidth={1.75} />
          </motion.button>
          <Tooltip label="Command palette" hint="⌘K" />
        </div>
        <div className="relative group">
          <motion.button
            onClick={toggle}
            aria-label="Toggle theme"
            whileHover={{ scale: 1.05 }}
            whileTap={{ scale: 0.92, rotate: 15 }}
            className="size-9 flex items-center justify-center rounded-lg text-[var(--color-fg-muted)] hover:bg-[var(--color-surface-2)] hover:text-[var(--color-fg)] transition-colors"
          >
            {theme === "dark" ? <Sun size={16} strokeWidth={1.75} /> : <Moon size={16} strokeWidth={1.75} />}
          </motion.button>
          <Tooltip label={theme === "dark" ? "Light mode" : "Dark mode"} />
        </div>
        <div className="relative group">
          <motion.button
            onClick={onOpenHelp}
            aria-label="About Crouton"
            whileHover={{ scale: 1.05 }}
            whileTap={{ scale: 0.92 }}
            className="size-9 flex items-center justify-center rounded-lg text-[var(--color-fg-muted)] hover:bg-[var(--color-surface-2)] hover:text-[var(--color-fg)] transition-colors"
          >
            <HelpCircle size={16} strokeWidth={1.75} />
          </motion.button>
          <Tooltip label="About Crouton" />
        </div>
      </div>
    </nav>
  );
}

function Tooltip({ label, hint }: { label: string; hint?: string }) {
  return (
    <span className="pointer-events-none absolute left-full ml-3 top-1/2 -translate-y-1/2 flex items-center gap-2 bg-[var(--color-fg)] text-[var(--color-bg)] rounded-md px-2 py-1 text-xs font-medium whitespace-nowrap shadow-stripe-elevated opacity-0 group-hover:opacity-100 transition-opacity z-50">
      {label}
      {hint && <span className="font-mono text-[10px] opacity-70">{hint}</span>}
    </span>
  );
}
