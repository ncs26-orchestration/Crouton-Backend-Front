import { useMemo } from "react";
import { useQuery } from "@tanstack/react-query";
import { ScrollText } from "lucide-react";

import { api } from "../lib/api";
import type { DepartmentPolicy } from "../lib/types";

interface PolicyGroup {
  department: string;
  policies: DepartmentPolicy[];
}

export function PoliciesView({ orgId }: { orgId: string }) {
  const { data, isLoading, isError, error } = useQuery({
    queryKey: ["policies", orgId],
    queryFn: () => api.listPolicies(orgId),
  });

  const groups = useMemo(() => groupByDepartment(data?.policies ?? []), [data]);

  return (
    <div className="flex-1 flex flex-col bg-[var(--color-bg)] text-[var(--color-fg)] overflow-auto nice-scroll">
      <div className="border-b border-[var(--color-border)] px-8 py-5">
        <h1 className="text-xl font-medium" style={{ fontFeatureSettings: '"ss01"' }}>
          Policies
        </h1>
        <p className="text-sm text-[var(--color-fg-muted)] mt-0.5">
          The department policies agents consult while reasoning. Read-only.
        </p>
      </div>

      <div className="px-8 py-6 w-full max-w-[820px]">
        {isLoading ? (
          <div className="flex flex-col gap-3">
            {Array.from({ length: 4 }).map((_, i) => (
              <div key={i} className="h-24 rounded-lg bg-[var(--color-surface-2)] animate-pulse" />
            ))}
          </div>
        ) : isError ? (
          <p className="text-sm text-[var(--color-danger)]">
            Could not load policies. {(error as Error)?.message}
          </p>
        ) : groups.length === 0 ? (
          <EmptyState />
        ) : (
          <div className="flex flex-col gap-8">
            {groups.map((g) => (
              <section key={g.department}>
                <h2 className="text-[11px] font-semibold uppercase tracking-wide text-[var(--color-fg-muted)] mb-3">
                  {g.department}
                </h2>
                <div className="flex flex-col gap-3">
                  {g.policies.map((p) => (
                    <PolicyCard key={p.id} policy={p} />
                  ))}
                </div>
              </section>
            ))}
          </div>
        )}
      </div>
    </div>
  );
}

function groupByDepartment(policies: DepartmentPolicy[]): PolicyGroup[] {
  const byDept = new Map<string, DepartmentPolicy[]>();
  for (const p of policies) {
    const dept = p.team_name || "General";
    const list = byDept.get(dept) ?? [];
    list.push(p);
    byDept.set(dept, list);
  }
  return [...byDept.entries()]
    .map(([department, list]) => ({ department, policies: list }))
    .sort((a, b) => a.department.localeCompare(b.department));
}

function PolicyCard({ policy }: { policy: DepartmentPolicy }) {
  return (
    <article className="rounded-lg border border-[var(--color-border)] bg-[var(--color-surface)] p-4 shadow-stripe-ambient">
      <div className="flex items-start gap-3">
        <div className="size-9 rounded-md bg-[var(--color-accent-bg)] border border-[var(--color-accent-border)] flex items-center justify-center shrink-0">
          <ScrollText size={16} className="text-[var(--color-brand)]" strokeWidth={1.75} />
        </div>
        <div className="min-w-0">
          <h3 className="text-sm font-medium text-[var(--color-fg)]">{policy.title}</h3>
          <p className="text-xs text-[var(--color-fg-muted)] leading-relaxed mt-1">{policy.body}</p>
        </div>
      </div>
    </article>
  );
}

function EmptyState() {
  return (
    <div className="flex flex-col items-center justify-center gap-4 py-20">
      <div className="size-14 rounded-2xl bg-[var(--color-accent-bg)] border border-[var(--color-accent-border)] flex items-center justify-center">
        <ScrollText size={24} className="text-[var(--color-brand)]" strokeWidth={1.5} />
      </div>
      <div className="text-center">
        <p className="text-sm font-medium text-[var(--color-fg)]">No policies yet</p>
        <p className="text-xs text-[var(--color-fg-muted)] mt-1">
          Department policies are seeded when the organization is created.
        </p>
      </div>
    </div>
  );
}
