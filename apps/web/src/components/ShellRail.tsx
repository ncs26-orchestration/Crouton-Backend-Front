import { motion } from "framer-motion";
import { BarChart3, Bot, FileText, HelpCircle, Home, Inbox, ListTodo, Moon, Plug, ScrollText, Settings, Sun, Users, Workflow } from "lucide-react";

import { BrandMark } from "./Brand";
import { useTheme } from "../lib/theme";
import { useAuth } from "../contexts/AuthContext";

// ShellRail — the narrow icon column on the far left. Maps to the
// MVP specification: main sections + TEAMS group at the bottom.
// "help" is a transient overlay triggered from any section.

export type ShellSection = "home" | "my-work" | "requests" | "workflows" | "agents" | "reports" | "policies" | "integrations" | "teams" | "settings" | "help";

interface Props {
  active: ShellSection;
  onSelect: (section: ShellSection) => void;
  // Optional: clicking the brand mark fires this. Typical use is
  // "go home AND clear the scoped project", which the parent
  // implements differently from `onSelect("home")`.
  onBrandClick?: () => void;
  onUserClick?: () => void;
}

const ITEMS: { id: ShellSection; icon: typeof Home; label: string; hint?: string }[] = [
  { id: "home",        icon: Home,       label: "Home",         hint: "Dashboard" },
  { id: "my-work",     icon: Inbox,      label: "My Work",      hint: "Tasks & approvals" },
  { id: "requests",    icon: ListTodo,   label: "Requests",     hint: "All requests" },
  { id: "workflows",   icon: Workflow,   label: "Workflows",    hint: "Workflow canvas" },
  { id: "agents",      icon: Bot,        label: "Agents",       hint: "Agent roster" },
  { id: "reports",     icon: BarChart3,  label: "Reports",      hint: "Audit trail" },
  { id: "policies",    icon: ScrollText, label: "Policies",     hint: "Department policies" },
  { id: "integrations",icon: Plug,       label: "Integrations", hint: "Connected systems" },
];

const TEAMS: { id: string; name: string; color: string }[] = [
  { id: "finance", name: "Finance Team",  color: "#533afd" },
  { id: "it",      name: "IT Team",       color: "#15be53" },
  { id: "hr",      name: "HR Team",       color: "#f59e0b" },
  { id: "ops",     name: "Operations Team", color: "#ea2261" },
];

export function ShellRail({ active, onSelect, onBrandClick, onUserClick }: Props) {
  const { theme, toggle } = useTheme();
  const { user } = useAuth();

  const initials = user
    ? user.name
        .split(" ")
        .map((w) => w[0])
        .join("")
        .toUpperCase()
        .slice(0, 2)
    : "?";

  return (
    <nav className="w-14 shrink-0 flex flex-col items-center py-3 bg-[var(--color-surface)] border-r border-[var(--color-border)]">
      <motion.button
        onClick={onBrandClick}
        aria-label="Go to root"
        className="mb-3 relative group rounded-md focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[var(--color-accent-border)]"
        whileHover={{ scale: 1.06 }}
        whileTap={{ scale: 0.94 }}
        transition={{ type: "spring", stiffness: 340, damping: 20 }}
      >
        <BrandMark size={26} />
        <RailTooltip label="Crouton" hint="Go to root" />
      </motion.button>

      <div className="h-px w-6 bg-[var(--color-border)] mb-3" />

      <ul className="flex flex-col gap-1">
        {ITEMS.map((it) => {
          const Icon = it.icon;
          const isActive = it.id === active;
          return (
            <li key={it.id} className="relative group">
              <motion.button
                onClick={() => onSelect(it.id)}
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
                <Icon size={16} strokeWidth={isActive ? 2.25 : 1.75} />
              </motion.button>
              {isActive && (
                <motion.span
                  layoutId="shellrail-indicator"
                  className="absolute left-0 top-1/2 -translate-y-1/2 h-5 w-0.5 bg-[var(--color-brand)] rounded-r"
                  transition={{ type: "spring", stiffness: 500, damping: 28 }}
                />
              )}
              <RailTooltip label={it.label} hint={it.hint} />
            </li>
          );
        })}
      </ul>

      {/* TEAMS section divider */}
      <div className="mt-3 mb-2 flex flex-col items-center gap-2">
        <span className="text-[8px] uppercase tracking-[0.2em] text-[var(--color-fg-subtle)] font-medium">
          Teams
        </span>
        <div className="flex flex-col gap-1">
          {TEAMS.map((team) => {
            const isTeamActive = active === "teams" || false;
            return (
              <div key={team.id} className="relative group">
                <motion.button
                  onClick={() => onSelect("teams")}
                  aria-label={team.name}
                  whileHover={{ scale: 1.05 }}
                  whileTap={{ scale: 0.92 }}
                  transition={{ type: "spring", stiffness: 400, damping: 20 }}
                  className="size-9 flex items-center justify-center rounded-lg text-[var(--color-fg-muted)] hover:bg-[var(--color-surface-2)] hover:text-[var(--color-fg)] transition-colors"
                >
                  <span className="size-2 rounded-full" style={{ backgroundColor: team.color }} />
                </motion.button>
                <RailTooltip label={team.name} hint="Team" />
              </div>
            );
          })}
        </div>
      </div>

      <div className="flex-1" />

      {/* User avatar / logout */}
      {user && (
        <div className="relative group mb-1">
          <motion.button
            onClick={onUserClick}
            aria-label={`${user.name} — click to sign out`}
            whileHover={{ scale: 1.05 }}
            whileTap={{ scale: 0.92 }}
            className="size-9 flex items-center justify-center rounded-lg bg-[var(--color-accent-bg)] text-[var(--color-brand)] text-[10px] font-bold transition-colors hover:opacity-80"
          >
            {initials}
          </motion.button>
          <RailTooltip label={user.name} hint="Sign out" />
        </div>
      )}

      {/* Theme toggle */}
      <div className="relative group">
        <motion.button
          onClick={toggle}
          aria-label="Toggle theme"
          whileHover={{ scale: 1.05 }}
          whileTap={{ scale: 0.92, rotate: 15 }}
          className="size-9 flex items-center justify-center rounded-lg text-[var(--color-fg-muted)] hover:bg-[var(--color-surface-2)] hover:text-[var(--color-fg)] transition-colors"
        >
          {theme === "dark" ? <Sun size={15} strokeWidth={1.75} /> : <Moon size={15} strokeWidth={1.75} />}
        </motion.button>
        <RailTooltip label={theme === "dark" ? "Light mode" : "Dark mode"} />
      </div>
    </nav>
  );
}

function RailTooltip({ label, hint }: { label: string; hint?: string }) {
  return (
    <span className="pointer-events-none absolute left-full ml-3 top-1/2 -translate-y-1/2 flex items-center gap-2 bg-[var(--color-fg)] text-[var(--color-bg)] rounded-md px-2 py-1 text-xs font-medium whitespace-nowrap shadow-stripe-elevated opacity-0 group-hover:opacity-100 transition-opacity z-50">
      {label}
      {hint && <span className="font-mono text-[10px] opacity-70">{hint}</span>}
    </span>
  );
}
