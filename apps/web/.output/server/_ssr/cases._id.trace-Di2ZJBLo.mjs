import { __toESM } from "../_runtime.mjs";
import { Link, require_jsx_runtime, require_react } from "../_libs/@tanstack/react-router+[...].mjs";
import { api } from "./api-DVbl_fr0.mjs";
import { SourceTag, VerdictText } from "./ui-BlurfRhS.mjs";
import { ArrowLeft } from "../_libs/lucide-react.mjs";
import { Route } from "./cases._id.trace-DWjo3JOX.mjs";
//#region node_modules/.nitro/vite/services/ssr/assets/cases._id.trace-Di2ZJBLo.js
var import_react = /* @__PURE__ */ __toESM(require_react());
var import_jsx_runtime = require_jsx_runtime();
function TracePage() {
	const { id } = Route.useParams();
	const [events, setEvents] = (0, import_react.useState)([]);
	(0, import_react.useEffect)(() => {
		api.getEvents(id).then(setEvents).catch(() => setEvents([]));
	}, [id]);
	return /* @__PURE__ */ (0, import_jsx_runtime.jsxs)("div", {
		className: "space-y-6",
		children: [
			/* @__PURE__ */ (0, import_jsx_runtime.jsxs)(Link, {
				to: "/cases/$id",
				params: { id },
				className: "inline-flex items-center gap-1.5 text-sm text-slate-500 hover:text-slate-900",
				children: [/* @__PURE__ */ (0, import_jsx_runtime.jsx)(ArrowLeft, { className: "h-4 w-4" }), "Back to status"]
			}),
			/* @__PURE__ */ (0, import_jsx_runtime.jsxs)("div", { children: [/* @__PURE__ */ (0, import_jsx_runtime.jsx)("h1", {
				className: "text-xl font-bold",
				children: "Decision trace"
			}), /* @__PURE__ */ (0, import_jsx_runtime.jsxs)("p", {
				className: "mt-1 text-sm text-slate-500",
				children: [events.length, " events. This is the source of truth; the status page is just a fold over it."]
			})] }),
			/* @__PURE__ */ (0, import_jsx_runtime.jsx)("ol", {
				className: "relative space-y-0 border-l border-slate-200 pl-6",
				children: events.map((e) => /* @__PURE__ */ (0, import_jsx_runtime.jsxs)("li", {
					className: "relative pb-6",
					children: [
						/* @__PURE__ */ (0, import_jsx_runtime.jsx)("span", { className: "absolute -left-[1.65rem] mt-1 h-3 w-3 rounded-full border-2 border-white bg-slate-300" }),
						/* @__PURE__ */ (0, import_jsx_runtime.jsxs)("div", {
							className: "flex items-center gap-2 text-sm",
							children: [
								/* @__PURE__ */ (0, import_jsx_runtime.jsxs)("span", {
									className: "font-mono text-xs text-slate-400",
									children: ["#", e.seq]
								}),
								/* @__PURE__ */ (0, import_jsx_runtime.jsx)(EventLabel, { type: e.type }),
								e.nodeId && /* @__PURE__ */ (0, import_jsx_runtime.jsxs)("span", {
									className: "text-slate-400",
									children: ["· ", e.nodeId]
								}),
								/* @__PURE__ */ (0, import_jsx_runtime.jsxs)("span", {
									className: "text-slate-400",
									children: ["· ", e.actor]
								})
							]
						}),
						e.payload?.decision && /* @__PURE__ */ (0, import_jsx_runtime.jsxs)("div", {
							className: "mt-1.5 rounded-lg bg-slate-50 p-3 text-sm",
							children: [
								/* @__PURE__ */ (0, import_jsx_runtime.jsxs)("div", {
									className: "flex items-center gap-2",
									children: [
										/* @__PURE__ */ (0, import_jsx_runtime.jsx)(VerdictText, { verdict: e.payload.decision.verdict }),
										/* @__PURE__ */ (0, import_jsx_runtime.jsx)(SourceTag, { source: e.payload.decision.source }),
										typeof e.payload.decision.confidence === "number" && /* @__PURE__ */ (0, import_jsx_runtime.jsxs)("span", {
											className: "text-xs text-slate-400",
											children: [
												"confidence ",
												Math.round(e.payload.decision.confidence * 100),
												"%"
											]
										})
									]
								}),
								/* @__PURE__ */ (0, import_jsx_runtime.jsx)("p", {
									className: "mt-1 text-slate-600",
									children: e.payload.decision.reasoning
								}),
								e.payload.decision.policyRefs?.length > 0 && /* @__PURE__ */ (0, import_jsx_runtime.jsxs)("p", {
									className: "mt-1 text-xs text-slate-400",
									children: ["policy: ", e.payload.decision.policyRefs.join(", ")]
								})
							]
						}),
						e.type === "completed" && /* @__PURE__ */ (0, import_jsx_runtime.jsxs)("p", {
							className: "mt-1 text-sm text-slate-600",
							children: [
								"outcome: ",
								/* @__PURE__ */ (0, import_jsx_runtime.jsx)("strong", { children: e.payload.outcome }),
								e.payload.result?.po ? ` · PO ${e.payload.result.po}` : ""
							]
						})
					]
				}, e.seq))
			})
		]
	});
}
var EVENT_LABELS = {
	submitted: "Request submitted",
	stage_entered: "Entered stage",
	decision: "Agent decision",
	routed: "Routed",
	human_requested: "Parked for human",
	human_decided: "Human decided",
	info_requested: "Requested info",
	info_provided: "Info provided",
	completed: "Completed"
};
function EventLabel({ type }) {
	return /* @__PURE__ */ (0, import_jsx_runtime.jsx)("span", {
		className: "font-medium",
		children: EVENT_LABELS[type] ?? type
	});
}
//#endregion
export { TracePage as component };
