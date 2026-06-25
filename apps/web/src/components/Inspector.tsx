import { motion } from "framer-motion";
import { Cog, FileCode2, Sparkles, User, X } from "lucide-react";

import {
  TIER_COLOR,
  TIER_LABEL,
  TIER_TEXT,
  taskConfidence,
  tierOf,
} from "../lib/confidence";
import type { ISRegistry, Task, Workflow } from "../lib/types";

interface Props {
  workflow: Workflow | null;
  selectedTaskId: string | null;
  is?: ISRegistry;
  onClose: () => void;
}

export function Inspector({ workflow, selectedTaskId, is, onClose }: Props) {
  if (!workflow || !selectedTaskId) return null;
  const task = workflow.tasks.find((t) => t.id === selectedTaskId);
  if (!task) return null;

  const Icon = task.type === "user" ? User : task.type === "service" ? Cog : FileCode2;

  const binding = task.binding;
  const assignee = binding?.assignee_user_id
    ? is?.users.find((u) => u.id === binding.assignee_user_id)
    : undefined;
  const group = binding?.candidate_group_id
    ? is?.groups.find((g) => g.id === binding.candidate_group_id)
    : undefined;
  const system = binding?.system_ref ? is?.systems.find((s) => s.id === binding.system_ref) : undefined;
  const capabilityValid =
    system && binding?.capability ? system.capabilities.includes(binding.capability) : false;
  const actor = task.actor_ref ? workflow.actors.find((a) => a.id === task.actor_ref) : undefined;
  const form = task.form_ref ? workflow.forms?.find((f) => f.id === task.form_ref) : undefined;

  return (
    <motion.aside
      key={task.id}
      initial={{ x: 40, opacity: 0 }}
      animate={{ x: 0, opacity: 1 }}
      exit={{ x: 40, opacity: 0 }}
      transition={{ type: "spring", stiffness: 320, damping: 30 }}
      className="w-80 shrink-0 bg-[var(--color-surface)] border-l border-[var(--color-border)] flex flex-col h-full">
      <div className="px-4 py-3 border-b border-[var(--color-border)] flex items-center justify-between">
        <div className="flex items-center gap-2 min-w-0">
          <Icon size={14} strokeWidth={2} className="text-[var(--color-fg)] shrink-0" />
          <div className="min-w-0">
            <div className="text-[10px] uppercase tracking-widest text-[var(--color-fg-subtle)]">{task.type} task</div>
            <div className="text-sm font-medium text-[var(--color-fg)] truncate">{task.name}</div>
          </div>
        </div>
        <button onClick={onClose} className="text-[var(--color-fg-subtle)] hover:text-[var(--color-fg)]" aria-label="close inspector">
          <X size={14} />
        </button>
      </div>

      <div className="flex-1 overflow-y-auto nice-scroll px-4 py-3 space-y-4 text-xs">
        <WhyThis task={task} />

        <Field label="id">
          <code className="text-[11px] text-[var(--color-fg)]">{task.id}</code>
        </Field>

        {actor && (
          <Field label="actor">
            <div className="flex flex-col gap-0.5">
              <span className="font-medium text-[var(--color-fg)]">{actor.name}</span>
              <code className="text-[11px] text-[var(--color-fg-muted)]">
                {actor.kind} · {actor.id}
              </code>
              {actor.is_ref?.user_id && (
                <code className="text-[11px] text-emerald-600 dark:text-emerald-400">is_ref.user_id = {actor.is_ref.user_id}</code>
              )}
              {actor.is_ref?.group_id && (
                <code className="text-[11px] text-emerald-600 dark:text-emerald-400">is_ref.group_id = {actor.is_ref.group_id}</code>
              )}
            </div>
          </Field>
        )}

        <Field label="binding">
          {!binding && <span className="italic text-[var(--color-fg-subtle)]">unbound — will compile to a human stub</span>}
          {binding && (
            <div className="flex flex-col gap-1.5">
              {binding.assignee_user_id && (
                <Provenance
                  ok={!!assignee}
                  reference={binding.assignee_user_id}
                  kind="assignee"
                  resolved={assignee ? `${assignee.name} (${assignee.id})` : "NOT FOUND"}
                />
              )}
              {binding.candidate_group_id && (
                <Provenance
                  ok={!!group}
                  reference={binding.candidate_group_id}
                  kind="group"
                  resolved={group ? `${group.name} (${group.id})` : "NOT FOUND"}
                />
              )}
              {binding.system_ref && (
                <>
                  <Provenance
                    ok={!!system}
                    reference={binding.system_ref}
                    kind="system"
                    resolved={system ? `${system.id} · ${system.kind}` : "NOT FOUND"}
                  />
                  {binding.capability && (
                    <Provenance
                      ok={capabilityValid}
                      reference={binding.capability}
                      kind="capability"
                      resolved={
                        capabilityValid
                          ? `declared by ${system?.id}`
                          : system
                          ? `${system.id} does not declare this`
                          : "system unknown"
                      }
                    />
                  )}
                </>
              )}
              {binding.form_key && (
                <Provenance
                  ok={!!is?.deployed_forms?.some((f) => f.form_key === binding.form_key)}
                  reference={binding.form_key}
                  kind="form_key"
                  resolved={
                    is?.deployed_forms?.some((f) => f.form_key === binding.form_key)
                      ? "deployed on engine"
                      : "not deployed"
                  }
                />
              )}
              {binding.params && Object.keys(binding.params).length > 0 && (
                <div className="mt-1">
                  <div className="text-[10px] uppercase tracking-widest text-[var(--color-fg-subtle)]">params</div>
                  <pre className="text-[11px] font-mono bg-[var(--color-surface-2)] border border-[var(--color-border)] rounded-md p-2 mt-1 overflow-x-auto text-[var(--color-fg)]">
                    {JSON.stringify(binding.params, null, 2)}
                  </pre>
                </div>
              )}
            </div>
          )}
        </Field>

        {form && (
          <Field label="form">
            <div className="flex flex-col gap-0.5">
              <code className="text-[11px] text-[var(--color-fg)]">{form.id}</code>
              <ul className="mt-1 space-y-0.5">
                {form.fields.map((f) => (
                  <li key={f.id} className="text-[11px] text-[var(--color-fg-muted)] flex items-center gap-1.5">
                    <code className="text-[var(--color-fg)]">{f.id}</code>
                    <span className="text-[var(--color-fg-subtle)]">:</span>
                    <span>{f.type}</span>
                    {f.required && <span className="text-rose-500 text-[10px]">req</span>}
                  </li>
                ))}
              </ul>
            </div>
          </Field>
        )}

        <Field label="compiles to">
          <CompileHint task={task} />
        </Field>
      </div>
    </motion.aside>
  );
}

