// A small initials avatar with a stable per-name color. Used for assigned
// verifiers on nodes, chips, and pickers — no uploaded images.

const COLORS = [
  "#6366f1", "#0ea5e9", "#14b8a6", "#f59e0b", "#ec4899", "#8b5cf6", "#ef4444", "#10b981",
];

function colorFor(seed: string): string {
  let hash = 0;
  for (let i = 0; i < seed.length; i++) hash = (hash * 31 + seed.charCodeAt(i)) | 0;
  return COLORS[Math.abs(hash) % COLORS.length]!;
}

function initials(name: string): string {
  const words = name.trim().split(/\s+/).filter(Boolean);
  if (words.length === 0) return "?";
  if (words.length === 1) return words[0]!.slice(0, 2).toUpperCase();
  return (words[0]![0]! + words[1]![0]!).toUpperCase();
}

export function Avatar({
  name,
  size = 20,
  title,
}: {
  name: string;
  size?: number;
  title?: string;
}) {
  return (
    <span
      title={title ?? name}
      className="inline-flex items-center justify-center rounded-full font-semibold text-white shrink-0"
      style={{
        width: size,
        height: size,
        background: colorFor(name),
        fontSize: Math.round(size * 0.4),
      }}
    >
      {initials(name)}
    </span>
  );
}
