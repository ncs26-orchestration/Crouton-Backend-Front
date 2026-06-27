import { ArrowDown, ArrowUp, GripVertical, Plus, X } from "lucide-react";

import type { AgentRosterEntry, WorkflowStep, WorkflowStepEdge } from "../lib/types";
import { departmentColor } from "../lib/department";

// PlanEditor edits an ordered list of workflow steps: each step is a department
// agent, with a display name. Steps run top to bottom; edges are the linear
// sequence between them. Reused for authoring a workflow and editing a draft.

// linearEdges turns an ordered step list into sequential edges (step[i] -> step[i+1]).
export function linearEdges(steps: WorkflowStep[]): WorkflowStepEdge[] {
  const edges: WorkflowStepEdge[] = [];
  for (let i = 0; i < steps.length - 1; i++) {
    edges.push({ from: steps[i]!.key, to: steps[i + 1]!.key, type: "sequence" });
  }
  return edges;
}

function newKey(): string {
  return "n_" + Math.random().toString(36).slice(2, 8);
}

export function PlanEditor({
  steps,
  agents,
  onChange,
}: {
  steps: WorkflowStep[];
  agents: AgentRosterEntry[];
  onChange: (steps: WorkflowStep[]) => void;
}) {
  // One option per department agent (skip duplicates by agent_type).
  const options = agents.filter(
    (a, i) => agents.findIndex((b) => b.agent_type === a.agent_type) === i,
  );

  const update = (i: number, patch: Partial<WorkflowStep>) =>
    onChange(steps.map((s, idx) => (idx === i ? { ...s, ...patch } : s)));

  const move = (i: number, dir: -1 | 1) => {
    const j = i + dir;
    if (j < 0 || j >= steps.length) return;
    const next = [...steps];
    [next[i], next[j]] = [next[j]!, next[i]!];
    onChange(next);
  };

  const remove = (i: number) => onChange(steps.filter((_, idx) => idx !== i));

  const add = () => {
    const first = options[0];
    onChange([
      ...steps,
      {
        key: newKey(),
        name: first ? `${first.team_name || "Department"} Review` : "New step",
        agent_type: first?.agent_type ?? "",
        department: first?.team_name ?? "",
      },
    ]);
  };

  return (
    <div className="flex flex-col gap-2">
      {steps.length === 0 && (
        <p className="text-xs text-[var(--color-fg-subtle)] py-2">
          No steps yet. Add the departments this process runs through, in order.
        </p>
      )}
      {steps.map((s, i) => (
        <div
          key={s.key}
          className="flex items-center gap-2 rounded-lg border border-[var(--color-border)] bg-[var(--color-surface)] p-2"
        >
          <GripVertical size={14} className="text-[var(--color-fg-subtle)] shrink-0" />
          <span
            className="size-2 rounded-full shrink-0"
            style={{ background: departmentColor(s.department || "?") }}
          />
          <input
            value={s.name}
            onChange={(e) => update(i, { name: e.target.value })}
            placeholder="Step name"
            className="flex-1 min-w-0 rounded border border-[var(--color-border)] bg-[var(--color-bg)] px-2 py-1 text-xs text-[var(--color-fg)] focus:outline-none focus:ring-1 focus:ring-[var(--color-brand)]"
          />
          <select
            value={s.agent_type}
            onChange={(e) => {
              const a = options.find((o) => o.agent_type === e.target.value);
              update(i, {
                agent_type: e.target.value,
                department: a?.team_name ?? s.department,
              });
            }}
            className="shrink-0 rounded border border-[var(--color-border)] bg-[var(--color-bg)] px-2 py-1 text-xs text-[var(--color-fg)] focus:outline-none focus:ring-1 focus:ring-[var(--color-brand)]"
          >
            {options.length === 0 && <option value="">No agents</option>}
            {options.map((o) => (
              <option key={o.agent_type} value={o.agent_type}>
                {o.team_name || o.agent_type}
              </option>
            ))}
          </select>
          <div className="flex items-center shrink-0">
            <button
              type="button"
              onClick={() => move(i, -1)}
              disabled={i === 0}
              className="size-6 flex items-center justify-center rounded text-[var(--color-fg-subtle)] hover:text-[var(--color-fg)] disabled:opacity-30"
              title="Move up"
            >
              <ArrowUp size={13} />
            </button>
            <button
              type="button"
              onClick={() => move(i, 1)}
              disabled={i === steps.length - 1}
              className="size-6 flex items-center justify-center rounded text-[var(--color-fg-subtle)] hover:text-[var(--color-fg)] disabled:opacity-30"
              title="Move down"
            >
              <ArrowDown size={13} />
            </button>
            <button
              type="button"
              onClick={() => remove(i)}
              className="size-6 flex items-center justify-center rounded text-[var(--color-fg-subtle)] hover:text-[var(--color-danger)]"
              title="Remove step"
            >
              <X size={13} />
            </button>
          </div>
        </div>
      ))}
      <button
        type="button"
        onClick={add}
        disabled={options.length === 0}
        className="self-start flex items-center gap-1.5 rounded-md border border-dashed border-[var(--color-border)] px-2.5 py-1.5 text-xs font-medium text-[var(--color-fg-muted)] hover:text-[var(--color-fg)] hover:border-[var(--color-fg-subtle)] disabled:opacity-40 transition-colors"
      >
        <Plus size={13} /> Add step
      </button>
    </div>
  );
}