function Field({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div>
      <div className="text-[10px] uppercase tracking-widest text-[var(--color-fg-subtle)] mb-1">{label}</div>
      <div>{children}</div>
    </div>
  );
}

function Provenance({
  ok,
  reference,
  kind,
  resolved,
}: {
  ok: boolean;
  reference: string;
  kind: string;
  resolved: string;
}) {
  return (
    <div
      className={`border rounded-md px-2 py-1.5 ${
        ok ? "border-emerald-500/30 bg-emerald-500/10" : "border-rose-500/30 bg-rose-500/10"
      }`}
    >
      <div className="flex items-center gap-1.5 text-[10.5px]">
        <span className="text-[var(--color-fg-subtle)] uppercase tracking-widest">{kind}</span>
        <code className={ok ? "text-emerald-600 dark:text-emerald-300" : "text-rose-600 dark:text-rose-300"}>
          {reference}
        </code>
      </div>
      <div className={`text-[11px] mt-0.5 ${ok ? "text-emerald-600 dark:text-emerald-400" : "text-rose-600 dark:text-rose-400"}`}>
        {ok ? "→ " : "✗ "}
        {resolved}
      </div>
    </div>
  );
}

function CompileHint({ task }: { task: Task }) {
  const b = task.binding;
  if (task.type === "user") {
    const parts: string[] = [];
    if (b?.assignee_user_id) parts.push(`camunda:assignee="${b.assignee_user_id}"`);
    if (b?.candidate_group_id) parts.push(`camunda:candidateGroups="${b.candidate_group_id}"`);
    if (b?.form_key) parts.push(`camunda:formKey="${b.form_key}"`);
    return (
      <pre className="text-[11px] font-mono bg-[var(--color-surface-2)] border border-[var(--color-border)] rounded-md p-2 whitespace-pre-wrap text-[var(--color-fg)]">
{`<bpmn:userTask id="${task.id}" name=${JSON.stringify(task.name)}${parts.length ? "\n  " + parts.join("\n  ") : ""} />`}
      </pre>
    );
  }
  if (task.type === "service") {
    const topic = b?.system_ref && b?.capability ? `${b.system_ref}.${b.capability}` : "";
    return (
      <pre className="text-[11px] font-mono bg-[var(--color-surface-2)] border border-[var(--color-border)] rounded-md p-2 whitespace-pre-wrap text-[var(--color-fg)]">
{`<bpmn:serviceTask id="${task.id}" name=${JSON.stringify(task.name)}
  camunda:type="external"${topic ? `\n  camunda:topic="${topic}"` : ""} />`}
      </pre>
    );
  }
  return (
    <pre className="text-[11px] font-mono bg-[var(--color-surface-2)] border border-[var(--color-border)] rounded-md p-2 text-[var(--color-fg)]">
{`<bpmn:scriptTask id="${task.id}" name=${JSON.stringify(task.name)} />`}
    </pre>
  );
}

