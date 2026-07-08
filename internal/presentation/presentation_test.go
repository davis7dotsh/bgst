package presentation

import (
	"testing"
	"time"
)

func TestRelativeTime(t *testing.T) {
	now := time.Date(2026, time.July, 8, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		name string
		then time.Time
		want string
	}{
		{name: "now", then: now.Add(-10 * time.Second), want: "just now"},
		{name: "minutes", then: now.Add(-3 * time.Minute), want: "3 minutes ago"},
		{name: "one hour", then: now.Add(-time.Hour), want: "1 hour ago"},
		{name: "days", then: now.Add(-72 * time.Hour), want: "3 days ago"},
		{name: "future", then: now.Add(time.Minute), want: "in the future"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := RelativeTime(test.then, now); got != test.want {
				t.Fatalf("RelativeTime() = %q, want %q", got, test.want)
			}
		})
	}
}
