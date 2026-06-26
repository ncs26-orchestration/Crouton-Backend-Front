import { HeadContent, Link, Scripts, createFileRoute, createRootRoute, createRouter, lazyRouteComponent, require_jsx_runtime } from "../_libs/@tanstack/react-router+[...].mjs";
import { Route as Route$3 } from "./cases._id-CqRRudD0.mjs";
import { Inbox, Network } from "../_libs/lucide-react.mjs";
import { Route as Route$4 } from "./cases._id.trace-DWjo3JOX.mjs";
//#region node_modules/.nitro/vite/services/ssr/assets/router-CsM1f2bp.js
var import_jsx_runtime = require_jsx_runtime();
var styles_default = "/assets/styles-D0XXoTzd.css";
var Route$2 = createRootRoute({
	head: () => ({
		meta: [
			{ charSet: "utf-8" },
			{
				name: "viewport",
				content: "width=device-width, initial-scale=1"
			},
			{ title: "Org OS — Enterprise Workflow Runtime" }
		],
		links: [{
			rel: "stylesheet",
			href: styles_default
		}]
	}),
	shellComponent: RootDocument
});
function RootDocument({ children }) {
	return /* @__PURE__ */ (0, import_jsx_runtime.jsxs)("html", {
		lang: "en",
		children: [/* @__PURE__ */ (0, import_jsx_runtime.jsx)("head", { children: /* @__PURE__ */ (0, import_jsx_runtime.jsx)(HeadContent, {}) }), /* @__PURE__ */ (0, import_jsx_runtime.jsxs)("body", {
			className: "bg-slate-50 text-slate-900 antialiased",
			children: [
				/* @__PURE__ */ (0, import_jsx_runtime.jsx)("header", {
					className: "border-b border-slate-200 bg-white",
					children: /* @__PURE__ */ (0, import_jsx_runtime.jsxs)("div", {
						className: "mx-auto flex max-w-5xl items-center justify-between px-6 py-3",
						children: [/* @__PURE__ */ (0, import_jsx_runtime.jsxs)(Link, {
							to: "/",
							className: "flex items-center gap-2 font-semibold",
							children: [
								/* @__PURE__ */ (0, import_jsx_runtime.jsx)(Network, { className: "h-5 w-5 text-indigo-600" }),
								/* @__PURE__ */ (0, import_jsx_runtime.jsx)("span", { children: "Org OS" }),
								/* @__PURE__ */ (0, import_jsx_runtime.jsx)("span", {
									className: "hidden text-sm font-normal text-slate-400 sm:inline",
									children: "Enterprise Workflow Runtime"
								})
							]
						}), /* @__PURE__ */ (0, import_jsx_runtime.jsxs)("nav", {
							className: "flex items-center gap-1 text-sm",
							children: [/* @__PURE__ */ (0, import_jsx_runtime.jsx)(Link, {
								to: "/",
								className: "rounded-md px-3 py-1.5 text-slate-600 hover:bg-slate-100",
								activeOptions: { exact: true },
								activeProps: { className: "rounded-md px-3 py-1.5 bg-slate-100 font-medium text-slate-900" },
								children: "New request"
							}), /* @__PURE__ */ (0, import_jsx_runtime.jsxs)(Link, {
								to: "/inbox",
								className: "flex items-center gap-1.5 rounded-md px-3 py-1.5 text-slate-600 hover:bg-slate-100",
								activeProps: { className: "flex items-center gap-1.5 rounded-md px-3 py-1.5 bg-slate-100 font-medium text-slate-900" },
								children: [/* @__PURE__ */ (0, import_jsx_runtime.jsx)(Inbox, { className: "h-4 w-4" }), "Approver inbox"]
							})]
						})]
					})
				}),
				/* @__PURE__ */ (0, import_jsx_runtime.jsx)("main", {
					className: "mx-auto max-w-5xl px-6 py-8",
					children
				}),
				/* @__PURE__ */ (0, import_jsx_runtime.jsx)(Scripts, {})
			]
		})]
	});
}
var $$splitComponentImporter$1 = () => import("./inbox-CEpu0mVZ.mjs");
var Route$1 = createFileRoute("/inbox")({ component: lazyRouteComponent($$splitComponentImporter$1, "component") });
var $$splitComponentImporter = () => import("./routes-R83Ri0WN.mjs");
var Route = createFileRoute("/")({ component: lazyRouteComponent($$splitComponentImporter, "component") });
var InboxRoute = Route$1.update({
	id: "/inbox",
	path: "/inbox",
	getParentRoute: () => Route$2
});
var IndexRoute = Route.update({
	id: "/",
	path: "/",
	getParentRoute: () => Route$2
});
var CasesIdRoute = Route$3.update({
	id: "/cases/$id",
	path: "/cases/$id",
	getParentRoute: () => Route$2
});
var CasesIdRouteChildren = { CasesIdTraceRoute: Route$4.update({
	id: "/trace",
	path: "/trace",
	getParentRoute: () => CasesIdRoute
}) };
var rootRouteChildren = {
	IndexRoute,
	InboxRoute,
	CasesIdRoute: CasesIdRoute._addFileChildren(CasesIdRouteChildren)
};
var routeTree = Route$2._addFileChildren(rootRouteChildren)._addFileTypes();
function getRouter() {
	return createRouter({
		routeTree,
		scrollRestoration: true,
		defaultPreload: "intent",
		defaultPreloadStaleTime: 0
	});
}
//#endregion
export { getRouter };
