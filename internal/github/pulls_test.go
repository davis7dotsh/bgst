package github

import (
	"context"
	"reflect"
	"testing"
)

type fakeRunner struct {
	outputs []string
	calls   [][]string
}

func (r *fakeRunner) Run(_ context.Context, _ string, name string, args ...string) (string, error) {
	call := append([]string{name}, args...)
	r.calls = append(r.calls, call)
	return r.outputs[len(r.calls)-1], nil
}

func TestOpenPullRequestsFetchesFiveMostRecentPerSection(t *testing.T) {
	runner := &fakeRunner{outputs: []string{
		`[
			{"number":105,"isDraft":true},
			{"number":104,"isDraft":true},
			{"number":103,"isDraft":true},
			{"number":102,"isDraft":true},
			{"number":101,"isDraft":true},
			{"number":100,"isDraft":true}
		]`,
		`[
			{"number":205,"isDraft":false},
			{"number":204,"isDraft":false},
			{"number":203,"isDraft":false},
			{"number":202,"isDraft":false},
			{"number":201,"isDraft":false},
			{"number":200,"isDraft":false}
		]`,
	}}
	client := Client{runner: runner}

	pulls, err := client.OpenPullRequests(context.Background(), "/repo", "owner/repo")
	if err != nil {
		t.Fatalf("OpenPullRequests() error = %v", err)
	}

	wantNumbers := []int{105, 104, 103, 102, 101, 205, 204, 203, 202, 201}
	gotNumbers := make([]int, 0, len(pulls))
	for _, pull := range pulls {
		gotNumbers = append(gotNumbers, pull.Number)
	}
	if !reflect.DeepEqual(gotNumbers, wantNumbers) {
		t.Fatalf("pull request numbers = %v, want %v", gotNumbers, wantNumbers)
	}

	wantCalls := [][]string{
		{"gh", "pr", "list", "--repo", "owner/repo", "--state", "open", "--search", "draft:true sort:updated-desc", "--limit", "5", "--json", "number,title,url,isDraft,headRefName,baseRefName,reviewDecision,mergeStateStatus,statusCheckRollup"},
		{"gh", "pr", "list", "--repo", "owner/repo", "--state", "open", "--search", "draft:false sort:updated-desc", "--limit", "5", "--json", "number,title,url,isDraft,headRefName,baseRefName,reviewDecision,mergeStateStatus,statusCheckRollup"},
	}
	if !reflect.DeepEqual(runner.calls, wantCalls) {
		t.Fatalf("runner calls = %#v, want %#v", runner.calls, wantCalls)
	}
}

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
