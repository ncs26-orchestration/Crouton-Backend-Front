package ir

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

// Schemas are embedded at build time so validation has no filesystem
// dependency in production binaries and tests.

//go:embed workflow.schema.json
var workflowSchemaBytes []byte

//go:embed is-registry.schema.json
var isRegistrySchemaBytes []byte

// Validator runs JSON Schema checks and cross-reference checks on IRs
// and IS Registries. Build it once per process; it is safe to share
// across goroutines (jsonschema.Schema is immutable once compiled).
type Validator struct {
	workflow   *jsonschema.Schema
	isRegistry *jsonschema.Schema
}

// Diagnostic is returned for every structural or semantic problem. It
// carries enough context for the UI to point the user at the offending
// IR element.
type Diagnostic struct {
	Severity   string `json:"severity"`              // "error" | "warning"
	IRRef      string `json:"ir_ref,omitempty"`       // JSON pointer-ish path into the IR
	Message    string `json:"message"`
	Suggestion string `json:"suggestion,omitempty"`
}

// NewValidator loads the embedded schemas and returns a ready-to-use
// Validator. Panics only on a build-time error in the embedded
// schemas, which would surface in tests immediately.
func NewValidator() (*Validator, error) {
	compiler := jsonschema.NewCompiler()

	workflowDoc, err := jsonschema.UnmarshalJSON(bytes.NewReader(workflowSchemaBytes))
	if err != nil {
		return nil, fmt.Errorf("unmarshal workflow schema: %w", err)
	}
	if err := compiler.AddResource("workflow.schema.json", workflowDoc); err != nil {
		return nil, fmt.Errorf("add workflow schema: %w", err)
	}

	isDoc, err := jsonschema.UnmarshalJSON(bytes.NewReader(isRegistrySchemaBytes))
	if err != nil {
		return nil, fmt.Errorf("unmarshal is-registry schema: %w", err)
	}
	if err := compiler.AddResource("is-registry.schema.json", isDoc); err != nil {
		return nil, fmt.Errorf("add is-registry schema: %w", err)
	}

	workflowSchema, err := compiler.Compile("workflow.schema.json")
	if err != nil {
		return nil, fmt.Errorf("compile workflow schema: %w", err)
	}
	isRegistrySchema, err := compiler.Compile("is-registry.schema.json")
	if err != nil {
		return nil, fmt.Errorf("compile is-registry schema: %w", err)
	}
	return &Validator{workflow: workflowSchema, isRegistry: isRegistrySchema}, nil
}

// ValidateWorkflowJSON checks raw JSON bytes against the Workflow IR
// schema. Returns the parsed Workflow plus any structural diagnostics.
// A non-nil error means the input was not valid JSON at all; a nil
// error with a non-empty diagnostics slice means the JSON parsed but
// failed schema validation.
func (v *Validator) ValidateWorkflowJSON(data []byte) (*Workflow, []Diagnostic, error) {
	doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(data))
	if err != nil {
		return nil, nil, fmt.Errorf("invalid JSON: %w", err)
	}
	if err := v.workflow.Validate(doc); err != nil {
		return nil, schemaErrToDiagnostics(err), nil
	}
	var wf Workflow
	if err := json.Unmarshal(data, &wf); err != nil {
		return nil, nil, fmt.Errorf("unmarshal workflow: %w", err)
	}
	return &wf, nil, nil
}

// ValidateISRegistryJSON checks raw JSON bytes against the IS Registry
// schema.
func (v *Validator) ValidateISRegistryJSON(data []byte) (*ISRegistry, []Diagnostic, error) {
	doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(data))
	if err != nil {
		return nil, nil, fmt.Errorf("invalid JSON: %w", err)
	}
	if err := v.isRegistry.Validate(doc); err != nil {
		return nil, schemaErrToDiagnostics(err), nil
	}
	var r ISRegistry
	if err := json.Unmarshal(data, &r); err != nil {
		return nil, nil, fmt.Errorf("unmarshal is-registry: %w", err)
	}
	return &r, nil, nil
}

