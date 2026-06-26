// Package bpmn compiles a Workflow IR into BPMN 2.0 XML suitable for
// deployment to Camunda 7. It emits the camunda: extension namespace,
// JUEL expressions for sequence-flow conditions, embedded formData for
// user-task forms, and external-task topics for service tasks.
//
// Determinism: given the same IR and profile, Compile produces
// byte-identical output. This is enforced by a golden-file test and
// relied on by drift detection.
package bpmn

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"sort"
	"strings"

	"github.com/ncs26-orchestration/solution/apps/api/internal/ir"
)

// Profile is the minimal subset of the Camunda 7 engine capability
// profile this compiler reads. Full profile lives in
// packages/ir/engine-profiles/camunda7.json; the compiler only needs
// the fields that affect emission.
type Profile struct {
	ID                 string
	ExpressionLanguage string // "juel" | "feel"
	ServiceTaskModel   string // "external-task"
	StrictMode         bool
}

// DefaultCamunda7Profile mirrors packages/ir/engine-profiles/camunda7.json
// for the fields this compiler cares about.
func DefaultCamunda7Profile() Profile {
	return Profile{
		ID:                 "camunda7",
		ExpressionLanguage: "juel",
		ServiceTaskModel:   "external-task",
		StrictMode:         true,
	}
}

// Diagnostic is re-exported from the ir package shape so callers have
// one type to pattern-match on. We keep the compile-time version here
// to avoid an import cycle if the ir package ever grows a dep on us.
type Diagnostic = ir.Diagnostic

// Compile turns a validated Workflow IR into BPMN 2.0 XML bytes. It
// returns a diagnostics slice alongside the artifact: non-empty
// diagnostics in strict mode mean compilation was refused (err != nil,
// artifact == nil); non-empty diagnostics in lenient mode would carry
// lossy lowering warnings (not implemented in v0.1 — strict only).
func Compile(wf *ir.Workflow, p Profile) ([]byte, []Diagnostic, error) {
	if wf == nil {
		return nil, nil, fmt.Errorf("nil workflow")
	}

	diags := preflight(wf, p)
	if len(diags) > 0 && p.StrictMode {
		return nil, diags, fmt.Errorf("compilation refused: %d diagnostic(s)", len(diags))
	}

	processKey := slugify(wf.Metadata.Name)
	if processKey == "" {
		processKey = "process"
	}

	def := buildDefinitions(wf, processKey, p)

	var buf bytes.Buffer
	buf.WriteString(xml.Header)
	enc := xml.NewEncoder(&buf)
	enc.Indent("", "  ")
	if err := enc.Encode(def); err != nil {
		return nil, diags, fmt.Errorf("xml encode: %w", err)
	}
	if err := enc.Flush(); err != nil {
		return nil, diags, fmt.Errorf("xml flush: %w", err)
	}
	// Deterministic trailing newline.
	out := append(buf.Bytes(), '\n')
	return out, diags, nil
}

// preflight enforces the strict-mode profile check: every IR feature
// used must be declared by the profile. In v0.1 the only knob is
// action types, gateway types, event types. Anything beyond returns a
// diagnostic.
func preflight(wf *ir.Workflow, p Profile) []Diagnostic {
	var diags []Diagnostic
	allowedActions := map[string]bool{"user": true, "service": true, "script": true}
	allowedGateways := map[string]bool{"exclusive": true, "parallel": true}
	allowedEvents := map[string]bool{"start": true, "end": true}

	for i, t := range wf.Tasks {
		if !allowedActions[t.Type] {
			diags = append(diags, Diagnostic{
				Severity: "error",
				IRRef:    fmt.Sprintf("/tasks/%d/type", i),
				Message:  fmt.Sprintf("profile %q does not support task type %q", p.ID, t.Type),
			})
		}
	}
	for i, g := range wf.Gateways {
		if !allowedGateways[g.Type] {
			diags = append(diags, Diagnostic{
				Severity: "error",
				IRRef:    fmt.Sprintf("/gateways/%d/type", i),
				Message:  fmt.Sprintf("profile %q does not support gateway type %q", p.ID, g.Type),
			})
		}
	}
	for i, e := range wf.Events {
		if !allowedEvents[e.Type] {
			diags = append(diags, Diagnostic{
				Severity: "error",
				IRRef:    fmt.Sprintf("/events/%d/type", i),
				Message:  fmt.Sprintf("profile %q does not support event type %q", p.ID, e.Type),
			})
		}
	}
	return diags
}

