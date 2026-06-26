import { Table2 } from "lucide-react";
// DecisionTableView — compact grid showing a consolidated decision
// cascade. One column per shared variable, one row per outgoing
// flow. The Inspector renders this instead of the usual gateway
// "just a diamond" view when AnalyzeDecisionTables detects a
// shared-variable cascade.
export function DecisionTableView({ table }) {
    const vars = table.variables;
    return (<section className="rounded-md border border-[var(--color-border)] bg-[var(--color-surface-2)] overflow-hidden">
      <header className="px-3 py-2 border-b border-[var(--color-border)] flex items-center gap-1.5">
        <Table2 size={11} className="text-[var(--color-brand)]"/>
        <span className="text-[10px] uppercase tracking-[0.14em] text-[var(--color-fg-muted)]" style={{ fontWeight: 500 }}>
          decision table
        </span>
        <span className="flex-1"/>
        <span className="text-[10px] font-mono text-[var(--color-fg-subtle)] tnum">
          {table.rules.length} rules
        </span>
      </header>
      <div className="overflow-x-auto nice-scroll">
        <table className="w-full text-[11px]">
          <thead>
            <tr className="bg-[var(--color-surface)]">
              {vars.map((v) => (<th key={v} className="px-2 py-1.5 text-left font-mono text-[10.5px] text-[var(--color-fg-muted)] border-b border-[var(--color-border)]" style={{ fontWeight: 500 }}>
                  {v}
                </th>))}
              <th className="px-2 py-1.5 text-left text-[10.5px] text-[var(--color-fg-muted)] border-b border-[var(--color-border)]" style={{ fontWeight: 500 }}>
                target
              </th>
            </tr>
          </thead>
          <tbody>
            {table.rules.map((rule) => (<tr key={rule.flow_id} className={`border-b border-[var(--color-border)] last:border-b-0 ${rule.is_default
                ? "bg-[var(--color-accent-bg)]/40"
                : "hover:bg-[var(--color-surface)]"}`}>
                {vars.map((v) => {
                const pred = rule.predicates[v];
                return (<td key={v} className="px-2 py-1.5 font-mono text-[var(--color-fg)] align-top">
                      {pred ?? (<span className="text-[var(--color-fg-subtle)] italic">—</span>)}
                    </td>);
            })}
                <td className="px-2 py-1.5 font-mono text-[var(--color-fg)] align-top">
                  <span className="inline-flex items-center gap-1">
                    <span className="text-[var(--color-fg-subtle)]">→</span>
                    <span>{rule.target}</span>
                    {rule.is_default && (<span className="text-[9.5px] uppercase tracking-[0.12em] text-[var(--color-brand)]" style={{ fontWeight: 500 }}>
                        default
                      </span>)}
                  </span>
                </td>
              </tr>))}
          </tbody>
        </table>
      </div>
      {table.evidence && table.evidence.length > 0 && (<div className="px-3 py-2 border-t border-[var(--color-border)] text-[10.5px] text-[var(--color-fg-muted)] italic space-y-0.5">
          {table.evidence.map((ev, i) => (<div key={i}>“{ev}”</div>))}
        </div>)}
    </section>);
}
