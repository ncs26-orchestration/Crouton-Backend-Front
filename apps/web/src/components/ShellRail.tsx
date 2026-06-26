import { useEffect, useState } from "react";
import {
  BarChart3,
  Bot,
  FileText,
  HelpCircle,
  Home,
  Inbox,
  Link2,
  LogOut,
  Moon,
  PanelLeftClose,
  PanelLeftOpen,
  ScrollText,
  Settings,
  Sun,
  Users,
  Workflow,
} from "lucide-react";

import { BrandMark } from "./Brand";
import { useTheme } from "../lib/theme";
import { useAuth } from "../contexts/AuthContext";

// ShellRail — the left navigation. Collapses to an icon rail or expands to a
// full sidebar with labels; the choice is remembered across sessions.

export type ShellSection =
  | "home"
  | "my-work"
  | "requests"
  | "workflows"
  | "agents"
  | "reports"
  | "policies"
  | "integrations"
  | "teams"
  | "settings"
  | "help";

interface Props {
  active: ShellSection;
  onSelect: (section: ShellSection) => void;
  onBrandClick?: () => void;
  onUserClick?: () => void;
}

interface Item {
  id: ShellSection;
  icon: typeof Home;
  label: string;
  hint?: string;
  dividerBefore?: boolean;
}

// Primary nav (top) and utility nav (bottom) so Settings/Help sit apart.
const PRIMARY: Item[] = [
  { id: "home", icon: Home, label: "Home", hint: "Dashboard" },
  { id: "my-work", icon: Inbox, label: "My Work", hint: "Tasks & approvals" },
  { id: "requests", icon: FileText, label: "Requests", hint: "Submit & track" },
  { id: "workflows", icon: Workflow, label: "Workflows", hint: "Live canvas" },
  { id: "agents", icon: Bot, label: "Agents", hint: "Agent roster" },
  { id: "reports", icon: BarChart3, label: "Reports", hint: "Audit & analytics" },
  { id: "policies", icon: ScrollText, label: "Policies", hint: "Department rules" },
  { id: "integrations", icon: Link2, label: "Integrations", hint: "Connected systems" },
  { id: "teams", icon: Users, label: "Teams", hint: "Departments", dividerBefore: true },
];

const UTILITY: Item[] = [
  { id: "settings", icon: Settings, label: "Settings", hint: "Preferences" },
  { id: "help", icon: HelpCircle, label: "Help", hint: "Shortcuts" },
];

const STORAGE_KEY = "aios.sidebar";

