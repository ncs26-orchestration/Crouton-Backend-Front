// Subset of the Workflow IR + IS Registry types mirrored from
// packages/ir/*. Kept hand-written (not generated) for clarity and to
// keep the web bundle small.

export type TaskType = "user" | "service" | "script";
export type GatewayType = "exclusive" | "parallel";
export type EventType = "start" | "end";
export type ActorKind = "role" | "person" | "group";

// Confidence (0..1) and evidence (short source quote) are optional on
// every element the extractor emits. They drive the UI's
// "needs-confirmation" signal and the Copilot's Clarify queue.
export interface Provenance {
  confidence?: number;
  evidence?: string;
}

export interface Binding extends Provenance {
  assignee_user_id?: string;
  candidate_group_id?: string;
  form_key?: string;
  system_ref?: string;
  capability?: string;
  params?: Record<string, unknown>;
}

export interface Actor extends Provenance {
  id: string;
  name: string;
  kind: ActorKind;
  is_ref?: { user_id?: string; group_id?: string };
}

export interface Task extends Provenance {
  id: string;
  name: string;
  type: TaskType;
  actor_ref?: string;
  form_ref?: string;
  binding?: Binding;
}

export interface Gateway extends Provenance {
  id: string;
  type: GatewayType;
}

export interface EventNode extends Provenance {
  id: string;
  type: EventType;
}

export interface Condition extends Provenance {
  expression: string;
  language?: "juel" | "feel";
}

export interface Flow extends Provenance {
  id: string;
  from: string;
  to: string;
  condition?: Condition;
}

export interface FormField {
  id: string;
  label: string;
  type: "string" | "long" | "boolean" | "date" | "enum";
  required?: boolean;
  options?: string[];
}

export interface Form {
  id: string;
  fields: FormField[];
}

export interface Workflow {
  version: "0.1";
  metadata: { name: string; description?: string; tenant_id?: string; source_ids?: string[] };
  actors: Actor[];
  tasks: Task[];
  gateways?: Gateway[];
  events: EventNode[];
  flows: Flow[];
  forms?: Form[];
}

// IS Registry projection (matches apps/api GET /is output).

export interface ISUser {
  id: string;
  name: string;
  email?: string;
  group_ids?: string[];
  engine_ref?: string;
}

export interface ISGroup {
  id: string;
  name: string;
  engine_ref?: string;
}

export interface ISSystem {
  id: string;
  name?: string;
  kind: "ecm" | "erp" | "comms" | "idp" | "crm" | "signer" | "other";
  endpoint?: string;
  capabilities: string[];
}

export interface EngineConnection {
  id: string;
  kind: "camunda7" | "camunda8" | "elsa3";
  endpoint: string;
  last_synced_at?: string;
}

export interface ISRegistry {
  tenant_id: string;
  engine_connections?: EngineConnection[];
  users: ISUser[];
  groups: ISGroup[];
  deployed_forms?: { form_key: string; engine_ref?: string }[];
  systems: ISSystem[];
}

export interface Diagnostic {
  severity: "error" | "warning";
  ir_ref?: string;
  message: string;
  suggestion?: string;
}

export interface ExtractResponse {
  ir: Workflow | null;
  diagnostics: Diagnostic[];
  error?: string;
}

export interface DecisionRule {
  flow_id: string;
  target: string;
  predicates: Record<string, string>;
  is_default?: boolean;
}

export interface DecisionTable {
  gateway_id: string;
  variables: string[];
  rules: DecisionRule[];
  evidence?: string[];
}

export interface CompileResponse {
  artifact: string;
  mime?: string;
  target?: string;
  diagnostics: Diagnostic[];
  decision_tables?: DecisionTable[];
  error?: string;
}

// Adapter descriptor returned by GET /engines/adapters — drives the
// target selector and Compile/Deploy capability flags.
export interface EngineAdapter {
  kind: string;
  name: string;
  capabilities: {
    can_discover: boolean;
    can_deploy: boolean;
    artifact_mime: string;
    artifact_ext: string;
  };
}

