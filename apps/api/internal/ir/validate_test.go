package ir_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ncs26-orchestration/solution/apps/api/internal/ir"
)

// packagesIRRoot returns the absolute path to packages/ir/ so we can
// load the canonical example IRs for validation. Tests assume the
// standard monorepo layout (apps/api → packages/ir is two levels up).
func packagesIRRoot(t *testing.T) string {
	t.Helper()
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	root := filepath.Clean(filepath.Join(cwd, "..", "..", "..", "..", "packages", "ir"))
	if _, err := os.Stat(root); err != nil {
		t.Fatalf("packages/ir not found at %s: %v", root, err)
	}
	return root
}

func mustRead(t *testing.T, path string) []byte {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return b
}

func TestValidator_EmbeddedSchemasCompile(t *testing.T) {
	v, err := ir.NewValidator()
	if err != nil {
		t.Fatalf("NewValidator: %v", err)
	}
	if v == nil {
		t.Fatalf("validator was nil with no error")
	}
}

func TestValidator_ExamplesPassSchema(t *testing.T) {
	v, err := ir.NewValidator()
	if err != nil {
		t.Fatalf("NewValidator: %v", err)
	}
	root := packagesIRRoot(t)
	examples := []string{"simple-gateway.json", "expense-approval.json", "onboarding.json"}

	for _, name := range examples {
		t.Run(name, func(t *testing.T) {
			data := mustRead(t, filepath.Join(root, "examples", name))
			wf, diags, err := v.ValidateWorkflowJSON(data)
			if err != nil {
				t.Fatalf("validate returned error: %v", err)
			}
			if len(diags) > 0 {
				t.Fatalf("expected example to validate, got diagnostics: %+v", diags)
			}
			if wf == nil {
				t.Fatalf("validator returned nil workflow with no diagnostics")
			}
			if wf.Version != "0.1" {
				t.Errorf("version: want 0.1, got %q", wf.Version)
			}
			if wf.Metadata.Name == "" {
				t.Errorf("metadata.name is empty")
			}
			if len(wf.Actors) == 0 {
				t.Errorf("expected at least one actor")
			}
			if len(wf.Tasks) == 0 {
				t.Errorf("expected at least one task")
			}
		})
	}
}

func TestValidator_RejectsMalformed(t *testing.T) {
	v, err := ir.NewValidator()
	if err != nil {
		t.Fatalf("NewValidator: %v", err)
	}

	cases := []struct {
		name string
		json string
	}{
		{
			name: "missing_version",
			json: `{"metadata":{"name":"x"},"actors":[],"tasks":[],"events":[],"flows":[]}`,
		},
		{
			name: "wrong_version",
			json: `{"version":"0.9","metadata":{"name":"x"},"actors":[],"tasks":[],"events":[],"flows":[]}`,
		},
		{
			name: "missing_metadata",
			json: `{"version":"0.1","actors":[],"tasks":[],"events":[],"flows":[]}`,
		},
		{
			name: "unknown_task_type",
			json: `{
				"version":"0.1",
				"metadata":{"name":"x"},
				"actors":[],
				"tasks":[{"id":"t1","name":"bad","type":"wobble"}],
				"events":[],
				"flows":[]
			}`,
		},
		{
			name: "unknown_gateway_type",
			json: `{
				"version":"0.1",
				"metadata":{"name":"x"},
				"actors":[],
				"tasks":[],
				"gateways":[{"id":"g1","type":"complex"}],
				"events":[],
				"flows":[]
			}`,
		},
		{
			name: "invalid_event_type",
			json: `{
				"version":"0.1",
				"metadata":{"name":"x"},
				"actors":[],
				"tasks":[],
				"events":[{"id":"e1","type":"timer"}],
				"flows":[]
			}`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, diags, err := v.ValidateWorkflowJSON([]byte(tc.json))
			if err != nil {
				t.Fatalf("unexpected error on malformed input: %v", err)
			}
			if len(diags) == 0 {
				t.Errorf("expected schema diagnostics for %s, got none", tc.name)
			}
		})
	}
}

func TestValidator_CrossRef_CatchesUnknownReferences(t *testing.T) {
	v, err := ir.NewValidator()
	if err != nil {
		t.Fatalf("NewValidator: %v", err)
	}

	// Intentionally broken: actor_ref + flow source point at non-existent ids.
	broken := []byte(`{
		"version":"0.1",
		"metadata":{"name":"broken"},
		"actors":[{"id":"a1","name":"A","kind":"role"}],
		"tasks":[{"id":"t1","name":"go","type":"user","actor_ref":"ghost"}],
		"events":[{"id":"e_start","type":"start"},{"id":"e_end","type":"end"}],
		"flows":[
			{"id":"f1","from":"e_start","to":"t1"},
			{"id":"f2","from":"t1","to":"nowhere"}
		]
	}`)
	wf, diags, err := v.ValidateWorkflowJSON(broken)
	if err != nil || len(diags) > 0 {
		t.Fatalf("broken-ref fixture should pass schema (structural is fine); got diags=%+v err=%v", diags, err)
	}
	crossDiags := v.CrossRef(wf, nil)
	if len(crossDiags) < 2 {
		t.Fatalf("expected >=2 cross-ref diagnostics (actor_ref ghost, flow to nowhere); got %d: %+v", len(crossDiags), crossDiags)
	}
}

func TestValidator_CrossRef_BindingAgainstISRegistry(t *testing.T) {
	v, err := ir.NewValidator()
	if err != nil {
		t.Fatalf("NewValidator: %v", err)
	}

	wf := &ir.Workflow{
		Version:  "0.1",
		Metadata: ir.Metadata{Name: "bind"},
		Actors:   []ir.Actor{{ID: "a1", Name: "A", Kind: "group"}},
		Tasks: []ir.Task{
			{ID: "t1", Name: "review", Type: "user", ActorRef: "a1",
				Binding: &ir.Binding{CandidateGroupID: "accounting"}},
			{ID: "t2", Name: "archive", Type: "service",
				Binding: &ir.Binding{SystemRef: "openbee", Capability: "document.archive"}},
		},
		Events: []ir.Event{{ID: "s", Type: "start"}, {ID: "e", Type: "end"}},
		Flows:  []ir.Flow{{ID: "f1", From: "s", To: "t1"}, {ID: "f2", From: "t1", To: "t2"}, {ID: "f3", From: "t2", To: "e"}},
	}

	// Happy path: registry contains everything referenced.
	good := &ir.ISRegistry{
		TenantID: "demo",
		Users:    []ir.ISUser{{ID: "mary", Name: "Mary"}},
		Groups:   []ir.ISGroup{{ID: "accounting", Name: "Accounting"}},
		Systems: []ir.ISSystem{
			{ID: "openbee", Kind: "ecm", Capabilities: []string{"document.archive"}},
		},
	}
	if diags := v.CrossRef(wf, good); len(diags) > 0 {
		t.Fatalf("happy path produced diagnostics: %+v", diags)
	}

	// Missing group.
	badGroup := *good
	badGroup.Groups = nil
	if diags := v.CrossRef(wf, &badGroup); len(diags) == 0 {
		t.Errorf("expected diagnostic for missing group 'accounting'")
	}

	// System exists but capability not declared.
	badCap := *good
	badCap.Systems = []ir.ISSystem{{ID: "openbee", Kind: "ecm", Capabilities: []string{"document.store"}}}
	diags := v.CrossRef(wf, &badCap)
	found := false
	for _, d := range diags {
		if d.IRRef == "/tasks/1/binding/capability" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected capability diagnostic, got: %+v", diags)
	}
}
