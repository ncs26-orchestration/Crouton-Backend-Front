import {
  DollarSign,
  Scale,
  Server,
  Users,
  Boxes,
  ShieldCheck,
  type LucideIcon,
} from "lucide-react";

// The policies each department agent checks a request against. Reference
// content for now (PRD F9 calls for a seeded department_policies table; this
// is the read-only browser over that data, populated with the rules the demo
// agents reason about).
interface Policy {
  title: string;
  rule: string;
}
interface DepartmentPolicies {
  department: string;
  icon: LucideIcon;
  agent: string;
  policies: Policy[];
}

const DEPARTMENTS: DepartmentPolicies[] = [
  {
    department: "Finance",
    icon: DollarSign,
    agent: "Finance Reviewer",
    policies: [
      { title: "Spend approval threshold", rule: "Any spend above $50,000 requires executive sign-off before procurement." },
      { title: "ROI floor", rule: "Capital requests must show a projected payback within 24 months." },
      { title: "Budget headroom", rule: "Department budgets may not be committed beyond 90% without finance review." },
    ],
  },
  {
    department: "Legal",
    icon: Scale,
    agent: "Legal Reviewer",
    policies: [
      { title: "Contract review", rule: "All vendor contracts over 12 months are reviewed before signature." },
      { title: "Data residency", rule: "Personal data must stay within approved regions unless legal clears a transfer." },
      { title: "Regulated markets", rule: "Expansion into a new country triggers a local-compliance check." },
    ],
  },
  {
    department: "IT",
    icon: Server,
    agent: "IT Manager",
    policies: [
      { title: "Security baseline", rule: "New systems must pass an access-control and encryption review." },
      { title: "Vendor risk", rule: "Third-party tools handling company data need a security assessment." },
      { title: "Capacity", rule: "Infrastructure changes estimate cost and headroom before approval." },
    ],
  },
  {
    department: "HR",
    icon: Users,
    agent: "HR Manager",
    policies: [
      { title: "Headcount approval", rule: "New roles are confirmed against the approved hiring plan." },
      { title: "Onboarding", rule: "Every hire gets equipment, access, and a 30-day onboarding plan." },
      { title: "Policy alignment", rule: "Workplace changes are checked against current people policies." },
    ],
  },
  {
    department: "Operations",
    icon: Boxes,
    agent: "Operations Manager",
    policies: [
      { title: "Facilities", rule: "Site changes confirm space, lease, and timeline feasibility." },
      { title: "Logistics SLA", rule: "Procurement and delivery plans target the request's deadline." },
      { title: "Continuity", rule: "Operational changes assess impact on existing services." },
    ],
  },
  {
    department: "Executive",
    icon: ShieldCheck,
    agent: "Executive Approver",
    policies: [
      { title: "Gate criteria", rule: "Approve only when every upstream review is complete or resolved." },
      { title: "Written rationale", rule: "Each decision records a justification in the audit trail." },
      { title: "Escalation", rule: "Unresolved risk flags hold the request instead of auto-approving." },
    ],
  },
];

export function PoliciesView() {
  return (
    <div className="flex-1 flex flex-col bg-[var(--color-bg)] text-[var(--color-fg)] overflow-auto nice-scroll">
      <div className="border-b border-[var(--color-border)] px-8 py-5">
        <h1 className="text-xl font-medium" style={{ fontFeatureSettings: '"ss01"' }}>
          Policies
        </h1>
        <p className="text-sm text-[var(--color-fg-muted)] mt-0.5">
          The rules each department agent checks a request against
        </p>
      </div>

      <div className="px-8 py-6 w-full">
        <div className="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-3 gap-4">
          {DEPARTMENTS.map((d) => (
            <DepartmentCard key={d.department} dept={d} />
          ))}
        </div>
      </div>
    </div>
  );
}

function DepartmentCard({ dept }: { dept: DepartmentPolicies }) {
  const { icon: Icon } = dept;
  return (
    <div className="rounded-lg border border-[var(--color-border)] bg-[var(--color-surface)] p-4 shadow-stripe-ambient flex flex-col gap-3">
      <div className="flex items-center gap-2.5">
        <div className="size-9 rounded-md bg-[var(--color-accent-bg)] border border-[var(--color-accent-border)] flex items-center justify-center shrink-0">
          <Icon size={18} className="text-[var(--color-brand)]" strokeWidth={1.75} />
        </div>
        <div>
          <h3 className="text-sm font-medium text-[var(--color-fg)]">{dept.department}</h3>
          <p className="text-[11px] text-[var(--color-fg-muted)]">{dept.agent}</p>
        </div>
      </div>

      <ul className="flex flex-col gap-2.5">
        {dept.policies.map((p) => (
          <li key={p.title}>
            <p className="text-xs font-medium text-[var(--color-fg)]">{p.title}</p>
            <p className="text-xs text-[var(--color-fg-muted)] leading-snug mt-0.5">{p.rule}</p>
          </li>
        ))}
      </ul>
    </div>
  );
}
