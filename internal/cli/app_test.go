package cli

import (
	"bytes"
	"context"
	"os/exec"
	"strings"
	"testing"
	"time"

	gh "github.com/davis7dotsh/bgst/internal/github"
)

type blockingPullRequestClient struct {
	started chan struct{}
	release chan struct{}
}

func (c blockingPullRequestClient) OpenPullRequests(_ context.Context, _, _ string) ([]gh.PullRequest, error) {
	close(c.started)
	<-c.release
	return []gh.PullRequest{{Number: 42, Title: "Ready", URL: "https://example.com/42"}}, nil
}

func TestStatusPrintsRepositoryBeforePullRequestsComplete(t *testing.T) {
	repo := t.TempDir()
	runGit(t, repo, "init", "-q", "-b", "main")
	runGit(t, repo, "-c", "user.name=BGST Test", "-c", "user.email=bgst@example.com", "commit", "-q", "--allow-empty", "-m", "initial")
	runGit(t, repo, "remote", "add", "origin", "https://github.com/owner/repo.git")
	t.Chdir(repo)

	started := make(chan struct{})
	release := make(chan struct{})
	var output bytes.Buffer
	app := New(strings.NewReader(""), &output, BuildInfo{})
	app.github = blockingPullRequestClient{started: started, release: release}
	done := make(chan error, 1)
	go func() {
		done <- app.Run(context.Background(), nil)
	}()

	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("pull request request did not start")
	}
	if !strings.Contains(output.String(), "Repository\n") {
		t.Fatal("repository details were not printed before pull requests started")
	}
	if strings.Contains(output.String(), "Open PRs") {
		t.Fatal("pull requests were printed before the request completed")
	}

	close(release)
	if err := <-done; err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if !strings.Contains(output.String(), "Open PRs (1)") {
		t.Fatal("pull requests were not printed after the request completed")
	}
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	command := exec.Command("git", args...)
	command.Dir = dir
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, output)
	}
}
