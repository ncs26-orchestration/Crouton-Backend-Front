import { Sun, Moon, User, Building2, Palette, Info, type LucideIcon } from "lucide-react";

import { useTheme } from "../lib/theme";
import { useAuth } from "../contexts/AuthContext";
import { useOrg } from "../contexts/OrgContext";
import { prettyLabel } from "../lib/request-format";

export function SettingsView() {
  const { theme, setTheme } = useTheme();
  const { user } = useAuth();
  const { activeOrg } = useOrg();

  const initials = user
    ? user.name.split(" ").map((w) => w[0]).join("").toUpperCase().slice(0, 2)
    : "?";

  return (
    <div className="flex-1 flex flex-col bg-[var(--color-bg)] text-[var(--color-fg)] overflow-auto nice-scroll">
      <div className="border-b border-[var(--color-border)] px-4 md:px-8 py-4 md:py-5">
        <h1 className="text-xl font-medium" style={{ fontFeatureSettings: '"ss01"' }}>
          Settings
        </h1>
        <p className="text-sm text-[var(--color-fg-muted)] mt-0.5">Your profile, organization, and preferences</p>
      </div>

      <div className="px-4 md:px-8 py-4 md:py-6 w-full grid grid-cols-1 lg:grid-cols-2 gap-5">
        {/* Profile */}
        <Card icon={User} title="Profile">
          <div className="flex items-center gap-3 mb-4">
            <span className="size-12 rounded-full bg-[var(--color-accent-bg)] text-[var(--color-brand)] flex items-center justify-center text-base font-semibold">
              {initials}
            </span>
            <div className="min-w-0">
              <p className="text-sm font-medium text-[var(--color-fg)] truncate">{user?.name ?? "—"}</p>
              <p className="text-xs text-[var(--color-fg-muted)] truncate">{user?.email ?? "—"}</p>
            </div>
          </div>
          <Field label="Full name" value={user?.name ?? "—"} />
          <Field label="Email" value={user?.email ?? "—"} />
        </Card>

        {/* Organization */}
        <Card icon={Building2} title="Organization">
          <Field label="Name" value={activeOrg?.name ?? "—"} />
          <Field label="Slug" value={activeOrg?.slug ?? "—"} mono />
          <Field label="Your role" value={activeOrg ? prettyLabel(activeOrg.role) : "—"} />
        </Card>

        {/* Appearance */}
        <Card icon={Palette} title="Appearance">
          <p className="text-xs text-[var(--color-fg-muted)] mb-2">Theme</p>
          <div className="inline-flex rounded-lg border border-[var(--color-border)] p-0.5 bg-[var(--color-surface-2)]">
            <ThemeOption icon={Sun} label="Light" active={theme === "light"} onClick={() => setTheme("light")} />
            <ThemeOption icon={Moon} label="Dark" active={theme === "dark"} onClick={() => setTheme("dark")} />
          </div>
          <p className="text-xs text-[var(--color-fg-subtle)] mt-3 leading-relaxed">
            The interface defaults to the light Stripe-inspired theme. Your choice is saved on this device.
          </p>
        </Card>

        {/* About */}
        <Card icon={Info} title="About">
          <Field label="Product" value="AI Organization OS" />
          <Field label="What it does" value="Multi-agent workflows for cross-department requests" />
          <Field label="Departments" value="Finance · Legal · IT · HR · Operations" />
        </Card>
      </div>
    </div>
  );
}

function Card({ icon: Icon, title, children }: { icon: LucideIcon; title: string; children: React.ReactNode }) {
  return (
    <section className="rounded-lg border border-[var(--color-border)] bg-[var(--color-surface)] shadow-stripe-ambient">
      <header className="flex items-center gap-2 px-4 py-3 border-b border-[var(--color-border)]">
        <Icon size={15} className="text-[var(--color-brand)]" strokeWidth={1.75} />
        <h2 className="text-sm font-medium">{title}</h2>
      </header>
      <div className="px-4 py-4">{children}</div>
    </section>
  );
}

function Field({ label, value, mono }: { label: string; value: string; mono?: boolean }) {
  return (
    <div className="flex items-center justify-between gap-4 py-2 border-b border-[var(--color-border)] last:border-0">
      <span className="text-xs text-[var(--color-fg-muted)]">{label}</span>
      <span className={`text-sm text-[var(--color-fg)] text-right truncate ${mono ? "font-mono text-xs" : ""}`}>
        {value}
      </span>
    </div>
  );
}

function ThemeOption({
  icon: Icon,
  label,
  active,
  onClick,
}: {
  icon: LucideIcon;
  label: string;
  active: boolean;
  onClick: () => void;
}) {
  return (
    <button
      onClick={onClick}
      className={`flex items-center gap-1.5 rounded-md px-3 py-1.5 text-sm font-medium transition-colors ${
        active
          ? "bg-[var(--color-surface)] text-[var(--color-fg)] shadow-stripe-ambient"
          : "text-[var(--color-fg-muted)] hover:text-[var(--color-fg)]"
      }`}
    >
      <Icon size={14} />
      {label}
    </button>
  );
}
