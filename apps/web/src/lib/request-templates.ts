// Request templates: optional structured fields per category. These are a
// convenience for the operator and give the agents concrete facts to validate
// against — they are NOT a rigid workflow-type system. "General" is freeform and
// intake still classifies and plans any request dynamically.

export type FieldType = "text" | "number" | "date" | "select";

export interface TemplateField {
  key: string;
  label: string;
  type: FieldType;
  placeholder?: string;
  options?: string[];
}

export interface RequestTemplate {
  id: string;
  label: string;
  hint: string;
  fields: TemplateField[];
}

export const REQUEST_TEMPLATES: RequestTemplate[] = [
  {
    id: "general",
    label: "General",
    hint: "Anything else. Just describe what you need.",
    fields: [],
  },
  {
    id: "procurement",
    label: "Procurement",
    hint: "Buy goods or services.",
    fields: [
      { key: "vendor", label: "Vendor", type: "text", placeholder: "Dell" },
      { key: "quantity", label: "Quantity", type: "number", placeholder: "50" },
      { key: "unit_cost", label: "Unit cost (USD)", type: "number", placeholder: "1840" },
      { key: "total_cost", label: "Total cost (USD)", type: "number", placeholder: "92000" },
      { key: "needed_by", label: "Needed by", type: "date" },
    ],
  },
  {
    id: "hiring",
    label: "Hiring",
    hint: "Open new roles or add headcount.",
    fields: [
      { key: "role", label: "Role", type: "text", placeholder: "Backend Engineer" },
      { key: "headcount", label: "Headcount", type: "number", placeholder: "3" },
      {
        key: "seniority",
        label: "Seniority",
        type: "select",
        options: ["Junior", "Mid", "Senior", "Staff", "Lead"],
      },
      { key: "comp_band", label: "Comp band (USD)", type: "text", placeholder: "120k-160k" },
      { key: "start_date", label: "Target start", type: "date" },
    ],
  },
  {
    id: "budget",
    label: "Budget / spend",
    hint: "Request funding for an initiative.",
    fields: [
      { key: "amount", label: "Amount (USD)", type: "number", placeholder: "75000" },
      { key: "category", label: "Category", type: "text", placeholder: "Marketing" },
      {
        key: "period",
        label: "Period",
        type: "select",
        options: ["One-time", "Monthly", "Quarterly", "Annual"],
      },
    ],
  },
  {
    id: "timeoff",
    label: "Time off",
    hint: "Request leave.",
    fields: [
      {
        key: "leave_type",
        label: "Type",
        type: "select",
        options: ["Vacation", "Sick", "Parental", "Unpaid", "Other"],
      },
      { key: "start_date", label: "Start", type: "date" },
      { key: "end_date", label: "End", type: "date" },
      { key: "coverage", label: "Coverage", type: "text", placeholder: "Who covers your work" },
    ],
  },
  {
    id: "support",
    label: "Support",
    hint: "Report an issue or request help.",
    fields: [
      { key: "system", label: "System", type: "text", placeholder: "CRM" },
      {
        key: "impact",
        label: "Impact",
        type: "select",
        options: ["Blocking", "Major", "Minor"],
      },
    ],
  },
  {
    id: "policy_change",
    label: "Policy change",
    hint: "Propose a change to a company policy.",
    fields: [
      { key: "policy_area", label: "Policy area", type: "text", placeholder: "Remote work" },
      { key: "effective_date", label: "Effective", type: "date" },
    ],
  },
  {
    id: "infra",
    label: "IT / Infrastructure",
    hint: "Provision or change systems or infrastructure.",
    fields: [
      { key: "system", label: "System", type: "text", placeholder: "Data warehouse" },
      { key: "environment", label: "Environment", type: "text", placeholder: "Production" },
      { key: "est_cost", label: "Est. monthly cost (USD)", type: "number", placeholder: "4000" },
    ],
  },
];

// Build a clean details object: drop empty values and coerce numbers.
export function collectDetails(
  template: RequestTemplate,
  raw: Record<string, string>,
): Record<string, string | number> {
  const out: Record<string, string | number> = {};
  for (const f of template.fields) {
    const v = (raw[f.key] ?? "").trim();
    if (!v) continue;
    out[f.key] = f.type === "number" && !Number.isNaN(Number(v)) ? Number(v) : v;
  }
  return out;
}

// A short, human label for a details key ("unit_cost" -> "Unit cost").
export function detailLabel(key: string): string {
  return key.replace(/_/g, " ").replace(/\b\w/g, (c) => c.toUpperCase());
}
