import { useEffect, useMemo, useRef, useState } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import {
  AlertCircle,
  FileText,
  Loader2,
  Plus,
  Upload,
  Wrench,
  X,
} from "lucide-react";

import { api } from "../lib/api";
import { useToasts } from "../components/Toasts";
import type { Machine, MachineStatus } from "../lib/types";

const STATUSES: MachineStatus[] = ["operational", "degraded", "down", "maintenance"];

const STATUS_LABELS: Record<string, string> = {
  operational: "Operational",
  degraded: "Degraded",
  down: "Down",
  maintenance: "Maintenance",
};

const STATUS_BADGE: Record<string, string> = {
  operational:
    "text-emerald-700 bg-emerald-50 dark:text-emerald-300 dark:bg-emerald-950",
  degraded:
    "text-amber-700 bg-amber-50 dark:text-amber-300 dark:bg-amber-950",
  down: "text-red-700 bg-red-50 dark:text-red-300 dark:bg-red-950",
  maintenance:
    "text-blue-700 bg-blue-50 dark:text-blue-300 dark:bg-blue-950",
};

interface Props {
  orgId: string;
}

export function MachinesView({ orgId }: Props) {
  const qc = useQueryClient();
  const [addModal, setAddModal] = useState(false);
  const [uploadTarget, setUploadTarget] = useState<Machine | null>(null);
  const [statusFilter, setStatusFilter] = useState<MachineStatus | "all">("all");

  const { data, isLoading, error } = useQuery({
    queryKey: ["machines", orgId],
    queryFn: () => api.listMachines(orgId),
  });

  const machines = useMemo(() => data?.machines ?? [], [data]);

  const filtered = useMemo(
    () =>
      statusFilter === "all"
        ? machines
        : machines.filter((m) => m.status === statusFilter),
    [machines, statusFilter],
  );

  return (
    <div className="flex-1 flex flex-col overflow-hidden">
      <div className="shrink-0 px-4 md:px-6 py-4 border-b border-[var(--color-border)] flex items-center justify-between gap-3">
        <div>
          <h1
            className="text-lg font-medium text-[var(--color-fg)]"
            style={{ fontFeatureSettings: '"ss01"' }}
          >
            Machines
          </h1>
          <p className="text-xs text-[var(--color-fg-muted)] mt-0.5">
            Monitor and manage equipment across the organization
          </p>
        </div>
        <button
          onClick={() => setAddModal(true)}
          className="flex items-center gap-1.5 px-3 py-2 rounded bg-[var(--color-brand)] text-white text-sm font-medium hover:bg-[var(--color-brand-hover)] transition-colors"
          style={{ fontFeatureSettings: '"ss01"' }}
        >
          <Plus size={14} strokeWidth={2} />
          Add Machine
        </button>
      </div>

      <div className="shrink-0 px-4 md:px-6 py-2.5 border-b border-[var(--color-border)] flex items-center gap-3 flex-wrap">
        <label className="flex items-center gap-1.5 text-xs text-[var(--color-fg-muted)]">
          Status
          <select
            value={statusFilter}
            onChange={(e) =>
              setStatusFilter(e.target.value as MachineStatus | "all")
            }
            className="rounded border border-[var(--color-border)] bg-[var(--color-bg)] px-2 py-1 text-xs text-[var(--color-fg)] focus:outline-none focus:ring-2 focus:ring-[var(--color-brand)]"
          >
            <option value="all">All</option>
            {STATUSES.map((s) => (
              <option key={s} value={s}>
                {STATUS_LABELS[s]}
              </option>
            ))}
          </select>
        </label>
      </div>

      <div className="flex-1 overflow-auto">
        {isLoading && (
          <div className="flex items-center justify-center h-40">
            <div className="size-6 rounded-full border-2 border-[var(--color-brand)] border-t-transparent animate-spin" />
          </div>
        )}

        {error && (
          <div className="flex items-center justify-center h-40 gap-2 text-sm text-[var(--color-danger)]">
            <AlertCircle size={16} />
            Failed to load machines
          </div>
        )}

        {!isLoading && !error && filtered.length === 0 && (
          <div className="flex flex-col items-center justify-center h-60 gap-3 text-center">
            <div className="size-10 rounded-lg bg-[var(--color-surface-2)] flex items-center justify-center">
              <Wrench size={20} className="text-[var(--color-fg-subtle)]" />
            </div>
            <p className="text-sm text-[var(--color-fg-muted)]">
              {machines.length === 0
                ? "No machines registered yet"
                : "No machines match this filter"}
            </p>
            {machines.length === 0 && (
              <button
                onClick={() => setAddModal(true)}
                className="text-sm text-[var(--color-brand)] hover:underline"
              >
                Register your first machine
              </button>
            )}
          </div>
        )}

        {filtered.length > 0 && (
          <>
            {/* Desktop table */}
            <table className="w-full text-sm hidden md:table">
              <thead>
                <tr className="text-left text-xs text-[var(--color-fg-muted)] border-b border-[var(--color-border)]">
                  <th className="px-6 py-2.5 font-medium w-8" />
                  <th className="px-4 py-2.5 font-medium">Name</th>
                  <th className="px-4 py-2.5 font-medium">Type</th>
                  <th className="px-4 py-2.5 font-medium">Location</th>
                  <th className="px-4 py-2.5 font-medium">Serial</th>
                  <th className="px-4 py-2.5 font-medium">Status</th>
                  <th className="px-4 py-2.5 font-medium text-right">Docs</th>
                </tr>
              </thead>
              <tbody>
                {filtered.map((m) => (
                  <MachineRow
                    key={m.id}
                    machine={m}
                    onUpload={() => setUploadTarget(m)}
                  />
                ))}
              </tbody>
            </table>

            {/* Mobile cards */}
            <div className="md:hidden flex flex-col gap-2 p-4">
              {filtered.map((m) => (
                <div
                  key={m.id}
                  className="rounded-lg border border-[var(--color-border)] bg-[var(--color-surface)] p-4 shadow-stripe-ambient"
                >
                  <div className="flex items-start justify-between gap-3">
                    <div className="min-w-0 flex-1">
                      <div className="flex items-center gap-2">
                        <div className={`size-2 rounded-full shrink-0 ${STATUS_COLORS[m.status] ?? "bg-gray-400"}`} />
                        <span className="text-sm font-medium text-[var(--color-fg)] truncate" style={{ fontFeatureSettings: '"ss01"' }}>
                          {m.name}
                        </span>
                      </div>
                      <p className="text-xs text-[var(--color-fg-muted)] mt-1">
                        {m.machine_type || "\u2014"} · {m.location || "\u2014"}
                      </p>
                      {m.serial_number && (
                        <p className="text-xs font-mono text-[var(--color-fg-subtle)] mt-0.5">{m.serial_number}</p>
                      )}
                    </div>
                    <div className="flex flex-col items-end gap-2 shrink-0">
                      <span className={`inline-block rounded-md px-2 py-0.5 text-xs font-medium ${STATUS_BADGE[m.status] ?? ""}`}>
                        {STATUS_LABELS[m.status] ?? m.status}
                      </span>
                      <button
                        onClick={() => setUploadTarget(m)}
                        className="inline-flex items-center gap-1 rounded px-2 py-1 text-xs text-[var(--color-fg-muted)] hover:text-[var(--color-brand)] hover:bg-[var(--color-accent-bg)] transition-colors"
                      >
                        <Upload size={13} strokeWidth={1.75} />
                        Upload
                      </button>
                    </div>
                  </div>
                </div>
              ))}
            </div>
          </>
        )}
      </div>

      {addModal && (
        <AddMachineModal
          orgId={orgId}
          onClose={() => setAddModal(false)}
          onAdded={() => {
            setAddModal(false);
            qc.invalidateQueries({ queryKey: ["machines", orgId] });
          }}
        />
      )}

      {uploadTarget && (
        <DocumentUploadModal
          machine={uploadTarget}
          onClose={() => setUploadTarget(null)}
          onUploaded={() => setUploadTarget(null)}
        />
      )}
    </div>
  );
}

