import { useState, type FormEvent, useRef } from "react";
import { AnimatePresence, motion } from "framer-motion";
import { Building2, Check, ChevronRight, Mail, Plus, Shield, Workflow, X, FileText, Trash2, Cpu } from "lucide-react";
import { api } from "../lib/api";
import { BrandMark } from "../components/Brand";

interface Props {
  onDone: (org: { id: string; name: string; slug: string; role: string }) => void;
}

function toSlug(name: string) {
  return name
    .toLowerCase()
    .trim()
    .replace(/[^a-z0-9]+/g, "-")
    .replace(/^-+|-+$/g, "");
}

// Colour options for the org avatar
const ACCENT_COLORS = [
  { id: "violet", bg: "#7c3aed", label: "Violet" },
  { id: "blue",   bg: "#2563eb", label: "Blue"   },
  { id: "cyan",   bg: "#0891b2", label: "Cyan"   },
  { id: "green",  bg: "#16a34a", label: "Green"  },
  { id: "amber",  bg: "#d97706", label: "Amber"  },
  { id: "rose",   bg: "#e11d48", label: "Rose"   },
];

const ROLES = [
  {
    id: "admin",
    icon: Shield,
    title: "Organization admin",
    desc: "Manage teams, policies, and all workflows across the org.",
  },
  {
    id: "executor",
    icon: Workflow,
    title: "Workflow executor",
    desc: "Build, run, and monitor workflows for your team.",
  },
];

const MACHINE_TYPES = [
  "CNC Mill", "Press Line", "Laser Cutter", "Server Rack", "HVAC Unit",
  "Assembly Line", "Robot Arm", "Pump System", "Conveyor Belt", "Other",
] as const;

const STEPS = ["Organization", "Your role", "Invite teammates", "Add machines"];

// Slide animation — enter from right, exit to left
const variants = {
  enter: { x: 40, opacity: 0 },
  center: { x: 0,  opacity: 1 },
  exit:  { x: -40, opacity: 0 },
};

