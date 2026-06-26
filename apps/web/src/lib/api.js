import { authStore } from "./auth";
async function fetchJSON(url, init) {
    const token = authStore.get();
    const authHeader = token
        ? { Authorization: `Bearer ${token}` }
        : {};
    const res = await fetch(url, {
        ...init,
        headers: {
            "content-type": "application/json",
            ...authHeader,
            ...(init?.headers ?? {}),
        },
    });
    const text = await res.text();
    let parsed;
    try {
        parsed = text ? JSON.parse(text) : {};
    }
    catch {
        throw new Error(`non-JSON response (${res.status}): ${text.slice(0, 200)}`);
    }
    if (!res.ok) {
        const obj = parsed;
        if (obj.error && obj.details) {
            throw new Error(`${obj.error}: ${obj.details}`);
        }
        throw new Error(obj.error ?? obj.details ?? `HTTP ${res.status}`);
    }
    return parsed;
}
export const api = {
    getIS: (tenantId = "demo") => fetchJSON(`/api/is?tenant=${encodeURIComponent(tenantId)}`),
    extract: (text, tenantId = "demo") => fetchJSON(`/api/extract`, {
        method: "POST",
        body: JSON.stringify({ text, tenant_id: tenantId }),
    }),
    compileBPMN: (ir) => fetchJSON(`/api/compile/bpmn`, {
        method: "POST",
        body: JSON.stringify(ir),
    }),
    // Generic compile — target is the adapter kind ("camunda7" |
    // "elsa3" | ...). The response includes `mime` and `target` so
    // the UI can name the download file by the adapter's artifact_ext.
    compile: (target, ir) => fetchJSON(`/api/compile/${encodeURIComponent(target)}`, {
        method: "POST",
        body: JSON.stringify({ ir }),
    }),
    listAdapters: () => fetchJSON(`/api/engines/adapters`),
    analyzeDecisionTables: (ir) => fetchJSON(`/api/analyze/decision-tables`, {
        method: "POST",
        body: JSON.stringify({ ir }),
    }),
    // --- Copilot (Round 4) ---
    copilotAsk: (payload) => fetchJSON(`/api/copilot/ask`, {
        method: "POST",
        body: JSON.stringify({ tenant_id: "demo", ...payload }),
    }),
    copilotClarify: (payload) => fetchJSON(`/api/copilot/clarify`, {
        method: "POST",
        body: JSON.stringify({ tenant_id: "demo", ...payload }),
    }),
    copilotApply: (payload) => fetchJSON(`/api/copilot/apply`, {
        method: "POST",
        body: JSON.stringify({ tenant_id: "demo", ...payload }),
    }),
    registerCamunda: (payload) => fetchJSON(`/api/engines`, {
        method: "POST",
        body: JSON.stringify({ ...payload, kind: "camunda7" }),
    }),
    sync: (id) => fetchJSON(`/api/engines/${encodeURIComponent(id)}/sync`, { method: "POST" }),
    deleteEngine: (id) => fetch(`/api/engines/${encodeURIComponent(id)}`, { method: "DELETE" }).then((r) => {
        if (!r.ok && r.status !== 204)
            throw new Error(`DELETE /engines ${r.status}`);
    }),
    deploy: (ir, opts) => fetchJSON(`/api/deploy/camunda`, {
        method: "POST",
        body: JSON.stringify({
            ir,
            engine_id: opts?.engineId,
            start: opts?.start ?? false,
            start_variables: opts?.startVariables,
        }),
    }),
    // --- Projects / Chats (Round A–B) ---
    listProjects: (orgId) => fetchJSON(`/api/orgs/${encodeURIComponent(orgId)}/projects`),
    createProject: (orgId, payload) => fetchJSON(`/api/orgs/${encodeURIComponent(orgId)}/projects`, {
        method: "POST",
        body: JSON.stringify(payload),
    }),
    getProject: (id) => fetchJSON(`/api/projects/${encodeURIComponent(id)}`),
    updateProject: (id, payload) => fetchJSON(`/api/projects/${encodeURIComponent(id)}`, {
        method: "PATCH",
        body: JSON.stringify(payload),
    }),
    archiveProject: (id) => fetch(`/api/projects/${encodeURIComponent(id)}`, { method: "DELETE" }).then((r) => {
        if (!r.ok && r.status !== 204)
            throw new Error(`DELETE /projects ${r.status}`);
    }),
    listChats: (projectId) => fetchJSON(`/api/projects/${encodeURIComponent(projectId)}/chats`),
    createChat: (projectId, payload) => fetchJSON(`/api/projects/${encodeURIComponent(projectId)}/chats`, {
        method: "POST",
        body: JSON.stringify(payload),
    }),
    getChat: (id) => fetchJSON(`/api/chats/${encodeURIComponent(id)}`),
    renameChat: (id, title) => fetch(`/api/chats/${encodeURIComponent(id)}`, {
        method: "PATCH",
        headers: { "content-type": "application/json" },
        body: JSON.stringify({ title }),
    }).then((r) => {
        if (!r.ok && r.status !== 204)
            throw new Error(`PATCH /chats ${r.status}`);
    }),
    deleteChat: (id) => fetch(`/api/chats/${encodeURIComponent(id)}`, { method: "DELETE" }).then((r) => {
        if (!r.ok && r.status !== 204)
            throw new Error(`DELETE /chats ${r.status}`);
    }),
    listMessages: (chatId) => fetchJSON(`/api/chats/${encodeURIComponent(chatId)}/messages`),
    appendMessage: (chatId, payload) => fetchJSON(`/api/chats/${encodeURIComponent(chatId)}/messages`, {
        method: "POST",
        body: JSON.stringify(payload),
    }),
    // --- Attachments (Round D) ---
    listAttachments: (chatId) => fetchJSON(`/api/chats/${encodeURIComponent(chatId)}/attachments`),
    // Multipart upload. The server proxies to the agent for text
    // extraction (PDF -> text via pypdf, TXT passthrough) and persists
    // the normalized text_content; the chip needs only the preview.
    uploadAttachment: async (chatId, file) => {
        const form = new FormData();
        form.append("file", file);
        const res = await fetch(`/api/chats/${encodeURIComponent(chatId)}/attachments`, {
            method: "POST",
            body: form,
        });
        const text = await res.text();
        let parsed;
        try {
            parsed = text ? JSON.parse(text) : {};
        }
        catch {
            throw new Error(`non-JSON response (${res.status}): ${text.slice(0, 200)}`);
        }
        if (!res.ok) {
            const obj = parsed;
            throw new Error(obj.detail ?? obj.error ?? `HTTP ${res.status}`);
        }
        return parsed;
    },
    listDeployTargets: (projectId) => fetchJSON(`/api/projects/${encodeURIComponent(projectId)}/deploy-targets`),
    createDeployTarget: (projectId, payload) => fetchJSON(`/api/projects/${encodeURIComponent(projectId)}/deploy-targets`, {
        method: "POST",
        body: JSON.stringify(payload),
    }),
    deleteDeployTarget: (id) => fetch(`/api/deploy-targets/${encodeURIComponent(id)}`, { method: "DELETE" }).then((r) => {
        if (!r.ok && r.status !== 204)
            throw new Error(`DELETE /deploy-targets ${r.status}`);
    }),
    // Approve — locks the chat's current workflow version as the
    // operator-sanctioned final snapshot. Server rejects unless stage
    // is "ready" (no drafting, not already approved).
    approveChat: (chatId) => fetchJSON(`/api/chats/${encodeURIComponent(chatId)}/approve`, { method: "POST" }),
    // Compile + push the chat's latest IR to the named deploy target.
    deployChat: (chatId, targetId) => fetchJSON(`/api/chats/${encodeURIComponent(chatId)}/deploy`, {
        method: "POST",
        body: JSON.stringify({ target_id: targetId }),
    }),
    // --- Onboarding (Phase 3) ---
    getOnboardingQuestions: () => fetchJSON(`/api/onboarding`),
    onboardProject: (projectId, payload) => fetchJSON(`/api/projects/${encodeURIComponent(projectId)}/onboarding`, {
        method: "POST",
        body: JSON.stringify(payload),
    }),
    updateProjectOverview: (projectId, overview) => fetchJSON(`/api/projects/${encodeURIComponent(projectId)}/overview`, {
        method: "PATCH",
        body: JSON.stringify({ overview }),
    }),
    // --- Workflow Versioning (Phase 4) ---
    listWorkflowVersions: (chatId) => fetchJSON(`/api/chats/${encodeURIComponent(chatId)}/workflow/versions`),
    getWorkflowVersion: (id) => fetchJSON(`/api/workflow-versions/${encodeURIComponent(id)}`),
    forkWorkflow: (chatId, payload) => fetchJSON(`/api/chats/${encodeURIComponent(chatId)}/workflow/fork`, {
        method: "POST",
        body: JSON.stringify(payload),
    }),
    restoreWorkflowVersion: (versionId) => fetchJSON(`/api/workflow-versions/${encodeURIComponent(versionId)}/restore`, {
        method: "POST",
    }),
    diffWorkflowVersions: (versionId1, versionId2) => fetchJSON(`/api/workflow-versions/${encodeURIComponent(versionId1)}/diff`, {
        method: "POST",
        body: JSON.stringify({ other_version_id: versionId2 }),
    }),
    // --- Auth ---
    register: (payload) => fetchJSON(`/api/auth/register`, {
        method: "POST",
        body: JSON.stringify(payload),
    }),
    login: (payload) => fetchJSON(`/api/auth/login`, {
        method: "POST",
        body: JSON.stringify(payload),
    }),
    // --- Orgs ---
    // Backend returns a bare array — wrap it in { orgs } for consistent usage.
    listOrgs: () => fetchJSON(`/api/orgs`)
        .then((arr) => ({ orgs: Array.isArray(arr) ? arr : [] })),
    createOrg: (payload) => fetchJSON(`/api/orgs`, {
        method: "POST",
        body: JSON.stringify(payload),
    }),
    // --- Teams ---
    listTeams: (orgId) => fetchJSON(`/api/orgs/${encodeURIComponent(orgId)}/teams`),
    getTeam: (orgId, teamId) => fetchJSON(`/api/orgs/${encodeURIComponent(orgId)}/teams/${encodeURIComponent(teamId)}`),
    createTeam: (orgId, payload) => fetchJSON(`/api/orgs/${encodeURIComponent(orgId)}/teams`, {
        method: "POST",
        body: JSON.stringify(payload),
    }),
    updateTeam: (orgId, teamId, payload) => fetchJSON(`/api/orgs/${encodeURIComponent(orgId)}/teams/${encodeURIComponent(teamId)}`, {
        method: "PATCH",
        body: JSON.stringify(payload),
    }),
    deleteTeam: (orgId, teamId) => {
        const token = authStore.get();
        return fetch(`/api/orgs/${encodeURIComponent(orgId)}/teams/${encodeURIComponent(teamId)}`, {
            method: "DELETE",
            headers: token ? { Authorization: `Bearer ${token}` } : {},
        }).then((r) => {
            if (!r.ok && r.status !== 204)
                throw new Error(`DELETE /teams ${r.status}`);
        });
    },
    // --- Org members ---
    listOrgMembers: (orgId) => fetchJSON(`/api/orgs/${encodeURIComponent(orgId)}/members`),
    addOrgMember: (orgId, payload) => fetchJSON(`/api/orgs/${encodeURIComponent(orgId)}/members`, {
        method: "POST",
        body: JSON.stringify(payload),
    }),
    updateOrgMemberRole: (orgId, userId, role) => fetchJSON(`/api/orgs/${encodeURIComponent(orgId)}/members/${encodeURIComponent(String(userId))}`, {
        method: "PATCH",
        body: JSON.stringify({ role }),
    }),
    removeOrgMember: (orgId, userId) => {
        const token = authStore.get();
        return fetch(`/api/orgs/${encodeURIComponent(orgId)}/members/${encodeURIComponent(String(userId))}`, {
            method: "DELETE",
            headers: token ? { Authorization: `Bearer ${token}` } : {},
        }).then((r) => {
            if (!r.ok && r.status !== 204)
                throw new Error(`DELETE /members ${r.status}`);
        });
    },
    // --- Team members ---
    addTeamMember: (orgId, teamId, payload) => fetchJSON(`/api/orgs/${encodeURIComponent(orgId)}/teams/${encodeURIComponent(teamId)}/members`, {
        method: "POST",
        body: JSON.stringify(payload),
    }),
    removeTeamMember: (orgId, teamId, userId) => {
        const token = authStore.get();
        return fetch(`/api/orgs/${encodeURIComponent(orgId)}/teams/${encodeURIComponent(teamId)}/members/${encodeURIComponent(String(userId))}`, {
            method: "DELETE",
            headers: token ? { Authorization: `Bearer ${token}` } : {},
        }).then((r) => {
            if (!r.ok && r.status !== 204)
                throw new Error(`DELETE /team-members ${r.status}`);
        });
    },
    // --- User lookup ---
    lookupUser: (email) => fetchJSON(`/api/users/lookup?email=${encodeURIComponent(email)}`),
};