const STATUS_COLORS: Record<string, string> = {
  operational: "bg-emerald-500",
  degraded: "bg-amber-400",
  down: "bg-red-500",
  maintenance: "bg-blue-400",
};

function MachineRow({
  machine: m,
  onUpload,
}: {
  machine: Machine;
  onUpload: () => void;
}) {
  return (
    <tr className="border-b border-[var(--color-border)] hover:bg-[var(--color-surface-2)] transition-colors">
      <td className="px-6 py-3">
        <div
          className={`size-2 rounded-full ${STATUS_COLORS[m.status] ?? "bg-gray-400"}`}
        />
      </td>
      <td className="px-4 py-3">
        <span
          className="font-medium text-[var(--color-fg)]"
          style={{ fontFeatureSettings: '"ss01"' }}
        >
          {m.name}
        </span>
      </td>
      <td className="px-4 py-3 text-[var(--color-fg-muted)]">
        {m.machine_type || "\u2014"}
      </td>
      <td className="px-4 py-3 text-[var(--color-fg-muted)]">
        {m.location || "\u2014"}
      </td>
      <td className="px-4 py-3 text-[var(--color-fg-muted)] font-mono text-xs">
        {m.serial_number || "\u2014"}
      </td>
      <td className="px-4 py-3">
        <span
          className={`inline-block rounded-md px-2 py-0.5 text-xs font-medium ${STATUS_BADGE[m.status] ?? ""}`}
        >
          {STATUS_LABELS[m.status] ?? m.status}
        </span>
      </td>
      <td className="px-4 py-3 text-right">
        <button
          onClick={onUpload}
          title="Upload document"
          className="inline-flex items-center gap-1 rounded px-2 py-1 text-xs text-[var(--color-fg-muted)] hover:text-[var(--color-brand)] hover:bg-[var(--color-accent-bg)] transition-colors"
        >
          <Upload size={13} strokeWidth={1.75} />
          Upload
        </button>
      </td>
    </tr>
  );
}