export function ShellRail({ active, onSelect, onBrandClick, onUserClick }: Props) {
  const { theme, toggle } = useTheme();
  const { user } = useAuth();

  const [expanded, setExpanded] = useState<boolean>(() => {
    if (typeof window === "undefined") return true;
    return window.localStorage.getItem(STORAGE_KEY) !== "collapsed";
  });
  useEffect(() => {
    window.localStorage.setItem(STORAGE_KEY, expanded ? "expanded" : "collapsed");
  }, [expanded]);

  const initials = user
    ? user.name.split(" ").map((w) => w[0]).join("").toUpperCase().slice(0, 2)
    : "?";

  return (
    <nav
      className={`shrink-0 flex flex-col bg-[var(--color-surface)] border-r border-[var(--color-border)] transition-[width] duration-200 ease-out ${
        expanded ? "w-56" : "w-14"
      }`}
    >
      {/* Brand + collapse toggle */}
      <div className={`flex items-center h-14 shrink-0 ${expanded ? "px-3 justify-between" : "justify-center"}`}>
        <button
          onClick={onBrandClick}
          aria-label="Go to home"
          className="flex items-center gap-2 rounded-md focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[var(--color-accent-border)]"
        >
          <BrandMark size={24} />
          {expanded && (
            <span className="text-sm font-semibold text-[var(--color-fg)]" style={{ fontFeatureSettings: '"ss01"' }}>
              Org OS
            </span>
          )}
        </button>
        {expanded && (
          <button
            onClick={() => setExpanded(false)}
            aria-label="Collapse sidebar"
            className="size-7 flex items-center justify-center rounded-md text-[var(--color-fg-muted)] hover:bg-[var(--color-surface-2)] hover:text-[var(--color-fg)] transition-colors"
          >
            <PanelLeftClose size={16} strokeWidth={1.75} />
          </button>
        )}
      </div>

      {!expanded && (
        <button
          onClick={() => setExpanded(true)}
          aria-label="Expand sidebar"
          className="mx-auto mb-1 size-9 flex items-center justify-center rounded-lg text-[var(--color-fg-muted)] hover:bg-[var(--color-surface-2)] hover:text-[var(--color-fg)] transition-colors"
        >
          <PanelLeftOpen size={16} strokeWidth={1.75} />
        </button>
      )}

      <div className="h-px bg-[var(--color-border)] mx-3" />

      {/* Primary nav */}
      <ul className="flex flex-col gap-0.5 px-2 py-3 overflow-y-auto nice-scroll flex-1">
        {PRIMARY.map((it) => (
          <NavRow key={it.id} item={it} active={active === it.id} expanded={expanded} onSelect={onSelect} />
        ))}
      </ul>

      {/* Utility nav */}
      <div className="h-px bg-[var(--color-border)] mx-3" />
      <ul className="flex flex-col gap-0.5 px-2 py-2">
        {UTILITY.map((it) => (
          <NavRow key={it.id} item={it} active={active === it.id} expanded={expanded} onSelect={onSelect} />
        ))}
      </ul>

      {/* Footer: user + theme */}
      <div className="h-px bg-[var(--color-border)] mx-3" />
      <div className={`p-2 flex ${expanded ? "items-center gap-2" : "flex-col items-center gap-1"}`}>
        {user && (
          <button
            onClick={onUserClick}
            aria-label={`${user.name} — sign out`}
            className={`group flex items-center gap-2 rounded-lg transition-colors hover:bg-[var(--color-surface-2)] ${
              expanded ? "flex-1 min-w-0 px-2 py-1.5" : "size-9 justify-center"
            }`}
          >
            <span className="size-7 shrink-0 flex items-center justify-center rounded-full bg-[var(--color-accent-bg)] text-[var(--color-brand)] text-[10px] font-bold">
              {initials}
            </span>
            {expanded && user && (
              <>
                <span className="flex-1 min-w-0 text-left">
                  <span className="block text-xs font-medium text-[var(--color-fg)] truncate">{user.name}</span>
                  <span className="block text-[10px] text-[var(--color-fg-muted)]">Sign out</span>
                </span>
                <LogOut size={13} className="text-[var(--color-fg-subtle)] group-hover:text-[var(--color-fg-muted)]" />
              </>
            )}
          </button>
        )}
        <button
          onClick={toggle}
          aria-label={theme === "dark" ? "Switch to light mode" : "Switch to dark mode"}
          className="size-9 shrink-0 flex items-center justify-center rounded-lg text-[var(--color-fg-muted)] hover:bg-[var(--color-surface-2)] hover:text-[var(--color-fg)] transition-colors"
        >
          {theme === "dark" ? <Sun size={15} strokeWidth={1.75} /> : <Moon size={15} strokeWidth={1.75} />}
        </button>
      </div>
    </nav>
  );
}

function NavRow({
  item,
  active,
  expanded,
  onSelect,
}: {
  item: Item;
  active: boolean;
  expanded: boolean;
  onSelect: (s: ShellSection) => void;
}) {
  const Icon = item.icon;
  return (
    <li>
      {item.dividerBefore && <div className="h-px bg-[var(--color-border)] mx-1 my-1.5" />}
      <div className="relative group">
        <button
          onClick={() => onSelect(item.id)}
          aria-label={item.label}
          aria-current={active ? "page" : undefined}
          className={`w-full flex items-center rounded-lg transition-colors ${
            expanded ? "gap-3 px-2.5 py-2" : "justify-center py-2"
          } ${
            active
              ? "bg-[var(--color-accent-bg)] text-[var(--color-brand)]"
              : "text-[var(--color-fg-muted)] hover:bg-[var(--color-surface-2)] hover:text-[var(--color-fg)]"
          }`}
        >
          <Icon size={17} strokeWidth={active ? 2.25 : 1.75} className="shrink-0" />
          {expanded && <span className="text-sm font-medium truncate">{item.label}</span>}
        </button>
        {!expanded && (
          <span className="pointer-events-none absolute left-full ml-3 top-1/2 -translate-y-1/2 flex items-center gap-2 bg-[var(--color-fg)] text-[var(--color-bg)] rounded-md px-2 py-1 text-xs font-medium whitespace-nowrap shadow-stripe-elevated opacity-0 group-hover:opacity-100 transition-opacity z-50">
            {item.label}
            {item.hint && <span className="font-mono text-[10px] opacity-70">{item.hint}</span>}
          </span>
        )}
      </div>
    </li>
  );
}
