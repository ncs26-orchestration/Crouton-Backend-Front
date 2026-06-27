package handler

import "testing"

// TestAgentLiveStatus pins the precedence of the derived roster status: a
// blocked node wins over an in_progress one (the agent is genuinely waiting on
// another department, not just working), and no active work is idle.
func TestAgentLiveStatus(t *testing.T) {
	tests := []struct {
		name    string
		active  int
		blocked int
		want    string
	}{
		{"no activity", 0, 0, "idle"},
		{"one in progress", 1, 0, "busy"},
		{"only blocked", 0, 1, "blocked"},
		{"blocked wins over active", 2, 1, "blocked"},
		{"many active", 5, 0, "busy"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := agentLiveStatus(tt.active, tt.blocked); got != tt.want {
				t.Errorf("agentLiveStatus(active=%d, blocked=%d) = %q, want %q", tt.active, tt.blocked, got, tt.want)
			}
		})
	}
}
