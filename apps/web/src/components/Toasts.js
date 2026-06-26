import { createContext, useCallback, useContext, useEffect, useState } from "react";
import { AnimatePresence, motion } from "framer-motion";
import { AlertTriangle, CheckCircle2, Info, X } from "lucide-react";
const Ctx = createContext(null);
export function useToasts() {
    const ctx = useContext(Ctx);
    if (!ctx)
        throw new Error("useToasts must be used inside <ToastProvider>");
    return ctx;
}
let idCounter = 0;
export function ToastProvider({ children }) {
    const [toasts, setToasts] = useState([]);
    const push = useCallback((t) => {
        const id = ++idCounter;
        setToasts((xs) => [...xs, { id, ttlMs: 5000, ...t }]);
    }, []);
    const dismiss = useCallback((id) => {
        setToasts((xs) => xs.filter((t) => t.id !== id));
    }, []);
    return (<Ctx.Provider value={{ push, dismiss }}>
      {children}
      <div className="fixed z-50 bottom-4 right-4 flex flex-col gap-2 max-w-sm">
        <AnimatePresence initial={false}>
          {toasts.map((t) => (<ToastCard key={t.id} toast={t} onDismiss={() => dismiss(t.id)}/>))}
        </AnimatePresence>
      </div>
    </Ctx.Provider>);
}
function ToastCard({ toast, onDismiss }) {
    useEffect(() => {
        if (!toast.ttlMs)
            return;
        const id = window.setTimeout(onDismiss, toast.ttlMs);
        return () => window.clearTimeout(id);
    }, [toast.ttlMs, onDismiss]);
    const tone = toast.kind === "success"
        ? "border-emerald-500/30 bg-emerald-500/10"
        : toast.kind === "error"
            ? "border-rose-500/30 bg-rose-500/10"
            : "border-[var(--color-border)] bg-[var(--color-surface)]";
    const Icon = toast.kind === "success" ? CheckCircle2 : toast.kind === "error" ? AlertTriangle : Info;
    const iconTone = toast.kind === "success" ? "text-emerald-500" : toast.kind === "error" ? "text-rose-500" : "text-[var(--color-fg-muted)]";
    return (<motion.div layout initial={{ opacity: 0, x: 20, scale: 0.96 }} animate={{ opacity: 1, x: 0, scale: 1 }} exit={{ opacity: 0, x: 20, scale: 0.96, transition: { duration: 0.15 } }} transition={{ type: "spring", stiffness: 340, damping: 26 }} className={`relative border rounded-lg shadow-stripe-elevated px-3 py-2.5 pr-8 text-sm backdrop-blur-sm ${tone}`} role="status">
      <button onClick={onDismiss} className="absolute top-1.5 right-1.5 text-[var(--color-fg-subtle)] hover:text-[var(--color-fg)]" aria-label="dismiss">
        <X size={12}/>
      </button>
      <div className="flex items-start gap-2">
        <Icon size={14} className={`mt-0.5 shrink-0 ${iconTone}`}/>
        <div className="flex flex-col gap-1 min-w-0">
          <div className="font-medium text-[var(--color-fg)]">{toast.title}</div>
          {toast.body && <div className="text-[12px] text-[var(--color-fg-muted)]">{toast.body}</div>}
          {toast.action && (<a href={toast.action.href} target="_blank" rel="noreferrer" className="text-[12px] font-medium text-[var(--color-accent)] hover:underline">
              {toast.action.label} →
            </a>)}
        </div>
      </div>
    </motion.div>);
}
