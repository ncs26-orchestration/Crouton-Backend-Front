import { Check, FileText, Rocket, Settings2 } from "lucide-react";
import { motion } from "framer-motion";
const STAGES = [
    { id: "process", label: "Process", Icon: FileText },
    { id: "executable", label: "Executable", Icon: Settings2 },
    { id: "deployed", label: "Deployed", Icon: Rocket },
];
const ORDER = { process: 0, executable: 1, deployed: 2 };
export function PipelineChips({ active, furthest, onStageClick }) {
    return (<div className="flex items-center gap-1" role="navigation" aria-label="compilation pipeline">
      {STAGES.map((s, idx) => {
            const isActive = s.id === active;
            const isDone = ORDER[s.id] < ORDER[furthest] || (ORDER[s.id] === ORDER[furthest] && !isActive);
            const isPending = !isActive && !isDone;
            const clickable = !isPending && !!onStageClick;
            return (<div key={s.id} className="flex items-center gap-1">
            <motion.button onClick={() => clickable && onStageClick(s.id)} disabled={!clickable} whileTap={clickable ? { scale: 0.96 } : undefined} className={`relative flex items-center gap-1.5 text-[10px] font-mono px-2 py-0.5 border rounded-full transition-colors ${isActive
                    ? "border-[var(--color-brand)] text-[var(--color-brand)] bg-[var(--color-accent-bg)]"
                    : isDone
                        ? "border-emerald-500/40 text-emerald-600 dark:text-emerald-400 hover:bg-emerald-500/5"
                        : "border-[var(--color-border)] text-[var(--color-fg-subtle)]"} ${clickable ? "cursor-pointer" : "cursor-default"}`} style={{ fontWeight: 500 }} aria-current={isActive ? "step" : undefined}>
              {isDone ? (<Check size={10} strokeWidth={2.5} className="text-emerald-500"/>) : (<s.Icon size={10} strokeWidth={2}/>)}
              <span className="uppercase tracking-[0.1em]">{s.label}</span>
              {isActive && (<motion.span layoutId="pipeline-chip-indicator" className="absolute -bottom-[5px] left-1/2 -translate-x-1/2 size-1 rounded-full bg-[var(--color-brand)]" transition={{ type: "spring", stiffness: 500, damping: 26 }}/>)}
            </motion.button>
            {idx < STAGES.length - 1 && (<div className={`w-3 h-px ${ORDER[s.id] < ORDER[furthest] ? "bg-emerald-500/50" : "bg-[var(--color-border)]"}`} aria-hidden/>)}
          </div>);
        })}
    </div>);
}