// ── Document Upload Modal ─────────────────────────────────────────

function DocumentUploadModal({
  machine,
  onClose,
  onUploaded,
}: {
  machine: Machine;
  onClose: () => void;
  onUploaded: () => void;
}) {
  const toasts = useToasts();
  const dialogRef = useRef<HTMLDivElement>(null);
  const fileInputRef = useRef<HTMLInputElement>(null);
  const [file, setFile] = useState<File | null>(null);
  const [docType, setDocType] = useState("manual");

  const mutation = useMutation({
    mutationFn: () => {
      if (!file) throw new Error("No file selected");
      return api.uploadMachineDocument(machine.id, file, docType);
    },
    onSuccess: () => {
      toasts.push({
        kind: "success",
        title: `Document uploaded for ${machine.name}`,
      });
      onUploaded();
    },
    onError: (e: Error) => toasts.push({ kind: "error", title: e.message }),
  });

  useEffect(() => {
    const onKeyDown = (e: KeyboardEvent) => {
      if (e.key === "Escape") {
        e.stopPropagation();
        onClose();
        return;
      }
      if (e.key !== "Tab") return;
      const root = dialogRef.current;
      if (!root) return;
      const focusable = root.querySelectorAll<HTMLElement>(
        'a[href], button:not([disabled]), input, textarea, select, [tabindex]:not([tabindex="-1"])',
      );
      if (focusable.length === 0) return;
      const first = focusable[0]!;
      const last = focusable[focusable.length - 1]!;
      if (e.shiftKey && document.activeElement === first) {
        e.preventDefault();
        last.focus();
      } else if (!e.shiftKey && document.activeElement === last) {
        e.preventDefault();
        first.focus();
      }
    };
    document.addEventListener("keydown", onKeyDown);
    return () => document.removeEventListener("keydown", onKeyDown);
  }, [onClose]);

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center">
      <div className="absolute inset-0 bg-black/40" onClick={onClose} />
      <div
        ref={dialogRef}
        role="dialog"
        aria-modal="true"
        aria-labelledby="upload-doc-title"
        className="relative bg-[var(--color-surface)] rounded-lg shadow-stripe-elevated w-full max-w-sm p-6 border border-[var(--color-border)] mx-4 md:mx-0"
      >
        <div className="flex items-start justify-between mb-4">
          <div>
            <h2
              id="upload-doc-title"
              className="text-base font-medium text-[var(--color-fg)]"
              style={{ fontFeatureSettings: '"ss01"' }}
            >
              Upload Document
            </h2>
            <p className="text-xs text-[var(--color-fg-muted)] mt-0.5">
              {machine.name}
            </p>
          </div>
          <button
            onClick={onClose}
            aria-label="Close"
            className="text-[var(--color-fg-muted)] hover:text-[var(--color-fg)] transition-colors"
          >
            <X size={18} />
          </button>
        </div>

        <div className="flex flex-col gap-4">
          <div>
            <label className="block text-xs font-medium text-[var(--color-fg-label)] mb-1">
              Document Type
            </label>
            <select
              value={docType}
              onChange={(e) => setDocType(e.target.value)}
              className="w-full px-3 py-2 text-sm rounded border border-[var(--color-border)] bg-[var(--color-bg)] text-[var(--color-fg)] focus:outline-none focus:ring-2 focus:ring-[var(--color-brand)] focus:border-transparent"
            >
              <option value="manual">Manual / Specification</option>
              <option value="maintenance_log">Maintenance Log</option>
              <option value="safety_sheet">Safety Sheet</option>
              <option value="warranty">Warranty</option>
              <option value="other">Other</option>
            </select>
          </div>

          <div>
            <label className="block text-xs font-medium text-[var(--color-fg-label)] mb-1">
              File
            </label>
            <div className="flex gap-2">
              <button
                type="button"
                onClick={() => fileInputRef.current?.click()}
                className="flex-1 flex items-center gap-2 rounded border border-dashed border-[var(--color-border)] px-3 py-2 text-sm text-[var(--color-fg-muted)] hover:border-[var(--color-brand)] hover:text-[var(--color-brand)] transition-colors"
              >
                <FileText size={14} />
                <span className="truncate">
                  {file ? file.name : "Choose PDF, TXT, or MD"}
                </span>
              </button>
              {file && (
                <button
                  type="button"
                  onClick={() => setFile(null)}
                  className="text-[var(--color-fg-muted)] hover:text-red-500 transition-colors"
                >
                  <X size={16} />
                </button>
              )}
            </div>
            <input
              ref={fileInputRef}
              type="file"
              accept=".pdf,.txt,.md"
              className="hidden"
              onChange={(e) => setFile(e.target.files?.[0] ?? null)}
            />
          </div>
        </div>

        <div className="flex justify-end gap-2 mt-5">
          <button
            onClick={onClose}
            className="px-3 py-2 text-sm rounded border border-[var(--color-border)] text-[var(--color-fg-muted)] hover:bg-[var(--color-surface-2)] transition-colors"
          >
            Cancel
          </button>
          <button
            onClick={() => mutation.mutate()}
            disabled={!file || mutation.isPending}
            className="flex items-center gap-1.5 px-3 py-2 text-sm rounded bg-[var(--color-brand)] text-white font-medium hover:bg-[var(--color-brand-hover)] transition-colors disabled:opacity-50"
            style={{ fontFeatureSettings: '"ss01"' }}
          >
            {mutation.isPending && (
              <Loader2 size={13} className="animate-spin" />
            )}
            {mutation.isPending ? "Uploading..." : "Upload"}
          </button>
        </div>
      </div>
    </div>
  );
}

