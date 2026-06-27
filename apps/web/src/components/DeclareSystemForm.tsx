import { useState } from "react";
import { Database, Loader2, X } from "lucide-react";

import { useToasts } from "./Toasts";

interface Props {
  onClose: () => void;
  onDeclared: () => void;
}

const KIND_OPTIONS = ["ecm", "erp", "comms", "idp", "crm", "signer", "other"];
const DEFAULT_CAPS: Record<string, string> = {
  ecm: "document.store, document.archive, document.sign",
  erp: "expense.create, expense.approve, invoice.emit",
  comms: "user.notify.email, user.notify.teams",
  idp: "user.resolve, group.resolve",
  crm: "",
  signer: "",
  other: "",
};

export function DeclareSystemForm({ onClose, onDeclared }: Props) {
  const toasts = useToasts();
  const [id, setId] = useState("");
  const [kind, setKind] = useState("ecm");
  const [caps, setCaps] = useState<string>(DEFAULT_CAPS.ecm ?? "");
  const [busy, setBusy] = useState(false);

  return (
    <form
      className="border border-[var(--color-border)] rounded-lg bg-[var(--color-surface)] p-3 space-y-2 relative"
      onSubmit={async (e) => {
        e.preventDefault();
        const capabilities = caps
          .split(/[,\n]/)
          .map((s) => s.trim())
          .filter(Boolean);
        if (!id || capabilities.length === 0) return;
        setBusy(true);
        try {
          await fetch("/api/systems", {
            method: "POST",
            headers: { "content-type": "application/json" },
            body: JSON.stringify({ id, kind, capabilities }),
          });
          toasts.push({
            kind: "success",
            title: "System declared",
            body: `${id} (${kind}) — ${capabilities.length} capabilities`,
          });
          onDeclared();
          onClose();
        } catch (err) {
          toasts.push({
            kind: "error",
            title: "Couldn't declare system",
            body: err instanceof Error ? err.message : String(err),
          });
        } finally {
          setBusy(false);
        }
      }}
    >
      <button
        type="button"
        onClick={onClose}
        className="btn-sm absolute top-1.5 right-1.5 text-[var(--color-fg-subtle)] hover:text-[var(--color-fg)]"
        aria-label="close"
      >
        <X size={12} />
      </button>
      <div className="flex items-center gap-1.5 text-[10px] uppercase tracking-widest text-[var(--color-fg-muted)]">
        <Database size={11} />
        declare a system
      </div>

      <label className="flex flex-col gap-0.5">
        <span className="text-[10px] uppercase tracking-widest text-[var(--color-fg-subtle)]">id</span>
        <input
          value={id}
          onChange={(e) => setId(e.target.value)}
          placeholder="openbee, odoo, m365…"
          className="bg-[var(--color-surface-2)] border border-[var(--color-border)] rounded-md px-2 py-1 text-xs text-[var(--color-fg)] focus:outline-none focus:ring-2 focus:ring-[var(--color-accent-border)] focus:bg-[var(--color-surface)]"
        />
      </label>

      <label className="flex flex-col gap-0.5">
        <span className="text-[10px] uppercase tracking-widest text-[var(--color-fg-subtle)]">kind</span>
        <select
          value={kind}
          onChange={(e) => {
            const k = e.target.value;
            setKind(k);
            setCaps(DEFAULT_CAPS[k] ?? "");
          }}
          className="bg-[var(--color-surface-2)] border border-[var(--color-border)] rounded-md px-2 py-1 text-xs text-[var(--color-fg)]"
        >
          {KIND_OPTIONS.map((k) => (
            <option key={k} value={k}>
              {k}
            </option>
          ))}
        </select>
      </label>

      <label className="flex flex-col gap-0.5">
        <span className="text-[10px] uppercase tracking-widest text-[var(--color-fg-subtle)]">capabilities (comma-separated)</span>
        <textarea
          value={caps}
          onChange={(e) => setCaps(e.target.value)}
          rows={2}
          className="bg-[var(--color-surface-2)] border border-[var(--color-border)] rounded-md px-2 py-1 text-xs text-[var(--color-fg)] font-mono resize-none focus:outline-none focus:ring-2 focus:ring-[var(--color-accent-border)] focus:bg-[var(--color-surface)]"
        />
      </label>

      <button
        type="submit"
        disabled={busy || !id || !caps.trim()}
        className="w-full flex items-center justify-center gap-1.5 text-xs font-medium px-3 py-1.5 rounded-md bg-[var(--color-accent)] text-white hover:brightness-110 disabled:opacity-40"
      >
        {busy ? <Loader2 size={12} className="animate-spin" /> : <Database size={12} />}
        {busy ? "Saving…" : "Declare"}
      </button>
    </form>
  );
}
