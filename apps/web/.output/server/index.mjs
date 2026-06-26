globalThis.__nitro_main__ = import.meta.url;
import { H3Core, HTTPError, NodeResponse, defineHandler, defineLazyEventHandler, serve, toEventHandler } from "./_libs/h3+rou3+srvx.mjs";
import { decodePath, joinURL, withLeadingSlash, withoutTrailingSlash } from "./_libs/ufo.mjs";
import { promises } from "node:fs";
import { fileURLToPath } from "node:url";
import { dirname, resolve } from "node:path";
//#region ../../node_modules/.pnpm/nitro-nightly@3.0.1-20260624-221007-37db9eee_chokidar@5.0.0_jiti@2.7.0_lru-cache@11.5.1_vite@_vyhg7ysnubnilu3frofpszy6ua/node_modules/nitro-nightly/dist/runtime/internal/route-rules.mjs
var headers = ((m) => function headersRouteRule(event) {
	for (const [key, value] of Object.entries(m.options || {})) event.res.headers.set(key, value);
});
//#endregion
//#region #nitro/virtual/public-assets-data
var public_assets_data_default = {
	"/favicon.ico": {
		"type": "image/vnd.microsoft.icon",
		"etag": "\"f1e-ESBTjHetHyiokkO0tT/irBbMO8Y\"",
		"mtime": "2026-06-25T23:47:57.767Z",
		"size": 3870,
		"path": "../public/favicon.ico"
	},
	"/logo192.png": {
		"type": "image/png",
		"etag": "\"14e3-f08taHgqf6/O2oRVTsq5tImHdQA\"",
		"mtime": "2026-06-25T23:47:57.767Z",
		"size": 5347,
		"path": "../public/logo192.png"
	},
	"/logo512.png": {
		"type": "image/png",
		"etag": "\"25c0-RpFfnQJpTtSb/HqVNJR2hBA9w/4\"",
		"mtime": "2026-06-25T23:47:57.767Z",
		"size": 9664,
		"path": "../public/logo512.png"
	},
	"/manifest.json": {
		"type": "application/json",
		"etag": "\"1f2-Oqn/x1R1hBTtEjA8nFhpBeFJJNg\"",
		"mtime": "2026-06-25T23:47:57.767Z",
		"size": 498,
		"path": "../public/manifest.json"
	},
	"/robots.txt": {
		"type": "text/plain; charset=utf-8",
		"etag": "\"43-BEzmj4PuhUNHX+oW9uOnPSihxtU\"",
		"mtime": "2026-06-25T23:47:57.767Z",
		"size": 67,
		"path": "../public/robots.txt"
	},
	"/assets/api-CA5LvOU0.js": {
		"type": "text/javascript; charset=utf-8",
		"etag": "\"362-DUU/sJNd4dIgv2COGQE31B8GUxo\"",
		"mtime": "2026-06-25T23:47:57.571Z",
		"size": 866,
		"path": "../public/assets/api-CA5LvOU0.js"
	},
	"/assets/cases._id-BM_M-lVt.js": {
		"type": "text/javascript; charset=utf-8",
		"etag": "\"21ef-MJgd2ihRkhOkwCF9CCvolp1QME8\"",
		"mtime": "2026-06-25T23:47:57.571Z",
		"size": 8687,
		"path": "../public/assets/cases._id-BM_M-lVt.js"
	},
	"/assets/cases._id.trace-Bhxj88hk.js": {
		"type": "text/javascript; charset=utf-8",
		"etag": "\"b57-fga5Ku0d5uQRuavxzQ26HURtOOc\"",
		"mtime": "2026-06-25T23:47:57.571Z",
		"size": 2903,
		"path": "../public/assets/cases._id.trace-Bhxj88hk.js"
	},
	"/assets/clock-4KiHH3wt.js": {
		"type": "text/javascript; charset=utf-8",
		"etag": "\"a9-YI3w8ujPJ69FV8VzcldB6KrfFYc\"",
		"mtime": "2026-06-25T23:47:57.572Z",
		"size": 169,
		"path": "../public/assets/clock-4KiHH3wt.js"
	},
	"/assets/createLucideIcon-BVed1LHN.js": {
		"type": "text/javascript; charset=utf-8",
		"etag": "\"27a5-k/8J7+bYX7IyCl2KDD45lgZcL1A\"",
		"mtime": "2026-06-25T23:47:57.572Z",
		"size": 10149,
		"path": "../public/assets/createLucideIcon-BVed1LHN.js"
	},
	"/assets/inbox-CM0TEDds.js": {
		"type": "text/javascript; charset=utf-8",
		"etag": "\"6e8-yoi1S8Z7FXvYoayDqAeXBdtXbhQ\"",
		"mtime": "2026-06-25T23:47:57.572Z",
		"size": 1768,
		"path": "../public/assets/inbox-CM0TEDds.js"
	},
	"/assets/routes-BxyzE_Ij.js": {
		"type": "text/javascript; charset=utf-8",
		"etag": "\"132f-F+LmR+Shf311JgZj4JEtJtiHeUk\"",
		"mtime": "2026-06-25T23:47:57.573Z",
		"size": 4911,
		"path": "../public/assets/routes-BxyzE_Ij.js"
	},
	"/assets/styles-D0XXoTzd.css": {
		"type": "text/css; charset=utf-8",
		"etag": "\"4fbc-GnXlBdHAoGzcHgeeTi3FobTIpWQ\"",
		"mtime": "2026-06-25T23:47:57.573Z",
		"size": 20412,
		"path": "../public/assets/styles-D0XXoTzd.css"
	},
	"/assets/ui-CRwRlSQr.js": {
		"type": "text/javascript; charset=utf-8",
		"etag": "\"704-WzrwQiDuALceNkx4x7LOafgJXvU\"",
		"mtime": "2026-06-25T23:47:57.573Z",
		"size": 1796,
		"path": "../public/assets/ui-CRwRlSQr.js"
	},
	"/assets/link-DDJIqse6.js": {
		"type": "text/javascript; charset=utf-8",
		"etag": "\"6622-66SZvqhbgFJPeN4cuH52hWh21QI\"",
		"mtime": "2026-06-25T23:47:57.572Z",
		"size": 26146,
		"path": "../public/assets/link-DDJIqse6.js"
	},
	"/assets/index-C-mRkfwb.js": {
		"type": "text/javascript; charset=utf-8",
		"etag": "\"45500-VGgATWnBAcBJmz0t5rS3EjoRZH4\"",
		"mtime": "2026-06-25T23:47:57.571Z",
		"size": 283904,
		"path": "../public/assets/index-C-mRkfwb.js"
	}
};
//#endregion
//#region #nitro/virtual/public-assets-node
function readAsset(id) {
	const serverDir = dirname(fileURLToPath(globalThis.__nitro_main__));
	return promises.readFile(resolve(serverDir, public_assets_data_default[id].path));
}
//#endregion
//#region #nitro/virtual/public-assets
var publicAssetBases = {};
function isPublicAssetURL(id = "") {
	if (public_assets_data_default[id]) return true;
	for (const base in publicAssetBases) if (id.startsWith(base)) return true;
	return false;
}
function getAsset(id) {
	return public_assets_data_default[id];
}
//#endregion
//#region ../../node_modules/.pnpm/nitro-nightly@3.0.1-20260624-221007-37db9eee_chokidar@5.0.0_jiti@2.7.0_lru-cache@11.5.1_vite@_vyhg7ysnubnilu3frofpszy6ua/node_modules/nitro-nightly/dist/runtime/internal/static.mjs
var METHODS = /* @__PURE__ */ new Set(["HEAD", "GET"]);
var EncodingMap = {
	gzip: ".gz",
	br: ".br",
	zstd: ".zst"
};
var static_default = defineHandler((event) => {
	if (event.req.method && !METHODS.has(event.req.method)) return;
	let id = decodePath(withLeadingSlash(withoutTrailingSlash(event.url.pathname)));
	let asset;
	const encodings = [...(event.req.headers.get("accept-encoding") || "").split(",").map((e) => EncodingMap[e.trim()]).filter(Boolean).sort(), ""];
	for (const encoding of encodings) for (const _id of [id + encoding, joinURL(id, "index.html" + encoding)]) {
		const _asset = getAsset(_id);
		if (_asset) {
			asset = _asset;
			id = _id;
			break;
		}
	}
	if (!asset) {
		if (isPublicAssetURL(id)) {
			event.res.headers.delete("Cache-Control");
			throw new HTTPError({ status: 404 });
		}
		return;
	}
	if (encodings.length > 1) event.res.headers.append("Vary", "Accept-Encoding");
	if (event.req.headers.get("if-none-match") === asset.etag) {
		event.res.status = 304;
		event.res.statusText = "Not Modified";
		return "";
	}
	const ifModifiedSinceH = event.req.headers.get("if-modified-since");
	const mtimeDate = new Date(asset.mtime);
	if (ifModifiedSinceH && asset.mtime && new Date(ifModifiedSinceH) >= mtimeDate) {
		event.res.status = 304;
		event.res.statusText = "Not Modified";
		return "";
	}
	if (asset.type) event.res.headers.set("Content-Type", asset.type);
	if (asset.etag && !event.res.headers.has("ETag")) event.res.headers.set("ETag", asset.etag);
	if (asset.mtime && !event.res.headers.has("Last-Modified")) event.res.headers.set("Last-Modified", mtimeDate.toUTCString());
	if (asset.encoding && !event.res.headers.has("Content-Encoding")) event.res.headers.set("Content-Encoding", asset.encoding);
	if (asset.size > 0 && !event.res.headers.has("Content-Length")) event.res.headers.set("Content-Length", asset.size.toString());
	return readAsset(id);
});
//#endregion
//#region #nitro/virtual/routing
var findRouteRules = /* @__PURE__ */ (() => {
	const $0 = [{
		name: "headers",
		route: "/assets/**",
		handler: headers,
		options: { "cache-control": "public, max-age=31536000, immutable" }
	}];
	return (m, p) => {
		let r = [];
		if (p.charCodeAt(p.length - 1) === 47) p = p.slice(0, -1) || "/";
		let s = p.split("/");
		if (s.length > 1) {
			if (s[1] === "assets") r.unshift({
				data: $0,
				params: { "_": s.slice(2).join("/") }
			});
		}
		return r;
	};
})();
var _lazy_bvov5V = defineLazyEventHandler(() => import("./_chunks/ssr-renderer.mjs"));
var findRoute = /* @__PURE__ */ (() => {
	const data = {
		route: "/**",
		handler: _lazy_bvov5V
	};
	return ((_m, p) => {
		return {
			data,
			params: { "_": p.slice(1) }
		};
	});
})();
var globalMiddleware = [toEventHandler(static_default)].filter(Boolean);
//#endregion
//#region ../../node_modules/.pnpm/nitro-nightly@3.0.1-20260624-221007-37db9eee_chokidar@5.0.0_jiti@2.7.0_lru-cache@11.5.1_vite@_vyhg7ysnubnilu3frofpszy6ua/node_modules/nitro-nightly/dist/runtime/internal/error/prod.mjs
var errorHandler = (error, event) => {
	const res = defaultHandler(error, event);
	return new NodeResponse(typeof res.body === "string" ? res.body : JSON.stringify(res.body, null, 2), res);
};
function defaultHandler(error, event) {
	const unhandled = error.unhandled ?? !HTTPError.isError(error);
	const { status = 500, statusText = "" } = unhandled ? {} : error;
	if (status === 404) {
		const url = event.url || new URL(event.req.url);
		const baseURL = "/";
		if (/^\/[^/]/.test(baseURL) && !url.pathname.startsWith(baseURL)) return {
			status: 302,
			headers: new Headers({ location: `${baseURL}${url.pathname.slice(1)}${url.search}` })
		};
	}
	const headers = new Headers(unhandled ? {} : error.headers);
	headers.set("content-type", "application/json; charset=utf-8");
	return {
		status,
		statusText,
		headers,
		body: {
			error: true,
			...unhandled ? {
				status,
				unhandled: true
			} : typeof error.toJSON === "function" ? error.toJSON() : {
				status,
				statusText,
				message: error.message
			}
		}
	};
}
//#endregion
//#region #nitro/virtual/error-handler
var errorHandlers = [errorHandler];
async function error_handler_default(error, event) {
	for (const handler of errorHandlers) try {
		const response = await handler(error, event, { defaultHandler });
		if (response) return response;
	} catch (error) {
		console.error(error);
	}
}
//#endregion
//#region #nitro/virtual/app
function createNitroApp() {
	const captureError = (error, errorCtx) => {
		if (errorCtx?.event) {
			const errors = errorCtx.event.req.context?.nitro?.errors;
			if (errors) errors.push({
				error,
				context: errorCtx
			});
		}
	};
	const h3App = createH3App({ onError(error, event) {
		return error_handler_default(error, event);
	} });
	let appHandler = (req) => {
		req.context ||= {};
		req.context.nitro = req.context.nitro || { errors: [] };
		return h3App.fetch(req);
	};
	return {
		fetch: appHandler,
		h3: h3App,
		hooks: void 0,
		captureError
	};
}
function createH3App(config) {
	const h3App = new H3Core(config);
	h3App["~findRoute"] = (event) => findRoute(event.req.method, event.url.pathname);
	h3App["~middleware"].push(...globalMiddleware);
	h3App["~getMiddleware"] = (event, route) => {
		const pathname = event.url.pathname;
		const method = event.req.method;
		const middleware = [];
		const routeRules = getRouteRules(method, pathname);
		event.context.routeRules = routeRules?.routeRules;
		if (routeRules?.routeRuleMiddleware.length) middleware.push(...routeRules.routeRuleMiddleware);
		middleware.push(...h3App["~middleware"]);
		if (route?.data?.middleware?.length) middleware.push(...route.data.middleware);
		return middleware;
	};
	return h3App;
}
//#endregion
//#region ../../node_modules/.pnpm/nitro-nightly@3.0.1-20260624-221007-37db9eee_chokidar@5.0.0_jiti@2.7.0_lru-cache@11.5.1_vite@_vyhg7ysnubnilu3frofpszy6ua/node_modules/nitro-nightly/dist/runtime/internal/app.mjs
var APP_ID = "default";
function useNitroApp() {
	let instance = useNitroApp._instance;
	if (instance) return instance;
	instance = useNitroApp._instance = createNitroApp();
	globalThis.__nitro__ = globalThis.__nitro__ || {};
	globalThis.__nitro__[APP_ID] = instance;
	return instance;
}
function getRouteRules(method, pathname) {
	const m = findRouteRules(method, pathname);
	if (!m?.length) return { routeRuleMiddleware: [] };
	const routeRules = {};
	for (const layer of m) for (const rule of layer.data) {
		const currentRule = routeRules[rule.name];
		if (currentRule) {
			if (rule.options === false) {
				delete routeRules[rule.name];
				continue;
			}
			if (typeof currentRule.options === "object" && typeof rule.options === "object") currentRule.options = {
				...currentRule.options,
				...rule.options
			};
			else currentRule.options = rule.options;
			currentRule.route = rule.route;
			currentRule.params = {
				...currentRule.params,
				...layer.params
			};
		} else if (rule.options !== false) routeRules[rule.name] = {
			...rule,
			params: layer.params
		};
	}
	const middleware = [];
	const orderedRules = Object.values(routeRules).sort((a, b) => (a.handler?.order || 0) - (b.handler?.order || 0));
	for (const rule of orderedRules) {
		if (rule.options === false || !rule.handler) continue;
		middleware.push(rule.handler(rule));
	}
	return {
		routeRules,
		routeRuleMiddleware: middleware
	};
}
//#endregion
//#region ../../node_modules/.pnpm/nitro-nightly@3.0.1-20260624-221007-37db9eee_chokidar@5.0.0_jiti@2.7.0_lru-cache@11.5.1_vite@_vyhg7ysnubnilu3frofpszy6ua/node_modules/nitro-nightly/dist/runtime/internal/error/hooks.mjs
function _captureError(error, type) {
	console.error(`[${type}]`, error);
	useNitroApp().captureError?.(error, { tags: [type] });
}
function trapUnhandledErrors() {
	process.on("unhandledRejection", (error) => _captureError(error, "unhandledRejection"));
	process.on("uncaughtException", (error) => _captureError(error, "uncaughtException"));
}
//#endregion
//#region #nitro/virtual/tracing
var tracingSrvxPlugins = [];
//#endregion
//#region ../../node_modules/.pnpm/nitro-nightly@3.0.1-20260624-221007-37db9eee_chokidar@5.0.0_jiti@2.7.0_lru-cache@11.5.1_vite@_vyhg7ysnubnilu3frofpszy6ua/node_modules/nitro-nightly/dist/presets/node/runtime/node-server.mjs
var _parsedPort = Number.parseInt(process.env.NITRO_PORT ?? process.env.PORT ?? "");
var port = Number.isNaN(_parsedPort) ? 3e3 : _parsedPort;
var host = process.env.NITRO_HOST || process.env.HOST;
var cert = process.env.NITRO_SSL_CERT;
var key = process.env.NITRO_SSL_KEY;
var nitroApp = useNitroApp();
serve({
	port,
	hostname: host,
	tls: cert && key ? {
		cert,
		key
	} : void 0,
	fetch: nitroApp.fetch,
	plugins: [...tracingSrvxPlugins]
});
trapUnhandledErrors();
var node_server_default = {};
//#endregion
export { node_server_default as default };