// BPMN 2.0 XML structs. Names use explicit Marshaler tags to keep
// attribute ordering stable across Go versions.

type definitions struct {
	XMLName         xml.Name     `xml:"bpmn:definitions"`
	XmlnsBpmn       string       `xml:"xmlns:bpmn,attr"`
	XmlnsCamunda    string       `xml:"xmlns:camunda,attr"`
	XmlnsBpmndi     string       `xml:"xmlns:bpmndi,attr"`
	XmlnsDc         string       `xml:"xmlns:dc,attr"`
	XmlnsDi         string       `xml:"xmlns:di,attr"`
	XmlnsXsi        string       `xml:"xmlns:xsi,attr"`
	TargetNamespace string       `xml:"targetNamespace,attr"`
	ID              string       `xml:"id,attr"`
	Process         process      `xml:"bpmn:process"`
	Diagram         *bpmnDiagram `xml:"bpmndi:BPMNDiagram,omitempty"`
}

// BPMN DI (Diagram Interchange) structures. Camunda Cockpit and
// Modeler read these to render the visual — without them the process
// executes fine but Cockpit shows an empty diagram pane. Coordinates
// come from layout.go.
type bpmnDiagram struct {
	ID    string    `xml:"id,attr"`
	Plane bpmnPlane `xml:"bpmndi:BPMNPlane"`
}

type bpmnPlane struct {
	ID          string `xml:"id,attr"`
	BpmnElement string `xml:"bpmnElement,attr"`
	Items       []any  `xml:",any"`
}

type bpmnShape struct {
	XMLName     xml.Name `xml:"bpmndi:BPMNShape"`
	ID          string   `xml:"id,attr"`
	BpmnElement string   `xml:"bpmnElement,attr"`
	Bounds      dcBounds `xml:"dc:Bounds"`
}

type dcBounds struct {
	X      int `xml:"x,attr"`
	Y      int `xml:"y,attr"`
	Width  int `xml:"width,attr"`
	Height int `xml:"height,attr"`
}

type bpmnEdge struct {
	XMLName     xml.Name     `xml:"bpmndi:BPMNEdge"`
	ID          string       `xml:"id,attr"`
	BpmnElement string       `xml:"bpmnElement,attr"`
	Waypoints   []diWaypoint `xml:"di:waypoint"`
	// Label is emitted only for edges that carry a name (conditional
	// flows). Camunda 7 will fall back to a default position if
	// BPMNLabel is omitted, which ends up overlapping the edge itself
	// — providing explicit Bounds gives the condition chip its own
	// breathing room.
	Label *bpmnLabel `xml:"bpmndi:BPMNLabel,omitempty"`
}

type bpmnLabel struct {
	Bounds dcBounds `xml:"dc:Bounds"`
}

type diWaypoint struct {
	X int `xml:"x,attr"`
	Y int `xml:"y,attr"`
}

type process struct {
	ID           string `xml:"id,attr"`
	Name         string `xml:"name,attr"`
	IsExecutable bool   `xml:"isExecutable,attr"`
	// Camunda 7 refuses deployments whose processes do not set
	// historyTimeToLive (ENGINE-12018). Default to 180 days — enough
	// history for audit, short enough that the cleanup runs catch
	// abandoned test deployments.
	HistoryTimeToLive string `xml:"camunda:historyTimeToLive,attr,omitempty"`
	Items             []any  `xml:",any"`
}

type startEvent struct {
	XMLName xml.Name `xml:"bpmn:startEvent"`
	ID      string   `xml:"id,attr"`
	Name    string   `xml:"name,attr,omitempty"`
}

type endEvent struct {
	XMLName xml.Name `xml:"bpmn:endEvent"`
	ID      string   `xml:"id,attr"`
	Name    string   `xml:"name,attr,omitempty"`
}

type userTask struct {
	XMLName         xml.Name            `xml:"bpmn:userTask"`
	ID              string              `xml:"id,attr"`
	Name            string              `xml:"name,attr"`
	Assignee        string              `xml:"camunda:assignee,attr,omitempty"`
	CandidateGroups string              `xml:"camunda:candidateGroups,attr,omitempty"`
	CandidateUsers  string              `xml:"camunda:candidateUsers,attr,omitempty"`
	FormKey         string              `xml:"camunda:formKey,attr,omitempty"`
	Extensions      *userTaskExtensions `xml:"bpmn:extensionElements,omitempty"`
}

