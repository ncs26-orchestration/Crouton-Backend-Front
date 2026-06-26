import { useEffect, useRef, useState } from "react";
import { ArrowRight, Loader2, Sparkles } from "lucide-react";
const PLACEHOLDER = "Describe a process in plain language. Example: Quand un employé soumet une note de frais…";
export function Composer({ onSubmit, busy, sampleText, initialText, onAfterLoadSample }) {
    const [text, setText] = useState(initialText ?? "");
    const [focused, setFocused] = useState(false);
    const ref = useRef(null);
    useEffect(() => {
        const el = ref.current;
        if (!el)
            return;
        el.style.height = "auto";
        el.style.height = Math.min(Math.max(el.scrollHeight, 44), 160) + "px";
    }, [text]);
    useEffect(() => {
        if (initialText) {
            setText(initialText);
            onAfterLoadSample?.();
        }
        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, []);
    return (<form className="border-t border-[var(--color-border)] bg-[var(--color-surface)]" onSubmit={(e) => {
            e.preventDefault();
            if (text.trim() && !busy)
                onSubmit(text.trim());
        }}>
      <div className="px-5 py-3 mx-auto w-full max-w-3xl">
        {/* Indeterminate progress bar — shown only while Pablo is
            thinking. Gives the user a visible "something is
            happening" cue even though we don't actually stream
            tokens yet. */}
        {busy && (<div className="h-[2px] mb-2 rounded overflow-hidden bg-[var(--color-surface-2)]">
            <div className="h-full w-1/3 rounded bg-[var(--color-brand)] anim-indeterminate"/>
          </div>)}
        <div className={`rounded-lg border transition-all ${focused
            ? "border-[var(--color-brand-light)] bg-[var(--color-surface)]"
            : "border-[var(--color-border)] bg-[var(--color-surface-2)]"}`}>
          <div className="flex items-center justify-between px-3 pt-2">
            <label className="flex items-center gap-1.5 text-[10px] uppercase tracking-[0.14em] text-[var(--color-fg-subtle)]" style={{ fontWeight: 400 }}>
              <Sparkles size={11} className="text-[var(--color-brand)]"/>
              describe the process
            </label>
            {sampleText && !text && (<button type="button" disabled={busy} onClick={() => setText(sampleText)} className="text-[11px] text-[var(--color-fg-muted)] hover:text-[var(--color-brand)] disabled:opacity-40 transition-colors" style={{ fontWeight: 400 }}>
                load sample →
              </button>)}
          </div>
          <div className="flex items-end gap-3 px-3 pb-2.5 pt-1">
            <textarea ref={ref} value={text} onChange={(e) => setText(e.target.value)} onFocus={() => setFocused(true)} onBlur={() => setFocused(false)} placeholder={PLACEHOLDER} rows={1} disabled={busy} className="flex-1 min-w-0 resize-none bg-transparent text-[14px] text-[var(--color-fg)] placeholder:text-[var(--color-fg-subtle)] focus:outline-none disabled:opacity-60 leading-[1.55]" style={{ fontWeight: 300 }} onKeyDown={(e) => {
            if ((e.metaKey || e.ctrlKey) && e.key === "Enter" && text.trim() && !busy) {
                onSubmit(text.trim());
            }
        }}/>
            <button type="submit" disabled={!text.trim() || busy} className="shrink-0 inline-flex items-center gap-1.5 text-[13px] px-3 py-1.5 rounded-md bg-[var(--color-brand)] text-white hover:bg-[var(--color-brand-hover)] disabled:opacity-40 disabled:cursor-not-allowed transition-colors" style={{ fontWeight: 400 }} aria-label="Extract workflow">
              {busy ? <Loader2 size={13} className="animate-spin"/> : <ArrowRight size={13}/>}
              <span>{busy ? "Extracting…" : "Extract"}</span>
              <kbd className="hidden md:inline font-mono text-[10px] text-white/70 ml-1 border border-white/20 rounded px-1 py-0.5">⌘↵</kbd>
            </button>
          </div>
        </div>
      </div>
    </form>);
}
