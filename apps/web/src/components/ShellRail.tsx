import { useEffect, useState } from "react";
import {
  BarChart3,
  Bot,
  ChevronDown,
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
  Wrench,
} from "lucide-react";

import { BrandMark } from "./Brand";
import { useTheme } from "../lib/theme";
import { useAuth } from "../contexts/AuthContext";

// ShellRail — the left navigation. On desktop it's a collapsible sidebar;
// on mobile (<768px) it becomes a bottom navigation bar with a user menu
// that includes the logout action.

export type ShellSection =
  | "home"
  | "my-work"
  | "requests"
  | "workflows"
  | "machines"
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

const PRIMARY: Item[] = [
  { id: "home", icon: Home, label: "Home", hint: "Dashboard" },
  { id: "my-work", icon: Inbox, label: "My Work", hint: "Tasks & approvals" },
  { id: "requests", icon: FileText, label: "Requests", hint: "Submit & track" },
  { id: "workflows", icon: Workflow, label: "Workflows", hint: "Live canvas" },
  { id: "machines", icon: Wrench, label: "Machines", hint: "Equipment" },
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

const MOBILE_NAV_ITEMS: Item[] = [
  { id: "home", icon: Home, label: "Home" },
  { id: "my-work", icon: Inbox, label: "My Work" },
  { id: "requests", icon: FileText, label: "Requests" },
];

const STORAGE_KEY = "aios.sidebar";

export function ShellRail({ active, onSelect, onBrandClick, onUserClick }: Props) {
  const { theme, toggle } = useTheme();
  const { user } = useAuth();

  const [expanded, setExpanded] = useState<boolean>(() => {
    if (typeof window === "undefined") return true;
    return window.localStorage.getItem(STORAGE_KEY) !== "collapsed";
  });
  const [userMenuOpen, setUserMenuOpen] = useState(false);

  useEffect(() => {
    window.localStorage.setItem(STORAGE_KEY, expanded ? "expanded" : "collapsed");
  }, [expanded]);

  const initials = user
    ? user.name.split(" ").map((w) => w[0]).join("").toUpperCase().slice(0, 2)
    : "?";

  return (
    <>
      <nav
        className={`hidden md:flex shrink-0 flex-col bg-[var(--color-surface)] border-r border-[var(--color-border)] transition-[width] duration-200 ease-out ${
          expanded ? "w-56" : "w-14"
        }`}
      >
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

        <ul className="flex flex-col gap-0.5 px-2 py-3 overflow-y-auto nice-scroll flex-1">
          {PRIMARY.map((it) => (
            <NavRow key={it.id} item={it} active={active === it.id} expanded={expanded} onSelect={onSelect} />
          ))}
        </ul>

        <div className="h-px bg-[var(--color-border)] mx-3" />
        <ul className="flex flex-col gap-0.5 px-2 py-2">
          {UTILITY.map((it) => (
            <NavRow key={it.id} item={it} active={active === it.id} expanded={expanded} onSelect={onSelect} />
          ))}
        </ul>

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

      {/* Mobile bottom nav — Home, My Work, Requests, User menu */}
      <nav className="md:hidden fixed bottom-0 left-0 right-0 z-50 bg-[var(--color-surface)] border-t border-[var(--color-border)] flex items-center justify-around px-1 safe-area-bottom">
        {MOBILE_NAV_ITEMS.map((it) => {
          const Icon = it.icon;
          const isActive = active === it.id;
          return (
            <button
              key={it.id}
              onClick={() => onSelect(it.id)}
              className={`flex flex-col items-center gap-0.5 py-2 px-2 min-w-0 flex-1 transition-colors ${
                isActive
                  ? "text-[var(--color-brand)]"
                  : "text-[var(--color-fg-muted)] hover:text-[var(--color-fg)]"
              }`}
            >
              <Icon size={20} strokeWidth={isActive ? 2.25 : 1.75} />
              <span className="text-[10px] font-medium truncate w-full text-center">{it.label}</span>
            </button>
          );
        })}

        {/* User avatar button */}
        <button
          onClick={() => setUserMenuOpen(!userMenuOpen)}
          className={`flex flex-col items-center gap-0.5 py-2 px-2 min-w-0 flex-1 transition-colors ${
            userMenuOpen ? "text-[var(--color-brand)]" : "text-[var(--color-fg-muted)] hover:text-[var(--color-fg)]"
          }`}
        >
          <span className="size-6 flex items-center justify-center rounded-full bg-[var(--color-accent-bg)] text-[var(--color-brand)] text-[9px] font-bold">
            {initials}
          </span>
          <span className="text-[10px] font-medium truncate w-full text-center">Account</span>
        </button>
      </nav>

      {/* Mobile user menu drawer — always shows logout */}
      {userMenuOpen && (
        <>
          <div className="md:hidden fixed inset-0 z-40" onClick={() => setUserMenuOpen(false)} />
          <div className="md:hidden fixed bottom-16 left-2 right-2 z-50 bg-[var(--color-surface)] border border-[var(--color-border)] rounded-xl shadow-stripe-elevated overflow-hidden">
            {/* User info + logout row */}
            <div className="p-4 border-b border-[var(--color-border)]">
              <div className="flex items-center gap-3">
                <span className="size-10 flex items-center justify-center rounded-full bg-[var(--color-accent-bg)] text-[var(--color-brand)] text-sm font-bold">
                  {initials}
                </span>
                <div className="min-w-0 flex-1">
                  <p className="text-sm font-medium text-[var(--color-fg)] truncate">{user?.name}</p>
                  <p className="text-xs text-[var(--color-fg-muted)] truncate">{user?.email}</p>
                </div>
              </div>
            </div>

            {/* Secondary nav items */}
            <div className="grid grid-cols-4 gap-1 p-3">
              {[...PRIMARY, ...UTILITY].filter(
                (it) => !MOBILE_NAV_ITEMS.some((m) => m.id === it.id)
              ).map((it) => {
                const Icon = it.icon;
                const isActive = active === it.id;
                return (
                  <button
                    key={it.id}
                    onClick={() => { onSelect(it.id); setUserMenuOpen(false); }}
                    className={`flex flex-col items-center gap-1 py-3 px-1 rounded-lg transition-colors min-h-[56px] ${
                      isActive
                        ? "bg-[var(--color-accent-bg)] text-[var(--color-brand)]"
                        : "text-[var(--color-fg-muted)] hover:bg-[var(--color-surface-2)] hover:text-[var(--color-fg)]"
                    }`}
                  >
                    <Icon size={18} strokeWidth={isActive ? 2.25 : 1.75} />
                    <span className="text-[9px] font-medium text-center truncate w-full">{it.label}</span>
                  </button>
                );
              })}
            </div>

            {/* Actions: theme toggle + sign out */}
            <div className="flex items-center gap-2 border-t border-[var(--color-border)] px-4 py-3">
              <button
                onClick={toggle}
                className="flex items-center gap-2 px-3 py-2 rounded-lg text-sm text-[var(--color-fg-muted)] hover:bg-[var(--color-surface-2)] hover:text-[var(--color-fg)] transition-colors"
              >
                {theme === "dark" ? <Sun size={15} /> : <Moon size={15} />}
                {theme === "dark" ? "Light mode" : "Dark mode"}
              </button>
              <button
                onClick={onUserClick}
                className="flex items-center gap-2 px-3 py-2 rounded-lg text-sm text-[var(--color-danger)] hover:bg-red-50 dark:hover:bg-red-950/30 transition-colors ml-auto"
              >
                <LogOut size={15} />
                Sign out
              </button>
            </div>
          </div>
        </>
      )}
    </>
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
