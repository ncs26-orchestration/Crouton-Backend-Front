// Package compiler owns the ProcessIR -> ExecutableIR transformation
// and registers the engine-specific backends (BPMN, Elsa, ...). The
// lowering pass lives at this package root because it is target-
// agnostic: every backend consumes the same ExecutableIR.
package compiler

import (
	"fmt"
	"sort"

	"github.com/ncs26-orchestration/solution/apps/api/internal/ir"
)

// LoweringSource is the diagnostic `ir_ref` prefix for issues that
// come from the Lower pass specifically — downstream readers (the
// web Suggestions tab) can filter on this.
const LoweringSource = "lowering"

// Lower turns a ProcessIR into an ExecutableIR. It applies three
// rewrites:
//
//  1. Actor resolution — for every user task whose actor has an
//     is_ref but whose binding lacks both assignee_user_id and
//     candidate_group_id, populate the binding from the actor.
//
//  2. Default-branch synthesis — every exclusive gateway with
//     outgoing flows and no "default" (unconditional) flow gets one
//     synthesized pointing at a newly-minted end event. This
//     guarantees the BPMN compiler can always find a safe path
//     when no condition matches at runtime.
//
//  3. Condition normalization — flow conditions with an empty
//     language get "juel" as the default, matching Camunda 7's
//     expected shape.
//
// It also propagates confidence values so a task surfaces as the
// weakest of (task, binding) — the Inspector "Why this" card and the
// Diagnostics "Low confidence" tab rely on this.
//
// The function is pure: no IO, no mutation of the input (the input
// pointer is copied and modified via the copy). Returns a new
// *ir.ExecutableIR plus a slice of diagnostics describing what it
// rewrote.
//
// IS registry is optional. When nil, actor resolution is skipped (a
// diagnostic is emitted instead) and capability validity checks are
// deferred to the compiler. Pre-CrossRef-validated input is the
// typical caller (see handler/compile.go).
func Lower(proc *ir.ProcessIR, reg *ir.ISRegistry) (*ir.ExecutableIR, []ir.Diagnostic, error) {
	if proc == nil {
		return nil, nil, fmt.Errorf("nil process IR")
	}

	// Deep copy — we don't want to mutate caller state.
	exe := cloneWorkflow(proc)

	var diags []ir.Diagnostic
	diags = append(diags, resolveActors(exe, reg)...)
	diags = append(diags, synthesizeDefaults(exe)...)
	diags = append(diags, normalizeConditions(exe)...)
	propagateConfidence(exe)

	return exe, diags, nil
}

