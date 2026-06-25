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

// Binding state drives node color. Resolved at canvas build time
// against the IS Registry; a task with no binding is "idle", a valid
// binding is "ok", a binding whose id does not resolve is "error",
// and an explicit ambiguity flag (set by later extractor iterations)
// is "warn".
export type BindingState = "ok" | "warn" | "idle" | "error";

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
