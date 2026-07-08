package presentation

import (
	"fmt"
	"io"
	"strings"
	"time"

	gh "github.com/davis7dotsh/bgst/internal/github"
	"github.com/davis7dotsh/bgst/internal/repository"
)

type Presenter struct {
	w     io.Writer
	color bool
}

func New(w io.Writer, color bool) Presenter {
	return Presenter{w: w, color: color}
}

func (p Presenter) Overview(info repository.Info, pulls []gh.PullRequest, pullErr error) {
	name := "local repository"
	if info.Remote != nil && info.Remote.NameWithOwner() != "" {
		name = info.Remote.NameWithOwner()
	}
	fmt.Fprintf(p.w, "%s %s\n", p.bold("bgst ·"), p.bold(name))
	if info.Remote != nil {
		if info.Remote.GitHubURL != "" {
			fmt.Fprintf(p.w, "%s\n", p.blue(info.Remote.GitHubURL))
		} else {
			fmt.Fprintf(p.w, "%s\n", info.Remote.RawURL)
		}
	}

	p.section("Repository")
	p.row("Root", info.Root)
	branch := info.Branch
	if info.Detached {
		branch = "detached at " + branch
	}
	p.row("Branch", branch)
	p.row("Worktree", p.worktree(info.Worktree))
	p.row("Tracking", p.tracking(info))
	if info.LastCommit == nil {
		p.row("Last commit", "no commits yet")
	} else {
		commit := fmt.Sprintf("%s · %s by %s", p.yellow(info.LastCommit.Short), RelativeTime(info.LastCommit.Time, time.Now()), info.LastCommit.Author)
		p.row("Last commit", commit)
		p.row("", info.LastCommit.Subject)
	}

	if pullErr != nil {
		p.section("Pull requests")
		fmt.Fprintf(p.w, "  %s %v\n", p.yellow("!"), pullErr)
		return
	}

	drafts := make([]gh.PullRequest, 0)
	ready := make([]gh.PullRequest, 0)
	for _, pull := range pulls {
		if pull.IsDraft {
			drafts = append(drafts, pull)
		} else {
			ready = append(ready, pull)
		}
	}
	p.pullSection("Draft PRs", drafts)
	p.pullSection("Open PRs", ready)
}

func (p Presenter) Changes(worktree repository.Worktree) {
	p.section("Changes to commit")
	for _, entry := range worktree.Entries {
		fmt.Fprintf(p.w, "  %s\n", entry)
	}
}

func (p Presenter) Success(message string) {
	fmt.Fprintf(p.w, "%s %s\n", p.green("✓"), message)
}

func (p Presenter) Note(message string) {
	fmt.Fprintf(p.w, "%s %s\n", p.yellow("!"), message)
}

func (p Presenter) section(title string) {
	fmt.Fprintf(p.w, "\n%s\n", p.bold(title))
}

func (p Presenter) row(label, value string) {
	if label == "" {
		fmt.Fprintf(p.w, "             %s\n", value)
		return
	}
	fmt.Fprintf(p.w, "  %-10s %s\n", label, value)
}

func (p Presenter) worktree(worktree repository.Worktree) string {
	if worktree.Clean() {
		return p.green("clean")
	}
	parts := make([]string, 0, 3)
	if worktree.Staged > 0 {
		parts = append(parts, plural(worktree.Staged, "staged change"))
	}
	if worktree.Unstaged > 0 {
		parts = append(parts, plural(worktree.Unstaged, "unstaged change"))
	}
	if worktree.Untracked > 0 {
		parts = append(parts, plural(worktree.Untracked, "untracked file"))
	}
	return p.yellow(strings.Join(parts, " · "))
}

func (p Presenter) tracking(info repository.Info) string {
	if info.Upstream == "" || !info.HasTracking {
		return p.yellow("no upstream branch")
	}
	if info.Ahead == 0 && info.Behind == 0 {
		return fmt.Sprintf("%s · %s", info.Upstream, p.green("up to date"))
	}
	parts := make([]string, 0, 2)
	if info.Ahead > 0 {
		parts = append(parts, fmt.Sprintf("%d ahead", info.Ahead))
	}
	if info.Behind > 0 {
		parts = append(parts, fmt.Sprintf("%d behind", info.Behind))
	}
	return fmt.Sprintf("%s · %s", info.Upstream, p.yellow(strings.Join(parts, ", ")))
}

func (p Presenter) pullSection(title string, pulls []gh.PullRequest) {
	p.section(fmt.Sprintf("%s (%d)", title, len(pulls)))
	if len(pulls) == 0 {
		fmt.Fprintln(p.w, "  None")
		return
	}
	for _, pull := range pulls {
		fmt.Fprintf(p.w, "  %s %s\n", p.blue(fmt.Sprintf("#%d", pull.Number)), pull.Title)
		fmt.Fprintf(p.w, "      %s\n", p.pullStatus(pull))
		fmt.Fprintf(p.w, "      %s\n", p.blue(pull.URL))
	}
}

func (p Presenter) pullStatus(pull gh.PullRequest) string {
	parts := []string{p.checkStatus(pull.CheckStatus())}
	switch pull.ReviewDecision {
	case "APPROVED":
		parts = append(parts, p.green("approved"))
	case "CHANGES_REQUESTED":
		parts = append(parts, p.red("changes requested"))
	case "REVIEW_REQUIRED":
		parts = append(parts, p.yellow("review required"))
	default:
		parts = append(parts, "no review decision")
	}

	switch pull.MergeStateStatus {
	case "CLEAN":
		parts = append(parts, p.green("mergeable"))
	case "DIRTY":
		parts = append(parts, p.red("conflicts"))
	case "BEHIND":
		parts = append(parts, p.yellow("base branch ahead"))
	case "BLOCKED":
		parts = append(parts, p.yellow("blocked"))
	case "UNSTABLE":
		parts = append(parts, p.yellow("unstable"))
	}
	return strings.Join(parts, " · ")
}

func (p Presenter) checkStatus(status gh.CheckStatus) string {
	switch status {
	case gh.ChecksPassing:
		return p.green("checks passing")
	case gh.ChecksPending:
		return p.yellow("checks pending")
	case gh.ChecksFailing:
		return p.red("checks failing")
	default:
		return "no checks"
	}
}

func (p Presenter) style(code, value string) string {
	if !p.color {
		return value
	}
	return "\x1b[" + code + "m" + value + "\x1b[0m"
}

func (p Presenter) bold(value string) string   { return p.style("1", value) }
func (p Presenter) blue(value string) string   { return p.style("36", value) }
func (p Presenter) green(value string) string  { return p.style("32", value) }
func (p Presenter) yellow(value string) string { return p.style("33", value) }
func (p Presenter) red(value string) string    { return p.style("31", value) }

func plural(count int, noun string) string {
	if count == 1 {
		return fmt.Sprintf("%d %s", count, noun)
	}
	return fmt.Sprintf("%d %ss", count, noun)
}

func RelativeTime(then, now time.Time) string {
	delta := now.Sub(then)
	if delta < 0 {
		return "in the future"
	}
	if delta < time.Minute {
		return "just now"
	}
	if delta < time.Hour {
		return plural(int(delta.Minutes()), "minute") + " ago"
	}
	if delta < 24*time.Hour {
		return plural(int(delta.Hours()), "hour") + " ago"
	}
	if delta < 30*24*time.Hour {
		return plural(int(delta.Hours()/24), "day") + " ago"
	}
	if delta < 365*24*time.Hour {
		return plural(int(delta.Hours()/(24*30)), "month") + " ago"
	}
	return plural(int(delta.Hours()/(24*365)), "year") + " ago"
}
