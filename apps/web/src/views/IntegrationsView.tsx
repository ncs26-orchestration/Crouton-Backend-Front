import { Plug } from "lucide-react";

export function IntegrationsView() {
  return (
    <div className="flex-1 flex flex-col bg-[var(--color-bg)] text-[var(--color-fg)]">
      <div className="border-b border-[var(--color-border)] px-8 py-5">
        <h1 className="text-xl font-semibold text-[var(--color-fg)]">Integrations</h1>
        <p className="text-sm text-[var(--color-fg-muted)] mt-0.5">Connected systems and data sources</p>
      </div>
      <div className="flex-1 flex flex-col items-center justify-center gap-4">
        <div className="size-14 rounded-2xl bg-[var(--color-accent-bg)] border border-[var(--color-accent-border)] flex items-center justify-center">
          <Plug size={24} className="text-[var(--color-brand)]" strokeWidth={1.5} />
        </div>
        <div className="text-center">
          <p className="text-sm font-medium text-[var(--color-fg)]">No integrations configured</p>
          <p className="text-xs text-[var(--color-fg-muted)] mt-1">Connected systems agents use for decision-making will appear here</p>
        </div>
      </div>
    </div>
  );
}
