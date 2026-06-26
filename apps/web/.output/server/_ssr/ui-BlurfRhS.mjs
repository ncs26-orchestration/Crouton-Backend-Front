import { require_jsx_runtime } from "../_libs/@tanstack/react-router+[...].mjs";
//#region node_modules/.nitro/vite/services/ssr/assets/ui-BlurfRhS.js
var import_jsx_runtime = require_jsx_runtime();
var STATUS_STYLES = {
	approved: "bg-emerald-100 text-emerald-800 ring-emerald-600/20",
	rejected: "bg-rose-100 text-rose-800 ring-rose-600/20",
	waiting_human: "bg-amber-100 text-amber-800 ring-amber-600/20",
	waiting: "bg-amber-100 text-amber-800 ring-amber-600/20",
	needs_info: "bg-violet-100 text-violet-800 ring-violet-600/20",
	running: "bg-sky-100 text-sky-800 ring-sky-600/20",
	in_progress: "bg-sky-100 text-sky-800 ring-sky-600/20",
	escalated: "bg-orange-100 text-orange-800 ring-orange-600/20",
	pending: "bg-slate-100 text-slate-500 ring-slate-500/20"
};
var LABELS = {
	waiting_human: "Waiting on approval",
	needs_info: "Needs info",
	in_progress: "In progress"
};
function StatusBadge({ status }) {
	const style = STATUS_STYLES[status] ?? STATUS_STYLES.pending;
	const label = LABELS[status] ?? status.replace(/_/g, " ");
	return /* @__PURE__ */ (0, import_jsx_runtime.jsx)("span", {
		className: `inline-flex items-center rounded-full px-2.5 py-0.5 text-xs font-medium capitalize ring-1 ring-inset ${style}`,
		children: label
	});
}
var VERDICT_STYLES = {
	approve: "text-emerald-700",
	reject: "text-rose-700",
	escalate: "text-orange-700",
	request_info: "text-violet-700"
};
function VerdictText({ verdict }) {
	return /* @__PURE__ */ (0, import_jsx_runtime.jsx)("span", {
		className: `font-semibold capitalize ${VERDICT_STYLES[verdict] ?? "text-slate-700"}`,
		children: verdict.replace(/_/g, " ")
	});
}
function SourceTag({ source }) {
	const map = {
		policy: {
			label: "policy rule",
			cls: "bg-slate-200 text-slate-700"
		},
		llm: {
			label: "LLM judgment",
			cls: "bg-indigo-100 text-indigo-700"
		},
		fallback: {
			label: "deterministic",
			cls: "bg-slate-100 text-slate-600"
		},
		human: {
			label: "human",
			cls: "bg-amber-100 text-amber-800"
		}
	};
	const s = map[source] ?? map.fallback;
	return /* @__PURE__ */ (0, import_jsx_runtime.jsx)("span", {
		className: `rounded px-1.5 py-0.5 text-[10px] font-medium uppercase tracking-wide ${s.cls}`,
		children: s.label
	});
}
function money(v) {
	const n = typeof v === "number" ? v : Number(v);
	if (Number.isNaN(n)) return String(v);
	return n.toLocaleString("en-US", {
		style: "currency",
		currency: "USD",
		maximumFractionDigits: 0
	});
}
//#endregion
export { SourceTag, StatusBadge, VerdictText, money };
