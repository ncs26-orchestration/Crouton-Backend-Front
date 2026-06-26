import type {
  Attachment,
  Chat,
  ChatMessage,
  CompileResponse,
  DecisionTable,
  Diagnostic,
  DeployResponse,
  DeployTarget,
  EngineAdapter,
  ExtractResponse,
  ISRegistry,
  OrgRequest,
  Project,
  RequestDetail,
  RequestPriority,
  Workflow,
  WorkflowDiff,
  WorkflowVersion,
  WorkflowVersionListItem,
} from "./types";
import { authStore } from "./auth";

async function fetchJSON<T>(url: string, init?: RequestInit): Promise<T> {
  const token = authStore.get();
  const authHeader: Record<string, string> = token
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
  let parsed: unknown;
  try {
    parsed = text ? JSON.parse(text) : {};
  } catch {
    throw new Error(`non-JSON response (${res.status}): ${text.slice(0, 200)}`);
  }
  if (!res.ok) {
    const obj = parsed as { error?: string; details?: string };
    if (obj.error && obj.details) {
      throw new Error(`${obj.error}: ${obj.details}`);
    }
    throw new Error(obj.error ?? obj.details ?? `HTTP ${res.status}`);
  }
  return parsed as T;
}

export const api = {
  getIS: (tenantId = "demo"): Promise<ISRegistry> =>
    fetchJSON(`/api/is?tenant=${encodeURIComponent(tenantId)}`),

  extract: (text: string, tenantId = "demo"): Promise<ExtractResponse> =>
    fetchJSON(`/api/extract`, {
      method: "POST",
      body: JSON.stringify({ text, tenant_id: tenantId }),
    }),

  compileBPMN: (ir: Workflow): Promise<CompileResponse> =>
    fetchJSON(`/api/compile/bpmn`, {
      method: "POST",
      body: JSON.stringify(ir),
    }),

  // Generic compile — target is the adapter kind ("camunda7" |
  // "elsa3" | ...). The response includes `mime` and `target` so
  // the UI can name the download file by the adapter's artifact_ext.
  compile: (target: string, ir: Workflow): Promise<CompileResponse> =>
    fetchJSON(`/api/compile/${encodeURIComponent(target)}`, {
      method: "POST",
      body: JSON.stringify({ ir }),
    }),

  listAdapters: (): Promise<{ adapters: EngineAdapter[] }> =>
    fetchJSON(`/api/engines/adapters`),

  analyzeDecisionTables: (ir: Workflow): Promise<{ decision_tables: DecisionTable[] }> =>
    fetchJSON(`/api/analyze/decision-tables`, {
      method: "POST",
      body: JSON.stringify({ ir }),
    }),

  // --- Copilot (Round 4) ---

  copilotAsk: (payload: {
    ir: Workflow;
    question: string;
    tenant_id?: string;
  }): Promise<{
    answer: string;
    evidence: { ir_ref?: string; quote?: string }[];
    error?: string;
  }> =>
    fetchJSON(`/api/copilot/ask`, {
      method: "POST",
      body: JSON.stringify({ tenant_id: "demo", ...payload }),
    }),

  copilotClarify: (payload: {
    ir: Workflow;
    tenant_id?: string;
    kind: "task" | "actor" | "gateway" | "condition";
    element_id: string;
    current?: unknown;
    evidence?: string;
    confidence?: number;
  }): Promise<{
    suggestions: { label: string; rationale?: string; patch: unknown[] }[];
    error?: string;
  }> =>
    fetchJSON(`/api/copilot/clarify`, {
      method: "POST",
      body: JSON.stringify({ tenant_id: "demo", ...payload }),
    }),

  copilotApply: (payload: {
    ir: Workflow;
    patch: unknown[];
    tenant_id?: string;
  }): Promise<{
    ir: Workflow;
    diagnostics?: unknown[];
    normalized?: boolean;
    error?: string;
  }> =>
    fetchJSON(`/api/copilot/apply`, {
      method: "POST",
      body: JSON.stringify({ tenant_id: "demo", ...payload }),
    }),

  registerCamunda: (payload: {
    id: string;
    endpoint: string;
    username?: string;
    password?: string;
  }): Promise<unknown> =>
    fetchJSON(`/api/engines`, {
      method: "POST",
      body: JSON.stringify({ ...payload, kind: "camunda7" }),
    }),

  sync: (id: string): Promise<unknown> =>
    fetchJSON(`/api/engines/${encodeURIComponent(id)}/sync`, { method: "POST" }),

  deleteEngine: (id: string): Promise<void> =>
    fetch(`/api/engines/${encodeURIComponent(id)}`, { method: "DELETE" }).then((r) => {
      if (!r.ok && r.status !== 204) throw new Error(`DELETE /engines ${r.status}`);
    }),

  deploy: (ir: Workflow, opts?: { engineId?: string; start?: boolean; startVariables?: Record<string, unknown> }): Promise<DeployResponse> =>
    fetchJSON(`/api/deploy/camunda`, {
      method: "POST",
      body: JSON.stringify({
        ir,
        engine_id: opts?.engineId,
        start: opts?.start ?? false,
        start_variables: opts?.startVariables,
      }),
    }),

  // --- Projects / Chats (Round A–B) ---

  listProjects: (orgId: string): Promise<{ projects: Project[] }> =>
    fetchJSON(`/api/orgs/${encodeURIComponent(orgId)}/projects`),

  createProject: (orgId: string, payload: { name: string; description?: string }): Promise<Project> =>
    fetchJSON(`/api/orgs/${encodeURIComponent(orgId)}/projects`, {
      method: "POST",
      body: JSON.stringify(payload),
    }),

  getProject: (id: string): Promise<{ project: Project; chats: Chat[] }> =>
    fetchJSON(`/api/projects/${encodeURIComponent(id)}`),

  updateProject: (id: string, payload: { name?: string; description?: string }): Promise<Project> =>
    fetchJSON(`/api/projects/${encodeURIComponent(id)}`, {
      method: "PATCH",
      body: JSON.stringify(payload),
    }),

  archiveProject: (id: string): Promise<void> =>
    fetch(`/api/projects/${encodeURIComponent(id)}`, { method: "DELETE" }).then((r) => {
      if (!r.ok && r.status !== 204) throw new Error(`DELETE /projects ${r.status}`);
    }),

  listChats: (projectId: string): Promise<{ chats: Chat[] }> =>
    fetchJSON(`/api/projects/${encodeURIComponent(projectId)}/chats`),

  createChat: (projectId: string, payload: { title?: string; summary?: string }): Promise<Chat> =>
    fetchJSON(`/api/projects/${encodeURIComponent(projectId)}/chats`, {
      method: "POST",
      body: JSON.stringify(payload),
    }),

  getChat: (
    id: string,
  ): Promise<{
    chat: Chat;
    workflow?: {
      id: string;
      stage: "drafting" | "ready" | "approved";
      ir: Workflow;
      diagnostics?: unknown;
      created_at: string;
    };
  }> => fetchJSON(`/api/chats/${encodeURIComponent(id)}`),

  renameChat: (id: string, title: string): Promise<void> =>
    fetch(`/api/chats/${encodeURIComponent(id)}`, {
      method: "PATCH",
      headers: { "content-type": "application/json" },
      body: JSON.stringify({ title }),
    }).then((r) => {
      if (!r.ok && r.status !== 204) throw new Error(`PATCH /chats ${r.status}`);
    }),

  deleteChat: (id: string): Promise<void> =>
    fetch(`/api/chats/${encodeURIComponent(id)}`, { method: "DELETE" }).then((r) => {
      if (!r.ok && r.status !== 204) throw new Error(`DELETE /chats ${r.status}`);
    }),

  listMessages: (chatId: string): Promise<{ messages: ChatMessage[] }> =>
    fetchJSON(`/api/chats/${encodeURIComponent(chatId)}/messages`),

  appendMessage: (
    chatId: string,
    payload: { role?: "user" | "assistant" | "system"; body: Record<string, unknown> },
  ): Promise<{
    user: ChatMessage;
    assistant?: ChatMessage;
    workflow?: {
      id: string;
      stage: "drafting" | "ready" | "approved";
      ir: Workflow;
      diagnostics?: unknown;
      created_at: string;
    };
    error?: string;
  }> =>
    fetchJSON(`/api/chats/${encodeURIComponent(chatId)}/messages`, {
      method: "POST",
      body: JSON.stringify(payload),
    }),

  // --- Attachments (Round D) ---

  listAttachments: (chatId: string): Promise<{ attachments: Attachment[] }> =>
    fetchJSON(`/api/chats/${encodeURIComponent(chatId)}/attachments`),

  // Multipart upload. The server proxies to the agent for text
  // extraction (PDF -> text via pypdf, TXT passthrough) and persists
  // the normalized text_content; the chip needs only the preview.
  uploadAttachment: async (chatId: string, file: File): Promise<Attachment> => {
    const form = new FormData();
    form.append("file", file);
    const res = await fetch(`/api/chats/${encodeURIComponent(chatId)}/attachments`, {
      method: "POST",
      body: form,
    });
    const text = await res.text();
    let parsed: unknown;
    try {
      parsed = text ? JSON.parse(text) : {};
    } catch {
      throw new Error(`non-JSON response (${res.status}): ${text.slice(0, 200)}`);
    }
    if (!res.ok) {
      const obj = parsed as { error?: string; detail?: string };
      throw new Error(obj.detail ?? obj.error ?? `HTTP ${res.status}`);
    }
    return parsed as Attachment;
  },

  listDeployTargets: (projectId: string): Promise<{ deploy_targets: DeployTarget[] }> =>
    fetchJSON(`/api/projects/${encodeURIComponent(projectId)}/deploy-targets`),

  createDeployTarget: (
    projectId: string,
    payload: {
      kind: "camunda7" | "elsa3";
      name: string;
      endpoint: string;
      auth_kind?: string;
      auth_user?: string;
      auth_secret?: string;
    },
  ): Promise<DeployTarget> =>
    fetchJSON(`/api/projects/${encodeURIComponent(projectId)}/deploy-targets`, {
      method: "POST",
      body: JSON.stringify(payload),
    }),

  deleteDeployTarget: (id: string): Promise<void> =>
    fetch(`/api/deploy-targets/${encodeURIComponent(id)}`, { method: "DELETE" }).then((r) => {
      if (!r.ok && r.status !== 204) throw new Error(`DELETE /deploy-targets ${r.status}`);
    }),

  // Approve — locks the chat's current workflow version as the
  // operator-sanctioned final snapshot. Server rejects unless stage
  // is "ready" (no drafting, not already approved).
  approveChat: (chatId: string): Promise<{ workflow_version_id: string; stage: "approved" }> =>
    fetchJSON(`/api/chats/${encodeURIComponent(chatId)}/approve`, { method: "POST" }),

  // Compile + push the chat's latest IR to the named deploy target.
  deployChat: (
    chatId: string,
    targetId: string,
  ): Promise<{
    target_id: string;
    kind: string;
    deployment_id: string;
    process_definition_id?: string;
    process_key?: string;
    artifact_bytes: number;
    diagnostics?: Diagnostic[];
    studio_url?: string;
  }> =>
    fetchJSON(`/api/chats/${encodeURIComponent(chatId)}/deploy`, {
      method: "POST",
      body: JSON.stringify({ target_id: targetId }),
    }),

  // --- Onboarding (Phase 3) ---

  getOnboardingQuestions: (): Promise<{
    overview?: Record<string, unknown>;
    questions?: { index: number; id: string; text: string; type: string; options?: string[]; required: boolean; placeholder?: string }[];
    complete: boolean;
    current_step: number;
    error?: string;
  }> => fetchJSON(`/api/onboarding`),

  onboardProject: (
    projectId: string,
    payload: { question_index: number; answer?: string; multi_select?: string[] },
  ): Promise<{
    overview?: Record<string, unknown>;
    questions?: { index: number; id: string; text: string; type: string; options?: string[]; required: boolean; placeholder?: string }[];
    complete: boolean;
    current_step: number;
    error?: string;
  }> =>
    fetchJSON(`/api/projects/${encodeURIComponent(projectId)}/onboarding`, {
      method: "POST",
      body: JSON.stringify(payload),
    }),

  updateProjectOverview: (
    projectId: string,
    overview: Record<string, unknown>,
  ): Promise<{ status: string }> =>
    fetchJSON(`/api/projects/${encodeURIComponent(projectId)}/overview`, {
      method: "PATCH",
      body: JSON.stringify({ overview }),
    }),

  // --- Workflow Versioning (Phase 4) ---

  listWorkflowVersions: (chatId: string): Promise<{ versions: WorkflowVersionListItem[] }> =>
    fetchJSON(`/api/chats/${encodeURIComponent(chatId)}/workflow/versions`),

  getWorkflowVersion: (id: string): Promise<WorkflowVersion> =>
    fetchJSON(`/api/workflow-versions/${encodeURIComponent(id)}`),

  forkWorkflow: (
    chatId: string,
    payload: { version_id?: string; target_chat_id?: string; title?: string },
  ): Promise<{ chat: Chat; version: WorkflowVersion }> =>
    fetchJSON(`/api/chats/${encodeURIComponent(chatId)}/workflow/fork`, {
      method: "POST",
      body: JSON.stringify(payload),
    }),

  restoreWorkflowVersion: (versionId: string): Promise<WorkflowVersion> =>
    fetchJSON(`/api/workflow-versions/${encodeURIComponent(versionId)}/restore`, {
      method: "POST",
    }),

  diffWorkflowVersions: (
    versionId1: string,
    versionId2: string,
  ): Promise<WorkflowDiff> =>
    fetchJSON(`/api/workflow-versions/${encodeURIComponent(versionId1)}/diff`, {
      method: "POST",
      body: JSON.stringify({ other_version_id: versionId2 }),
    }),

  // --- Auth ---

  register: (payload: { name: string; email: string; password: string }): Promise<{ token: string; user: { id: number; email: string; name: string } }> =>
    fetchJSON(`/api/auth/register`, {
      method: "POST",
      body: JSON.stringify(payload),
    }),

  login: (payload: { email: string; password: string }): Promise<{ token: string; user: { id: number; email: string; name: string } }> =>
    fetchJSON(`/api/auth/login`, {
      method: "POST",
      body: JSON.stringify(payload),
    }),

  // --- Orgs ---

  // Backend returns a bare array — wrap it in { orgs } for consistent usage.
  listOrgs: (): Promise<{ orgs: Array<{ id: string; name: string; slug: string; role: string; created_at: string }> }> =>
    fetchJSON<Array<{ id: string; name: string; slug: string; role: string; created_at: string }>>(`/api/orgs`)
      .then((arr) => ({ orgs: Array.isArray(arr) ? arr : [] })),

  createOrg: (payload: { name: string; slug: string }): Promise<{ id: string; name: string; slug: string; created_at: string }> =>
    fetchJSON(`/api/orgs`, {
      method: "POST",
      body: JSON.stringify(payload),
    }),

  // --- Teams ---

  listTeams: (orgId: string): Promise<{ teams: Array<{ id: string; name: string; description: string; created_at: string; member_count?: number }> }> =>
    fetchJSON(`/api/orgs/${encodeURIComponent(orgId)}/teams`),

  getTeam: (orgId: string, teamId: string): Promise<{
    id: string; name: string; description: string; created_at: string;
    members: Array<{ id: number; name: string; email: string; role: string; joined_at: string }>;
  }> =>
    fetchJSON(`/api/orgs/${encodeURIComponent(orgId)}/teams/${encodeURIComponent(teamId)}`),

  createTeam: (orgId: string, payload: { name: string; description?: string }): Promise<{ id: string; name: string; description: string; created_at: string }> =>
    fetchJSON(`/api/orgs/${encodeURIComponent(orgId)}/teams`, {
      method: "POST",
      body: JSON.stringify(payload),
    }),

  updateTeam: (orgId: string, teamId: string, payload: { name?: string; description?: string }): Promise<{ id: string; name: string; description: string }> =>
    fetchJSON(`/api/orgs/${encodeURIComponent(orgId)}/teams/${encodeURIComponent(teamId)}`, {
      method: "PATCH",
      body: JSON.stringify(payload),
    }),

  deleteTeam: (orgId: string, teamId: string): Promise<void> => {
    const token = authStore.get();
    return fetch(`/api/orgs/${encodeURIComponent(orgId)}/teams/${encodeURIComponent(teamId)}`, {
      method: "DELETE",
      headers: token ? { Authorization: `Bearer ${token}` } : {},
    }).then((r) => {
      if (!r.ok && r.status !== 204) throw new Error(`DELETE /teams ${r.status}`);
    });
  },

  // --- Org members ---

  listOrgMembers: (orgId: string): Promise<{ members: Array<{ user_id: number; name: string; email: string; role: string; joined_at: string }> }> =>
    fetchJSON(`/api/orgs/${encodeURIComponent(orgId)}/members`),

  addOrgMember: (orgId: string, payload: { user_id: number; role: string }): Promise<void> =>
    fetchJSON(`/api/orgs/${encodeURIComponent(orgId)}/members`, {
      method: "POST",
      body: JSON.stringify(payload),
    }),

  updateOrgMemberRole: (orgId: string, userId: number, role: string): Promise<void> =>
    fetchJSON(`/api/orgs/${encodeURIComponent(orgId)}/members/${encodeURIComponent(String(userId))}`, {
      method: "PATCH",
      body: JSON.stringify({ role }),
    }),

  removeOrgMember: (orgId: string, userId: number): Promise<void> => {
    const token = authStore.get();
    return fetch(`/api/orgs/${encodeURIComponent(orgId)}/members/${encodeURIComponent(String(userId))}`, {
      method: "DELETE",
      headers: token ? { Authorization: `Bearer ${token}` } : {},
    }).then((r) => {
      if (!r.ok && r.status !== 204) throw new Error(`DELETE /members ${r.status}`);
    });
  },

  // --- Team members ---

  addTeamMember: (orgId: string, teamId: string, payload: { user_id: number; role: string }): Promise<void> =>
    fetchJSON(`/api/orgs/${encodeURIComponent(orgId)}/teams/${encodeURIComponent(teamId)}/members`, {
      method: "POST",
      body: JSON.stringify(payload),
    }),

  removeTeamMember: (orgId: string, teamId: string, userId: number): Promise<void> => {
    const token = authStore.get();
    return fetch(`/api/orgs/${encodeURIComponent(orgId)}/teams/${encodeURIComponent(teamId)}/members/${encodeURIComponent(String(userId))}`, {
      method: "DELETE",
      headers: token ? { Authorization: `Bearer ${token}` } : {},
    }).then((r) => {
      if (!r.ok && r.status !== 204) throw new Error(`DELETE /team-members ${r.status}`);
    });
  },

  // --- User lookup ---

  lookupUser: (email: string): Promise<{ id: number; name: string; email: string }> =>
    fetchJSON(`/api/users/lookup?email=${encodeURIComponent(email)}`),

  // --- Requests (F1) ---

  listRequests: (orgId: string): Promise<{ requests: OrgRequest[] }> =>
    fetchJSON(`/api/orgs/${encodeURIComponent(orgId)}/requests`),

  createRequest: (
    orgId: string,
    payload: { title: string; description?: string; priority: RequestPriority },
  ): Promise<{ request: OrgRequest }> =>
    fetchJSON(`/api/orgs/${encodeURIComponent(orgId)}/requests`, {
      method: "POST",
      body: JSON.stringify(payload),
    }),

  getRequest: (id: string): Promise<RequestDetail> =>
    fetchJSON(`/api/requests/${encodeURIComponent(id)}`),
};