// resolveActors fills in binding assignees from each task's actor's
// is_ref when the binding doesn't already have routing information.
// Emits a diagnostic for user tasks whose actor has no is_ref and no
// binding — those compile to a human stub with no automatic routing.
func resolveActors(wf *ir.ExecutableIR, reg *ir.ISRegistry) []ir.Diagnostic {
	actors := map[string]*ir.Actor{}
	for i := range wf.Actors {
		a := &wf.Actors[i]
		actors[a.ID] = a
	}

	var diags []ir.Diagnostic
	for i := range wf.Tasks {
		t := &wf.Tasks[i]
		if t.Type != "user" {
			continue
		}
		a, ok := actors[t.ActorRef]
		if !ok {
			continue
		}
		// Build a binding if the task has none so we have a place to
		// write resolved ids.
		if t.Binding == nil {
			t.Binding = &ir.Binding{}
		}
		if t.Binding.AssigneeUserID == "" && t.Binding.CandidateGroupID == "" {
			if a.IsRef == nil {
				// No hint at all — let the BPMN compiler render a
				// human stub and surface this in Suggestions.
				diags = append(diags, ir.Diagnostic{
					Severity:   "warning",
					IRRef:      fmt.Sprintf("%s/tasks/%d/actor_ref", LoweringSource, i),
					Message:    fmt.Sprintf("user task %q has no assignee and its actor %q has no is_ref — task will render as a human stub", t.ID, a.ID),
					Suggestion: "Map the actor to a real user or group in the IS Registry.",
				})
				continue
			}
			switch {
			case a.IsRef.UserID != "":
				t.Binding.AssigneeUserID = a.IsRef.UserID
				diags = append(diags, ir.Diagnostic{
					Severity: "warning",
					IRRef:    fmt.Sprintf("%s/tasks/%d/binding", LoweringSource, i),
					Message:  fmt.Sprintf("resolved task %q assignee to user %q via actor is_ref", t.ID, a.IsRef.UserID),
				})
			case a.IsRef.GroupID != "":
				t.Binding.CandidateGroupID = a.IsRef.GroupID
				diags = append(diags, ir.Diagnostic{
					Severity: "warning",
					IRRef:    fmt.Sprintf("%s/tasks/%d/binding", LoweringSource, i),
					Message:  fmt.Sprintf("resolved task %q candidate group to %q via actor is_ref", t.ID, a.IsRef.GroupID),
				})
			}
		}
	}

	// If the IS was supplied, flag any bindings that reference ids
	// absent from the projection — same mechanism as CrossRef but
	// expressed as lowering diagnostics so the compile pipeline can
	// keep going and the UI can surface it under Suggestions.
	if reg != nil {
		userIDs := map[string]bool{}
		for _, u := range reg.Users {
			userIDs[u.ID] = true
		}
		groupIDs := map[string]bool{}
		for _, g := range reg.Groups {
			groupIDs[g.ID] = true
		}
		for i, t := range wf.Tasks {
			if t.Binding == nil {
				continue
			}
			if t.Binding.AssigneeUserID != "" && !userIDs[t.Binding.AssigneeUserID] {
				diags = append(diags, ir.Diagnostic{
					Severity:   "warning",
					IRRef:      fmt.Sprintf("%s/tasks/%d/binding/assignee_user_id", LoweringSource, i),
					Message:    fmt.Sprintf("assignee_user_id %q is not in the IS Registry projection", t.Binding.AssigneeUserID),
					Suggestion: "Run a fresh engine sync, or pick an existing user.",
				})
			}
			if t.Binding.CandidateGroupID != "" && !groupIDs[t.Binding.CandidateGroupID] {
				diags = append(diags, ir.Diagnostic{
					Severity: "warning",
					IRRef:    fmt.Sprintf("%s/tasks/%d/binding/candidate_group_id", LoweringSource, i),
					Message:  fmt.Sprintf("candidate_group_id %q is not in the IS Registry projection", t.Binding.CandidateGroupID),
				})
			}
		}
	}

	return diags
}

// synthesizeDefaults finds exclusive gateways whose outgoing flows all
// carry conditions, synthesizes one unconditional "default" flow per
// such gateway pointing at a new end event, and reports each as a
// warning (so the user sees what Lower did).
func synthesizeDefaults(wf *ir.ExecutableIR) []ir.Diagnostic {
	// Only exclusive gateways need defaults. Parallel gateways fan
	// out unconditionally.
	exclusive := map[string]bool{}
	for _, g := range wf.Gateways {
		if g.Type == "exclusive" {
			exclusive[g.ID] = true
		}
	}

	// Outgoing flows per node, deterministic order.
	out := map[string][]*ir.Flow{}
	for i := range wf.Flows {
		f := &wf.Flows[i]
		out[f.From] = append(out[f.From], f)
	}
	for id := range out {
		sort.SliceStable(out[id], func(i, j int) bool { return out[id][i].ID < out[id][j].ID })
	}

	var diags []ir.Diagnostic
	for _, g := range wf.Gateways {
		if !exclusive[g.ID] {
			continue
		}
		flows := out[g.ID]
		if len(flows) == 0 {
			// Gateway with no outgoing flows — upstream validation
			// should have caught this, but flag it so compile isn't
			// silent about the dead-end.
			diags = append(diags, ir.Diagnostic{
				Severity: "warning",
				IRRef:    LoweringSource + "/gateways/" + g.ID,
				Message:  fmt.Sprintf("exclusive gateway %q has no outgoing flows", g.ID),
			})
			continue
		}
		hasDefault := false
		for _, f := range flows {
			if f.Condition == nil || f.Condition.Expression == "" {
				hasDefault = true
				break
			}
		}
		if hasDefault {
			continue
		}

		// Synthesize: create a new end event and one unconditional
		// flow from the gateway to it.
		endID := uniqueID(wf, "end_default_"+g.ID)
		flowID := uniqueID(wf, "f_default_"+g.ID)
		wf.Events = append(wf.Events, ir.Event{
			ID:       endID,
			Type:     "end",
			Evidence: "synthesized by Lower — default branch for exclusive gateway " + g.ID,
		})
		wf.Flows = append(wf.Flows, ir.Flow{
			ID:       flowID,
			From:     g.ID,
			To:       endID,
			Evidence: "synthesized by Lower — covers the branch where no condition matches",
		})
		diags = append(diags, ir.Diagnostic{
			Severity: "warning",
			IRRef:    LoweringSource + "/gateways/" + g.ID,
			Message: fmt.Sprintf(
				"synthesized default branch for exclusive gateway %q → new end event %q",
				g.ID, endID,
			),
			Suggestion: "Add an explicit default path if you want the workflow to end elsewhere or take a different action when no condition matches.",
		})
	}
	return diags
}