// WhyThis — the "no hallucination" pillar made visible. Shows where
// the extractor got this task from, how sure it was, and which
// evidence quote drove it.
function WhyThis({ task }: { task: Task }) {
  const conf = taskConfidence(task);
  const tier = tierOf(conf);
  const pct = conf != null ? Math.round(conf * 100) : null;
  // Task-level evidence takes priority over binding-level.
  const evidence = task.evidence || task.binding?.evidence || "";

  // Suppress the panel entirely if the extractor emitted no
  // confidence and no evidence — nothing useful to show.
  if (conf == null && !evidence) return null;

  return (
    <section className="rounded-md border border-[var(--color-border)] bg-[var(--color-surface-2)] px-3 py-2.5 space-y-2">
      <header className="flex items-center justify-between">
        <div className="flex items-center gap-1.5 text-[10px] uppercase tracking-[0.14em] text-[var(--color-fg-muted)]" style={{ fontWeight: 500 }}>
          <Sparkles size={10} className={TIER_TEXT[tier]} />
          why this
        </div>
        {pct !== null && (
          <span className={`text-[10px] font-mono tnum ${TIER_TEXT[tier]}`}>
            {pct}% · {TIER_LABEL[tier]}
          </span>
        )}
      </header>

      {conf != null && (
        <div
          className="h-[4px] rounded-full bg-[var(--color-surface)] overflow-hidden"
          role="meter"
          aria-valuemin={0}
          aria-valuemax={100}
          aria-valuenow={pct ?? 0}
          title={`Extractor confidence ${pct}%`}
        >
          <div
            className={`h-full ${TIER_COLOR[tier]} transition-[width] duration-300`}
            style={{ width: `${conf * 100}%` }}
          />
        </div>
      )}

      {evidence && (
        <blockquote className="text-[12px] leading-snug text-[var(--color-fg)] border-l-2 border-[var(--color-brand)] pl-2 italic">
          &ldquo;{evidence}&rdquo;
        </blockquote>
      )}
    </section>
  );
}