// ── Add Machine Modal ─────────────────────────────────────────────

function AddMachineModal({
  orgId,
  onClose,
  onAdded,
}: {
  orgId: string;
  onClose: () => void;
  onAdded: () => void;
}) {
  const qc = useQueryClient();
  const toasts = useToasts();
  const dialogRef = useRef<HTMLDivElement>(null);
  const [name, setName] = useState("");
  const [machineType, setMachineType] = useState("");
  const [location, setLocation] = useState("");
  const [serialNumber, setSerialNumber] = useState("");

  const mutation = useMutation({
    mutationFn: () =>
      api.createMachine(orgId, {
        name: name.trim(),
        machine_type: machineType.trim(),
        location: location.trim(),
        serial_number: serialNumber.trim(),
      }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["machines", orgId] });
      onAdded();
    },
    onError: (e: Error) => toasts.push({ kind: "error", title: e.message }),
  });

  useEffect(() => {
    const onKeyDown = (e: KeyboardEvent) => {
      if (e.key === "Escape") {
        e.stopPropagation();
        onClose();
        return;
      }
      if (e.key !== "Tab") return;
      const root = dialogRef.current;
      if (!root) return;
      const focusable = root.querySelectorAll<HTMLElement>(
        'a[href], button:not([disabled]), input, textarea, select, [tabindex]:not([tabindex="-1"])',
      );
      if (focusable.length === 0) return;
      const first = focusable[0]!;
      const last = focusable[focusable.length - 1]!;
      if (e.shiftKey && document.activeElement === first) {
        e.preventDefault();
        last.focus();
      } else if (!e.shiftKey && document.activeElement === last) {
        e.preventDefault();
        first.focus();
      }
    };
    document.addEventListener("keydown", onKeyDown);
    return () => document.removeEventListener("keydown", onKeyDown);
  }, [onClose]);

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center">
      <div className="absolute inset-0 bg-black/40" onClick={onClose} />
      <div
        ref={dialogRef}
        role="dialog"
        aria-modal="true"
        aria-labelledby="add-machine-title"
        className="relative bg-[var(--color-surface)] rounded-lg shadow-stripe-elevated w-full max-w-md p-6 border border-[var(--color-border)] mx-4 md:mx-0"
      >
        <div className="flex items-start justify-between mb-4">
          <h2
            id="add-machine-title"
            className="text-base font-medium text-[var(--color-fg)]"
            style={{ fontFeatureSettings: '"ss01"' }}
          >
            Add Machine
          </h2>
          <button
            onClick={onClose}
            aria-label="Close"
            className="text-[var(--color-fg-muted)] hover:text-[var(--color-fg)] transition-colors"
          >
            <X size={18} />
          </button>
        </div>

        <div className="flex flex-col gap-3">
          <div>
            <label className="block text-xs font-medium text-[var(--color-fg-label)] mb-1">
              Name *
            </label>
            <input
              autoFocus
              type="text"
              value={name}
              onChange={(e) => setName(e.target.value)}
              placeholder="CNC Machine #4"
              className="w-full px-3 py-2 text-sm rounded border border-[var(--color-border)] bg-[var(--color-bg)] text-[var(--color-fg)] placeholder:text-[var(--color-fg-subtle)] focus:outline-none focus:ring-2 focus:ring-[var(--color-brand)] focus:border-transparent"
            />
          </div>

          <div>
            <label className="block text-xs font-medium text-[var(--color-fg-label)] mb-1">
              Type
            </label>
            <select
              value={machineType}
              onChange={(e) => setMachineType(e.target.value)}
              className="w-full px-3 py-2 text-sm rounded border border-[var(--color-border)] bg-[var(--color-bg)] text-[var(--color-fg)] focus:outline-none focus:ring-2 focus:ring-[var(--color-brand)] focus:border-transparent"
            >
              <option value="">Select type...</option>
              <option value="cnc">CNC Machine</option>
              <option value="pump">Pump</option>
              <option value="compressor">Compressor</option>
              <option value="conveyor">Conveyor</option>
              <option value="generator">Generator</option>
              <option value="robot_arm">Robot Arm</option>
              <option value="Server Rack">Server Rack</option>
              <option value="HVAC">HVAC Unit</option>
            </select>
          </div>

          <div>
            <label className="block text-xs font-medium text-[var(--color-fg-label)] mb-1">
              Location
            </label>
            <input
              type="text"
              value={location}
              onChange={(e) => setLocation(e.target.value)}
              placeholder="Floor 3, Building B"
              className="w-full px-3 py-2 text-sm rounded border border-[var(--color-border)] bg-[var(--color-bg)] text-[var(--color-fg)] placeholder:text-[var(--color-fg-subtle)] focus:outline-none focus:ring-2 focus:ring-[var(--color-brand)] focus:border-transparent"
            />
          </div>

          <div>
            <label className="block text-xs font-medium text-[var(--color-fg-label)] mb-1">
              Serial Number
            </label>
            <input
              type="text"
              value={serialNumber}
              onChange={(e) => setSerialNumber(e.target.value)}
              placeholder="SN-2024-0042"
              className="w-full px-3 py-2 text-sm rounded border border-[var(--color-border)] bg-[var(--color-bg)] text-[var(--color-fg)] placeholder:text-[var(--color-fg-subtle)] focus:outline-none focus:ring-2 focus:ring-[var(--color-brand)] focus:border-transparent"
            />
          </div>
        </div>

        <div className="flex justify-end gap-2 mt-5">
          <button
            onClick={onClose}
            className="px-3 py-2 text-sm rounded border border-[var(--color-border)] text-[var(--color-fg-muted)] hover:bg-[var(--color-surface-2)] transition-colors"
          >
            Cancel
          </button>
          <button
            onClick={() => mutation.mutate()}
            disabled={!name.trim() || mutation.isPending}
            className="flex items-center gap-1.5 px-3 py-2 text-sm rounded bg-[var(--color-brand)] text-white font-medium hover:bg-[var(--color-brand-hover)] transition-colors disabled:opacity-50"
            style={{ fontFeatureSettings: '"ss01"' }}
          >
            {mutation.isPending && (
              <Loader2 size={13} className="animate-spin" />
            )}
            {mutation.isPending ? "Adding..." : "Add Machine"}
          </button>
        </div>
      </div>
    </div>
  );
}
