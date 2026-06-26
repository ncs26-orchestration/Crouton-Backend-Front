package bpmn

import (
	"sort"

	"github.com/ncs26-orchestration/solution/apps/api/internal/ir"
)

// Standard Camunda Modeler node sizes (px). Keeping these fixed means
// shapes in Cockpit look identical to what the Modeler would draw by
// hand, and gives our layout a predictable bounding box.
const (
	eventSize = 36 // start/end events render as 36x36 circles
	gatewayW  = 50 // exclusive/parallel render as 50x50 diamonds
	gatewayH  = 50
	taskW     = 100 // user/service/script render as 100x80 rounded rects
	taskH     = 80
	rankDx    = 180 // horizontal spacing between ranks (left→right flow)
	rankDy    = 110 // vertical spacing between siblings in the same rank
	marginX   = 40
	marginY   = 40
)

// shapeBox holds the pixel bounds of one emitted BPMNShape.
type shapeBox struct {
	id string
	x  int
	y  int
	w  int
	h  int
}

// edgeLine carries the waypoints for one BPMNEdge. Two waypoints are
// a straight line; three or four let us route around side-branch
// nodes when the direct line would pass through unrelated shapes.
type edgeLine struct {
	id        string
	sourceID  string
	targetID  string
	waypoints []diPoint
}

type diPoint struct {
	X int
	Y int
}

