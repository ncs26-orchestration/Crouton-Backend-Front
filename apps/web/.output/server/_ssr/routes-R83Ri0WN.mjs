import { __toESM } from "../_runtime.mjs";
import { require_jsx_runtime, require_react, useNavigate } from "../_libs/@tanstack/react-router+[...].mjs";
import { api } from "./api-DVbl_fr0.mjs";
import { ArrowRight, Building2 } from "../_libs/lucide-react.mjs";
//#region node_modules/.nitro/vite/services/ssr/assets/routes-R83Ri0WN.js
var import_react = /* @__PURE__ */ __toESM(require_react());
var import_jsx_runtime = require_jsx_runtime();
function Home() {
	const navigate = useNavigate();
	const [tenants, setTenants] = (0, import_react.useState)([]);
	const [tenant, setTenant] = (0, import_react.useState)("");
	const [workflows, setWorkflows] = (0, import_react.useState)([]);
	const [workflow, setWorkflow] = (0, import_react.useState)(null);
	const [values, setValues] = (0, import_react.useState)({});
	const [error, setError] = (0, import_react.useState)(null);
	const [submitting, setSubmitting] = (0, import_react.useState)(false);
	(0, import_react.useEffect)(() => {
		api.tenants().then((t) => {
			setTenants(t);
			if (t[0]) setTenant(t[0].id);
		});
	}, []);
	(0, import_react.useEffect)(() => {
		if (!tenant) return;
		api.workflows(tenant).then((w) => {
			setWorkflows(w);
			setWorkflow(w[0] ?? null);
			setValues({});
		});
	}, [tenant]);
	const onSubmit = async (e) => {
		e.preventDefault();
		if (!workflow) return;
		setSubmitting(true);
		setError(null);
		try {
			const payload = {};
			for (const f of workflow.fields) {
				const v = values[f] ?? "";
				payload[f] = f === "amount" ? Number(v) : v;
			}
			const { caseId } = await api.createRequest({
				tenantId: tenant,
				workflowId: workflow.id,
				requester: `employee@${tenant}`,
				payload
			});
			navigate({
				to: "/cases/$id",
				params: { id: caseId }
			});
		} catch (err) {
			setError(String(err));
			setSubmitting(false);
		}
	};
	return /* @__PURE__ */ (0, import_jsx_runtime.jsxs)("div", {
		className: "grid gap-8 md:grid-cols-[1fr_360px]",
		children: [/* @__PURE__ */ (0, import_jsx_runtime.jsxs)("section", { children: [
			/* @__PURE__ */ (0, import_jsx_runtime.jsx)("h1", {
				className: "text-2xl font-bold tracking-tight",
				children: "Submit a purchase request"
			}),
			/* @__PURE__ */ (0, import_jsx_runtime.jsx)("p", {
				className: "mt-1 text-sm text-slate-500",
				children: "The same workflow runs in every organization. Which departments review it, and the thresholds they enforce, are configuration — not code."
			}),
			/* @__PURE__ */ (0, import_jsx_runtime.jsxs)("form", {
				onSubmit,
				className: "mt-6 space-y-5 rounded-xl border border-slate-200 bg-white p-6",
				children: [
					/* @__PURE__ */ (0, import_jsx_runtime.jsxs)("div", { children: [/* @__PURE__ */ (0, import_jsx_runtime.jsx)("label", {
						className: "mb-1.5 block text-sm font-medium",
						children: "Organization"
					}), /* @__PURE__ */ (0, import_jsx_runtime.jsx)("div", {
						className: "grid grid-cols-2 gap-2",
						children: tenants.map((t) => /* @__PURE__ */ (0, import_jsx_runtime.jsxs)("button", {
							type: "button",
							onClick: () => setTenant(t.id),
							className: `flex items-center gap-2 rounded-lg border px-3 py-2 text-left text-sm ${tenant === t.id ? "border-indigo-500 bg-indigo-50 text-indigo-900 ring-1 ring-indigo-500" : "border-slate-200 hover:border-slate-300"}`,
							children: [/* @__PURE__ */ (0, import_jsx_runtime.jsx)(Building2, { className: "h-4 w-4 shrink-0 text-slate-400" }), /* @__PURE__ */ (0, import_jsx_runtime.jsx)("span", {
								className: "font-medium",
								children: t.name
							})]
						}, t.id))
					})] }),
					workflows.length > 1 && /* @__PURE__ */ (0, import_jsx_runtime.jsxs)("div", { children: [/* @__PURE__ */ (0, import_jsx_runtime.jsx)("label", {
						className: "mb-1.5 block text-sm font-medium",
						children: "Request type"
					}), /* @__PURE__ */ (0, import_jsx_runtime.jsx)("select", {
						className: "w-full rounded-lg border border-slate-200 px-3 py-2 text-sm",
						value: workflow?.id ?? "",
						onChange: (e) => setWorkflow(workflows.find((w) => w.id === e.target.value) ?? null),
						children: workflows.map((w) => /* @__PURE__ */ (0, import_jsx_runtime.jsx)("option", {
							value: w.id,
							children: w.label
						}, w.id))
					})] }),
					workflow?.fields.map((f) => /* @__PURE__ */ (0, import_jsx_runtime.jsxs)("div", { children: [/* @__PURE__ */ (0, import_jsx_runtime.jsx)("label", {
						className: "mb-1.5 block text-sm font-medium capitalize",
						children: f
					}), f === "justification" ? /* @__PURE__ */ (0, import_jsx_runtime.jsx)("textarea", {
						className: "w-full rounded-lg border border-slate-200 px-3 py-2 text-sm",
						rows: 2,
						value: values[f] ?? "",
						onChange: (e) => setValues({
							...values,
							[f]: e.target.value
						}),
						placeholder: "Business reason for the purchase"
					}) : /* @__PURE__ */ (0, import_jsx_runtime.jsx)("input", {
						className: "w-full rounded-lg border border-slate-200 px-3 py-2 text-sm",
						type: f === "amount" ? "number" : "text",
						value: values[f] ?? "",
						onChange: (e) => setValues({
							...values,
							[f]: e.target.value
						}),
						placeholder: placeholderFor(f)
					})] }, f)),
					error && /* @__PURE__ */ (0, import_jsx_runtime.jsx)("p", {
						className: "text-sm text-rose-600",
						children: error
					}),
					/* @__PURE__ */ (0, import_jsx_runtime.jsxs)("button", {
						type: "submit",
						disabled: submitting || !workflow,
						className: "inline-flex items-center gap-2 rounded-lg bg-indigo-600 px-4 py-2 text-sm font-medium text-white hover:bg-indigo-700 disabled:opacity-50",
						children: [submitting ? "Submitting…" : "Submit request", /* @__PURE__ */ (0, import_jsx_runtime.jsx)(ArrowRight, { className: "h-4 w-4" })]
					})
				]
			})
		] }), /* @__PURE__ */ (0, import_jsx_runtime.jsx)("aside", {
			className: "space-y-4",
			children: /* @__PURE__ */ (0, import_jsx_runtime.jsxs)("div", {
				className: "rounded-xl border border-slate-200 bg-white p-5 text-sm",
				children: [/* @__PURE__ */ (0, import_jsx_runtime.jsx)("h2", {
					className: "font-semibold",
					children: "Try the three demo paths"
				}), /* @__PURE__ */ (0, import_jsx_runtime.jsxs)("ul", {
					className: "mt-3 space-y-3 text-slate-600",
					children: [
						/* @__PURE__ */ (0, import_jsx_runtime.jsxs)("li", { children: [
							/* @__PURE__ */ (0, import_jsx_runtime.jsx)("span", {
								className: "font-medium text-slate-900",
								children: "Happy path + human pause."
							}),
							" Acme, $30,000, vendor ",
							/* @__PURE__ */ (0, import_jsx_runtime.jsx)("code", {
								className: "rounded bg-slate-100 px-1",
								children: "Dell"
							}),
							". Parks on the CEO."
						] }),
						/* @__PURE__ */ (0, import_jsx_runtime.jsxs)("li", { children: [
							/* @__PURE__ */ (0, import_jsx_runtime.jsx)("span", {
								className: "font-medium text-slate-900",
								children: "Multi-org, zero code."
							}),
							" Globex, $30,000, vendor ",
							/* @__PURE__ */ (0, import_jsx_runtime.jsx)("code", {
								className: "rounded bg-slate-100 px-1",
								children: "Siemens"
							}),
							". Cost Control escalates; a Regional Director holds it."
						] }),
						/* @__PURE__ */ (0, import_jsx_runtime.jsxs)("li", { children: [
							/* @__PURE__ */ (0, import_jsx_runtime.jsx)("span", {
								className: "font-medium text-slate-900",
								children: "Dependency / reject."
							}),
							" Acme, vendor",
							/* @__PURE__ */ (0, import_jsx_runtime.jsx)("code", {
								className: "rounded bg-slate-100 px-1",
								children: "ShadyVendorLLC"
							}),
							". Legal blocks it; it never reaches approval."
						] })
					]
				})]
			})
		})]
	});
}
function placeholderFor(field) {
	switch (field) {
		case "amount": return "30000";
		case "vendor": return "Dell";
		case "category": return "Hardware";
		default: return "";
	}
}
//#endregion
export { Home as component };
