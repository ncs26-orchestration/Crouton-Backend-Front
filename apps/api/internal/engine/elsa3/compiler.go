// Package elsa3 compiles an AIOS ExecutableIR into an Elsa 3
// WorkflowDefinition JSON document. The target is importable into
// Elsa Studio and (when a live Elsa server exists) round-trips
// through /elsa/api/workflow-definitions.
//
// This is a compile-only target for the hackathon scope — the
// Deploy method returns engine.ErrNotSupported. The point is to
// prove the plugin-architecture pillar: one IR, many engines.
//
// Mapping (IR → Elsa):
//
//	user task         Elsa.RunTask        (human-readable task name)
//	service task      Elsa.FlowSendHttpRequest (URL templated from system_ref.capability)
//	script task       Elsa.WriteLine      (text = task.name)
//	exclusive gateway Elsa.If / Elsa.Switch  (2 branches → If, 3+ → Switch)
//	parallel gateway  Elsa.Fork
//	start event       skipped             (Elsa starts from the first activity)
//	end event         skipped             (Elsa ends when there are no more activities)
//	flow              connection          (sourcePort "Done" / "Then" / "Else")
//
// The output JSON keys follow the shape documented at
// https://docs.elsaworkflows.io/guides/loading-workflows-from-json
// cross-referenced with the Elsa Studio FlowchartJsonConverter (for
// connection port naming).
package elsa3

import (
	"encoding/json"
	"fmt"
	"maps"
	"sort"

	"github.com/ncs26-orchestration/solution/apps/api/internal/ir"
)

// Compile takes a lowered ExecutableIR and returns Elsa 3
// WorkflowDefinition JSON bytes, plus any diagnostics that describe
// lossy translations (e.g., condition expressions left as-is because
// Elsa uses JS/Liquid, not JUEL).
func Compile(exe *ir.ExecutableIR) ([]byte, []ir.Diagnostic, error) {
	if exe == nil {
		return nil, nil, fmt.Errorf("nil executable IR")
	}

	processKey := slugify(exe.Metadata.Name)
	if processKey == "" {
		processKey = "workflow"
	}

	diags := []ir.Diagnostic{}
	activities, flowToPort, moreDiags := buildActivities(exe)
	diags = append(diags, moreDiags...)
	connections := buildConnections(exe, flowToPort)

	def := map[string]any{
		"id":           processKey + "-v1",
		"definitionId": processKey,
		"name":         exe.Metadata.Name,
		"isLatest":     true,
		"isPublished":  true,
		// Elsa carries a $type discriminator on nested objects so its
		// JSON converter can reinstate the right polymorphic class on
		// import; the top-level definition doesn't need it.
		"root": map[string]any{
			"id":          "Flowchart1",
			"type":        "Elsa.Flowchart",
			"activities":  activities,
			"connections": connections,
		},
	}
	out, err := json.MarshalIndent(def, "", "  ")
	if err != nil {
		return nil, diags, fmt.Errorf("json marshal: %w", err)
	}
	// Trailing newline for determinism + clean diffs.
	out = append(out, '\n')
	return out, diags, nil
}

