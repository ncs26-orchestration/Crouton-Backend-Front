package compiler_test

import (
	"testing"

	"github.com/ncs26-orchestration/solution/apps/api/internal/compiler"
	"github.com/ncs26-orchestration/solution/apps/api/internal/ir"
)

func TestAnalyzeDecisionTables_ThreeBranchCascadeOnSharedVariable(t *testing.T) {
	wf := &ir.Workflow{
		Version:  "0.1",
		Metadata: ir.Metadata{Name: "n"},
		Gateways: []ir.Gateway{{ID: "g1", Type: "exclusive"}},
		Events:   []ir.Event{{ID: "s", Type: "start"}, {ID: "e1", Type: "end"}, {ID: "e2", Type: "end"}, {ID: "e3", Type: "end"}},
		Flows: []ir.Flow{
			{ID: "f1", From: "s", To: "g1"},
			{ID: "f2", From: "g1", To: "e1", Condition: &ir.Condition{Expression: "${amount < 10000}"}},
			{ID: "f3", From: "g1", To: "e2", Condition: &ir.Condition{Expression: "${amount >= 10000 && amount < 50000}"}},
			{ID: "f4", From: "g1", To: "e3", Condition: &ir.Condition{Expression: "${amount >= 50000}"}},
		},
	}

	tables := compiler.AnalyzeDecisionTables(wf)
	if len(tables) != 1 {
		t.Fatalf("expected 1 table, got %d", len(tables))
	}
	tbl := tables[0]
	if tbl.GatewayID != "g1" {
		t.Errorf("gateway id: %q", tbl.GatewayID)
	}
	if len(tbl.Variables) != 1 || tbl.Variables[0] != "amount" {
		t.Errorf("variables: want [amount], got %v", tbl.Variables)
	}
	if len(tbl.Rules) != 3 {
		t.Fatalf("rules: want 3, got %d", len(tbl.Rules))
	}
	// First rule predicate on `amount`.
	if tbl.Rules[0].Predicates["amount"] != "amount < 10000" {
		t.Errorf("rule 0 predicate: %q", tbl.Rules[0].Predicates["amount"])
	}
}

func TestAnalyzeDecisionTables_SkipsWhenBelowThreshold(t *testing.T) {
	wf := &ir.Workflow{
		Version:  "0.1",
		Metadata: ir.Metadata{Name: "n"},
		Gateways: []ir.Gateway{{ID: "g1", Type: "exclusive"}},
		Events:   []ir.Event{{ID: "s", Type: "start"}, {ID: "a", Type: "end"}, {ID: "b", Type: "end"}},
		Flows: []ir.Flow{
			{ID: "f1", From: "s", To: "g1"},
			{ID: "f2", From: "g1", To: "a", Condition: &ir.Condition{Expression: "${x > 0}"}},
			{ID: "f3", From: "g1", To: "b", Condition: &ir.Condition{Expression: "${x <= 0}"}},
		},
	}
	tables := compiler.AnalyzeDecisionTables(wf)
	if len(tables) != 0 {
		t.Errorf("2-branch gateway should not consolidate, got %+v", tables)
	}
}

func TestAnalyzeDecisionTables_SkipsWhenNoSharedVariable(t *testing.T) {
	wf := &ir.Workflow{
		Version:  "0.1",
		Metadata: ir.Metadata{Name: "n"},
		Gateways: []ir.Gateway{{ID: "g1", Type: "exclusive"}},
		Events:   []ir.Event{{ID: "s", Type: "start"}, {ID: "a", Type: "end"}, {ID: "b", Type: "end"}, {ID: "c", Type: "end"}},
		Flows: []ir.Flow{
			{ID: "f1", From: "s", To: "g1"},
			{ID: "f2", From: "g1", To: "a", Condition: &ir.Condition{Expression: "${foo > 0}"}},
			{ID: "f3", From: "g1", To: "b", Condition: &ir.Condition{Expression: "${bar == true}"}},
			{ID: "f4", From: "g1", To: "c", Condition: &ir.Condition{Expression: "${baz in ['x']}"}},
		},
	}
	tables := compiler.AnalyzeDecisionTables(wf)
	if len(tables) != 0 {
		t.Errorf("gateway with no shared variable should not consolidate, got %+v", tables)
	}
}

func TestAnalyzeDecisionTables_HandlesMultiVariableCascade(t *testing.T) {
	wf := &ir.Workflow{
		Version:  "0.1",
		Metadata: ir.Metadata{Name: "n"},
		Gateways: []ir.Gateway{{ID: "g1", Type: "exclusive"}},
		Events: []ir.Event{
			{ID: "s", Type: "start"}, {ID: "e1", Type: "end"},
			{ID: "e2", Type: "end"}, {ID: "e3", Type: "end"},
		},
		Flows: []ir.Flow{
			{ID: "f1", From: "s", To: "g1"},
			{ID: "f2", From: "g1", To: "e1", Condition: &ir.Condition{Expression: "${amount >= 50000 && category == \"travel\"}"}},
			{ID: "f3", From: "g1", To: "e2", Condition: &ir.Condition{Expression: "${amount >= 50000 && category != \"travel\"}"}},
			{ID: "f4", From: "g1", To: "e3", Condition: &ir.Condition{Expression: "${amount < 50000}"}},
		},
	}
	tables := compiler.AnalyzeDecisionTables(wf)
	if len(tables) != 1 {
		t.Fatalf("expected 1 table, got %d", len(tables))
	}
	tbl := tables[0]
	// Two shared variables across branches (amount in all three,
	// category in two).
	if len(tbl.Variables) != 2 {
		t.Errorf("want 2 variables (amount, category), got %v", tbl.Variables)
	}
	// Row 0 should have predicates for both variables.
	if _, ok := tbl.Rules[0].Predicates["amount"]; !ok {
		t.Errorf("rule 0 missing amount predicate")
	}
	if _, ok := tbl.Rules[0].Predicates["category"]; !ok {
		t.Errorf("rule 0 missing category predicate")
	}
}

func TestAnalyzeDecisionTables_MarksDefaultBranch(t *testing.T) {
	wf := &ir.Workflow{
		Version:  "0.1",
		Metadata: ir.Metadata{Name: "n"},
		Gateways: []ir.Gateway{{ID: "g1", Type: "exclusive"}},
		Events: []ir.Event{
			{ID: "s", Type: "start"}, {ID: "e1", Type: "end"},
			{ID: "e2", Type: "end"}, {ID: "e3", Type: "end"},
		},
		Flows: []ir.Flow{
			{ID: "f1", From: "s", To: "g1"},
			{ID: "f2", From: "g1", To: "e1", Condition: &ir.Condition{Expression: "${x > 10}"}},
			{ID: "f3", From: "g1", To: "e2", Condition: &ir.Condition{Expression: "${x > 20}"}},
			{ID: "f4", From: "g1", To: "e3"}, // default
		},
	}
	tables := compiler.AnalyzeDecisionTables(wf)
	if len(tables) != 1 {
		t.Fatalf("expected 1 table, got %d", len(tables))
	}
	if !tables[0].Rules[2].IsDefault {
		t.Errorf("third rule (no condition) should be marked default")
	}
}
