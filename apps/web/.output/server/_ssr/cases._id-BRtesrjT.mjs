import { __toESM } from "../_runtime.mjs";
import { Link, require_jsx_runtime, require_react } from "../_libs/@tanstack/react-router+[...].mjs";
import { Route } from "./cases._id-CqRRudD0.mjs";
import { APPROVERS, api } from "./api-DVbl_fr0.mjs";
import { SourceTag, StatusBadge, VerdictText, money } from "./ui-BlurfRhS.mjs";
import { ChevronRight, CircleCheck, CircleX, Clock, FileSearch, LoaderCircle } from "../_libs/lucide-react.mjs";
//#region node_modules/.nitro/vite/services/ssr/assets/cases._id-BRtesrjT.js
var import_react = /* @__PURE__ */ __toESM(require_react());
var import_jsx_runtime = require_jsx_runtime();
function CasePage() {
	const { id } = Route.useParams();
	const [view, setView] = (0, import_react.useState)(null);
	const [error, setError] = (0, import_react.useState)(null);
	(0, import_react.useEffect)(() => {
		let closed = false;
		api.getCase(id).then(setView).catch((e) => setError(String(e)));
		const es = new EventSource(`/api/cases/${id}/stream`);
		es.onmessage = (ev) => {
			if (closed) return;
			try {
				setView(JSON.parse(ev.data));
			} catch {}
		};
		es.onerror = () => es.close();
		return () => {
			closed = true;
			es.close();
		};
	}, [id]);
	if (error) return /* @__PURE__ */ (0, import_jsx_runtime.jsx)("p", {
		className: "text-rose-600",
		children: error
	});
	if (!view) return /* @__PURE__ */ (0, import_jsx_runtime.jsx)("p", {
		className: "text-slate-500",
		children: "Loading…"
	});
	return /* @__PURE__ */ (0, import_jsx_runtime.jsxs)("div", {
		className: "space-y-6",
		children: [
			/* @__PURE__ */ (0, import_jsx_runtime.jsxs)("div", {
				className: "flex items-start justify-between",
				children: [/* @__PURE__ */ (0, import_jsx_runtime.jsxs)("div", { children: [/* @__PURE__ */ (0, import_jsx_runtime.jsxs)("div", {
					className: "flex items-center gap-3",
					children: [/* @__PURE__ */ (0, import_jsx_runtime.jsx)("h1", {
						className: "text-xl font-bold",
						children: title(view)
					}), /* @__PURE__ */ (0, import_jsx_runtime.jsx)(StatusBadge, { status: view.status })]
				}), /* @__PURE__ */ (0, import_jsx_runtime.jsxs)("p", {
					className: "mt-1 text-sm text-slate-500",
					children: [
						view.workflowLabel,
						" · ",
						view.tenantName,
						" · requested by ",
						view.requester
					]
				})] }), /* @__PURE__ */ (0, import_jsx_runtime.jsxs)(Link, {
					to: "/cases/$id/trace",
					params: { id },
					className: "flex items-center gap-1.5 rounded-lg border border-slate-200 bg-white px-3 py-1.5 text-sm text-slate-600 hover:bg-slate-50",
					children: [/* @__PURE__ */ (0, import_jsx_runtime.jsx)(FileSearch, { className: "h-4 w-4" }), "Trace"]
				})]
			}),
			/* @__PURE__ */ (0, import_jsx_runtime.jsx)(HolderBanner, { view }),
			/* @__PURE__ */ (0, import_jsx_runtime.jsxs)("div", {
				className: "grid gap-6 md:grid-cols-[1fr_300px]",
				children: [/* @__PURE__ */ (0, import_jsx_runtime.jsx)(Timeline, { stages: view.stages }), /* @__PURE__ */ (0, import_jsx_runtime.jsxs)("div", {
					className: "space-y-4",
					children: [
						/* @__PURE__ */ (0, import_jsx_runtime.jsx)(ContextCard, { view }),
						view.status === "waiting_human" && /* @__PURE__ */ (0, import_jsx_runtime.jsx)(ApproverPanel, {
							id,
							view,
							onUpdate: setView
						}),
						view.status === "needs_info" && /* @__PURE__ */ (0, import_jsx_runtime.jsx)(ProvideInfoPanel, {
							id,
							view,
							onUpdate: setView
						})
					]
				})]
			})
		]
	});
}
function HolderBanner({ view }) {
	if (view.status === "approved") return /* @__PURE__ */ (0, import_jsx_runtime.jsxs)("div", {
		className: "flex items-center gap-3 rounded-lg border border-emerald-200 bg-emerald-50 px-4 py-3 text-sm text-emerald-900",
		children: [/* @__PURE__ */ (0, import_jsx_runtime.jsx)(CircleCheck, { className: "h-5 w-5" }), /* @__PURE__ */ (0, import_jsx_runtime.jsxs)("span", { children: ["Approved and executed.", view.result?.po ? /* @__PURE__ */ (0, import_jsx_runtime.jsxs)("strong", {
			className: "ml-1",
			children: ["PO ", String(view.result.po)]
		}) : null] })]
	});
	if (view.status === "rejected") return /* @__PURE__ */ (0, import_jsx_runtime.jsxs)("div", {
		className: "flex items-center gap-3 rounded-lg border border-rose-200 bg-rose-50 px-4 py-3 text-sm text-rose-900",
		children: [/* @__PURE__ */ (0, import_jsx_runtime.jsx)(CircleX, { className: "h-5 w-5" }), /* @__PURE__ */ (0, import_jsx_runtime.jsxs)("span", { children: [
			"Rejected by ",
			/* @__PURE__ */ (0, import_jsx_runtime.jsx)("strong", { children: view.rejectedBy }),
			". ",
			view.reason
		] })]
	});
	if (view.status === "waiting_human") return /* @__PURE__ */ (0, import_jsx_runtime.jsxs)("div", {
		className: "flex items-center gap-3 rounded-lg border border-amber-200 bg-amber-50 px-4 py-3 text-sm text-amber-900",
		children: [/* @__PURE__ */ (0, import_jsx_runtime.jsx)(Clock, { className: "h-5 w-5" }), /* @__PURE__ */ (0, import_jsx_runtime.jsxs)("span", { children: [
			"Waiting on ",
			/* @__PURE__ */ (0, import_jsx_runtime.jsx)("strong", { children: view.holder }),
			". Nothing is running — the case is parked until they act."
		] })]
	});
	if (view.status === "needs_info") return /* @__PURE__ */ (0, import_jsx_runtime.jsxs)("div", {
		className: "flex items-center gap-3 rounded-lg border border-violet-200 bg-violet-50 px-4 py-3 text-sm text-violet-900",
		children: [/* @__PURE__ */ (0, import_jsx_runtime.jsx)(FileSearch, { className: "h-5 w-5" }), /* @__PURE__ */ (0, import_jsx_runtime.jsxs)("span", { children: ["More information needed. ", view.reason] })]
	});
	return /* @__PURE__ */ (0, import_jsx_runtime.jsxs)("div", {
		className: "flex items-center gap-3 rounded-lg border border-sky-200 bg-sky-50 px-4 py-3 text-sm text-sky-900",
		children: [/* @__PURE__ */ (0, import_jsx_runtime.jsx)(LoaderCircle, { className: "h-5 w-5 animate-spin" }), /* @__PURE__ */ (0, import_jsx_runtime.jsx)("span", { children: "Running…" })]
	});
}
function Timeline({ stages }) {
	return /* @__PURE__ */ (0, import_jsx_runtime.jsx)("ol", {
		className: "space-y-3",
		children: stages.map((s) => /* @__PURE__ */ (0, import_jsx_runtime.jsxs)("li", {
			className: "rounded-xl border border-slate-200 bg-white p-4",
			children: [/* @__PURE__ */ (0, import_jsx_runtime.jsxs)("div", {
				className: "flex items-center justify-between",
				children: [/* @__PURE__ */ (0, import_jsx_runtime.jsxs)("div", {
					className: "flex items-center gap-2",
					children: [
						/* @__PURE__ */ (0, import_jsx_runtime.jsx)(StageDot, { status: s.status }),
						/* @__PURE__ */ (0, import_jsx_runtime.jsx)("span", {
							className: "font-medium",
							children: s.label
						}),
						s.roleLabel && /* @__PURE__ */ (0, import_jsx_runtime.jsxs)("span", {
							className: "text-sm text-slate-400",
							children: ["· ", s.roleLabel]
						})
					]
				}), /* @__PURE__ */ (0, import_jsx_runtime.jsx)(StatusBadge, { status: s.status })]
			}), s.decision && /* @__PURE__ */ (0, import_jsx_runtime.jsxs)("div", {
				className: "mt-3 border-l-2 border-slate-100 pl-3 text-sm",
				children: [
					/* @__PURE__ */ (0, import_jsx_runtime.jsxs)("div", {
						className: "flex items-center gap-2",
						children: [/* @__PURE__ */ (0, import_jsx_runtime.jsx)(VerdictText, { verdict: s.decision.verdict }), /* @__PURE__ */ (0, import_jsx_runtime.jsx)(SourceTag, { source: s.decision.source })]
					}),
					/* @__PURE__ */ (0, import_jsx_runtime.jsx)("p", {
						className: "mt-1 text-slate-600",
						children: s.decision.reasoning
					}),
					s.decision.evidence && s.decision.evidence.length > 0 && /* @__PURE__ */ (0, import_jsx_runtime.jsx)("p", {
						className: "mt-1 text-xs text-slate-400",
						children: s.decision.evidence.join(" · ")
					})
				]
			})]
		}, s.nodeId))
	});
}
function StageDot({ status }) {
	if (status === "approved") return /* @__PURE__ */ (0, import_jsx_runtime.jsx)(CircleCheck, { className: "h-4 w-4 text-emerald-500" });
	if (status === "rejected") return /* @__PURE__ */ (0, import_jsx_runtime.jsx)(CircleX, { className: "h-4 w-4 text-rose-500" });
	if (status === "escalated") return /* @__PURE__ */ (0, import_jsx_runtime.jsx)(ChevronRight, { className: "h-4 w-4 text-orange-500" });
	if (status === "waiting") return /* @__PURE__ */ (0, import_jsx_runtime.jsx)(Clock, { className: "h-4 w-4 text-amber-500" });
	if (status === "in_progress") return /* @__PURE__ */ (0, import_jsx_runtime.jsx)(LoaderCircle, { className: "h-4 w-4 animate-spin text-sky-500" });
	return /* @__PURE__ */ (0, import_jsx_runtime.jsx)("span", { className: "inline-block h-4 w-4 rounded-full border-2 border-slate-300" });
}
function ContextCard({ view }) {
	return /* @__PURE__ */ (0, import_jsx_runtime.jsxs)("div", {
		className: "rounded-xl border border-slate-200 bg-white p-4 text-sm",
		children: [/* @__PURE__ */ (0, import_jsx_runtime.jsx)("h3", {
			className: "font-semibold",
			children: "Request"
		}), /* @__PURE__ */ (0, import_jsx_runtime.jsx)("dl", {
			className: "mt-2 space-y-1",
			children: Object.entries(view.context).map(([k, v]) => /* @__PURE__ */ (0, import_jsx_runtime.jsxs)("div", {
				className: "flex justify-between gap-3",
				children: [/* @__PURE__ */ (0, import_jsx_runtime.jsx)("dt", {
					className: "capitalize text-slate-400",
					children: k
				}), /* @__PURE__ */ (0, import_jsx_runtime.jsx)("dd", {
					className: "text-right font-medium",
					children: k === "amount" ? money(v) : String(v)
				})]
			}, k))
		})]
	});
}
function ApproverPanel({ id, view, onUpdate }) {
	const [actor, setActor] = (0, import_react.useState)(view.tenant === "globex" ? "director@globex" : "ceo@acme");
	const [reason, setReason] = (0, import_react.useState)("");
	const [busy, setBusy] = (0, import_react.useState)(false);
	const [err, setErr] = (0, import_react.useState)(null);
	const act = async (verdict) => {
		setBusy(true);
		setErr(null);
		try {
			onUpdate(await api.act(id, {
				actor,
				verdict,
				reason
			}));
		} catch (e) {
			setErr(String(e));
		} finally {
			setBusy(false);
		}
	};
	return /* @__PURE__ */ (0, import_jsx_runtime.jsxs)("div", {
		className: "rounded-xl border border-amber-200 bg-white p-4",
		children: [
			/* @__PURE__ */ (0, import_jsx_runtime.jsxs)("h3", {
				className: "text-sm font-semibold",
				children: ["You are ", view.holder]
			}),
			/* @__PURE__ */ (0, import_jsx_runtime.jsx)("p", {
				className: "mt-1 text-xs text-slate-500",
				children: "Deciding on top of the reviews above. The requester cannot approve their own request."
			}),
			/* @__PURE__ */ (0, import_jsx_runtime.jsx)("label", {
				className: "mt-3 block text-xs font-medium text-slate-500",
				children: "Act as"
			}),
			/* @__PURE__ */ (0, import_jsx_runtime.jsx)("select", {
				className: "mt-1 w-full rounded-lg border border-slate-200 px-2 py-1.5 text-sm",
				value: actor,
				onChange: (e) => setActor(e.target.value),
				children: Object.entries(APPROVERS).map(([k, label]) => /* @__PURE__ */ (0, import_jsx_runtime.jsx)("option", {
					value: k,
					children: label
				}, k))
			}),
			/* @__PURE__ */ (0, import_jsx_runtime.jsx)("textarea", {
				className: "mt-3 w-full rounded-lg border border-slate-200 px-2 py-1.5 text-sm",
				rows: 2,
				placeholder: "Reason (recorded in the trace)",
				value: reason,
				onChange: (e) => setReason(e.target.value)
			}),
			err && /* @__PURE__ */ (0, import_jsx_runtime.jsx)("p", {
				className: "mt-2 text-xs text-rose-600",
				children: err
			}),
			/* @__PURE__ */ (0, import_jsx_runtime.jsxs)("div", {
				className: "mt-3 flex gap-2",
				children: [/* @__PURE__ */ (0, import_jsx_runtime.jsx)("button", {
					onClick: () => act("approve"),
					disabled: busy,
					className: "flex-1 rounded-lg bg-emerald-600 px-3 py-1.5 text-sm font-medium text-white hover:bg-emerald-700 disabled:opacity-50",
					children: "Approve"
				}), /* @__PURE__ */ (0, import_jsx_runtime.jsx)("button", {
					onClick: () => act("reject"),
					disabled: busy,
					className: "flex-1 rounded-lg bg-rose-600 px-3 py-1.5 text-sm font-medium text-white hover:bg-rose-700 disabled:opacity-50",
					children: "Reject"
				})]
			})
		]
	});
}
function ProvideInfoPanel({ id, view, onUpdate }) {
	const [text, setText] = (0, import_react.useState)("");
	const [busy, setBusy] = (0, import_react.useState)(false);
	const submit = async () => {
		setBusy(true);
		try {
			onUpdate(await api.provideInfo(id, {
				actor: view.requester,
				patch: { justification: text }
			}));
		} finally {
			setBusy(false);
		}
	};
	return /* @__PURE__ */ (0, import_jsx_runtime.jsxs)("div", {
		className: "rounded-xl border border-violet-200 bg-white p-4",
		children: [
			/* @__PURE__ */ (0, import_jsx_runtime.jsx)("h3", {
				className: "text-sm font-semibold",
				children: "Provide more info"
			}),
			/* @__PURE__ */ (0, import_jsx_runtime.jsx)("p", {
				className: "mt-1 text-xs text-slate-500",
				children: view.reason
			}),
			/* @__PURE__ */ (0, import_jsx_runtime.jsx)("textarea", {
				className: "mt-3 w-full rounded-lg border border-slate-200 px-2 py-1.5 text-sm",
				rows: 2,
				placeholder: "Add the missing justification",
				value: text,
				onChange: (e) => setText(e.target.value)
			}),
			/* @__PURE__ */ (0, import_jsx_runtime.jsx)("button", {
				onClick: submit,
				disabled: busy || !text,
				className: "mt-3 w-full rounded-lg bg-violet-600 px-3 py-1.5 text-sm font-medium text-white hover:bg-violet-700 disabled:opacity-50",
				children: "Resubmit"
			})
		]
	});
}
function title(view) {
	const vendor = view.context.vendor;
	const amount = view.context.amount;
	if (vendor && amount != null) return `${vendor} — ${money(amount)}`;
	return view.workflowLabel;
}
//#endregion
export { CasePage as component };
