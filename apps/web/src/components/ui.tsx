import type { ReactNode } from "react";
import type { LucideIcon } from "lucide-react";

// Shared UI primitives so pages share consistent chrome (headers, cards, stats,
// empty states) while each page composes them into its own distinct layout.

export function PageHeader({
  title,
  subtitle,
  actions,
}: {
  title: string;
  subtitle?: string;
  actions?: ReactNode;
}) {
  return (
    <div className="border-b border-[var(--color-border)] px-8 py-5 flex items-start justify-between gap-4 shrink-0">
      <div className="min-w-0">
        <h1 className="text-xl font-medium text-[var(--color-fg)]" style={{ fontFeatureSettings: '"ss01"' }}>
          {title}
        </h1>
        {subtitle && <p className="text-sm text-[var(--color-fg-muted)] mt-0.5">{subtitle}</p>}
      </div>
      {actions && <div className="flex items-center gap-2 shrink-0">{actions}</div>}
    </div>
  );
}

export function SectionCard({
  title,
  action,
  className = "",
  bodyClassName = "",
  children,
}: {
  title?: string;
  action?: ReactNode;
  className?: string;
  bodyClassName?: string;
  children: ReactNode;
}) {
  return (
    <section className={`rounded-lg border border-[var(--color-border)] bg-[var(--color-surface)] shadow-stripe-ambient ${className}`}>
      {title && (
        <header className="flex items-center justify-between px-4 py-3 border-b border-[var(--color-border)]">
          <h2 className="text-sm font-medium text-[var(--color-fg)]">{title}</h2>
          {action}
        </header>
      )}
      <div className={bodyClassName || "p-4"}>{children}</div>
    </section>
  );
}

export function StatCard({
  icon: Icon,
  label,
  value,
  tone = "neutral",
  hint,
}: {
  icon?: LucideIcon;
  label: string;
  value: string | number;
  tone?: "neutral" | "brand" | "success" | "warning" | "danger";
  hint?: string;
}) {
  const toneClass = {
    neutral: "text-[var(--color-fg-muted)]",
    brand: "text-[var(--color-brand)]",
    success: "text-[var(--color-success)]",
    warning: "text-[var(--color-warning-fg)]",
    danger: "text-[var(--color-danger)]",
  }[tone];
  return (
    <div className="rounded-lg border border-[var(--color-border)] bg-[var(--color-surface)] px-4 py-3 shadow-stripe-ambient">
      <div className="flex items-center gap-1.5 text-[11px] uppercase tracking-wide text-[var(--color-fg-muted)]">
        {Icon && <Icon size={13} className={toneClass} />}
        {label}
      </div>
      <p className="mt-1.5 text-2xl font-light tnum text-[var(--color-fg)]">{value}</p>
      {hint && <p className="text-[11px] text-[var(--color-fg-subtle)] mt-0.5">{hint}</p>}
    </div>
  );
}

// A compact SVG progress ring for a single headline percentage.
export function ProgressRing({
  value,
  size = 92,
  stroke = 9,
  label,
}: {
  value: number;
  size?: number;
  stroke?: number;
  label?: string;
}) {
  const r = (size - stroke) / 2;
  const c = 2 * Math.PI * r;
  const pct = Math.max(0, Math.min(100, value));
  const offset = c - (pct / 100) * c;
  return (
    <div className="relative inline-flex items-center justify-center" style={{ width: size, height: size }}>
      <svg width={size} height={size} className="-rotate-90">
        <circle cx={size / 2} cy={size / 2} r={r} fill="none" stroke="var(--color-surface-3)" strokeWidth={stroke} />
        <circle
          cx={size / 2}
          cy={size / 2}
          r={r}
          fill="none"
          stroke="var(--color-brand)"
          strokeWidth={stroke}
          strokeLinecap="round"
          strokeDasharray={c}
          strokeDashoffset={offset}
          style={{ transition: "stroke-dashoffset 600ms cubic-bezier(0.16,1,0.3,1)" }}
        />
      </svg>
      <div className="absolute inset-0 flex flex-col items-center justify-center">
        <span className="text-lg font-medium tnum text-[var(--color-fg)]">{pct}%</span>
        {label && <span className="text-[10px] text-[var(--color-fg-muted)]">{label}</span>}
      </div>
    </div>
  );
}

export function EmptyState({
  icon: Icon,
  title,
  hint,
  action,
}: {
  icon: LucideIcon;
  title: string;
  hint?: string;
  action?: ReactNode;
}) {
  return (
    <div className="flex flex-col items-center justify-center gap-3 rounded-lg border border-dashed border-[var(--color-border)] bg-[var(--color-surface)] py-14 text-center px-6">
      <div className="size-11 rounded-xl bg-[var(--color-surface-2)] flex items-center justify-center">
        <Icon size={20} className="text-[var(--color-fg-subtle)]" strokeWidth={1.5} />
      </div>
      <div>
        <p className="text-sm font-medium text-[var(--color-fg)]">{title}</p>
        {hint && <p className="text-xs text-[var(--color-fg-muted)] mt-1 max-w-[42ch]">{hint}</p>}
      </div>
      {action}
    </div>
  );
}
