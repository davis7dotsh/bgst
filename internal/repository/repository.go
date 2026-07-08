package repository

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/davis7dotsh/bgst/internal/process"
)

type Remote struct {
	Name      string
	RawURL    string
	GitHubURL string
	Owner     string
	Repo      string
}

type Commit struct {
	Hash    string
	Short   string
	Time    time.Time
	Author  string
	Subject string
}

type Worktree struct {
	Staged    int
	Unstaged  int
	Untracked int
	Entries   []string
}

func (w Worktree) Clean() bool {
	return w.Staged == 0 && w.Unstaged == 0 && w.Untracked == 0
}

type Info struct {
	Root          string
	Branch        string
	Detached      bool
	Remote        *Remote
	DefaultBranch string
	Upstream      string
	Ahead         int
	Behind        int
	HasTracking   bool
	LastCommit    *Commit
	Worktree      Worktree
}

type Service struct {
	runner process.Runner
}

func New() Service {
	return Service{runner: process.Runner{}}
}

func (s Service) Inspect(ctx context.Context, dir string) (Info, error) {
	root, err := s.git(ctx, dir, "rev-parse", "--show-toplevel")
	if err != nil {
		return Info{}, errors.New("not inside a Git repository")
	}

	info := Info{Root: root}
	info.Branch, info.Detached = s.branch(ctx, root)
	info.Remote = s.remote(ctx, root)
	info.Worktree, err = s.worktree(ctx, root)
	if err != nil {
		return Info{}, err
	}
	info.LastCommit = s.lastCommit(ctx, root)

	if info.Remote != nil {
		info.DefaultBranch = s.defaultBranch(ctx, root, info.Remote.Name)
		info.Upstream = s.upstream(ctx, root, info.Branch, info.Remote.Name)
		if info.Upstream != "" && info.LastCommit != nil {
			info.Ahead, info.Behind, info.HasTracking = s.aheadBehind(ctx, root, info.Upstream)
		}
	}

	return info, nil
}

func (s Service) Fetch(ctx context.Context, dir string) error {
	root, err := s.git(ctx, dir, "rev-parse", "--show-toplevel")
	if err != nil {
		return errors.New("not inside a Git repository")
	}
	_, err = s.git(ctx, root, "fetch", "--all", "--prune", "--tags")
	return err
}

func (s Service) AddAll(ctx context.Context, root string) error {
	_, err := s.git(ctx, root, "add", "--all")
	return err
}

func (s Service) Commit(ctx context.Context, root, message string) (string, error) {
	if _, err := s.git(ctx, root, "commit", "-m", message); err != nil {
		return "", err
	}
	return s.git(ctx, root, "rev-parse", "--short", "HEAD")
}

func (s Service) PushToDefault(ctx context.Context, root, remote, branch string) error {
	_, err := s.git(ctx, root, "push", remote, "HEAD:refs/heads/"+branch)
	return err
}

func (s Service) git(ctx context.Context, dir string, args ...string) (string, error) {
	return s.runner.Run(ctx, dir, "git", args...)
}

func (s Service) branch(ctx context.Context, root string) (string, bool) {
	branch, err := s.git(ctx, root, "symbolic-ref", "--quiet", "--short", "HEAD")
	if err == nil {
		return branch, false
	}
	short, err := s.git(ctx, root, "rev-parse", "--short", "HEAD")
	if err == nil {
		return short, true
	}
	return "unborn", false
}

func (s Service) remote(ctx context.Context, root string) *Remote {
	namesText, err := s.git(ctx, root, "remote")
	if err != nil || namesText == "" {
		return nil
	}
	names := strings.Fields(namesText)
	name := names[0]
	for _, candidate := range []string{"origin", "upstream"} {
		for _, existing := range names {
			if existing == candidate {
				name = candidate
				break
			}
		}
		if name == candidate {
			break
		}
	}

	raw, err := s.git(ctx, root, "remote", "get-url", name)
	if err != nil {
		return nil
	}
	githubURL, owner, repo, _ := ParseGitHubRemote(raw)
	return &Remote{Name: name, RawURL: raw, GitHubURL: githubURL, Owner: owner, Repo: repo}
}

