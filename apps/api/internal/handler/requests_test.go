package handler

import (
	"strings"
	"testing"
)

func TestValidateRequestInput(t *testing.T) {
	longTitle := strings.Repeat("a", maxTitleLen+1)
	longDesc := strings.Repeat("a", maxDescriptionLen+1)

	tests := []struct {
		name         string
		title        string
		description  string
		priority     string
		wantTitle    string
		wantPriority string
		wantErr      string
	}{
		{
			name:         "valid high priority",
			title:        "Open a new office in Berlin",
			description:  "Expand into the EU market",
			priority:     "high",
			wantTitle:    "Open a new office in Berlin",
			wantPriority: "high",
		},
		{
			name:         "empty priority defaults to medium",
			title:        "Hire a contractor",
			priority:     "",
			wantTitle:    "Hire a contractor",
			wantPriority: "medium",
		},
		{
			name:         "title is trimmed",
			title:        "   padded title   ",
			priority:     "low",
			wantTitle:    "padded title",
			wantPriority: "low",
		},
		{
			name:     "missing title",
			title:    "",
			priority: "high",
			wantErr:  "title is required",
		},
		{
			name:     "whitespace-only title",
			title:    "    ",
			priority: "high",
			wantErr:  "title is required",
		},
		{
			name:     "invalid priority",
			title:    "Something",
			priority: "sky-high",
			wantErr:  "priority must be low, medium, high, or urgent",
		},
		{
			name:     "title too long",
			title:    longTitle,
			priority: "medium",
			wantErr:  "title must be at most 200 characters",
		},
		{
			name:        "description too long",
			title:       "Fine title",
			description: longDesc,
			priority:    "medium",
			wantErr:     "description must be at most 5000 characters",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotTitle, gotPriority, gotErr := validateRequestInput(tt.title, tt.description, tt.priority)
			if gotErr != tt.wantErr {
				t.Fatalf("errMsg = %q, want %q", gotErr, tt.wantErr)
			}
			if tt.wantErr != "" {
				return
			}
			if gotTitle != tt.wantTitle {
				t.Errorf("title = %q, want %q", gotTitle, tt.wantTitle)
			}
			if gotPriority != tt.wantPriority {
				t.Errorf("priority = %q, want %q", gotPriority, tt.wantPriority)
			}
		})
	}
}

func TestValidateApprovalInput(t *testing.T) {
	longJustification := strings.Repeat("a", maxJustificationLen+1)

	tests := []struct {
		name              string
		decision          string
		justification     string
		wantJustification string
		wantErr           string
	}{
		{
			name:              "approve with reason",
			decision:          "approve",
			justification:     "Budget and risk are acceptable.",
			wantJustification: "Budget and risk are acceptable.",
		},
		{
			name:              "reject with reason",
			decision:          "reject",
			justification:     "Out of budget this quarter.",
			wantJustification: "Out of budget this quarter.",
		},
		{
			name:              "justification is trimmed",
			decision:          "approve",
			justification:     "  looks good  ",
			wantJustification: "looks good",
		},
		{
			name:          "invalid decision",
			decision:      "maybe",
			justification: "whatever",
			wantErr:       "decision must be approve or reject",
		},
		{
			name:          "missing justification",
			decision:      "approve",
			justification: "",
			wantErr:       "justification is required",
		},
		{
			name:          "whitespace-only justification",
			decision:      "reject",
			justification: "   ",
			wantErr:       "justification is required",
		},
		{
			name:          "justification too long",
			decision:      "approve",
			justification: longJustification,
			wantErr:       "justification must be at most 2000 characters",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, gotErr := validateApprovalInput(tt.decision, tt.justification)
			if gotErr != tt.wantErr {
				t.Fatalf("errMsg = %q, want %q", gotErr, tt.wantErr)
			}
			if tt.wantErr != "" {
				return
			}
			if got != tt.wantJustification {
				t.Errorf("justification = %q, want %q", got, tt.wantJustification)
			}
		})
	}
}
