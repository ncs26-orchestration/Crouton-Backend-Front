package elsa3_test

import (
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"testing"

	"github.com/Noussour/aup/apps/api/internal/engine/elsa3"
	"github.com/Noussour/aup/apps/api/internal/ir"
)

var update = flag.Bool("update", false, "rewrite golden Elsa JSON files from current compiler output")

// TestCompile_Goldens runs each canonical IR example through the
// Elsa 3 compiler and compares against a checked-in .elsa.json. Pass
// -update to regenerate the goldens after an intentional shape change.
func TestCompile_Goldens(t *testing.T) {
	v, err := ir.NewValidator()
	if err != nil {
		t.Fatalf("NewValidator: %v", err)
	}

	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	examplesDir := filepath.Clean(filepath.Join(cwd, "..", "..", "..", "..", "..", "packages", "ir", "examples"))

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

			out, compDiags, err := elsa3.Compile(wf)
			if err != nil {
				t.Fatalf("compile: %v — diags: %+v", err, compDiags)
			}

			// Quick structural sanity — the file must be valid JSON
			// with the expected top-level keys.
			var decoded map[string]any
			if err := json.Unmarshal(out, &decoded); err != nil {
				t.Fatalf("compiler emitted invalid JSON: %v", err)
			}
			for _, key := range []string{"id", "definitionId", "name", "root"} {
				if _, ok := decoded[key]; !ok {
					t.Errorf("missing top-level key %q in output", key)
				}
			}

			goldenPath := filepath.Join("testdata", name+".elsa.json")
			if *update {
				if err := os.MkdirAll("testdata", 0o755); err != nil {
					t.Fatalf("mkdir testdata: %v", err)
				}
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
				t.Errorf("golden mismatch for %s\n--- want\n%s\n--- got\n%s", name, string(want), string(out))
			}
		})
	}
}

// TestCompile_Determinism ensures two compilations of the same IR
// produce byte-identical output. Drift detection relies on this.
func TestCompile_Determinism(t *testing.T) {
	v, _ := ir.NewValidator()
	cwd, _ := os.Getwd()
	data, err := os.ReadFile(filepath.Clean(filepath.Join(cwd, "..", "..", "..", "..", "..", "packages", "ir", "examples", "expense-approval.json")))
	if err != nil {
		t.Fatalf("read IR: %v", err)
	}
	wf, _, _ := v.ValidateWorkflowJSON(data)

	a, _, err := elsa3.Compile(wf)
	if err != nil {
		t.Fatalf("compile a: %v", err)
	}
	b, _, err := elsa3.Compile(wf)
	if err != nil {
		t.Fatalf("compile b: %v", err)
	}
	if string(a) != string(b) {
		t.Errorf("non-deterministic output")
	}
}
