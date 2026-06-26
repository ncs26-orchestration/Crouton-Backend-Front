package handler

import (
	"testing"
	"time"
)

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		name     string
		duration time.Duration
		want     string
	}{
		{
			name:     "zero",
			duration: 0,
			want:     "0s",
		},
		{
			name:     "seconds only",
			duration: 34 * time.Second,
			want:     "34s",
		},
		{
			name:     "minutes and seconds",
			duration: 2*time.Minute + 34*time.Second,
			want:     "2m 34s",
		},
		{
			name:     "hours only",
			duration: 3 * time.Hour,
			want:     "3h",
		},
		{
			name:     "hours and minutes",
			duration: 1*time.Hour + 5*time.Minute,
			want:     "1h 5m",
		},
		{
			name:     "rounds to nearest second",
			duration: 1*time.Minute + 34*time.Second + 500*time.Millisecond,
			want:     "1m 35s",
		},
		{
			name:     "large duration",
			duration: 2*time.Hour + 15*time.Minute + 30*time.Second,
			want:     "2h 15m",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatDuration(tt.duration)
			if got != tt.want {
				t.Errorf("formatDuration(%v) = %q, want %q", tt.duration, got, tt.want)
			}
		})
	}
}
