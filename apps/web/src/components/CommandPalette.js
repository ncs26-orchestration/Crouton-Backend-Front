import { useEffect, useMemo, useState } from "react";
import { motion } from "framer-motion";
import { Database, Download, ExternalLink, FileText, Plug, RefreshCw, Rocket, Search, Workflow } from "lucide-react";
const GROUP_LABEL = {
    actions: "Actions",
    setup: "Setup",
    navigate: "Navigate",
    links: "Open",
};
export function CommandPalette({ open, onClose, commands }) {
    const [query, setQuery] = useState("");
    const [cursor, setCursor] = useState(0);
    useEffect(() => {
        if (open) {
            setQuery("");
            setCursor(0);
        }
    }, [open]);
    const filtered = useMemo(() => {
        const q = query.trim().toLowerCase();
        if (!q)
            return commands.filter((c) => !c.disabled);
        return commands.filter((c) => !c.disabled &&
            (c.label.toLowerCase().includes(q) || (c.hint && c.hint.toLowerCase().includes(q))));
    }, [commands, query]);
    useEffect(() => {
        if (cursor >= filtered.length)
            setCursor(0);
    }, [cursor, filtered.length]);
    if (!open)
        return null;
    const grouped = filtered.reduce((acc, c) => {
        (acc[c.group] ||= []).push(c);
        return acc;
    }, {});
    const order = ["actions", "setup", "navigate", "links"];
    let flatIdx = 0;
    return (<motion.div initial={{ opacity: 0 }} animate={{ opacity: 1 }} exit={{ opacity: 0 }} transition={{ duration: 0.12 }} className="fixed inset-0 z-50 bg-black/50 backdrop-blur-sm flex items-start justify-center pt-[16vh] px-4" onMouseDown={onClose}>
      <motion.div initial={{ opacity: 0, y: -6, scale: 0.98 }} animate={{ opacity: 1, y: 0, scale: 1 }} exit={{ opacity: 0, y: -4 }} transition={{ type: "spring", stiffness: 380, damping: 30 }} className="w-full max-w-[520px] bg-[var(--color-surface)] border border-[var(--color-border)] rounded-lg shadow-stripe-deep overflow-hidden" onMouseDown={(e) => e.stopPropagation()}>
        {/* Input row — compact, no heavy focus ring */}
        <div className="flex items-center gap-2 px-3 h-11 border-b border-[var(--color-border)]">
          <Search size={14} className="text-[var(--color-fg-subtle)] shrink-0"/>
          <input autoFocus value={query} onChange={(e) => {
            setQuery(e.target.value);
            setCursor(0);
        }} onKeyDown={(e) => {
            if (e.key === "Escape")
                onClose();
            if (e.key === "ArrowDown") {
                e.preventDefault();
                setCursor((c) => Math.min(c + 1, filtered.length - 1));
            }
            if (e.key === "ArrowUp") {
                e.preventDefault();
                setCursor((c) => Math.max(c - 1, 0));
            }
            if (e.key === "Enter" && filtered[cursor]) {
                e.preventDefault();
                filtered[cursor].run();
                onClose();
            }
        }} placeholder="Search for a command…" className="flex-1 bg-transparent text-[13px] text-[var(--color-fg)] placeholder:text-[var(--color-fg-subtle)] focus:outline-none" style={{ fontWeight: 400 }}/>
          <kbd className="shrink-0 text-[10px] font-mono text-[var(--color-fg-subtle)] border border-[var(--color-border)] rounded px-1 py-0.5 bg-[var(--color-surface-2)]">
            esc
          </kbd>
        </div>

        {/* Results */}
        <div className="max-h-[46vh] overflow-y-auto nice-scroll py-1">
          {filtered.length === 0 && (<div className="px-4 py-8 text-center text-[12px] text-[var(--color-fg-subtle)]">
              No commands match <span className="font-mono">{query}</span>
            </div>)}
          {order.map((g) => grouped[g] && grouped[g].length > 0 ? (<div key={g} className="pt-1 pb-0.5">
                <div className="px-3 pt-2 pb-1 text-[10px] uppercase tracking-[0.14em] text-[var(--color-fg-subtle)]" style={{ fontWeight: 400 }}>
                  {GROUP_LABEL[g]}
                </div>
                {grouped[g].map((cmd) => {
                const isActive = cursor === flatIdx;
                const currentIdx = flatIdx;
                flatIdx++;
                const Icon = cmd.icon;
                return (<button key={cmd.id} onMouseEnter={() => setCursor(currentIdx)} onClick={() => {
                        cmd.run();
                        onClose();
                    }} className={`w-full group flex items-center gap-2.5 mx-1 px-2.5 h-8 rounded text-left transition-colors ${isActive
                        ? "bg-[var(--color-accent-bg)] text-[var(--color-fg)]"
                        : "text-[var(--color-fg)] hover:bg-[var(--color-surface-2)]"}`} style={{ fontWeight: 400, width: "calc(100% - 8px)" }}>
                      {Icon ? (<Icon size={13} strokeWidth={1.75} className={isActive ? "text-[var(--color-brand)]" : "text-[var(--color-fg-muted)]"}/>) : (<span className="w-[13px] shrink-0"/>)}
                      <span className="flex-1 text-[13px] truncate">{cmd.label}</span>
                      {cmd.hint && (<span className="text-[10px] font-mono text-[var(--color-fg-subtle)]">
                          {cmd.hint}
                        </span>)}
                    </button>);
            })}
              </div>) : null)}
        </div>

        {/* Footer — keyboard hints */}
        <div className="flex items-center justify-between px-3 h-8 border-t border-[var(--color-border)] bg-[var(--color-surface-2)] text-[10px] text-[var(--color-fg-subtle)]">
          <div className="flex items-center gap-3">
            <span className="flex items-center gap-1">
              <Kbd>↑</Kbd>
              <Kbd>↓</Kbd>
              <span>navigate</span>
            </span>
            <span className="flex items-center gap-1">
              <Kbd>↵</Kbd>
              <span>select</span>
            </span>
          </div>
          <span className="font-mono">⌘K</span>
        </div>
      </motion.div>
    </motion.div>);
}
function Kbd({ children }) {
    return (<kbd className="inline-flex items-center justify-center min-w-[15px] h-[15px] px-1 font-mono text-[10px] border border-[var(--color-border)] rounded bg-[var(--color-surface)] text-[var(--color-fg-muted)]">
      {children}
    </kbd>);
}
// Re-export icons commonly used to build command lists.
export { Database, Download, ExternalLink, FileText, Plug, RefreshCw, Rocket, Workflow };
