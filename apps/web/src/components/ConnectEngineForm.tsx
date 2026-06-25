import { useState } from "react";
import { Loader2, Plug, X } from "lucide-react";

import { api } from "../lib/api";
import { useToasts } from "./Toasts";

interface Props {
  onClose: () => void;
  onConnected: () => void;
}

export function ConnectEngineForm({ onClose, onConnected }: Props) {
  const toasts = useToasts();
  const [id, setId] = useState("local-camunda");
  const [endpoint, setEndpoint] = useState("http://camunda7:8080/engine-rest");
  const [username, setUsername] = useState("demo");
  const [password, setPassword] = useState("demo");
  const [busy, setBusy] = useState(false);

  return (
    <form
      className="border border-[var(--color-border)] rounded-lg bg-[var(--color-surface)] p-3 space-y-2 relative"
      onSubmit={async (e) => {
        e.preventDefault();
        setBusy(true);
        try {
          await api.registerCamunda({ id, endpoint, username, password });
          toasts.push({ kind: "success", title: "Engine connected", body: `${id} synced` });
          onConnected();
          onClose();
        } catch (err) {
          toasts.push({
            kind: "error",
            title: "Couldn't connect",
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
        className="absolute top-1.5 right-1.5 text-[var(--color-fg-subtle)] hover:text-[var(--color-fg)]"
        aria-label="close"
      >
        <X size={12} />
      </button>
      <div className="flex items-center gap-1.5 text-[10px] uppercase tracking-widest text-[var(--color-fg-muted)]">
        <Plug size={11} />
        connect Camunda 7
      </div>
      <Input label="id" value={id} onChange={setId} placeholder="local-camunda" />
      <Input label="endpoint" value={endpoint} onChange={setEndpoint} placeholder="http://camunda7:8080/engine-rest" mono />
      <div className="grid grid-cols-2 gap-2">
        <Input label="user" value={username} onChange={setUsername} placeholder="demo" />
        <Input label="password" value={password} onChange={setPassword} type="password" />
      </div>
      <button
        type="submit"
        disabled={busy}
        className="w-full flex items-center justify-center gap-1.5 text-xs font-medium px-3 py-1.5 rounded-md bg-[var(--color-accent)] text-white hover:brightness-110 disabled:opacity-40"
      >
        {busy ? <Loader2 size={12} className="animate-spin" /> : <Plug size={12} />}
        {busy ? "Connecting…" : "Connect + sync"}
      </button>
    </form>
  );
}

function Input({
  label,
  value,
  onChange,
  placeholder,
  type,
  mono,
}: {
  label: string;
  value: string;
  onChange: (v: string) => void;
  placeholder?: string;
  type?: string;
  mono?: boolean;
}) {
  return (
    <label className="flex flex-col gap-0.5">
      <span className="text-[10px] uppercase tracking-widest text-[var(--color-fg-subtle)]">{label}</span>
      <input
        type={type}
        value={value}
        onChange={(e) => onChange(e.target.value)}
        placeholder={placeholder}
        className={`bg-[var(--color-surface-2)] border border-[var(--color-border)] rounded-md px-2 py-1 text-xs text-[var(--color-fg)] placeholder:text-[var(--color-fg-subtle)] focus:outline-none focus:ring-2 focus:ring-[var(--color-accent-border)] focus:bg-[var(--color-surface)] ${
          mono ? "font-mono" : ""
        }`}
      />
    </label>
  );
}