type userTaskExtensions struct {
	FormData *formData `xml:"camunda:formData,omitempty"`
}

type formData struct {
	Fields []formField `xml:"camunda:formField"`
}

type formField struct {
	ID         string      `xml:"id,attr"`
	Label      string      `xml:"label,attr"`
	Type       string      `xml:"type,attr"`
	Validation *validation `xml:"camunda:validation,omitempty"`
	Values     []formValue `xml:"camunda:value,omitempty"`
}

type validation struct {
	Constraints []constraint `xml:"camunda:constraint"`
}

type constraint struct {
	Name string `xml:"name,attr"`
}

type formValue struct {
	ID   string `xml:"id,attr"`
	Name string `xml:"name,attr"`
}

type serviceTask struct {
	XMLName xml.Name `xml:"bpmn:serviceTask"`
	ID      string   `xml:"id,attr"`
	Name    string   `xml:"name,attr"`
	Type    string   `xml:"camunda:type,attr,omitempty"`
	Topic   string   `xml:"camunda:topic,attr,omitempty"`
}

type manualTask struct {
	XMLName xml.Name `xml:"bpmn:manualTask"`
	ID      string   `xml:"id,attr"`
	Name    string   `xml:"name,attr"`
}

type scriptTask struct {
	XMLName xml.Name `xml:"bpmn:scriptTask"`
	ID      string   `xml:"id,attr"`
	Name    string   `xml:"name,attr"`
}

type exclusiveGateway struct {
	XMLName xml.Name `xml:"bpmn:exclusiveGateway"`
	ID      string   `xml:"id,attr"`
}

type parallelGateway struct {
	XMLName xml.Name `xml:"bpmn:parallelGateway"`
	ID      string   `xml:"id,attr"`
}

type sequenceFlow struct {
	XMLName xml.Name `xml:"bpmn:sequenceFlow"`
	ID      string   `xml:"id,attr"`
	// Name surfaces as the visible edge label in Camunda Cockpit. We
	// set it to the condition expression so conditional flows show
	// `${amount > 50000}` on the diagram — the runtime evaluation
	// still uses the <conditionExpression> child below, not this
	// attribute.
	Name                string               `xml:"name,attr,omitempty"`
	SourceRef           string               `xml:"sourceRef,attr"`
	TargetRef           string               `xml:"targetRef,attr"`
	ConditionExpression *conditionExpression `xml:"bpmn:conditionExpression,omitempty"`
}

type conditionExpression struct {
	XsiType string `xml:"xsi:type,attr"`
	Body    string `xml:",chardata"`
}

