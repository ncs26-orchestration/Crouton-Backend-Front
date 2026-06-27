package graphspec

import "testing"

func TestValidate(t *testing.T) {
	good := []Node{
		{Key: "a", Name: "A", AgentType: "hr", Department: "HR"},
		{Key: "b", Name: "B", AgentType: "finance", Department: "Finance"},
	}
	goodEdges := []Edge{{From: "a", To: "b", Type: "sequence"}}

	tests := []struct {
		name    string
		nodes   []Node
		edges   []Edge
		wantErr bool
	}{
		{"valid", good, goodEdges, false},
		{"no nodes", nil, nil, true},
		{"missing key", []Node{{Name: "A", AgentType: "hr", Department: "HR"}}, nil, true},
		{"duplicate key", []Node{good[0], good[0]}, nil, true},
		{"missing agent", []Node{{Key: "a", Name: "A"}}, nil, true},
		{"edge to unknown", good, []Edge{{From: "a", To: "z"}}, true},
		{"self loop", good, []Edge{{From: "a", To: "a"}}, true},
		{"cycle", good, []Edge{{From: "a", To: "b"}, {From: "b", To: "a"}}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Validate(tt.nodes, tt.edges)
			if (err != nil) != tt.wantErr {
				t.Fatalf("Validate() err = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