export function OrgSetupView({ onDone }: Props) {
  const [step, setStep]       = useState(0);
  const [direction, setDir]   = useState(1); // 1 = forward, -1 = back

  // Step 0 fields
  const [orgName,     setOrgName]     = useState("");
  const [slug,        setSlug]        = useState("");
  const [slugEdited,  setSlugEdited]  = useState(false);
  const [accentColor, setAccentColor] = useState(ACCENT_COLORS[0]!);

  // Step 1 fields
  const [role, setRole] = useState<"admin" | "executor">("admin");

  // Step 2 fields
  const [emailInput, setEmailInput] = useState("");
  const [invites,    setInvites]    = useState<string[]>([]);
  const [inviteRole, setInviteRole] = useState<"executor" | "employee">("executor");

	// Step 3 fields — machines
	type MachineInput = { name: string; machine_type: string; location: string; serial_number: string; file: File | null; fileName: string };
	const [machines, setMachines] = useState<MachineInput[]>([]);
	const [machineName, setMachineName] = useState("");
	const [machineType, setMachineType] = useState<string>(MACHINE_TYPES[0]);
	const [machineLocation, setMachineLocation] = useState("");
	const [machineSerial, setMachineSerial] = useState("");
	const [machineFile, setMachineFile] = useState<File | null>(null);
	const [machineFileName, setMachineFileName] = useState("");
	const fileInputRef = useRef<HTMLInputElement>(null);

	function addMachine() {
		if (!machineName.trim()) return;
		setMachines([...machines, {
			name: machineName.trim(),
			machine_type: machineType,
			location: machineLocation.trim(),
			serial_number: machineSerial.trim(),
			file: machineFile,
			fileName: machineFileName,
		}]);
		setMachineName("");
		setMachineLocation("");
		setMachineSerial("");
		setMachineFile(null);
		setMachineFileName("");
	}

	function removeMachine(index: number) {
		setMachines(machines.filter((_, i) => i !== index));
	}

	// Async state
	const [loading, setLoading] = useState(false);
	const [error,   setError]   = useState<string | null>(null);

  // Derived initials for avatar preview
  const initials = orgName
    .split(/\s+/)
    .filter(Boolean)
    .slice(0, 2)
    .map((w) => w[0]!.toUpperCase())
    .join("") || "?";

  function handleNameChange(v: string) {
    setOrgName(v);
    if (!slugEdited) setSlug(toSlug(v));
  }

  function addInvite() {
    const e = emailInput.trim().toLowerCase();
    if (!e || !e.includes("@") || invites.includes(e)) return;
    setInvites((prev) => [...prev, e]);
    setEmailInput("");
  }

  function removeInvite(email: string) {
    setInvites((prev) => prev.filter((e) => e !== email));
  }

  function go(next: number) {
    setDir(next > step ? 1 : -1);
    setStep(next);
    setError(null);
  }

  // Step 0 → 1: just validate, no API call yet
  function handleStep0(e: FormEvent) {
    e.preventDefault();
    if (!orgName.trim() || !slug.trim()) return;
    go(1);
  }

	// Step 1 → 2
	function handleStep1(e: FormEvent) {
		e.preventDefault();
		go(2);
	}

	// Step 2 → 3 (machines)
	function handleStep2(e: FormEvent) {
		e.preventDefault();
		go(3);
	}

	// Step 3 (final): create org, machines, upload docs
	async function handleFinish() {
		setError(null);
		setLoading(true);
		try {
			const result = await api.createOrg({ name: orgName.trim(), slug: slug.trim() });
			const orgId = result.id;

			// Create each machine and upload its document
			for (const m of machines) {
				const { machine } = await api.createMachine(orgId, {
					name: m.name,
					machine_type: m.machine_type,
					location: m.location,
					serial_number: m.serial_number,
				});
				if (m.file) {
					await api.uploadMachineDocument(machine.id, m.file, "manual").catch(() => {});
				}
			}

			onDone({ id: orgId, name: result.name, slug: result.slug, role: "admin" });
		} catch (err) {
			setError(err instanceof Error ? err.message : "Something went wrong");
		} finally {
			setLoading(false);
		}
	}

  return (
    <div className="h-screen w-screen flex items-center justify-center bg-[var(--color-bg)]">
      <div className="w-full max-w-lg rounded-2xl border border-[var(--color-border)] bg-[var(--color-surface)] shadow-xl flex flex-col overflow-hidden mx-4">

        {/* ── Header ── */}
        <div className="flex flex-col items-center gap-2 px-4 md:px-8 pt-6 md:pt-8 pb-4">
          <BrandMark size={30} />
          <h1 className="text-base font-semibold text-[var(--color-fg)]">Set up your workspace</h1>
          <p className="text-xs text-[var(--color-fg-muted)]">Step {step + 1} of {STEPS.length}</p>
        </div>

        {/* ── Progress bar ── */}
        <div className="px-4 md:px-8 pb-6">
          <div className="flex items-center gap-2">
            {STEPS.map((label, i) => (
              <div key={label} className="flex items-center gap-2 flex-1 last:flex-none">
                <div className="flex flex-col items-center gap-1">
                  <div
                    className={`size-6 rounded-full flex items-center justify-center text-[10px] font-bold transition-colors duration-300 ${
                      i < step
                        ? "bg-[var(--color-brand)] text-white"
                        : i === step
                        ? "bg-[var(--color-accent-bg)] text-[var(--color-brand)] ring-2 ring-[var(--color-brand)]"
                        : "bg-[var(--color-surface-2)] text-[var(--color-fg-muted)]"
                    }`}
                  >
                    {i < step ? <Check size={11} /> : i + 1}
                  </div>
                  <span
                    className={`text-[10px] whitespace-nowrap transition-colors duration-300 ${
                      i === step ? "text-[var(--color-fg)] font-medium" : "text-[var(--color-fg-muted)]"
                    }`}
                  >
                    {label}
                  </span>
                </div>
                {i < STEPS.length - 1 && (
                  <div
                    className={`flex-1 h-px mb-5 transition-colors duration-300 ${
                      i < step ? "bg-[var(--color-brand)]" : "bg-[var(--color-border)]"
                    }`}
                  />
                )}
              </div>
            ))}
          </div>
        </div>

        {/* ── Animated step content ── */}
        <div className="relative overflow-hidden min-h-[320px]">
          <AnimatePresence mode="wait" custom={direction}>
            {step === 0 && (
              <motion.form
                key="step0"
                custom={direction}
                variants={variants}
                initial="enter"
                animate="center"
                exit="exit"
                transition={{ duration: 0.22, ease: "easeInOut" }}
                onSubmit={handleStep0}
                className="px-4 md:px-8 pb-8 flex flex-col gap-5"
              >
                {/* Avatar preview + color picker */}
                <div className="flex flex-col items-center gap-3">
                  <div
                    className="size-16 rounded-2xl flex items-center justify-center text-2xl font-bold text-white shadow-md transition-colors duration-300"
                    style={{ background: accentColor.bg }}
                  >
                    {initials}
                  </div>
                  <div className="flex gap-2">
                    {ACCENT_COLORS.map((c) => (
                      <button
                        key={c.id}
                        type="button"
                        onClick={() => setAccentColor(c)}
                        aria-label={c.label}
                        className="size-5 rounded-full transition-transform hover:scale-110 focus:outline-none"
                        style={{
                          background: c.bg,
                          outline: accentColor.id === c.id ? `2px solid ${c.bg}` : "none",
                          outlineOffset: "2px",
                        }}
                      />
                    ))}
                  </div>
                  <p className="text-xs text-[var(--color-fg-muted)]">Pick an accent colour</p>
                </div>

                {/* Name */}
                <div className="flex flex-col gap-1.5">
                  <label className="text-xs font-medium text-[var(--color-fg-muted)] uppercase tracking-wide" htmlFor="org-name">
                    Organization name
                  </label>
                  <input
                    id="org-name"
                    type="text"
                    required
                    autoFocus
                    value={orgName}
                    onChange={(e) => handleNameChange(e.target.value)}
                    placeholder="Acme Corp"
                    className="rounded-lg border border-[var(--color-border)] bg-[var(--color-bg)] px-3 py-2 text-sm text-[var(--color-fg)] placeholder-[var(--color-fg-muted)] outline-none focus:border-[var(--color-brand)] focus:ring-2 focus:ring-[var(--color-accent-border)] transition-colors"
                  />
                </div>

                {/* Slug */}
                <div className="flex flex-col gap-1.5">
                  <label className="text-xs font-medium text-[var(--color-fg-muted)] uppercase tracking-wide" htmlFor="org-slug">
                    Slug
                  </label>
                  <div className="flex items-center rounded-lg border border-[var(--color-border)] bg-[var(--color-bg)] focus-within:border-[var(--color-brand)] focus-within:ring-2 focus-within:ring-[var(--color-accent-border)] transition-colors overflow-hidden">
                    <span className="px-3 py-2 text-sm text-[var(--color-fg-muted)] select-none border-r border-[var(--color-border)]">
                      crouton/
                    </span>
                    <input
                      id="org-slug"
                      type="text"
                      required
                      value={slug}
                      onChange={(e) => { setSlug(toSlug(e.target.value)); setSlugEdited(true); }}
                      placeholder="acme-corp"
                      className="flex-1 bg-transparent px-3 py-2 text-sm text-[var(--color-fg)] placeholder-[var(--color-fg-muted)] outline-none"
                    />
                  </div>
                </div>

                <button
                  type="submit"
                  disabled={!orgName.trim() || !slug.trim()}
                  className="mt-1 flex items-center justify-center gap-2 rounded-lg bg-[var(--color-brand)] px-4 py-2.5 text-sm font-medium text-white hover:opacity-90 disabled:opacity-40 transition-opacity"
                >
                  Continue <ChevronRight size={15} />
                </button>
              </motion.form>
            )}

            {step === 1 && (
              <motion.form
                key="step1"
                custom={direction}
                variants={variants}
                initial="enter"
                animate="center"
                exit="exit"
                transition={{ duration: 0.22, ease: "easeInOut" }}
                onSubmit={handleStep1}
                className="px-4 md:px-8 pb-8 flex flex-col gap-5"
              >
                <p className="text-sm text-[var(--color-fg-muted)]">
                  How will you primarily use Crouton?
                </p>

                <div className="flex flex-col gap-3">
                  {ROLES.map(({ id, icon: Icon, title, desc }) => {
                    const active = role === id;
                    return (
                      <button
                        key={id}
                        type="button"
                        onClick={() => setRole(id as "admin" | "executor")}
                        className={`flex items-start gap-4 rounded-xl border p-4 text-left transition-all ${
                          active
                            ? "border-[var(--color-brand)] bg-[var(--color-accent-bg)]"
                            : "border-[var(--color-border)] bg-[var(--color-bg)] hover:border-[var(--color-brand)]/50"
                        }`}
                      >
                        <div
                          className={`mt-0.5 size-9 rounded-lg flex items-center justify-center shrink-0 transition-colors ${
                            active
                              ? "bg-[var(--color-brand)] text-white"
                              : "bg-[var(--color-surface-2)] text-[var(--color-fg-muted)]"
                          }`}
                        >
                          <Icon size={18} strokeWidth={1.75} />
                        </div>
                        <div>
                          <p className={`text-sm font-medium ${active ? "text-[var(--color-brand)]" : "text-[var(--color-fg)]"}`}>
                            {title}
                          </p>
                          <p className="text-xs text-[var(--color-fg-muted)] mt-0.5">{desc}</p>
                        </div>
                        {active && (
                          <div className="ml-auto shrink-0 size-5 rounded-full bg-[var(--color-brand)] flex items-center justify-center">
                            <Check size={11} className="text-white" />
                          </div>
                        )}
                      </button>
                    );
                  })}
                </div>

                <div className="flex gap-3 mt-1">
                  <button
                    type="button"
                    onClick={() => go(0)}
                    className="flex-1 rounded-lg border border-[var(--color-border)] px-4 py-2.5 text-sm text-[var(--color-fg-muted)] hover:bg-[var(--color-surface-2)] transition-colors"
                  >
                    Back
                  </button>
                  <button
                    type="submit"
                    className="flex-1 flex items-center justify-center gap-2 rounded-lg bg-[var(--color-brand)] px-4 py-2.5 text-sm font-medium text-white hover:opacity-90 transition-opacity"
                  >
                    Continue <ChevronRight size={15} />
                  </button>
                </div>
              </motion.form>
            )}

            {step === 2 && (
              <motion.form
                key="step2"
                custom={direction}
                variants={variants}
                initial="enter"
                animate="center"
                exit="exit"
                transition={{ duration: 0.22, ease: "easeInOut" }}
                onSubmit={handleStep2}
                className="px-4 md:px-8 pb-8 flex flex-col gap-5"
              >
                <p className="text-sm text-[var(--color-fg-muted)]">
                  Add teammates by email. You can always do this later.
                </p>

                {/* Role selector for invites */}
                <div className="flex gap-2">
                  {(["executor", "employee"] as const).map((r) => (
                    <button
                      key={r}
                      type="button"
                      onClick={() => setInviteRole(r)}
                      className={`flex-1 rounded-lg border py-2 text-xs font-medium transition-all capitalize ${
                        inviteRole === r
                          ? "border-[var(--color-brand)] bg-[var(--color-accent-bg)] text-[var(--color-brand)]"
                          : "border-[var(--color-border)] text-[var(--color-fg-muted)] hover:border-[var(--color-brand)]/50"
                      }`}
                    >
                      {r}
                    </button>
                  ))}
                </div>

                {/* Email input */}
                <div className="flex gap-2">
                  <div className="flex-1 flex items-center rounded-lg border border-[var(--color-border)] bg-[var(--color-bg)] px-3 focus-within:border-[var(--color-brand)] focus-within:ring-2 focus-within:ring-[var(--color-accent-border)] transition-colors">
                    <Mail size={14} className="text-[var(--color-fg-muted)] shrink-0 mr-2" />
                    <input
                      type="email"
                      value={emailInput}
                      onChange={(e) => setEmailInput(e.target.value)}
                      onKeyDown={(e) => { if (e.key === "Enter") { e.preventDefault(); addInvite(); } }}
                      placeholder="colleague@company.com"
                      className="flex-1 bg-transparent py-2 text-sm text-[var(--color-fg)] placeholder-[var(--color-fg-muted)] outline-none"
                    />
                  </div>
                  <button
                    type="button"
                    onClick={addInvite}
                    disabled={!emailInput.trim() || !emailInput.includes("@")}
                    className="size-9 shrink-0 flex items-center justify-center rounded-lg bg-[var(--color-brand)] text-white hover:opacity-90 disabled:opacity-40 transition-opacity"
                  >
                    <Plus size={16} />
                  </button>
                </div>

                {/* Invite list */}
                {invites.length > 0 && (
                  <ul className="flex flex-col gap-2 max-h-36 overflow-y-auto">
                    {invites.map((email) => (
                      <li
                        key={email}
                        className="flex items-center gap-3 rounded-lg border border-[var(--color-border)] bg-[var(--color-bg)] px-3 py-2"
                      >
                        <div className="size-6 rounded-full bg-[var(--color-accent-bg)] flex items-center justify-center text-[10px] font-bold text-[var(--color-brand)]">
                          {email[0]!.toUpperCase()}
                        </div>
                        <span className="flex-1 text-xs text-[var(--color-fg)] truncate">{email}</span>
                        <span className="text-[10px] text-[var(--color-fg-muted)] capitalize px-1.5 py-0.5 rounded border border-[var(--color-border)]">
                          {inviteRole}
                        </span>
                        <button
                          type="button"
                          onClick={() => removeInvite(email)}
                          className="text-[var(--color-fg-muted)] hover:text-red-500 transition-colors"
                        >
                          <X size={13} />
                        </button>
                      </li>
                    ))}
                  </ul>
                )}

                {error && (
                  <p className="text-xs text-red-500 bg-red-50 dark:bg-red-950/30 border border-red-200 dark:border-red-800 rounded-lg px-3 py-2">
                    {error}
                  </p>
                )}

                <div className="flex gap-3 mt-1">
                  <button
                    type="button"
                    onClick={() => go(1)}
                    className="flex-1 rounded-lg border border-[var(--color-border)] px-4 py-2.5 text-sm text-[var(--color-fg-muted)] hover:bg-[var(--color-surface-2)] transition-colors"
                  >
                    Back
                  </button>
                  <button
                    type="submit"
                    className="flex-1 flex items-center justify-center gap-2 rounded-lg bg-[var(--color-brand)] px-4 py-2.5 text-sm font-medium text-white hover:opacity-90 transition-opacity"
                  >
                    Continue <ChevronRight size={15} />
                  </button>
                </div>
              </motion.form>
            )}

            {step === 3 && (
              <motion.form
                key="step3"
                custom={direction}
                variants={variants}
                initial="enter"
                animate="center"
                exit="exit"
                transition={{ duration: 0.22, ease: "easeInOut" }}
                onSubmit={(e) => { e.preventDefault(); handleFinish(); }}
                className="px-4 md:px-8 pb-8 flex flex-col gap-4"
              >
                <p className="text-sm text-[var(--color-fg-muted)]">
                  Add machines to your fleet. Upload a manual or spec sheet for AI-assisted diagnostics.
                </p>

                {/* Machine form */}
                <div className="flex flex-col gap-2.5 p-4 rounded-xl border border-[var(--color-border)] bg-[var(--color-bg)]">
                  <div className="flex gap-2.5">
                    <div className="flex-1">
                      <label className="text-[10px] font-medium text-[var(--color-fg-muted)] uppercase tracking-wide" htmlFor="mach-name">Name</label>
                      <input id="mach-name" type="text" value={machineName} onChange={(e) => setMachineName(e.target.value)}
                        placeholder="CNC Mill #4" className="w-full mt-1 rounded-lg border border-[var(--color-border)] bg-[var(--color-surface)] px-2.5 py-1.5 text-xs text-[var(--color-fg)] placeholder-[var(--color-fg-muted)] outline-none focus:border-[var(--color-brand)] transition-colors" />
                    </div>
                    <div className="flex-1">
                      <label className="text-[10px] font-medium text-[var(--color-fg-muted)] uppercase tracking-wide">Type</label>
                      <select value={machineType} onChange={(e) => setMachineType(e.target.value)}
                        className="w-full mt-1 rounded-lg border border-[var(--color-border)] bg-[var(--color-surface)] px-2.5 py-1.5 text-xs text-[var(--color-fg)] outline-none focus:border-[var(--color-brand)] transition-colors">
                        {MACHINE_TYPES.map((t) => <option key={t} value={t}>{t}</option>)}
                      </select>
                    </div>
                  </div>
                  <div className="flex gap-2.5">
                    <div className="flex-1">
                      <label className="text-[10px] font-medium text-[var(--color-fg-muted)] uppercase tracking-wide" htmlFor="mach-loc">Location</label>
                      <input id="mach-loc" type="text" value={machineLocation} onChange={(e) => setMachineLocation(e.target.value)}
                        placeholder="Floor 2, Bay 4" className="w-full mt-1 rounded-lg border border-[var(--color-border)] bg-[var(--color-surface)] px-2.5 py-1.5 text-xs text-[var(--color-fg)] placeholder-[var(--color-fg-muted)] outline-none focus:border-[var(--color-brand)] transition-colors" />
                    </div>
                    <div className="flex-1">
                      <label className="text-[10px] font-medium text-[var(--color-fg-muted)] uppercase tracking-wide" htmlFor="mach-sn">Serial</label>
                      <input id="mach-sn" type="text" value={machineSerial} onChange={(e) => setMachineSerial(e.target.value)}
                        placeholder="SN-2407-112" className="w-full mt-1 rounded-lg border border-[var(--color-border)] bg-[var(--color-surface)] px-2.5 py-1.5 text-xs text-[var(--color-fg)] placeholder-[var(--color-fg-muted)] outline-none focus:border-[var(--color-brand)] transition-colors" />
                    </div>
                  </div>
                  {/* File upload */}
                  <div>
                    <label className="text-[10px] font-medium text-[var(--color-fg-muted)] uppercase tracking-wide">Manual (optional)</label>
                    <div className="mt-1 flex gap-2">
                      <button type="button" onClick={() => fileInputRef.current?.click()}
                        className="flex items-center gap-1.5 rounded-lg border border-dashed border-[var(--color-border)] px-3 py-1.5 text-xs text-[var(--color-fg-muted)] hover:border-[var(--color-brand)] hover:text-[var(--color-brand)] transition-colors">
                        <FileText size={13} /> {machineFileName || "Upload PDF"}
                      </button>
                      <input ref={fileInputRef} type="file" accept=".pdf,.txt,.md" className="hidden"
                        onChange={(e) => {
                          const f = e.target.files?.[0];
                          if (f) { setMachineFile(f); setMachineFileName(f.name); }
                        }} />
                      {machineFileName && (
                        <button type="button" onClick={() => { setMachineFile(null); setMachineFileName(""); }}
                          className="text-[var(--color-fg-muted)] hover:text-red-500 transition-colors">
                          <X size={13} />
                        </button>
                      )}
                    </div>
                  </div>
                  <button type="button" onClick={addMachine} disabled={!machineName.trim()}
                    className="flex items-center justify-center gap-1.5 rounded-lg bg-[var(--color-accent-bg)] px-3 py-1.5 text-xs font-medium text-[var(--color-brand)] hover:bg-[var(--color-brand)]/10 disabled:opacity-40 transition-colors">
                    <Plus size={13} /> Add machine
                  </button>
                </div>

                {/* Machine list */}
                {machines.length > 0 && (
                  <ul className="flex flex-col gap-1.5 max-h-32 overflow-y-auto">
                    {machines.map((m, i) => (
                      <li key={i} className="flex items-center gap-2 rounded-lg border border-[var(--color-border)] bg-[var(--color-bg)] px-3 py-2">
                        <Cpu size={13} className="text-[var(--color-fg-muted)] shrink-0" />
                        <span className="flex-1 text-xs text-[var(--color-fg)] truncate">{m.name}</span>
                        <span className="text-[10px] text-[var(--color-fg-muted)]">{m.machine_type}</span>
                        {m.fileName && <FileText size={11} className="text-[var(--color-fg-muted)]" />}
                        <button type="button" onClick={() => removeMachine(i)}
                          className="text-[var(--color-fg-muted)] hover:text-red-500 transition-colors">
                          <Trash2 size={12} />
                        </button>
                      </li>
                    ))}
                  </ul>
                )}

                {error && (
                  <p className="text-xs text-red-500 bg-red-50 dark:bg-red-950/30 border border-red-200 dark:border-red-800 rounded-lg px-3 py-2">
                    {error}
                  </p>
                )}

                <div className="flex gap-3 mt-1">
                  <button type="button" onClick={() => go(2)}
                    className="flex-1 rounded-lg border border-[var(--color-border)] px-4 py-2.5 text-sm text-[var(--color-fg-muted)] hover:bg-[var(--color-surface-2)] transition-colors">
                    Back
                  </button>
                  <button type="submit" disabled={loading}
                    className="flex-1 flex items-center justify-center gap-2 rounded-lg bg-[var(--color-brand)] px-4 py-2.5 text-sm font-medium text-white hover:opacity-90 disabled:opacity-50 transition-opacity">
                    {loading ? "Creating…" : machines.length > 0 ? `Finish (${machines.length} machine${machines.length > 1 ? 's' : ''})` : "Finish"}
                    {!loading && <Building2 size={14} />}
                  </button>
                </div>

                {machines.length === 0 && (
                  <p className="text-center text-xs text-[var(--color-fg-faint)]">
                    You can add machines from the Machines tab later.
                  </p>
                )}
              </motion.form>
            )}
          </AnimatePresence>
        </div>
      </div>
    </div>
  );
}
