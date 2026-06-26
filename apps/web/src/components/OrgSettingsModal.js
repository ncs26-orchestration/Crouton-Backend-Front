import { useEffect, useState } from "react";
import { motion } from "framer-motion";
import { Building2, Loader2, X } from "lucide-react";
import { api } from "../lib/api";
export function OrgSettingsModal({ projectId, projectName, overview: initialOverview, onClose, onSaved }) {
    const [isLoading, setIsLoading] = useState(!initialOverview);
    const [formData, setFormData] = useState(initialOverview ?? {
        name: projectName,
        industry: "",
        size: "",
        regions: [],
        systems: [],
        processes: [],
        compliance: [],
        languages: [],
    });
    const [isSaving, setIsSaving] = useState(false);
    const [error, setError] = useState(null);
    useEffect(() => {
        if (initialOverview) {
            setFormData(initialOverview);
            setIsLoading(false);
            return;
        }
        api.getProject(projectId).then((data) => {
            const overview = data.project.overview_json;
            if (overview) {
                setFormData({
                    name: overview.name || projectName,
                    industry: overview.industry || "",
                    size: overview.size || "",
                    regions: overview.regions || [],
                    systems: overview.systems || [],
                    processes: overview.processes || [],
                    compliance: overview.compliance || [],
                    languages: overview.languages || [],
                });
            }
            else {
                setFormData((prev) => ({ ...prev, name: projectName }));
            }
            setIsLoading(false);
        }).catch(() => {
            setFormData((prev) => ({ ...prev, name: projectName }));
            setIsLoading(false);
        });
    }, [projectId, projectName, initialOverview]);
    const handleSave = async () => {
        setIsSaving(true);
        setError(null);
        try {
            await api.updateProjectOverview(projectId, formData);
            onSaved(formData);
            onClose();
        }
        catch (e) {
            setError(e instanceof Error ? e.message : "Failed to save");
        }
        finally {
            setIsSaving(false);
        }
    };
    const updateField = (field, value) => {
        setFormData((prev) => ({ ...prev, [field]: value }));
    };
    return (<motion.div initial={{ opacity: 0 }} animate={{ opacity: 1 }} exit={{ opacity: 0 }} className="fixed inset-0 z-50 bg-black/50 backdrop-blur-sm flex items-center justify-center p-6">
      <motion.div initial={{ opacity: 0, y: 20, scale: 0.98 }} animate={{ opacity: 1, y: 0, scale: 1 }} exit={{ opacity: 0, y: 10, scale: 0.99 }} transition={{ type: "spring", stiffness: 300, damping: 26 }} className="relative max-w-2xl w-full max-h-[90vh] bg-[var(--color-surface)] border border-[var(--color-border)] rounded-xl shadow-stripe-deep overflow-hidden flex flex-col">
        <div className="px-6 py-4 border-b border-[var(--color-border)] flex items-center justify-between">
          <div className="flex items-center gap-3">
            <Building2 size={18} className="text-[var(--color-brand)]"/>
            <div>
              <h2 className="text-lg font-semibold text-[var(--color-fg)]">Organisation Overview</h2>
              <p className="text-[11px] text-[var(--color-fg-muted)]">{projectName}</p>
            </div>
          </div>
          <button onClick={onClose} className="text-[var(--color-fg-subtle)] hover:text-[var(--color-fg)]">
            <X size={18}/>
          </button>
        </div>

        <div className="flex-1 overflow-y-auto px-6 py-5 space-y-6">
          {isLoading ? (<div className="flex items-center justify-center py-12">
              <Loader2 size={24} className="animate-spin text-[var(--color-brand)]"/>
            </div>) : (<>
              <Section title="Basic Information">
                <FieldRow label="Organisation Name">
                  <input value={formData.name ?? ""} onChange={(e) => updateField("name", e.target.value)} className="bg-[var(--color-surface-2)] border border-[var(--color-border)] rounded-md px-3 py-2 text-[13px] text-[var(--color-fg)] focus:outline-none focus:border-[var(--color-brand)] w-full"/>
                </FieldRow>
            <FieldRow label="Industry">
              <select value={formData.industry ?? ""} onChange={(e) => updateField("industry", e.target.value)} className="bg-[var(--color-surface-2)] border border-[var(--color-border)] rounded-md px-3 py-2 text-[13px] text-[var(--color-fg)] focus:outline-none focus:border-[var(--color-brand)] w-full">
                <option value="">Select industry...</option>
                <option value="Healthcare">Healthcare</option>
                <option value="Finance">Finance</option>
                <option value="Logistics">Logistics</option>
                <option value="Manufacturing">Manufacturing</option>
                <option value="Retail">Retail</option>
                <option value="Education">Education</option>
                <option value="Energy">Energy</option>
                <option value="Telecommunications">Telecommunications</option>
                <option value="Government">Government</option>
                <option value="Other">Other</option>
              </select>
            </FieldRow>
            <FieldRow label="Size">
              <select value={formData.size ?? ""} onChange={(e) => updateField("size", e.target.value)} className="bg-[var(--color-surface-2)] border border-[var(--color-border)] rounded-md px-3 py-2 text-[13px] text-[var(--color-fg)] focus:outline-none focus:border-[var(--color-brand)] w-full">
                <option value="">Select size...</option>
                <option value="1-10">1-10 employees</option>
                <option value="10-50">10-50 employees</option>
                <option value="50-200">50-200 employees</option>
                <option value="200-1000">200-1000 employees</option>
                <option value="1000+">1000+ employees</option>
              </select>
            </FieldRow>
          </Section>

          <Section title="Location & Industry">
            <FieldRow label="Regions">
              <ChipSelect options={["Algeria", "France", "Tunisia", "Morocco", "Egypt", "Libya", "Mauritania"]} values={formData.regions ?? []} onChange={(vals) => updateField("regions", vals)}/>
            </FieldRow>
            <FieldRow label="Languages">
              <ChipSelect options={["French", "Arabic", "English", "Spanish"]} values={formData.languages ?? []} onChange={(vals) => updateField("languages", vals)}/>
            </FieldRow>
          </Section>

          <Section title="Systems & Processes">
            <FieldRow label="Business Systems">
              <ChipSelect options={["ERP", "CRM", "Accounting", "HRM", "Supply Chain", "None"]} values={formData.systems ?? []} onChange={(vals) => updateField("systems", vals)}/>
            </FieldRow>
            <FieldRow label="Main Business Processes">
              <ChipSelect options={["Order Management", "Invoicing", "Inventory", "Procurement", "HR", "Customer Support", "Finance", "Marketing", "Sales", "None"]} values={formData.processes ?? []} onChange={(vals) => updateField("processes", vals)}/>
            </FieldRow>
          </Section>

          <Section title="Compliance">
            <FieldRow label="Compliance Requirements">
              <ChipSelect options={["GDPR", "ISO27001", "SOC2", "HIPAA", "PCI-DSS", "None"]} values={formData.compliance ?? []} onChange={(vals) => updateField("compliance", vals)}/>
            </FieldRow>
          </Section>

          {error && (<p className="text-[12px] text-red-500">{error}</p>)}
        </>)}
        </div>
      </motion.div>
    </motion.div>);
}
function Section({ title, children }) {
    return (<div>
      <h3 className="text-[12px] uppercase tracking-wider text-[var(--color-fg-muted)] mb-3" style={{ fontWeight: 500 }}>
        {title}
      </h3>
      <div className="space-y-3">{children}</div>
    </div>);
}
function FieldRow({ label, children }) {
    return (<div>
      <label className="flex flex-col gap-1">
        <span className="text-[11px] text-[var(--color-fg-subtle)]" style={{ fontWeight: 500 }}>
          {label}
        </span>
        {children}
      </label>
    </div>);
}
function ChipSelect({ options, values, onChange }) {
    const [inputValue, setInputValue] = useState("");
    const toggle = (option) => {
        if (values.includes(option)) {
            onChange(values.filter(v => v !== option));
        }
        else {
            onChange([...values, option]);
        }
    };
    const handleKeyDown = (e) => {
        if (e.key === "Enter" && inputValue.trim()) {
            e.preventDefault();
            if (!values.includes(inputValue.trim())) {
                onChange([...values, inputValue.trim()]);
            }
            setInputValue("");
        }
    };
    return (<div className="space-y-2">
      <div className="flex flex-wrap gap-1.5">
        {options.map((option) => (<button key={option} onClick={() => toggle(option)} className={`px-2.5 py-1 rounded-full border text-[11px] transition-colors ${values.includes(option)
                ? "border-[var(--color-brand)] bg-[var(--color-accent-bg)] text-[var(--color-fg)]"
                : "border-[var(--color-border)] text-[var(--color-fg-muted)] hover:border-[var(--color-fg-subtle)]"}`}>
            {option}
          </button>))}
      </div>
      <div className="flex flex-wrap gap-1.5 items-center p-1.5 bg-[var(--color-surface-2)] border border-[var(--color-border)] rounded-lg min-h-[32px]">
        {values.filter(v => !options.includes(v)).map((val) => (<span key={val} className="inline-flex items-center gap-1 px-2 py-0.5 rounded bg-[var(--color-accent-bg)] border border-[var(--color-brand)] text-[11px] text-[var(--color-fg)]">
            {val}
            <button onClick={() => onChange(values.filter(v => v !== val))} className="text-[var(--color-fg-muted)] hover:text-[var(--color-fg)]">
              <X size={10}/>
            </button>
          </span>))}
        <input value={inputValue} onChange={(e) => setInputValue(e.target.value)} onKeyDown={handleKeyDown} placeholder="Type custom and press Enter..." className="flex-1 min-w-[120px] bg-transparent border-none outline-none text-[12px] text-[var(--color-fg)] placeholder:text-[var(--color-fg-subtle)]"/>
      </div>
    </div>);
}
