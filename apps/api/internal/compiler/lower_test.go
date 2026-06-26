package compiler_test

import (
	"strings"
	"testing"

	"github.com/ncs26-orchestration/solution/apps/api/internal/compiler"
	"github.com/ncs26-orchestration/solution/apps/api/internal/ir"
)

func TestLower_ResolvesActorIsRefIntoBinding(t *testing.T) {
	proc := &ir.ProcessIR{
		Version:  "0.1",
		Metadata: ir.Metadata{Name: "n"},
		Actors: []ir.Actor{
			{ID: "mary", Kind: "person", Name: "Mary", IsRef: &ir.ActorRef{UserID: "mary"}},
			{ID: "acc", Kind: "group", Name: "Accounting", IsRef: &ir.ActorRef{GroupID: "accounting"}},
		},
		Tasks: []ir.Task{
			{ID: "t1", Type: "user", Name: "Review", ActorRef: "mary"},
			{ID: "t2", Type: "user", Name: "Approve", ActorRef: "acc"},
		},
		Events: []ir.Event{{ID: "s", Type: "start"}, {ID: "e", Type: "end"}},
		Flows: []ir.Flow{
			{ID: "f1", From: "s", To: "t1"},
			{ID: "f2", From: "t1", To: "t2"},
			{ID: "f3", From: "t2", To: "e"},
		},
	}

	exe, diags, err := compiler.Lower(proc, nil)
	if err != nil {
		t.Fatalf("Lower: %v", err)
	}
	if exe.Tasks[0].Binding == nil || exe.Tasks[0].Binding.AssigneeUserID != "mary" {
		t.Errorf("task t1 should resolve to assignee mary, got %+v", exe.Tasks[0].Binding)
	}
	if exe.Tasks[1].Binding == nil || exe.Tasks[1].Binding.CandidateGroupID != "accounting" {
		t.Errorf("task t2 should resolve to group accounting, got %+v", exe.Tasks[1].Binding)
	}
	// Input unchanged.
	if proc.Tasks[0].Binding != nil {
		t.Errorf("Lower mutated input: proc.Tasks[0].Binding should still be nil")
	}
	// Each resolution emits one diagnostic.
	var n int
	for _, d := range diags {
		if strings.HasPrefix(d.IRRef, compiler.LoweringSource) && strings.Contains(d.Message, "resolved") {
			n++
		}
	}
	if n != 2 {
		t.Errorf("expected 2 resolution diagnostics, got %d: %+v", n, diags)
	}
}

func TestLower_SynthesizesDefaultBranchForExclusiveGateway(t *testing.T) {
	proc := &ir.ProcessIR{
		Version:  "0.1",
		Metadata: ir.Metadata{Name: "n"},
		Tasks:    []ir.Task{{ID: "t1", Type: "user", Name: "A"}, {ID: "t2", Type: "user", Name: "B"}},
		Gateways: []ir.Gateway{{ID: "g1", Type: "exclusive"}},
		Events:   []ir.Event{{ID: "s", Type: "start"}, {ID: "e", Type: "end"}},
		Flows: []ir.Flow{
			{ID: "f1", From: "s", To: "g1"},
			{ID: "f2", From: "g1", To: "t1", Condition: &ir.Condition{Expression: "${x > 10}"}},
			{ID: "f3", From: "g1", To: "t2", Condition: &ir.Condition{Expression: "${x <= 10}"}},
			{ID: "f4", From: "t1", To: "e"},
			{ID: "f5", From: "t2", To: "e"},
		},
	}
	exe, diags, err := compiler.Lower(proc, nil)
	if err != nil {
		t.Fatalf("Lower: %v", err)
	}

	// A new end event and a new unconditional flow should exist.
	var defaultEnd string
	for _, ev := range exe.Events {
		if strings.HasPrefix(ev.ID, "end_default_g1") {
			defaultEnd = ev.ID
		}
	}
	if defaultEnd == "" {
		t.Fatalf("expected synthesized default end event, got events=%v", exe.Events)
	}
	var defaultFlow *ir.Flow
	for i := range exe.Flows {
		f := &exe.Flows[i]
		if f.From == "g1" && f.To == defaultEnd {
			defaultFlow = f
		}
	}
	if defaultFlow == nil {
		t.Fatalf("expected synthesized default flow from g1 to %s", defaultEnd)
	}
	if defaultFlow.Condition != nil {
		t.Errorf("synthesized default flow must be unconditional, got %+v", defaultFlow.Condition)
	}
	// Diagnostic should exist.
	var found bool
	for _, d := range diags {
		if strings.Contains(d.Message, "synthesized default branch") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected synthesis diagnostic, got %+v", diags)
	}
}