func ParseGitHubRemote(raw string) (webURL, owner, repo string, ok bool) {
	value := strings.TrimSpace(raw)
	path := ""

	if strings.HasPrefix(value, "git@github.com:") {
		path = strings.TrimPrefix(value, "git@github.com:")
	} else {
		parsed, err := url.Parse(value)
		if err != nil || !strings.EqualFold(parsed.Hostname(), "github.com") {
			return "", "", "", false
		}
		path = parsed.Path
	}

	path = strings.Trim(strings.TrimSuffix(path, ".git"), "/")
	parts := strings.Split(path, "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", "", false
	}

	owner, repo = parts[0], parts[1]
	return "https://github.com/" + owner + "/" + repo, owner, repo, true
}

func (s Service) worktree(ctx context.Context, root string) (Worktree, error) {
	output, err := s.git(ctx, root, "status", "--short", "--untracked-files=all")
	if err != nil {
		return Worktree{}, err
	}

	result := Worktree{}
	if output == "" {
		return result, nil
	}
	for _, line := range strings.Split(output, "\n") {
		if len(line) < 2 {
			continue
		}
		result.Entries = append(result.Entries, line)
		if strings.HasPrefix(line, "??") {
			result.Untracked++
			continue
		}
		if line[0] != ' ' {
			result.Staged++
		}
		if line[1] != ' ' {
			result.Unstaged++
		}
	}
	return result, nil
}

func (s Service) lastCommit(ctx context.Context, root string) *Commit {
	output, err := s.git(ctx, root, "log", "-1", "--format=%H%x00%h%x00%aI%x00%an%x00%s")
	if err != nil {
		return nil
	}
	parts := strings.Split(output, "\x00")
	if len(parts) != 5 {
		return nil
	}
	committedAt, err := time.Parse(time.RFC3339, parts[2])
	if err != nil {
		return nil
	}
	return &Commit{Hash: parts[0], Short: parts[1], Time: committedAt, Author: parts[3], Subject: parts[4]}
}

func (s Service) defaultBranch(ctx context.Context, root, remote string) string {
	ref, err := s.git(ctx, root, "symbolic-ref", "--quiet", "--short", "refs/remotes/"+remote+"/HEAD")
	if err == nil {
		if _, branch, found := strings.Cut(ref, "/"); found && branch != "" {
			return branch
		}
	}
	for _, branch := range []string{"main", "master"} {
		if _, err := s.git(ctx, root, "rev-parse", "--verify", "--quiet", "refs/remotes/"+remote+"/"+branch); err == nil {
			return branch
		}
	}
	return "main"
}

func (s Service) upstream(ctx context.Context, root, branch, remote string) string {
	upstream, err := s.git(ctx, root, "rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{upstream}")
	if err == nil {
		return upstream
	}
	if branch == "" || branch == "unborn" {
		return ""
	}
	candidate := remote + "/" + branch
	if _, err := s.git(ctx, root, "rev-parse", "--verify", "--quiet", "refs/remotes/"+candidate); err == nil {
		return candidate
	}
	return ""
}

func (s Service) aheadBehind(ctx context.Context, root, upstream string) (ahead, behind int, ok bool) {
	output, err := s.git(ctx, root, "rev-list", "--left-right", "--count", "HEAD..."+upstream)
	if err != nil {
		return 0, 0, false
	}
	parts := strings.Fields(output)
	if len(parts) != 2 {
		return 0, 0, false
	}
	ahead, err = strconv.Atoi(parts[0])
	if err != nil {
		return 0, 0, false
	}
	behind, err = strconv.Atoi(parts[1])
	if err != nil {
		return 0, 0, false
	}
	return ahead, behind, true
}

func (r Remote) NameWithOwner() string {
	if r.Owner == "" || r.Repo == "" {
		return ""
	}
	return fmt.Sprintf("%s/%s", r.Owner, r.Repo)
}