// buildActivities emits one Elsa activity per IR element (skipping
// events, which Elsa's Flowchart doesn't model explicitly), sorted
// deterministically. The returned map maps flow id -> source port
// name so buildConnections can emit the right True/False/Done port
// for conditional flows out of Elsa.If / Elsa.Switch.
func buildActivities(exe *ir.ExecutableIR) ([]map[string]any, map[string]string, []ir.Diagnostic) {
	var diags []ir.Diagnostic
	out := []map[string]any{}
	flowToPort := map[string]string{}

	// Group outgoing flows by source so we can decide whether a
	// gateway branches 2-way (If) or N-way (Switch) and assign port
	// names consistently.
	outFlows := map[string][]*ir.Flow{}
	for i := range exe.Flows {
		f := &exe.Flows[i]
		outFlows[f.From] = append(outFlows[f.From], f)
	}
	for from := range outFlows {
		sort.SliceStable(outFlows[from], func(i, j int) bool {
			return outFlows[from][i].ID < outFlows[from][j].ID
		})
	}

	// Tasks — sorted by id for determinism.
	tasks := make([]ir.Task, len(exe.Tasks))
	copy(tasks, exe.Tasks)
	sort.SliceStable(tasks, func(i, j int) bool { return tasks[i].ID < tasks[j].ID })
	for _, t := range tasks {
		switch t.Type {
		case "user":
			out = append(out, userTaskActivity(t))
		case "service":
			out = append(out, serviceTaskActivity(t))
		case "script":
			out = append(out, scriptTaskActivity(t))
		default:
			diags = append(diags, ir.Diagnostic{
				Severity: "warning",
				IRRef:    "/tasks/" + t.ID,
				Message:  fmt.Sprintf("elsa3: unsupported task type %q — skipped", t.Type),
			})
		}
	}

	// Gateways — id-sorted.
	gateways := make([]ir.Gateway, len(exe.Gateways))
	copy(gateways, exe.Gateways)
	sort.SliceStable(gateways, func(i, j int) bool { return gateways[i].ID < gateways[j].ID })
	for _, g := range gateways {
		flows := outFlows[g.ID]
		switch g.Type {
		case "exclusive":
			if len(flows) == 2 {
				act, ports := ifActivity(g, flows)
				out = append(out, act)
				maps.Copy(flowToPort, ports)
			} else {
				act, ports, moreDiags := switchActivity(g, flows)
				out = append(out, act)
				maps.Copy(flowToPort, ports)
				diags = append(diags, moreDiags...)
			}
		case "parallel":
			out = append(out, forkActivity(g))
		default:
			diags = append(diags, ir.Diagnostic{
				Severity: "warning",
				IRRef:    "/gateways/" + g.ID,
				Message:  fmt.Sprintf("elsa3: unsupported gateway type %q — skipped", g.Type),
			})
		}
	}

	return out, flowToPort, diags
}

// userTaskActivity maps a human step to Elsa.RunTask because the
// stock elsa-server-and-studio-v3 image does not ship HumanTask /
// UserTask descriptors. RunTask is installed by default and renders
// as a real Studio activity instead of "Not Found".
func userTaskActivity(t ir.Task) map[string]any {
	act := map[string]any{
		"id":       t.ID,
		"type":     "Elsa.RunTask",
		"name":     t.Name,
		"taskName": literalString(t.Name),
	}
	return act
}

// serviceTaskActivity — Elsa.FlowSendHttpRequest is the installed
// flow-friendly HTTP activity in the stock dev image. The URL is
// templated to the capability topic so the UI preview reads cleanly;
// in a real deploy the user wires the actual endpoint.
func serviceTaskActivity(t ir.Task) map[string]any {
	url := ""
	method := "POST"
	if t.Binding != nil && t.Binding.SystemRef != "" && t.Binding.Capability != "" {
		url = fmt.Sprintf("https://aios.dev/connectors/%s/%s", t.Binding.SystemRef, t.Binding.Capability)
	}
	return map[string]any{
		"id":     t.ID,
		"type":   "Elsa.FlowSendHttpRequest",
		"name":   t.Name,
		"url":    literalString(url),
		"method": literalString(method),
	}
}

func scriptTaskActivity(t ir.Task) map[string]any {
	return map[string]any{
		"id":   t.ID,
		"type": "Elsa.WriteLine",
		"name": t.Name,
		"text": literalString(t.Name),
	}
}

// ifActivity — maps a 2-branch exclusive gateway to Elsa.If. The
// flow whose condition resolves to the predicate goes to the True
// port; the other (typically the synthesized default) to False.
// Returns the activity plus a flow-id -> port-name map for the two
// outgoing flows.
func ifActivity(g ir.Gateway, flows []*ir.Flow) (map[string]any, map[string]string) {
	var trueFlow, falseFlow *ir.Flow
	// Default path (no condition) goes to False; the conditional one
	// to True. If both have conditions, first-in-sorted-order → True.
	for _, f := range flows {
		if f.Condition == nil || f.Condition.Expression == "" {
			falseFlow = f
		}
	}
	for _, f := range flows {
		if f == falseFlow {
			continue
		}
		trueFlow = f
		break
	}
	if trueFlow == nil {
		trueFlow = flows[0]
		if len(flows) > 1 {
			falseFlow = flows[1]
		}
	}
	if falseFlow == nil && len(flows) > 1 {
		for _, f := range flows {
			if f != trueFlow {
				falseFlow = f
				break
			}
		}
	}

	act := map[string]any{
		"id":   g.ID,
		"type": "Elsa.If",
	}
	if trueFlow != nil && trueFlow.Condition != nil {
		act["condition"] = jsExpr(trueFlow.Condition.Expression)
	}

	ports := map[string]string{}
	if trueFlow != nil {
		ports[trueFlow.ID] = "Then"
	}
	if falseFlow != nil {
		ports[falseFlow.ID] = "Else"
	}
	return act, ports
}