// --- Projects + Chats (Round A onward) ---

export interface Project {
  id: string;
  name: string;
  description: string;
  overview_json?: Record<string, unknown> | null;
  created_at: string;
}

export interface Chat {
  id: string;
  project_id: string;
  title: string;
  summary: string;
  latest_workflow_version_id?: string | null;
  created_at: string;
  updated_at: string;
}

// Clarifying question raised by the extractor for a low-confidence
// element. Rendered inline in the assistant's bubble as a numbered
// list. `ir_ref` is a JSON-Pointer so the UI can optionally
// highlight the targeted node on the canvas.
export interface ClarifyingQuestion {
  id: string;
  ir_ref?: string;
  text: string;
}

// Message body is a flexible envelope; common shapes:
//   user:       { text: string; attachment_ids?: string[] }
//   assistant:  { text: string; workflow_version_id?: string; questions?: ClarifyingQuestion[] }
//   system:     { kind: "extract_started" | ... ; ... }
export interface ChatMessage {
  id: string;
  chat_id: string;
  role: "user" | "assistant" | "system";
  body: Record<string, unknown>;
  created_at: string;
}

// Attachment surface — what the UI sees after uploading a file. The
// server keeps the full text_content; the chip shows just `text_preview`
// (first ~400 chars, enough to confirm "yes this is the right file").
export interface Attachment {
  id: string;
  chat_id: string;
  kind: "document" | "voice" | "image";
  filename: string;
  mime: string;
  size_bytes: number;
  text_preview: string;
  text_full: boolean;
  created_at: string;
}

export interface DeployTarget {
  id: string;
  project_id: string;
  kind: "camunda7" | "elsa3";
  name: string;
  endpoint: string;
  auth_kind: string;
  auth_user: string;
  created_at: string;
}

export interface DeployResponse {
  engine_id: string;
  deployment_id: string;
  process_definition_id?: string;
  process_key?: string;
  cockpit_url?: string;
  tasklist_url?: string;
  instance_id?: string;
  artifact_bytes: number;
  diagnostics?: Diagnostic[];
}

// --- AI Organization OS: Requests & Workflow ---

export type RequestPriority = "low" | "medium" | "high" | "urgent";

export type RequestStatus =
  | "submitted"
  | "in_progress"
  | "awaiting_approval"
  | "approved"
  | "rejected"
  | "completed";

export type NodeStatus = "pending" | "in_progress" | "completed" | "blocked";

// A human's call on the executive-approval gate (F7).
export type ApprovalDecision = "approve" | "reject";

// A business request submitted into the org. The workflow graph
// (nodes/edges) is attached by the intake planner (F2).
export interface OrgRequest {
  id: string;
  org_id: string;
  title: string;
  description: string;
  requester_user_id: number;
  requester_name: string;
  priority: RequestPriority;
  status: RequestStatus;
  progress: number;
  estimated_completion: string | null;
  created_at: string;
}

export interface WorkflowNodeData {
  id: string;
  key: string;
  name: string;
  agent_type: string;
  department: string;
  status: NodeStatus;
  description: string;
  progress_percent: number;
  status_text: string;
  started_at: string | null;
  completed_at: string | null;
  blocked_by?: { reason: string; blocked_at?: string } | null;
}

export interface WorkflowEdgeData {
  id: string;
  source_node_id: string;
  target_node_id: string;
  edge_type: string;
}

export interface RequestGraph {
  request: OrgRequest;
  nodes: WorkflowNodeData[];
  edges: WorkflowEdgeData[];
}

// A unit of work a department agent reported for a node.
export interface NodeTask {
  id: string;
  title: string;
  status: string;
  started_at: string | null;
  completed_at: string | null;
}

// A single audit event recording a state change.
export interface AuditEvent {
	id: string;
	request_id: string;
	node_id: string | null;
	actor: string;
	action: string;
	reason: string;
	created_at: string;
}

// Node detail payload from GET /requests/:id/nodes/:nodeId. activity is
// the node-scoped audit trail.
export interface NodeDetailResponse {
	node: WorkflowNodeData;
	tasks: NodeTask[];
	activity: AuditEvent[];
}