// buildDefinitions assembles the XML document in a deterministic
// order: events first (sorted by id), then gateways (sorted), then
// tasks (sorted), then flows (sorted). This stability matters for
// golden-file tests and drift detection.
func buildDefinitions(wf *ir.Workflow, processKey string, p Profile) definitions {
	proc := process{
		ID:                processKey,
		Name:              wf.Metadata.Name,
		IsExecutable:      true,
		HistoryTimeToLive: "P180D",
	}

	// Events.
	events := make([]ir.Event, len(wf.Events))
	copy(events, wf.Events)
	sort.SliceStable(events, func(i, j int) bool { return events[i].ID < events[j].ID })
	for _, e := range events {
		switch e.Type {
		case "start":
			proc.Items = append(proc.Items, startEvent{ID: e.ID})
		case "end":
			proc.Items = append(proc.Items, endEvent{ID: e.ID})
		}
	}

	// Gateways.
	gateways := make([]ir.Gateway, len(wf.Gateways))
	copy(gateways, wf.Gateways)
	sort.SliceStable(gateways, func(i, j int) bool { return gateways[i].ID < gateways[j].ID })
	for _, g := range gateways {
		switch g.Type {
		case "exclusive":
			proc.Items = append(proc.Items, exclusiveGateway{ID: g.ID})
		case "parallel":
			proc.Items = append(proc.Items, parallelGateway{ID: g.ID})
		}
	}

	// Tasks.
	tasks := make([]ir.Task, len(wf.Tasks))
	copy(tasks, wf.Tasks)
	sort.SliceStable(tasks, func(i, j int) bool { return tasks[i].ID < tasks[j].ID })
	formsByID := map[string]ir.Form{}
	for _, f := range wf.Forms {
		formsByID[f.ID] = f
	}
	for _, t := range tasks {
		switch t.Type {
		case "user":
			proc.Items = append(proc.Items, toUserTask(t, formsByID))
		case "service":
			proc.Items = append(proc.Items, toServiceTask(t, p))
		case "script":
			proc.Items = append(proc.Items, scriptTask{ID: t.ID, Name: t.Name})
		}
	}

	// Flows.
	flows := make([]ir.Flow, len(wf.Flows))
	copy(flows, wf.Flows)
	sort.SliceStable(flows, func(i, j int) bool { return flows[i].ID < flows[j].ID })
	for _, f := range flows {
		sf := sequenceFlow{
			ID:        f.ID,
			SourceRef: f.From,
			TargetRef: f.To,
		}
		if f.Condition != nil && f.Condition.Expression != "" {
			sf.Name = f.Condition.Expression
			sf.ConditionExpression = &conditionExpression{
				XsiType: "bpmn:tFormalExpression",
				Body:    f.Condition.Expression,
			}
		}
		proc.Items = append(proc.Items, sf)
	}

	return definitions{
		XmlnsBpmn:       "http://www.omg.org/spec/BPMN/20100524/MODEL",
		XmlnsCamunda:    "http://camunda.org/schema/1.0/bpmn",
		XmlnsBpmndi:     "http://www.omg.org/spec/BPMN/20100524/DI",
		XmlnsDc:         "http://www.omg.org/spec/DD/20100524/DC",
		XmlnsDi:         "http://www.omg.org/spec/DD/20100524/DI",
		XmlnsXsi:        "http://www.w3.org/2001/XMLSchema-instance",
		TargetNamespace: "http://aup.dev/bpmn",
		ID:              "defs-" + processKey,
		Process:         proc,
		Diagram:         buildDiagram(wf, processKey),
	}
}

// buildDiagram assembles the <bpmndi:BPMNDiagram> section. Camunda
// Cockpit needs this to render anything visually; without it the
// process runs but the diagram pane is empty. Shapes and edges are
// emitted in deterministic id-sorted order to keep golden tests stable.
func buildDiagram(wf *ir.Workflow, processKey string) *bpmnDiagram {
	boxes, edges := layout(wf)

	plane := bpmnPlane{
		ID:          "plane-" + processKey,
		BpmnElement: processKey,
	}

	// Shapes: iterate IR collections in their canonical order (events,
	// gateways, tasks — matching the <process> body order) and then
	// sort within each group by id for determinism.
	type shapeRef struct {
		id string
	}
	var shapeIDs []string
	for _, e := range wf.Events {
		shapeIDs = append(shapeIDs, e.ID)
	}
	for _, g := range wf.Gateways {
		shapeIDs = append(shapeIDs, g.ID)
	}
	for _, t := range wf.Tasks {
		shapeIDs = append(shapeIDs, t.ID)
	}
	sortStrings(shapeIDs)
	for _, id := range shapeIDs {
		b, ok := boxes[id]
		if !ok {
			continue
		}
		plane.Items = append(plane.Items, bpmnShape{
			ID:          id + "_di",
			BpmnElement: id,
			Bounds: dcBounds{
				X:      b.x,
				Y:      b.y,
				Width:  b.w,
				Height: b.h,
			},
		})
	}

	// Edges: layout() already returns them in id-sorted order. Build
	// a lookup so we can decorate conditional edges with their
	// expression as a BPMNLabel.
	flowByID := map[string]ir.Flow{}
	for _, f := range wf.Flows {
		flowByID[f.ID] = f
	}
	for _, e := range edges {
		wps := make([]diWaypoint, 0, len(e.waypoints))
		for _, p := range e.waypoints {
			wps = append(wps, diWaypoint{X: p.X, Y: p.Y})
		}
		be := bpmnEdge{
			ID:          e.id + "_di",
			BpmnElement: e.id,
			Waypoints:   wps,
		}
		// Give conditional flows an explicit label position at the
		// midpoint of their path. Camunda would otherwise pick a
		// default that often sits on top of the edge line.
		if f, ok := flowByID[e.id]; ok && f.Condition != nil && f.Condition.Expression != "" {
			mx, my := midpoint(e.waypoints)
			// Label width sized to the expression length (rough 6px per
			// char) clamped to [60, 220]; height = 14 (one line).
			w := max(60, min(220, len(f.Condition.Expression)*6))
			be.Label = &bpmnLabel{
				Bounds: dcBounds{
					X:      mx - w/2,
					Y:      my - 20,
					Width:  w,
					Height: 14,
				},
			}
		}
		plane.Items = append(plane.Items, be)
	}

	return &bpmnDiagram{
		ID:    "diagram-" + processKey,
		Plane: plane,
	}
}

