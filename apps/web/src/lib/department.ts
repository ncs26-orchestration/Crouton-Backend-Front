// Stable per-department color + initials, shared by the canvas nodes and the
// Agents page so an agent reads as the same identity everywhere. Known
// departments get a fixed hue; anything else is hashed to one.
const DEPT_COLORS: Record<string, string> = {
  planning: "#6366f1",
  finance: "#0ea5e9",
  legal: "#8b5cf6",
  it: "#14b8a6",
  hr: "#ec4899",
  operations: "#f59e0b",
  executive: "#64748b",
};
const DEPT_FALLBACK = ["#6366f1", "#0ea5e9", "#14b8a6", "#f59e0b", "#ec4899", "#8b5cf6", "#ef4444"];

export function departmentColor(department: string): string {
  const key = department.trim().toLowerCase();
  if (DEPT_COLORS[key]) return DEPT_COLORS[key]!;
  let hash = 0;
  for (let i = 0; i < key.length; i++) hash = (hash * 31 + key.charCodeAt(i)) | 0;
  return DEPT_FALLBACK[Math.abs(hash) % DEPT_FALLBACK.length]!;
}

export function departmentInitials(department: string): string {
  const words = department.trim().split(/\s+/).filter(Boolean);
  if (words.length === 0) return "?";
  if (words.length === 1) return words[0]!.slice(0, 2).toUpperCase();
  return (words[0]![0]! + words[1]![0]!).toUpperCase();
}