// CrossRef returns diagnostics for any reference in the workflow that
// cannot be resolved. It checks:
//
//   - task.actor_ref exists in workflow.actors
//   - task.form_ref exists in workflow.forms
//   - flow.from / flow.to point at a known task / gateway / event
//   - binding.assignee_user_id exists in is.users
//   - binding.candidate_group_id exists in is.groups
//   - binding.form_key exists in is.deployed_forms
//   - binding.system_ref exists in is.systems and declares the capability
//
// A nil registry skips the IS-dependent checks (useful for tests that
// compile a raw IR without a tenant context).
func (v *Validator) CrossRef(wf *Workflow, is *ISRegistry) []Diagnostic {
	var diags []Diagnostic

	actors := index(wf.Actors, func(a Actor) string { return a.ID })
	forms := index(wf.Forms, func(f Form) string { return f.ID })
	nodes := map[string]string{}
	for _, t := range wf.Tasks {
		nodes[t.ID] = "task"
	}
	for _, g := range wf.Gateways {
		nodes[g.ID] = "gateway"
	}
	for _, e := range wf.Events {
		nodes[e.ID] = "event"
	}

	// Task references.
	for i, t := range wf.Tasks {
		if t.ActorRef != "" {
			if _, ok := actors[t.ActorRef]; !ok {
				diags = append(diags, Diagnostic{
					Severity: "error",
					IRRef:    fmt.Sprintf("/tasks/%d/actor_ref", i),
					Message:  fmt.Sprintf("actor_ref %q does not resolve to any actor", t.ActorRef),
				})
			}
		}
		if t.FormRef != "" {
			if _, ok := forms[t.FormRef]; !ok {
				diags = append(diags, Diagnostic{
					Severity: "error",
					IRRef:    fmt.Sprintf("/tasks/%d/form_ref", i),
					Message:  fmt.Sprintf("form_ref %q does not resolve to any form", t.FormRef),
				})
			}
		}
		if t.Binding != nil && is != nil {
			diags = append(diags, checkBinding(i, t.Binding, is)...)
		}
	}

	// Flow endpoints.
	for i, f := range wf.Flows {
		if _, ok := nodes[f.From]; !ok {
			diags = append(diags, Diagnostic{
				Severity: "error",
				IRRef:    fmt.Sprintf("/flows/%d/from", i),
				Message:  fmt.Sprintf("flow source %q does not resolve to any task/gateway/event", f.From),
			})
		}
		if _, ok := nodes[f.To]; !ok {
			diags = append(diags, Diagnostic{
				Severity: "error",
				IRRef:    fmt.Sprintf("/flows/%d/to", i),
				Message:  fmt.Sprintf("flow target %q does not resolve to any task/gateway/event", f.To),
			})
		}
	}

	// Event-presence sanity.
	hasStart := false
	hasEnd := false
	for _, e := range wf.Events {
		switch e.Type {
		case "start":
			hasStart = true
		case "end":
			hasEnd = true
		}
	}
	if !hasStart {
		diags = append(diags, Diagnostic{Severity: "error", IRRef: "/events", Message: "workflow has no start event"})
	}
	if !hasEnd {
		diags = append(diags, Diagnostic{Severity: "error", IRRef: "/events", Message: "workflow has no end event"})
	}

	return diags
}

func checkBinding(taskIdx int, b *Binding, is *ISRegistry) []Diagnostic {
	var diags []Diagnostic
	path := fmt.Sprintf("/tasks/%d/binding", taskIdx)

	if b.AssigneeUserID != "" {
		if !containsFunc(is.Users, func(u ISUser) bool { return u.ID == b.AssigneeUserID }) {
			diags = append(diags, Diagnostic{
				Severity:   "error",
				IRRef:      path + "/assignee_user_id",
				Message:    fmt.Sprintf("assignee_user_id %q is not in the IS Registry", b.AssigneeUserID),
				Suggestion: "run engine sync or check that the user still exists upstream",
			})
		}
	}
	if b.CandidateGroupID != "" {
		if !containsFunc(is.Groups, func(g ISGroup) bool { return g.ID == b.CandidateGroupID }) {
			diags = append(diags, Diagnostic{
				Severity: "error",
				IRRef:    path + "/candidate_group_id",
				Message:  fmt.Sprintf("candidate_group_id %q is not in the IS Registry", b.CandidateGroupID),
			})
		}
	}
	if b.FormKey != "" {
		if !containsFunc(is.DeployedForms, func(f DeployedForm) bool { return f.FormKey == b.FormKey }) {
			diags = append(diags, Diagnostic{
				Severity: "error",
				IRRef:    path + "/form_key",
				Message:  fmt.Sprintf("form_key %q is not deployed on the engine", b.FormKey),
			})
		}
	}
	if b.SystemRef != "" {
		var sys *ISSystem
		for i := range is.Systems {
			if is.Systems[i].ID == b.SystemRef {
				sys = &is.Systems[i]
				break
			}
		}
		if sys == nil {
			diags = append(diags, Diagnostic{
				Severity: "error",
				IRRef:    path + "/system_ref",
				Message:  fmt.Sprintf("system_ref %q is not declared in the IS Registry", b.SystemRef),
			})
		} else if b.Capability != "" && !slices.Contains(sys.Capabilities, b.Capability) {
			diags = append(diags, Diagnostic{
				Severity: "error",
				IRRef:    path + "/capability",
				Message:  fmt.Sprintf("system %q does not declare capability %q", b.SystemRef, b.Capability),
			})
		}
	}

	return diags
}

// schemaErrToDiagnostics turns a jsonschema validation error tree into
// a flat list of diagnostics keyed by JSON path.
func schemaErrToDiagnostics(err error) []Diagnostic {
	msg := err.Error()
	// Strip the leading "jsonschema validation failed" banner to keep
	// diagnostic messages compact in API responses.
	if i := strings.Index(msg, "\n"); i >= 0 {
		return []Diagnostic{{Severity: "error", Message: strings.TrimSpace(msg[i:])}}
	}
	return []Diagnostic{{Severity: "error", Message: msg}}
}

func index[T any](xs []T, key func(T) string) map[string]T {
	m := make(map[string]T, len(xs))
	for _, x := range xs {
		m[key(x)] = x
	}
	return m
}

func containsFunc[T any](xs []T, pred func(T) bool) bool {
	return slices.ContainsFunc(xs, pred)
}