// switchActivity — maps a 3+-branch exclusive gateway to Elsa.Switch.
// Each branch becomes a case with its condition; the default (if
// present) becomes the Switch's defaultCase port.
func switchActivity(g ir.Gateway, flows []*ir.Flow) (map[string]any, map[string]string, []ir.Diagnostic) {
	diags := []ir.Diagnostic{}
	cases := []map[string]any{}
	ports := map[string]string{}
	for i, f := range flows {
		portName := fmt.Sprintf("Case%d", i+1)
		if f.Condition == nil || f.Condition.Expression == "" {
			portName = "Default"
		} else {
			cases = append(cases, map[string]any{
				"label":     portName,
				"condition": jsExpr(f.Condition.Expression),
			})
		}
		ports[f.ID] = portName
	}
	if len(cases) != len(flows)-1 && len(cases) != len(flows) {
		// Warn — Elsa's Switch needs either all branches as cases or
		// all-but-one with a default. We still emit; the user can
		// tweak inside Studio.
		diags = append(diags, ir.Diagnostic{
			Severity: "warning",
			IRRef:    "/gateways/" + g.ID,
			Message:  "elsa3: switch gateway has mixed conditional/default flows; review in Elsa Studio",
		})
	}
	return map[string]any{
		"id":    g.ID,
		"type":  "Elsa.Switch",
		"cases": cases,
	}, ports, diags
}

func forkActivity(g ir.Gateway) map[string]any {
	return map[string]any{
		"id":   g.ID,
		"type": "Elsa.Fork",
	}
}

// buildConnections emits one connection per IR flow, defaulting the
// port names to "Done" (out) / "In" (in). When flowToPort has a
// custom out port (If/Switch), that wins.
func buildConnections(exe *ir.ExecutableIR, flowToPort map[string]string) []map[string]any {
	// Skip flows whose source or target is an event — events aren't
	// materialized as activities in Elsa's Flowchart.
	eventIDs := map[string]bool{}
	for _, e := range exe.Events {
		eventIDs[e.ID] = true
	}

	flows := make([]ir.Flow, len(exe.Flows))
	copy(flows, exe.Flows)
	sort.SliceStable(flows, func(i, j int) bool { return flows[i].ID < flows[j].ID })

	out := []map[string]any{}
	for _, f := range flows {
		if eventIDs[f.From] || eventIDs[f.To] {
			continue
		}
		sourcePort := "Done"
		if p, ok := flowToPort[f.ID]; ok {
			sourcePort = p
		}
		out = append(out, map[string]any{
			"source":     f.From,
			"target":     f.To,
			"sourcePort": sourcePort,
			"targetPort": "In",
		})
	}
	return out
}

// literalString wraps a plain string value in Elsa's expression
// envelope. Elsa's JSON format uses { typeName, expression: { type,
// value } } so activities and their property values are polymorphic.
func literalString(s string) map[string]any {
	return map[string]any{
		"typeName": "String",
		"expression": map[string]any{
			"type":  "Literal",
			"value": s,
		},
	}
}

// jsExpr wraps a string condition as a JavaScript expression — Elsa's
// default expression language for runtime evaluation. We pass JUEL
// expressions through verbatim; the user typically rewrites them to
// Elsa's syntax in Studio. A diagnostic at the handler level would
// be nicer but isn't required for the compile-only demo path.
func jsExpr(expr string) map[string]any {
	return map[string]any{
		"typeName": "Boolean",
		"expression": map[string]any{
			"type":  "JavaScript",
			"value": expr,
		},
	}
}

// slugify mirrors bpmn/compiler.go's slugify so process keys are
// consistent across targets.
func slugify(s string) string {
	out := []rune{}
	prevDash := false
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			out = append(out, r)
			prevDash = false
		case r >= 'A' && r <= 'Z':
			out = append(out, r+('a'-'A'))
			prevDash = false
		case r == ' ' || r == '-' || r == '_':
			if !prevDash && len(out) > 0 {
				out = append(out, '_')
				prevDash = true
			}
		}
	}
	// trim trailing underscore
	for len(out) > 0 && out[len(out)-1] == '_' {
		out = out[:len(out)-1]
	}
	return string(out)
}
