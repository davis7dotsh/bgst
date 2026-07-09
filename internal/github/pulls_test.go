package github

import (
	"context"
	"errors"
	"reflect"
	"sync"
	"testing"
	"time"
)

type fakeRunner struct {
	mu      sync.Mutex
	outputs map[string]string
	calls   [][]string
}

type blockingRunner struct {
	started chan struct{}
	release chan struct{}
}

type cancelingRunner struct {
	siblingStarted  chan struct{}
	siblingCanceled chan struct{}
}

func (r blockingRunner) Run(_ context.Context, _ string, _ string, _ ...string) (string, error) {
	r.started <- struct{}{}
	<-r.release
	return "[]", nil
}

func (r cancelingRunner) Run(ctx context.Context, _ string, _ string, args ...string) (string, error) {
	if argumentValue(args, "--search") == "draft:false sort:updated-desc" {
		close(r.siblingStarted)
		<-ctx.Done()
		close(r.siblingCanceled)
		return "", ctx.Err()
	}
	<-r.siblingStarted
	return "", errors.New("query failed")
}

func (r *fakeRunner) Run(_ context.Context, _ string, name string, args ...string) (string, error) {
	call := append([]string{name}, args...)
	search := argumentValue(args, "--search")
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls = append(r.calls, call)
	return r.outputs[search], nil
}

func TestOpenPullRequestsFetchesFiveMostRecentPerSection(t *testing.T) {
	runner := &fakeRunner{outputs: map[string]string{
		"draft:true sort:updated-desc": `[
			{"number":105,"isDraft":true},
			{"number":104,"isDraft":true},
			{"number":103,"isDraft":true},
			{"number":102,"isDraft":true},
			{"number":101,"isDraft":true},
			{"number":100,"isDraft":true}
		]`,
		"draft:false sort:updated-desc": `[
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

	wantCalls := map[string][]string{
		"draft:true sort:updated-desc":  {"gh", "pr", "list", "--repo", "owner/repo", "--state", "open", "--search", "draft:true sort:updated-desc", "--limit", "5", "--json", "number,title,url,isDraft,headRefName,baseRefName,reviewDecision,mergeStateStatus,statusCheckRollup"},
		"draft:false sort:updated-desc": {"gh", "pr", "list", "--repo", "owner/repo", "--state", "open", "--search", "draft:false sort:updated-desc", "--limit", "5", "--json", "number,title,url,isDraft,headRefName,baseRefName,reviewDecision,mergeStateStatus,statusCheckRollup"},
	}
	if len(runner.calls) != len(wantCalls) {
		t.Fatalf("runner made %d calls, want %d", len(runner.calls), len(wantCalls))
	}
	for _, call := range runner.calls {
		search := argumentValue(call, "--search")
		if !reflect.DeepEqual(call, wantCalls[search]) {
			t.Fatalf("runner call = %#v, want %#v", call, wantCalls[search])
		}
	}
}

func TestOpenPullRequestsFetchesSectionsConcurrently(t *testing.T) {
	runner := blockingRunner{
		started: make(chan struct{}, 2),
		release: make(chan struct{}),
	}
	client := Client{runner: runner}
	done := make(chan error, 1)
	go func() {
		_, err := client.OpenPullRequests(context.Background(), "/repo", "owner/repo")
		done <- err
	}()

	for range 2 {
		select {
		case <-runner.started:
		case <-time.After(time.Second):
			close(runner.release)
			t.Fatal("pull request queries did not start concurrently")
		}
	}
	close(runner.release)
	if err := <-done; err != nil {
		t.Fatalf("OpenPullRequests() error = %v", err)
	}
}

func TestOpenPullRequestsCancelsSiblingQueryAfterFailure(t *testing.T) {
	runner := cancelingRunner{
		siblingStarted:  make(chan struct{}),
		siblingCanceled: make(chan struct{}),
	}
	client := Client{runner: runner}
	if _, err := client.OpenPullRequests(context.Background(), "/repo", "owner/repo"); err == nil {
		t.Fatal("OpenPullRequests() error = nil, want query failure")
	}
	select {
	case <-runner.siblingCanceled:
	default:
		t.Fatal("failed query did not cancel its sibling")
	}
}

func argumentValue(args []string, name string) string {
	for index := 0; index < len(args)-1; index++ {
		if args[index] == name {
			return args[index+1]
		}
	}
	return ""
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
