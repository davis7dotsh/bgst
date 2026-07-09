package presentation

import (
	"bytes"
	"strings"
	"testing"
	"time"

	gh "github.com/davis7dotsh/bgst/internal/github"
	"github.com/davis7dotsh/bgst/internal/repository"
)

func TestOverviewRendersRepositoryBeforePullRequests(t *testing.T) {
	var output bytes.Buffer
	presenter := New(&output, false, false)
	presenter.Repository(repository.Info{Root: "/repo", Branch: "main"})

	if strings.Contains(output.String(), "Draft PRs") {
		t.Fatal("repository phase rendered pull requests")
	}
	if !strings.Contains(output.String(), "Repository\n") {
		t.Fatal("repository phase did not render repository details")
	}

	presenter.PullRequests([]gh.PullRequest{{Number: 42, Title: "Ready", URL: "https://example.com/42"}}, nil)
	if !strings.Contains(output.String(), "Open PRs (1)") {
		t.Fatal("pull request phase did not render open pull requests")
	}
}

func TestLoadingPullRequestsIsTransientInInteractiveTerminals(t *testing.T) {
	var output bytes.Buffer
	presenter := New(&output, false, true)
	stop := presenter.LoadingPullRequests()
	stop()

	if !strings.Contains(output.String(), "Loading pull requests…") {
		t.Fatal("loading state was not rendered")
	}
	if !strings.HasSuffix(output.String(), "\r\x1b[2K\x1b[1A\r") {
		t.Fatal("loading state was not cleared")
	}
}

func TestLoadingPullRequestsIsHiddenWhenNotInteractive(t *testing.T) {
	var output bytes.Buffer
	presenter := New(&output, false, false)
	presenter.LoadingPullRequests()()
	if output.Len() != 0 {
		t.Fatalf("non-interactive loading output = %q, want none", output.String())
	}
}

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