// --- Roster + Policies (F9 — supporting tabs) ---

// Derived live status of a department agent: idle, busy while it owns an
// in_progress node, or blocked while one of its nodes waits on another team.
export type AgentStatus = "idle" | "busy" | "blocked";

// One department agent in the org roster (seeded by F10), with activity
// aggregated across all of the org's requests. The seeded fields (name, avatar,
// team, capabilities) come from the directory; the counts and status are
// derived live from the workflow nodes the agent owns. capabilities is a
// comma-separated free-text list as stored by the seed.
export interface AgentRosterEntry {
  id: string;
  org_id: string;
  agent_type: string;
  name: string;
  avatar: string;
  team_id: string;
  team_name: string;
  capabilities: string;
  created_at: string;
  status: AgentStatus;
  total: number;
  completed: number;
  active: number;
  blocked: number;
  request_count: number;
  latest_status: string;
}

// A read-only department policy agents consult while reasoning (seeded by F10).
export interface DepartmentPolicy {
  id: string;
  org_id: string;
  team_id: string;
  team_name: string;
  title: string;
  body: string;
  created_at: string;
}

// Binding state drives node color. Resolved at canvas build time
// against the IS Registry; a task with no binding is "idle", a valid
// binding is "ok", a binding whose id does not resolve is "error",
// and an explicit ambiguity flag (set by later extractor iterations)
// is "warn".
export type BindingState = "ok" | "warn" | "idle" | "error";

// --- Final Report (F8 — execution stage & final report) ---

// A task item within a report stage.
export interface ReportTask {
  title: string;
  status: string;
}

// Per-stage data in the final report.
export interface ReportStage {
  key: string;
  name: string;
  department: string;
  status: string;
  status_text: string;
  started_at: string | null;
  completed_at: string | null;
  duration_seconds: number;
  tasks: ReportTask[];
}

// Human approval info in the report.
export interface ReportApproval {
  decision: "approve" | "reject";
  justification: string;
  approved_by: string;
  approved_at: string;
}

// Notable event during execution (blocked, fallback).
export interface ReportFlag {
  stage_key: string;
  stage_name: string;
  severity: "warning" | "info";
  message: string;
}

// Aggregated report metadata.
export interface ReportSummary {
  total_stages: number;
  completed_stages: number;
  total_time_human: string;
}

// Request overview in the report.
export interface ReportRequestOverview {
  id: string;
  title: string;
  description: string;
  priority: string;
  requester_name: string;
  status: string;
  created_at: string;
  completed_at: string | null;
}

// Top-level response from GET /requests/:id/report.
export interface FinalReport {
  request: ReportRequestOverview;
  summary: ReportSummary;
  // The API omits these (null) when empty rather than sending [].
  stages: ReportStage[] | null;
  approval: ReportApproval | null;
  flags: ReportFlag[] | null;
}

// --- SSE Event Types (F4 — live canvas) ---

export type SSEEventType = "node_status" | "request_status" | "task" | "audit";

// The JSON body of every SSE event matches the bus.Event shape. Fields
// are optional because each event type carries only its own subset.
export interface SSEEvent {
  type: SSEEventType;
  request_id: string;
  node_id?: string;
  key?: string;
  status?: string;
  progress_percent?: number;
  status_text?: string;
  task_id?: string;
  title?: string;
  actor?: string;
  action?: string;
  reason?: string;
  at: string;
}

// --- Workflow Versioning (Phase 4) ---

export interface WorkflowVersion {
  id: string;
  chat_id: string;
  stage: "drafting" | "ready" | "approved";
  ir: Workflow;
  diagnostics?: Diagnostic[];
  source_message_id?: string;
  created_at: string;
}

export interface WorkflowVersionListItem {
  id: string;
  stage: "drafting" | "ready" | "approved";
  created_at: string;
}

export interface WorkflowDiff {
  added: string[];
  removed: string[];
  changed: string[];
}
