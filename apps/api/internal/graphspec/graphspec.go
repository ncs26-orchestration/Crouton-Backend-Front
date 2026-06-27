// Package graphspec validates a workflow graph (the node/edge shape every
// executable graph materializes through). It is the single source of truth for
// "is this a runnable graph", reused by workflow-definition create/update and by
// editing a draft request's graph.
package graphspec

import "fmt"

// Node is one stage in a graph: a department's agent step.
type Node struct {
	Key        string `json:"key"`
	Name       string `json:"name"`
	AgentType  string `json:"agent_type"`
	Department string `json:"department"`
}

// Edge connects two stages by their keys.
type Edge struct {
	From string `json:"from"`
	To   string `json:"to"`
	Type string `json:"type"`
}

// Validate checks that nodes/edges form a runnable DAG: at least one node, keys
// are present and unique, every node names an agent type and department, every
// edge references real keys, no self-loops, and the graph is acyclic.
func Validate(nodes []Node, edges []Edge) error {
	if len(nodes) == 0 {
		return fmt.Errorf("a workflow needs at least one step")
	}
	keys := make(map[string]bool, len(nodes))
	for _, n := range nodes {
		if n.Key == "" {
			return fmt.Errorf("every step needs a key")
		}
		if keys[n.Key] {
			return fmt.Errorf("duplicate step key %q", n.Key)
		}
		keys[n.Key] = true
		if n.Name == "" {
			return fmt.Errorf("step %q needs a name", n.Key)
		}
		if n.AgentType == "" || n.Department == "" {
			return fmt.Errorf("step %q needs an agent type and department", n.Key)
		}
	}

	// adjacency for cycle detection
	adj := make(map[string][]string, len(nodes))
	for _, e := range edges {
		if !keys[e.From] || !keys[e.To] {
			return fmt.Errorf("an edge references an unknown step (%s -> %s)", e.From, e.To)
		}
		if e.From == e.To {
			return fmt.Errorf("step %q cannot depend on itself", e.From)
		}
		adj[e.From] = append(adj[e.From], e.To)
	}
	if hasCycle(keys, adj) {
		return fmt.Errorf("the steps form a cycle; a workflow must be acyclic")
	}
	return nil
}

// hasCycle runs a DFS three-color cycle check over the adjacency map.
func hasCycle(keys map[string]bool, adj map[string][]string) bool {
	const (
		white = 0
		gray  = 1
		black = 2
	)
	color := make(map[string]int, len(keys))
	var visit func(string) bool
	visit = func(k string) bool {
		color[k] = gray
		for _, next := range adj[k] {
			switch color[next] {
			case gray:
				return true
			case white:
				if visit(next) {
					return true
				}
			}
		}
		color[k] = black
		return false
	}
	for k := range keys {
		if color[k] == white {
			if visit(k) {
				return true
			}
		}
	}
	return false
}
