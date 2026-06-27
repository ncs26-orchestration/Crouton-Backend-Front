import { useState, type FormEvent } from "react";
import { api } from "../lib/api";
import { useAuth } from "../contexts/AuthContext";
import { BrandMark } from "../components/Brand";

interface Props {
  onGoRegister: () => void;
}

export function LoginView({ onGoRegister }: Props) {
  const { login } = useAuth();
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);

  async function handleSubmit(e: FormEvent) {
    e.preventDefault();
    setError(null);
    setLoading(true);
    try {
      const res = await api.login({ email, password });
      login(res.token, res.user);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Login failed");
    } finally {
      setLoading(false);
    }
  }

  return (
    <div
      className="h-screen w-screen flex items-center justify-center bg-[var(--color-bg)]"
    >
      <div
        className="w-full max-w-sm rounded-xl border border-[var(--color-border)] bg-[var(--color-surface)] p-6 md:p-8 shadow-lg flex flex-col gap-6 mx-4"
      >
        {/* Brand */}
        <div className="flex flex-col items-center gap-2">
          <BrandMark size={36} />
          <h1 className="text-lg font-semibold text-[var(--color-fg)] mt-1">Sign in to Crouton</h1>
          <p className="text-sm text-[var(--color-fg-muted)]">Enterprise AI Organization OS</p>
        </div>

        {/* Form */}
        <form onSubmit={handleSubmit} className="flex flex-col gap-4">
          <div className="flex flex-col gap-1.5">
            <label className="text-xs font-medium text-[var(--color-fg-muted)] uppercase tracking-wide" htmlFor="login-email">
              Email
            </label>
            <input
              id="login-email"
              type="email"
              required
              autoComplete="email"
              value={email}
              onChange={(e) => setEmail(e.target.value)}
              placeholder="you@company.com"
              className="rounded-lg border border-[var(--color-border)] bg-[var(--color-bg)] px-3 py-2 text-sm text-[var(--color-fg)] placeholder-[var(--color-fg-muted)] outline-none focus:border-[var(--color-brand)] focus:ring-2 focus:ring-[var(--color-accent-border)] transition-colors"
            />
          </div>

          <div className="flex flex-col gap-1.5">
            <label className="text-xs font-medium text-[var(--color-fg-muted)] uppercase tracking-wide" htmlFor="login-password">
              Password
            </label>
            <input
              id="login-password"
              type="password"
              required
              autoComplete="current-password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              placeholder="••••••••"
              className="rounded-lg border border-[var(--color-border)] bg-[var(--color-bg)] px-3 py-2 text-sm text-[var(--color-fg)] placeholder-[var(--color-fg-muted)] outline-none focus:border-[var(--color-brand)] focus:ring-2 focus:ring-[var(--color-accent-border)] transition-colors"
            />
          </div>

          {error && (
            <p className="text-xs text-red-500 bg-red-50 dark:bg-red-950/30 border border-red-200 dark:border-red-800 rounded-lg px-3 py-2">
              {error}
            </p>
          )}

          <button
            type="submit"
            disabled={loading}
            className="rounded-lg bg-[var(--color-brand)] px-4 py-2 text-sm font-medium text-white hover:opacity-90 disabled:opacity-50 transition-opacity"
          >
            {loading ? "Signing in…" : "Sign in"}
          </button>
        </form>

        <p className="text-center text-xs text-[var(--color-fg-muted)]">
          Don&apos;t have an account?{" "}
          <button
            onClick={onGoRegister}
            className="text-[var(--color-brand)] hover:underline font-medium"
          >
            Create one
          </button>
        </p>
      </div>
    </div>
  );
}
