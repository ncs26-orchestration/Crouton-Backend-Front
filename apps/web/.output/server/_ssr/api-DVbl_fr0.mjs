//#region node_modules/.nitro/vite/services/ssr/assets/api-DVbl_fr0.js
async function get(url) {
	const res = await fetch(url);
	if (!res.ok) throw new Error(`${url} -> ${res.status}`);
	return res.json();
}
async function post(url, body) {
	const res = await fetch(url, {
		method: "POST",
		headers: { "Content-Type": "application/json" },
		body: JSON.stringify(body)
	});
	if (!res.ok) {
		const msg = await res.json().catch(() => ({ message: res.statusText }));
		throw new Error(msg.message || `${url} -> ${res.status}`);
	}
	return res.json();
}
var api = {
	tenants: () => get("/api/tenants"),
	workflows: (tenant) => get(`/api/workflows?tenant=${tenant}`),
	createRequest: (body) => post("/api/requests", body),
	getCase: (id) => get(`/api/cases/${id}`),
	getEvents: (id) => get(`/api/cases/${id}/events`),
	tasks: (tenant, assignee) => {
		const q = new URLSearchParams();
		if (tenant) q.set("tenant", tenant);
		if (assignee) q.set("assignee", assignee);
		return get(`/api/tasks?${q.toString()}`);
	},
	act: (caseId, body) => post(`/api/tasks/${caseId}/act`, body),
	provideInfo: (caseId, body) => post(`/api/cases/${caseId}/provide-info`, body)
};
var APPROVERS = {
	"ceo@acme": "CEO · Acme Corp",
	"director@globex": "Regional Director · Globex"
};
//#endregion
export { APPROVERS, api };