// sortStrings sorts in place without importing sort twice in the same
// file (the other callsites import sort for ir.Flow sorting); kept
// inline to keep the diff tight.
func sortStrings(s []string) {
	sort.Strings(s)
}

// midpoint returns a sensible label position for the path. For a
// two-waypoint straight line, that's the arithmetic midpoint. For a
// four-waypoint L-shape (emitted when the edge spans multiple ranks),
// we pick the middle of the horizontal "crossbar" so the label lands
// on a straight segment rather than a corner.
func midpoint(wps []diPoint) (int, int) {
	switch len(wps) {
	case 0:
		return 0, 0
	case 1:
		return wps[0].X, wps[0].Y
	case 2:
		return (wps[0].X + wps[1].X) / 2, (wps[0].Y + wps[1].Y) / 2
	default:
		// Pick the horizontal segment with the longest run. L-shapes
		// emitted by the layout have the form [A(sx,sy), B(mx,sy),
		// C(mx,ty), D(tx,ty)]; segments A–B and C–D are horizontal.
		// Use the segment closer to the vertical midline of the path
		// as the label home — that's usually the one with more space.
		a := wps[0]
		b := wps[len(wps)-1]
		return (a.X + b.X) / 2, (a.Y + b.Y) / 2
	}
}

func toUserTask(t ir.Task, forms map[string]ir.Form) userTask {
	ut := userTask{ID: t.ID, Name: t.Name}
	if t.Binding != nil {
		ut.Assignee = t.Binding.AssigneeUserID
		ut.CandidateGroups = t.Binding.CandidateGroupID
		ut.FormKey = t.Binding.FormKey
	}
	if t.FormRef != "" {
		if f, ok := forms[t.FormRef]; ok {
			ut.Extensions = &userTaskExtensions{FormData: formDataFromIR(f)}
		}
	}
	return ut
}

func toServiceTask(t ir.Task, p Profile) any {
	st := serviceTask{ID: t.ID, Name: t.Name}
	if p.ServiceTaskModel == "external-task" && t.Binding != nil && t.Binding.SystemRef != "" && t.Binding.Capability != "" {
		st.Type = "external"
		st.Topic = fmt.Sprintf("%s.%s", t.Binding.SystemRef, t.Binding.Capability)
		return st
	}
	return manualTask{ID: t.ID, Name: t.Name}
}

func formDataFromIR(f ir.Form) *formData {
	fd := &formData{}
	for _, fld := range f.Fields {
		ff := formField{
			ID:    fld.ID,
			Label: fld.Label,
			Type:  mapFormFieldType(fld.Type),
		}
		if fld.Required {
			ff.Validation = &validation{Constraints: []constraint{{Name: "required"}}}
		}
		if fld.Type == "enum" {
			ff.Type = "enum"
			for _, opt := range fld.Options {
				ff.Values = append(ff.Values, formValue{ID: opt, Name: opt})
			}
		}
		fd.Fields = append(fd.Fields, ff)
	}
	return fd
}

// mapFormFieldType translates IR form-field types to Camunda 7's
// generated-form vocabulary (string / long / boolean / date / enum).
func mapFormFieldType(t string) string {
	switch t {
	case "string", "long", "boolean", "date", "enum":
		return t
	default:
		return "string"
	}
}

// slugify turns a free-text name into a safe BPMN process id.
// Deterministic and idempotent.
func slugify(s string) string {
	var b strings.Builder
	prevDash := false
	for _, r := range strings.ToLower(s) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			prevDash = false
		case r == ' ' || r == '-' || r == '_':
			if !prevDash && b.Len() > 0 {
				b.WriteByte('_')
				prevDash = true
			}
		}
	}
	return strings.Trim(b.String(), "_")
}
