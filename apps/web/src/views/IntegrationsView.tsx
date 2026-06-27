import {
  DollarSign,
  Scale,
  Server,
  Users,
  MessagesSquare,
  Mail,
  type LucideIcon,
} from "lucide-react";

// The data sources each department agent would draw on in production. This
// page is intentionally display-only (see FEATURES.md F16): it shows the
// integration surface, not live connections. "connected" entries are the
// ones wired for the demo; the rest advertise where the system extends.
interface Integration {
  name: string;
  category: string;
  icon: LucideIcon;
  status: "connected" | "available";
  usedBy: string;
  description: string;
}

const INTEGRATIONS: Integration[] = [
  {
    name: "SAP / QuickBooks",
    category: "Financial systems",
    icon: DollarSign,
    status: "connected",
    usedBy: "Finance Reviewer",
    description: "Budgets, spend records and ledgers behind budget feasibility and ROI checks.",
  },
  {
    name: "LexisNexis",
    category: "Legal & compliance",
    icon: Scale,
    status: "connected",
    usedBy: "Legal Reviewer",
    description: "Regulatory and case databases for compliance and contract review.",
  },
  {
    name: "AWS / Azure / CMDB",
    category: "IT infrastructure",
    icon: Server,
    status: "connected",
    usedBy: "IT Manager",
    description: "Cloud accounts and the configuration database for feasibility and security.",
  },
  {
    name: "Workday / BambooHR",
    category: "HR systems",
    icon: Users,
    status: "available",
    usedBy: "HR Manager",
    description: "Headcount, roles and policy data for staffing and hiring plans.",
  },
  {
    name: "Slack",
    category: "Communication",
    icon: MessagesSquare,
    status: "connected",
    usedBy: "Notifications",
    description: "Channel updates when a request needs attention or a decision lands.",
  },
  {
    name: "Email",
    category: "Communication",
    icon: Mail,
    status: "available",
    usedBy: "Notifications",
    description: "Digest and approval-request delivery for executives and requesters.",
  },
];

export function IntegrationsView() {
  return (
    <div className="flex-1 flex flex-col bg-[var(--color-bg)] text-[var(--color-fg)] overflow-auto nice-scroll">
      <div className="border-b border-[var(--color-border)] px-4 md:px-8 py-4 md:py-5">
        <h1 className="text-xl font-medium" style={{ fontFeatureSettings: '"ss01"' }}>
          Integrations
        </h1>
        <p className="text-sm text-[var(--color-fg-muted)] mt-0.5">
          The systems and data sources agents draw on to make decisions
        </p>
      </div>

      <div className="px-4 md:px-8 py-4 md:py-6 w-full">
        <div className="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-3 2xl:grid-cols-4 gap-4">
          {INTEGRATIONS.map((it) => (
            <IntegrationCard key={it.name} integration={it} />
          ))}
        </div>
      </div>
    </div>
  );
}

function IntegrationCard({ integration }: { integration: Integration }) {
  const { icon: Icon, status } = integration;
  const connected = status === "connected";
  return (
    <div className="rounded-lg border border-[var(--color-border)] bg-[var(--color-surface)] p-4 shadow-stripe-ambient flex flex-col gap-3">
      <div className="flex items-start justify-between gap-2">
        <div className="size-9 rounded-md bg-[var(--color-surface-2)] border border-[var(--color-border)] flex items-center justify-center">
          <Icon size={18} className="text-[var(--color-brand)]" strokeWidth={1.75} />
        </div>
        <span
          className={`inline-flex items-center gap-1.5 rounded-full px-2 py-0.5 text-[11px] font-medium ${
            connected
              ? "bg-[var(--color-success)]/12 text-[var(--color-success)]"
              : "bg-[var(--color-surface-2)] text-[var(--color-fg-muted)]"
          }`}
        >
          <span
            className={`size-1.5 rounded-full ${
              connected ? "bg-[var(--color-success)]" : "bg-[var(--color-fg-subtle)]"
            }`}
          />
          {connected ? "Connected" : "Available"}
        </span>
      </div>

      <div>
        <p className="text-[11px] uppercase tracking-wide text-[var(--color-fg-muted)]">
          {integration.category}
        </p>
        <h3 className="text-sm font-medium text-[var(--color-fg)] mt-0.5">{integration.name}</h3>
      </div>

      <p className="text-xs text-[var(--color-fg-muted)] leading-relaxed">{integration.description}</p>

      <div className="mt-auto pt-2 border-t border-[var(--color-border)] text-xs text-[var(--color-fg-muted)]">
        Used by <span className="text-[var(--color-fg)]">{integration.usedBy}</span>
      </div>
    </div>
  );
}
