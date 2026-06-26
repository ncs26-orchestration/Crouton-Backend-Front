package bpmn_test

import (
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ncs26-orchestration/solution/apps/api/internal/compiler/bpmn"
	"github.com/ncs26-orchestration/solution/apps/api/internal/ir"
)

// -update regenerates the golden .bpmn.xml files. Use it when the
// compiler emission changes intentionally:
//
//	go test ./internal/compiler/bpmn/ -update
var update = flag.Bool("update", false, "rewrite golden BPMN files from current compiler output")

func packagesIRExamples(t *testing.T) string {
	t.Helper()
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	root := filepath.Clean(filepath.Join(cwd, "..", "..", "..", "..", "..", "packages", "ir", "examples"))
	if _, err := os.Stat(root); err != nil {
		t.Fatalf("packages/ir/examples not found at %s: %v", root, err)
	}
	return root
}

func TestCompile_ServiceTaskWithoutBindingFallsBackToManualTask(t *testing.T) {
	wf := &ir.Workflow{
		Version:  "0.1",
		Metadata: ir.Metadata{Name: "manual fallback"},
		Tasks: []ir.Task{{
			ID:   "release_employee",
			Name: "Release employee",
			Type: "service",
		}},
		Events: []ir.Event{{ID: "start", Type: "start"}, {ID: "end", Type: "end"}},
		Flows: []ir.Flow{
			{ID: "f1", From: "start", To: "release_employee"},
			{ID: "f2", From: "release_employee", To: "end"},
		},
	}

	out, diags, err := bpmn.Compile(wf, bpmn.DefaultCamunda7Profile())
	if err != nil {
		t.Fatalf("compile: %v — diags: %+v", err, diags)
	}
	xml := string(out)
	if !strings.Contains(xml, `<bpmn:manualTask id="release_employee" name="Release employee">`) {
		t.Fatalf("expected unresolved service task to render as manualTask, got:\n%s", xml)
	}
	if strings.Contains(xml, `<bpmn:serviceTask id="release_employee"`) {
		t.Fatalf("unresolved service task rendered as invalid serviceTask:\n%s", xml)
	}
}

// TestCompile_Goldens compiles each canonical example IR and compares
// the output against a checked-in .bpmn.xml. Pass -update to regenerate.
func TestCompile_Goldens(t *testing.T) {
	v, err := ir.NewValidator()
	if err != nil {
		t.Fatalf("NewValidator: %v", err)
	}
	profile := bpmn.DefaultCamunda7Profile()
	examplesDir := packagesIRExamples(t)

	cases := []string{"simple-gateway", "expense-approval", "onboarding"}
	for _, name := range cases {
		t.Run(name, func(t *testing.T) {
			data, err := os.ReadFile(filepath.Join(examplesDir, name+".json"))
			if err != nil {
				t.Fatalf("read IR: %v", err)
			}
			wf, diags, err := v.ValidateWorkflowJSON(data)
			if err != nil {
				t.Fatalf("validate: %v", err)
			}
			if len(diags) > 0 {
				t.Fatalf("schema diagnostics: %+v", diags)
			}

			out, compDiags, err := bpmn.Compile(wf, profile)
			if err != nil {
				t.Fatalf("compile: %v — diags: %+v", err, compDiags)
			}
			if len(compDiags) > 0 {
				t.Fatalf("unexpected compile diagnostics for %s: %+v", name, compDiags)
			}

			goldenPath := filepath.Join("testdata", name+".bpmn.xml")
			if *update {
				if err := os.WriteFile(goldenPath, out, 0o644); err != nil {
					t.Fatalf("write golden: %v", err)
				}
				t.Logf("updated %s (%d bytes)", goldenPath, len(out))
				return
			}

			want, err := os.ReadFile(goldenPath)
			if err != nil {
				t.Fatalf("read golden (run with -update to seed): %v", err)
			}
			if string(want) != string(out) {
				t.Errorf("golden mismatch for %s\n--- want\n%s--- got\n%s", name, string(want), string(out))
			}
		})
	}
}

// TestCompile_Determinism ensures two compilations of the same IR
// produce identical bytes. This underpins drift detection.
func TestCompile_Determinism(t *testing.T) {
	v, err := ir.NewValidator()
	if err != nil {
		t.Fatalf("NewValidator: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(packagesIRExamples(t), "expense-approval.json"))
	if err != nil {
		t.Fatalf("read IR: %v", err)
	}
	wf, _, _ := v.ValidateWorkflowJSON(data)

	profile := bpmn.DefaultCamunda7Profile()
	a, _, err := bpmn.Compile(wf, profile)
	if err != nil {
		t.Fatalf("compile a: %v", err)
	}
	b, _, err := bpmn.Compile(wf, profile)
	if err != nil {
		t.Fatalf("compile b: %v", err)
	}
	if string(a) != string(b) {
		t.Errorf("non-deterministic output:\n--- a\n%s--- b\n%s", string(a), string(b))
	}
}

// TestCompile_StrictRefusesUnsupportedFeature verifies the strict-mode
// profile check trips on an unknown task type.
func TestCompile_StrictRefusesUnsupportedFeature(t *testing.T) {
	wf := &ir.Workflow{
		Version:  "0.1",
		Metadata: ir.Metadata{Name: "bad"},
		Tasks:    []ir.Task{{ID: "t1", Name: "x", Type: "wobble"}}, // unknown
		Events:   []ir.Event{{ID: "s", Type: "start"}, {ID: "e", Type: "end"}},
		Flows:    []ir.Flow{{ID: "f1", From: "s", To: "t1"}, {ID: "f2", From: "t1", To: "e"}},
	}
	_, diags, err := bpmn.Compile(wf, bpmn.DefaultCamunda7Profile())
	if err == nil {
		t.Fatalf("expected strict-mode refusal, got nil error")
	}
	if len(diags) == 0 {
		t.Fatalf("expected diagnostics on refusal")
	}
}
