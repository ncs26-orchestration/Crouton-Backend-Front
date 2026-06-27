// Per-request node positions for the workflow canvas, persisted in the browser
// so a user's manual layout survives reloads. Keyed by request id; positions
// are a plain { [nodeId]: {x, y} } map. Falls back silently if storage is
// unavailable or the payload is malformed.

export type NodePositions = Record<string, { x: number; y: number }>;

const key = (requestId: string) => `aios.workflow-layout.${requestId}`;

export function loadNodePositions(requestId: string): NodePositions {
  try {
    const raw = localStorage.getItem(key(requestId));
    if (!raw) return {};
    const parsed = JSON.parse(raw) as unknown;
    if (!parsed || typeof parsed !== "object") return {};
    const out: NodePositions = {};
    for (const [id, pos] of Object.entries(parsed as Record<string, unknown>)) {
      if (
        pos &&
        typeof pos === "object" &&
        typeof (pos as { x?: unknown }).x === "number" &&
        typeof (pos as { y?: unknown }).y === "number"
      ) {
        out[id] = { x: (pos as { x: number }).x, y: (pos as { y: number }).y };
      }
    }
    return out;
  } catch {
    return {};
  }
}

export function saveNodePositions(requestId: string, positions: NodePositions): void {
  try {
    localStorage.setItem(key(requestId), JSON.stringify(positions));
  } catch {
    /* storage full or unavailable — non-fatal */
  }
}

export function clearNodePositions(requestId: string): void {
  try {
    localStorage.removeItem(key(requestId));
  } catch {
    /* non-fatal */
  }
}
