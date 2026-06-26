// Package ir defines the in-memory shape of the AIOS Workflow IR (v0.1)
// and the IS Registry it references. Types mirror
// packages/ir/schema.json and packages/ir/is-registry.schema.json.
package ir

// Workflow is the canonical, engine-agnostic description of a business
// process. It is the sole source of truth for workflow structure in
// AIOS; every compiler target (BPMN, Elsa, n8n, DMN) emits a derived
// artifact from an instance of this type.
type Workflow struct {
	Version  string    `json:"version"`
	Metadata Metadata  `json:"metadata"`
	Actors   []Actor   `json:"actors"`
	Tasks    []Task    `json:"tasks"`
	Gateways []Gateway `json:"gateways,omitempty"`
	Events   []Event   `json:"events"`
	Flows    []Flow    `json:"flows"`
	Forms    []Form    `json:"forms,omitempty"`
}

type Metadata struct {
	Name        string   `json:"name"`
	Description string   `json:"description,omitempty"`
	TenantID    string   `json:"tenant_id,omitempty"`
	SourceIDs   []string `json:"source_ids,omitempty"`
}

// Actor is a role/person/group referenced by tasks. An actor usually
// resolves to an identity in the client's IS via IsRef; that identity
// is what the engine routes tasks to.
type Actor struct {
	Kind       string    `json:"kind"`
	ID         string    `json:"id"`
	Name       string    `json:"name"`
	IsRef      *ActorRef `json:"is_ref,omitempty"`
	Confidence *float64  `json:"confidence,omitempty"`
	Evidence   string    `json:"evidence,omitempty"`
}

type ActorRef struct {
	UserID  string `json:"user_id,omitempty"`
	GroupID string `json:"group_id,omitempty"`
}

type Task struct {
	ID         string   `json:"id"`
	Name       string   `json:"name"`
	Type       string   `json:"type"` // "user" | "service" | "script"
	ActorRef   string   `json:"actor_ref,omitempty"`
	FormRef    string   `json:"form_ref,omitempty"`
	Binding    *Binding `json:"binding,omitempty"`
	Confidence *float64 `json:"confidence,omitempty"`
	Evidence   string   `json:"evidence,omitempty"`
}

// Binding resolves a task to concrete IS entities. User-task bindings
// carry assignee/group/form refs; service-task bindings carry
// system/capability/params.
type Binding struct {
	AssigneeUserID   string         `json:"assignee_user_id,omitempty"`
	CandidateGroupID string         `json:"candidate_group_id,omitempty"`
	FormKey          string         `json:"form_key,omitempty"`
	SystemRef        string         `json:"system_ref,omitempty"`
	Capability       string         `json:"capability,omitempty"`
	Params           map[string]any `json:"params,omitempty"`
	Confidence       *float64       `json:"confidence,omitempty"`
	Evidence         string         `json:"evidence,omitempty"`
}

type Gateway struct {
	ID         string   `json:"id"`
	Type       string   `json:"type"` // "exclusive" | "parallel"
	Confidence *float64 `json:"confidence,omitempty"`
	Evidence   string   `json:"evidence,omitempty"`
}

type Event struct {
	ID         string   `json:"id"`
	Type       string   `json:"type"` // "start" | "end"
	Confidence *float64 `json:"confidence,omitempty"`
	Evidence   string   `json:"evidence,omitempty"`
}

type Flow struct {
	ID         string     `json:"id"`
	From       string     `json:"from"`
	To         string     `json:"to"`
	Condition  *Condition `json:"condition,omitempty"`
	Confidence *float64   `json:"confidence,omitempty"`
	Evidence   string     `json:"evidence,omitempty"`
}

type Condition struct {
	Expression string   `json:"expression"`
	Language   string   `json:"language,omitempty"` // "juel" | "feel"
	Confidence *float64 `json:"confidence,omitempty"`
	Evidence   string   `json:"evidence,omitempty"`
}

type Form struct {
	ID     string      `json:"id"`
	Fields []FormField `json:"fields"`
}

type FormField struct {
	ID       string   `json:"id"`
	Label    string   `json:"label"`
	Type     string   `json:"type"` // "string" | "long" | "boolean" | "date" | "enum"
	Required bool     `json:"required,omitempty"`
	Options  []string `json:"options,omitempty"`
}

// ISRegistry is the per-tenant projection AIOS caches read-only. Users
// and groups come from the engine's identity service (or an external
// IdP). Systems are declared by the tenant and expose capability
// catalogs. Identities are never created here — this type is a cache,
// never a master record.
type ISRegistry struct {
	TenantID          string             `json:"tenant_id"`
	EngineConnections []EngineConnection `json:"engine_connections,omitempty"`
	Users             []ISUser           `json:"users"`
	Groups            []ISGroup          `json:"groups"`
	DeployedForms     []DeployedForm     `json:"deployed_forms,omitempty"`
	Systems           []ISSystem         `json:"systems"`
}

type EngineConnection struct {
	ID           string `json:"id"`
	Kind         string `json:"kind"` // "camunda7" | "camunda8" | "elsa3"
	Endpoint     string `json:"endpoint"`
	LastSyncedAt string `json:"last_synced_at,omitempty"`
}

type ISUser struct {
	ID        string   `json:"id"`
	Name      string   `json:"name"`
	Email     string   `json:"email,omitempty"`
	GroupIDs  []string `json:"group_ids,omitempty"`
	EngineRef string   `json:"engine_ref,omitempty"`
}

type ISGroup struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	EngineRef string `json:"engine_ref,omitempty"`
}

type DeployedForm struct {
	FormKey   string `json:"form_key"`
	EngineRef string `json:"engine_ref,omitempty"`
}

type ISSystem struct {
	ID           string   `json:"id"`
	Name         string   `json:"name,omitempty"`
	Kind         string   `json:"kind"` // "ecm" | "erp" | "comms" | "idp" | "crm" | "signer" | "other"
	Endpoint     string   `json:"endpoint,omitempty"`
	Capabilities []string `json:"capabilities"`
}