// layout assigns (x, y, w, h) to every BPMN element and computes
// straight waypoints between them. Algorithm is a simple BFS from
// start events to establish ranks, then siblings within a rank
// vertically spaced around the midline.
//
// Deterministic: inputs come from a sorted iteration, so the same IR
// always produces the same coordinates. Golden tests depend on this.
func layout(wf *ir.Workflow) (map[string]shapeBox, []edgeLine) {
	// Index every element by id, and record its size.
	type node struct {
		id string
		w  int
		h  int
	}
	nodes := map[string]node{}
	for _, e := range wf.Events {
		nodes[e.ID] = node{e.ID, eventSize, eventSize}
	}
	for _, g := range wf.Gateways {
		nodes[g.ID] = node{g.ID, gatewayW, gatewayH}
	}
	for _, t := range wf.Tasks {
		nodes[t.ID] = node{t.ID, taskW, taskH}
	}

	// Forward adjacency. Flow order already deterministic on IR ingest.
	out := map[string][]string{}
	in := map[string][]string{}
	for _, f := range wf.Flows {
		out[f.From] = append(out[f.From], f.To)
		in[f.To] = append(in[f.To], f.From)
	}

	// Rank assignment: nodes with no incoming flow start at rank 0.
	// All other nodes: max(rank of predecessors) + 1. Computed in a
	// topological pass so cycles don't hang us (cycles in BPMN
	// workflows are rare but possible — retries, escalations).
	rank := map[string]int{}
	var sources []string
	for id := range nodes {
		if len(in[id]) == 0 {
			sources = append(sources, id)
		}
	}
	sort.Strings(sources)
	queue := append([]string(nil), sources...)
	for _, id := range sources {
		rank[id] = 0
	}
	// Iterative relaxation: bounded by number of edges so cycles
	// terminate.
	maxIter := len(wf.Flows) + len(nodes) + 1
	for iter := 0; iter < maxIter && len(queue) > 0; iter++ {
		next := []string{}
		for _, id := range queue {
			r := rank[id]
			succs := append([]string(nil), out[id]...)
			sort.Strings(succs)
			for _, s := range succs {
				candidate := r + 1
				if cur, seen := rank[s]; !seen || candidate > cur {
					rank[s] = candidate
					next = append(next, s)
				}
			}
		}
		queue = next
	}
	// Any node still without a rank (fully disconnected) lands at 0.
	for id := range nodes {
		if _, ok := rank[id]; !ok {
			rank[id] = 0
		}
	}

	// Group by rank, sorted for determinism.
	byRank := map[int][]string{}
	maxRank := 0
	for id, r := range rank {
		byRank[r] = append(byRank[r], id)
		if r > maxRank {
			maxRank = r
		}
	}
	for r := range byRank {
		sort.Strings(byRank[r])
	}

	// Place each node. Vertical midline is marginY + (tallestColHeight/2);
	// for simplicity we pre-compute a column height per rank and
	// center siblings around a shared midline so flows look natural.
	colH := map[int]int{}
	for r, ids := range byRank {
		h := 0
		for _, id := range ids {
			h += nodes[id].h
		}
		h += (len(ids) - 1) * (rankDy - taskH)
		if h < taskH {
			h = taskH
		}
		colH[r] = h
	}
	maxColH := 0
	for _, h := range colH {
		if h > maxColH {
			maxColH = h
		}
	}
	midY := marginY + maxColH/2

	// Side-branch detection: a node is a side branch when an edge
	// skips its rank — i.e. some flow (A,B) exists with rank(A) <
	// rank(N) < rank(B). That skipping edge, if drawn straight,
	// passes through N's default position, which is what caused the
	// overlap reported by the user (gateway → archive crossing through
	// "approbation administrateur").
	sideBranch := map[string]bool{}
	for _, f := range wf.Flows {
		rFrom, rTo := rank[f.From], rank[f.To]
		if rTo-rFrom <= 1 {
			continue
		}
		for r := rFrom + 1; r < rTo; r++ {
			for _, id := range byRank[r] {
				sideBranch[id] = true
			}
		}
	}

	// Vertical offset for side branches, in pixels. Large enough to
	// clear the 80-px task height plus a gap, so a straight line at
	// midline can pass over without touching the offset shape.
	const sideBranchOffset = 120

	boxes := map[string]shapeBox{}
	for r := 0; r <= maxRank; r++ {
		ids := byRank[r]
		if len(ids) == 0 {
			continue
		}
		// Total height of this column, then vertically centered on midY.
		totalH := 0
		for _, id := range ids {
			totalH += nodes[id].h
		}
		totalH += (len(ids) - 1) * (rankDy - taskH)
		top := midY - totalH/2
		x := marginX + r*rankDx
		cursor := top
		for _, id := range ids {
			n := nodes[id]
			// Align smaller shapes (events, gateways) on the node's center
			// to the row center so edges don't kink.
			rowCenter := cursor + taskH/2
			by := rowCenter - n.h/2
			bx := x + (taskW-n.w)/2
			if sideBranch[id] {
				by += sideBranchOffset
			}
			boxes[id] = shapeBox{id: id, x: bx, y: by, w: n.w, h: n.h}
			cursor += n.h + (rankDy - taskH)
		}
	}

	// Edges: deterministic, id-sorted. For flows that skip one or
	// more ranks we emit an L-shaped 4-waypoint path so the line goes
	// around any side-branch shapes sitting in the skipped columns,
	// rather than through them. Same-rank-jump flows stay straight —
	// the source and target are always at the same Y there.
	flows := make([]ir.Flow, len(wf.Flows))
	copy(flows, wf.Flows)
	sort.SliceStable(flows, func(i, j int) bool { return flows[i].ID < flows[j].ID })

	edges := make([]edgeLine, 0, len(flows))
	for _, f := range flows {
		src, okS := boxes[f.From]
		tgt, okT := boxes[f.To]
		if !okS || !okT {
			continue
		}
		sx := src.x + src.w
		sy := src.y + src.h/2
		tx := tgt.x
		ty := tgt.y + tgt.h/2

		var wps []diPoint
		// When source and target sit on the same Y, a straight line is
		// fine regardless of rank distance — no side branch can sit on
		// the same Y because we've already offset them.
		if sy == ty {
			wps = []diPoint{{X: sx, Y: sy}, {X: tx, Y: ty}}
		} else {
			// Two-bend L-shape: leave source horizontally, drop or rise
			// at the midpoint between ranks, enter target horizontally.
			// This renders cleanly in Camunda and matches the visual
			// convention used by Camunda Modeler's auto-layout.
			midX := (sx + tx) / 2
			wps = []diPoint{
				{X: sx, Y: sy},
				{X: midX, Y: sy},
				{X: midX, Y: ty},
				{X: tx, Y: ty},
			}
		}
		edges = append(edges, edgeLine{
			id:        f.ID,
			sourceID:  f.From,
			targetID:  f.To,
			waypoints: wps,
		})
	}
	return boxes, edges
}
