import { useQuery } from "@tanstack/react-query";
import { ArrowLeft, Loader2, Workflow } from "lucide-react";

import { api } from "../lib/api";
import { prettyStatus, statusBadgeClass } from "../lib/request-format";

// Request detail shell (F1). Shows the request record and a placeholder
// where the live workflow canvas lands in F2/F3. Intentionally empty of
// graph content until the planner runs.
export function RequestDetailView({
  requestId,
  onBack,
}: {
  requestId: string;
  onBack: () => void;
}) {
  const detailQ = useQuery({
    queryKey: ["request", requestId],
    queryFn: () => api.getRequest(requestId),
  });

  return (
    <div className="flex-1 flex flex-col overflow-hidden bg-[var(--color-bg)] text-[var(--color-fg)]">
      <div className="border-b border-[var(--color-border)] px-8 py-5 shrink-0">
        <button
          onClick={onBack}
          className="flex items-center gap-1.5 text-sm text-[var(--color-fg-muted)] hover:text-[var(--color-fg)] transition-colors mb-3"
        >
          <ArrowLeft size={14} /> Back to requests
        </button>

        {detailQ.isLoading && (
          <div className="flex items-center gap-2 text-sm text-[var(--color-fg-muted)]">
            <Loader2 size={16} className="animate-spin" /> Loading request…
          </div>
        )}

        {detailQ.isError && (
          <div className="rounded-lg border border-[#ea2261]/30 bg-[#ea2261]/8 px-4 py-3 text-sm text-[#ea2261]">
            Failed to load request: {(detailQ.error as Error).message}
          </div>
        )}

        {detailQ.data && (
          <div className="flex items-start justify-between gap-4">
            <div>
              <h1 className="text-xl font-semibold">{detailQ.data.request.title}</h1>
              <p className="text-xs font-mono text-[var(--color-fg-muted)] mt-1">
                {detailQ.data.request.id}
              </p>
            </div>
            <span
              className={`inline-block rounded-md px-2.5 py-1 text-xs font-medium capitalize ${statusBadgeClass(detailQ.data.request.status)}`}
            >
              {prettyStatus(detailQ.data.request.status)}
            </span>
          </div>
        )}
      </div>

      {detailQ.data && (
        <div className="flex-1 overflow-y-auto">
          <div className="grid grid-cols-4 gap-px bg-[var(--color-border)] border-b border-[var(--color-border)]">
            <Field label="Requester" value={detailQ.data.request.requester_name || "—"} />
            <Field label="Priority" value={detailQ.data.request.priority} capitalize />
            <Field label="Progress" value={`${detailQ.data.request.progress}%`} />
            <Field
              label="Submitted"
              value={new Date(detailQ.data.request.created_at).toLocaleString()}
            />
          </div>

          {detailQ.data.request.description && (
            <div className="px-8 py-5 border-b border-[var(--color-border)]">
              <p className="text-xs font-medium uppercase tracking-wide text-[var(--color-fg-muted)] mb-1.5">
                Description
              </p>
              <p className="text-sm whitespace-pre-wrap">{detailQ.data.request.description}</p>
            </div>
          )}

          {/* Workflow canvas placeholder — F2 fills this with the planned graph. */}
          <div className="flex flex-col items-center justify-center gap-3 py-24 text-center">
            <div className="size-14 rounded-2xl bg-[var(--color-surface-2)] flex items-center justify-center">
              <Workflow size={24} className="text-[var(--color-fg-muted)]" strokeWidth={1.5} />
            </div>
            <div>
              <p className="text-sm font-medium">No workflow yet</p>
              <p className="text-xs text-[var(--color-fg-muted)] mt-1 max-w-xs">
                The department workflow for this request will appear here once planning is enabled.
              </p>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}

function Field({
  label,
  value,
  capitalize,
}: {
  label: string;
  value: string;
  capitalize?: boolean;
}) {
  return (
    <div className="bg-[var(--color-bg)] px-6 py-4">
      <p className="text-xs font-medium uppercase tracking-wide text-[var(--color-fg-muted)]">
        {label}
      </p>
      <p className={`text-sm mt-1 ${capitalize ? "capitalize" : ""}`}>{value}</p>
    </div>
  );
}
