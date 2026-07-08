package github

import "testing"

func TestPullRequestCheckStatus(t *testing.T) {
	tests := []struct {
		name   string
		checks []Check
		want   CheckStatus
	}{
		{name: "none", want: ChecksNone},
		{name: "passing check run", checks: []Check{{Type: "CheckRun", Status: "COMPLETED", Conclusion: "SUCCESS"}}, want: ChecksPassing},
		{name: "neutral is passing", checks: []Check{{Type: "CheckRun", Status: "COMPLETED", Conclusion: "NEUTRAL"}}, want: ChecksPassing},
		{name: "pending check run", checks: []Check{{Type: "CheckRun", Status: "IN_PROGRESS"}}, want: ChecksPending},
		{name: "passing status context", checks: []Check{{Type: "StatusContext", State: "SUCCESS"}}, want: ChecksPassing},
		{name: "pending status context", checks: []Check{{Type: "StatusContext", State: "PENDING"}}, want: ChecksPending},
		{name: "one failure wins", checks: []Check{{Type: "CheckRun", Status: "IN_PROGRESS"}, {Type: "StatusContext", State: "FAILURE"}}, want: ChecksFailing},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			pull := PullRequest{Checks: test.checks}
			if got := pull.CheckStatus(); got != test.want {
				t.Fatalf("CheckStatus() = %v, want %v", got, test.want)
			}
		})
	}
}
