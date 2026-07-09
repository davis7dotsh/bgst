package github

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/davis7dotsh/bgst/internal/process"
)

type Check struct {
	Type       string `json:"__typename"`
	Conclusion string `json:"conclusion"`
	Status     string `json:"status"`
	State      string `json:"state"`
}

type PullRequest struct {
	Number           int     `json:"number"`
	Title            string  `json:"title"`
	URL              string  `json:"url"`
	IsDraft          bool    `json:"isDraft"`
	HeadRefName      string  `json:"headRefName"`
	BaseRefName      string  `json:"baseRefName"`
	ReviewDecision   string  `json:"reviewDecision"`
	MergeStateStatus string  `json:"mergeStateStatus"`
	Checks           []Check `json:"statusCheckRollup"`
}

type commandRunner interface {
	Run(ctx context.Context, dir, name string, args ...string) (string, error)
}

type Client struct {
	runner commandRunner
}

const pullRequestLimit = 5

type pullRequestResult struct {
	index int
	pulls []PullRequest
	err   error
}

func New() Client {
	return Client{runner: process.Runner{}}
}

func (c Client) OpenPullRequests(ctx context.Context, dir, repo string) ([]PullRequest, error) {
	filters := []string{"draft:true", "draft:false"}
	results := make([]pullRequestResult, len(filters))
	responses := make(chan pullRequestResult, len(filters))
	queryContext, cancel := context.WithCancel(ctx)
	defer cancel()
	for index, filter := range filters {
		go func(index int, filter string) {
			pulls, err := c.pullRequests(queryContext, dir, repo, filter)
			responses <- pullRequestResult{index: index, pulls: pulls, err: err}
		}(index, filter)
	}

	var firstError error
	for range filters {
		result := <-responses
		results[result.index] = result
		if result.err != nil && firstError == nil {
			firstError = result.err
			cancel()
		}
	}
	if firstError != nil {
		return nil, firstError
	}
	return append(results[0].pulls, results[1].pulls...), nil
}

func (c Client) pullRequests(ctx context.Context, dir, repo, draftFilter string) ([]PullRequest, error) {
	output, err := c.runner.Run(
		ctx,
		dir,
		"gh",
		"pr", "list",
		"--repo", repo,
		"--state", "open",
		"--search", draftFilter+" sort:updated-desc",
		"--limit", strconv.Itoa(pullRequestLimit),
		"--json", "number,title,url,isDraft,headRefName,baseRefName,reviewDecision,mergeStateStatus,statusCheckRollup",
	)
	if err != nil {
		if strings.Contains(err.Error(), "executable file not found") {
			return nil, fmt.Errorf("GitHub CLI is not installed (install gh and run gh auth login)")
		}
		return nil, fmt.Errorf("could not load pull requests: %w", err)
	}

	var pulls []PullRequest
	if err := json.Unmarshal([]byte(output), &pulls); err != nil {
		return nil, fmt.Errorf("could not understand GitHub CLI output: %w", err)
	}
	if len(pulls) > pullRequestLimit {
		pulls = pulls[:pullRequestLimit]
	}
	return pulls, nil
}

type CheckStatus int

const (
	ChecksNone CheckStatus = iota
	ChecksPassing
	ChecksPending
	ChecksFailing
)

func (p PullRequest) CheckStatus() CheckStatus {
	if len(p.Checks) == 0 {
		return ChecksNone
	}
	status := ChecksPassing
	for _, check := range p.Checks {
		if checkFailed(check) {
			return ChecksFailing
		}
		if checkPending(check) {
			status = ChecksPending
		}
	}
	return status
}

func checkFailed(check Check) bool {
	value := strings.ToUpper(check.Conclusion)
	if check.Type == "StatusContext" {
		value = strings.ToUpper(check.State)
	}
	return value == "FAILURE" || value == "ERROR" || value == "CANCELLED" || value == "TIMED_OUT" || value == "ACTION_REQUIRED" || value == "STARTUP_FAILURE"
}

func checkPending(check Check) bool {
	if check.Type == "StatusContext" {
		state := strings.ToUpper(check.State)
		return state == "PENDING" || state == "EXPECTED" || state == ""
	}
	return strings.ToUpper(check.Status) != "COMPLETED"
}