// normalizeConditions fills in the default language for any flow
// condition that has an expression but no language hint. Camunda 7
// defaults to JUEL so we match that.
func normalizeConditions(wf *ir.ExecutableIR) []ir.Diagnostic {
	var diags []ir.Diagnostic
	for i := range wf.Flows {
		c := wf.Flows[i].Condition
		if c == nil || c.Expression == "" {
			continue
		}
		if c.Language == "" {
			c.Language = "juel"
			diags = append(diags, ir.Diagnostic{
				Severity: "warning",
				IRRef:    fmt.Sprintf("%s/flows/%d/condition/language", LoweringSource, i),
				Message:  fmt.Sprintf("flow %q condition language defaulted to juel", wf.Flows[i].ID),
			})
		}
	}
	return diags
}

// propagateConfidence rewrites each user/service task's own
// confidence to the minimum of its current value and its binding's
// confidence. The UI already does this on the client side but having
// it server-side means any downstream consumer (Copilot, export
// tooling) sees a consistent value.
func propagateConfidence(wf *ir.ExecutableIR) {
	for i := range wf.Tasks {
		t := &wf.Tasks[i]
		if t.Binding == nil || t.Binding.Confidence == nil {
			continue
		}
		bc := *t.Binding.Confidence
		if t.Confidence == nil {
			c := bc
			t.Confidence = &c
			continue
		}
		if bc < *t.Confidence {
			*t.Confidence = bc
		}
	}
}

// cloneWorkflow deep-copies a Workflow so Lower never mutates its
// input. The input lives in a request-scoped handler; we could skip
// the copy for perf, but the determinism guarantee is worth the few
// allocations.
func cloneWorkflow(wf *ir.Workflow) *ir.Workflow {
	out := *wf
	out.Actors = append([]ir.Actor(nil), wf.Actors...)
	out.Tasks = cloneTasks(wf.Tasks)
	out.Gateways = append([]ir.Gateway(nil), wf.Gateways...)
	out.Events = append([]ir.Event(nil), wf.Events...)
	out.Flows = cloneFlows(wf.Flows)
	out.Forms = append([]ir.Form(nil), wf.Forms...)
	return &out
}

func cloneTasks(in []ir.Task) []ir.Task {
	out := make([]ir.Task, len(in))
	for i, t := range in {
		out[i] = t
		if t.Binding != nil {
			b := *t.Binding
			out[i].Binding = &b
		}
		if t.Confidence != nil {
			c := *t.Confidence
			out[i].Confidence = &c
		}
	}
	return out
}

func cloneFlows(in []ir.Flow) []ir.Flow {
	out := make([]ir.Flow, len(in))
	for i, f := range in {
		out[i] = f
		if f.Condition != nil {
			c := *f.Condition
			out[i].Condition = &c
		}
	}
	return out
}

// uniqueID returns base if no existing node holds it, else suffixes
// _2, _3, ... until it finds an unused id. The caller appends to
// wf.Events/Flows after we return.
func uniqueID(wf *ir.Workflow, base string) string {
	used := map[string]bool{}
	for _, e := range wf.Events {
		used[e.ID] = true
	}
	for _, g := range wf.Gateways {
		used[g.ID] = true
	}
	for _, t := range wf.Tasks {
		used[t.ID] = true
	}
	for _, f := range wf.Flows {
		used[f.ID] = true
	}
	if !used[base] {
		return base
	}
	for i := 2; ; i++ {
		cand := fmt.Sprintf("%s_%d", base, i)
		if !used[cand] {
			return cand
		}
	}
}