func TestLower_LeavesGatewayWithExistingDefaultAlone(t *testing.T) {
	proc := &ir.ProcessIR{
		Version:  "0.1",
		Metadata: ir.Metadata{Name: "n"},
		Gateways: []ir.Gateway{{ID: "g1", Type: "exclusive"}},
		Events:   []ir.Event{{ID: "s", Type: "start"}, {ID: "e1", Type: "end"}, {ID: "e2", Type: "end"}},
		Flows: []ir.Flow{
			{ID: "f1", From: "s", To: "g1"},
			{ID: "f2", From: "g1", To: "e1", Condition: &ir.Condition{Expression: "${x > 10}"}},
			{ID: "f3", From: "g1", To: "e2"},
		},
	}
	exe, diags, err := compiler.Lower(proc, nil)
	if err != nil {
		t.Fatalf("Lower: %v", err)
	}
	if len(exe.Events) != 3 || len(exe.Flows) != 3 {
		t.Errorf("Lower should not touch gateways with an existing default; got events=%d flows=%d", len(exe.Events), len(exe.Flows))
	}
	for _, d := range diags {
		if strings.Contains(d.Message, "synthesized default") {
			t.Errorf("did not expect synthesis diagnostic, got %+v", d)
		}
	}
}

func TestLower_NormalizesMissingConditionLanguage(t *testing.T) {
	proc := &ir.ProcessIR{
		Version:  "0.1",
		Metadata: ir.Metadata{Name: "n"},
		Events:   []ir.Event{{ID: "s", Type: "start"}, {ID: "e", Type: "end"}},
		Flows: []ir.Flow{
			{ID: "f1", From: "s", To: "e", Condition: &ir.Condition{Expression: "${x > 10}"}},
		},
	}
	exe, _, err := compiler.Lower(proc, nil)
	if err != nil {
		t.Fatalf("Lower: %v", err)
	}
	if exe.Flows[0].Condition.Language != "juel" {
		t.Errorf("expected juel default, got %q", exe.Flows[0].Condition.Language)
	}
}

func TestLower_PropagatesConfidence(t *testing.T) {
	tc := 0.9
	bc := 0.5
	proc := &ir.ProcessIR{
		Version:  "0.1",
		Metadata: ir.Metadata{Name: "n"},
		Tasks: []ir.Task{
			{ID: "t1", Type: "user", Name: "A", Confidence: &tc, Binding: &ir.Binding{Confidence: &bc}},
		},
		Events: []ir.Event{{ID: "s", Type: "start"}, {ID: "e", Type: "end"}},
		Flows: []ir.Flow{
			{ID: "f1", From: "s", To: "t1"},
			{ID: "f2", From: "t1", To: "e"},
		},
	}
	exe, _, err := compiler.Lower(proc, nil)
	if err != nil {
		t.Fatalf("Lower: %v", err)
	}
	if exe.Tasks[0].Confidence == nil || *exe.Tasks[0].Confidence != 0.5 {
		t.Errorf("task confidence should propagate to the min of (0.9, 0.5) = 0.5, got %v", exe.Tasks[0].Confidence)
	}
	// Input unchanged.
	if proc.Tasks[0].Confidence == nil || *proc.Tasks[0].Confidence != 0.9 {
		t.Errorf("Lower mutated input task confidence: got %v", proc.Tasks[0].Confidence)
	}
}

func TestLower_FlagsActorWithoutIsRef(t *testing.T) {
	proc := &ir.ProcessIR{
		Version:  "0.1",
		Metadata: ir.Metadata{Name: "n"},
		Actors:   []ir.Actor{{ID: "unknown", Kind: "role", Name: "mystery"}},
		Tasks:    []ir.Task{{ID: "t1", Type: "user", Name: "A", ActorRef: "unknown"}},
		Events:   []ir.Event{{ID: "s", Type: "start"}, {ID: "e", Type: "end"}},
		Flows:    []ir.Flow{{ID: "f1", From: "s", To: "t1"}, {ID: "f2", From: "t1", To: "e"}},
	}
	_, diags, err := compiler.Lower(proc, nil)
	if err != nil {
		t.Fatalf("Lower: %v", err)
	}
	var found bool
	for _, d := range diags {
		if strings.Contains(d.Message, "no is_ref") && strings.Contains(d.Message, "human stub") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected human-stub diagnostic, got %+v", diags)
	}
}
