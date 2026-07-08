package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"runtime"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/x/term"
	gh "github.com/davis7dotsh/bgst/internal/github"
	"github.com/davis7dotsh/bgst/internal/presentation"
	"github.com/davis7dotsh/bgst/internal/repository"
	"github.com/davis7dotsh/bgst/internal/updater"
)

type BuildInfo struct {
	Version string
	Commit  string
	Date    string
}

type App struct {
	in        io.Reader
	out       io.Writer
	buildInfo BuildInfo
	repos     repository.Service
	github    gh.Client
	updater   updater.Service
	presenter presentation.Presenter
}

func New(in io.Reader, out io.Writer, buildInfo BuildInfo) App {
	color := false
	if file, ok := out.(*os.File); ok {
		color = term.IsTerminal(file.Fd()) && os.Getenv("NO_COLOR") == "" && os.Getenv("TERM") != "dumb"
	}
	return App{
		in:        in,
		out:       out,
		buildInfo: buildInfo,
		repos:     repository.New(),
		github:    gh.New(),
		updater:   updater.New(),
		presenter: presentation.New(out, color),
	}
}

func (a App) Run(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return a.status(ctx)
	}

	switch args[0] {
	case "status":
		if len(args) != 1 {
			return errors.New("status does not accept arguments")
		}
		return a.status(ctx)
	case "pull":
		if len(args) != 1 {
			return errors.New("pull does not accept arguments")
		}
		return a.pull(ctx)
	case "yeet":
		return a.yeet(ctx, args[1:])
	case "update":
		if len(args) != 1 {
			return errors.New("update does not accept arguments")
		}
		return a.update(ctx)
	case "version", "--version", "-v":
		if len(args) != 1 {
			return errors.New("version does not accept arguments")
		}
		a.printVersion()
		return nil
	case "help", "--help", "-h":
		a.printHelp()
		return nil
	default:
		return fmt.Errorf("unknown command %q (run bgst help)", args[0])
	}
}

func (a App) status(ctx context.Context) error {
	info, err := a.repos.Inspect(ctx, ".")
	if err != nil {
		return err
	}

	var pulls []gh.PullRequest
	var pullErr error
	if info.Remote == nil {
		pullErr = errors.New("no Git remote is configured")
	} else if info.Remote.NameWithOwner() == "" {
		pullErr = errors.New("the selected remote is not a github.com repository")
	} else {
		pulls, pullErr = a.github.OpenPullRequests(ctx, info.Root, info.Remote.NameWithOwner())
	}
	a.presenter.Overview(info, pulls, pullErr)
	return nil
}

func (a App) pull(ctx context.Context) error {
	fmt.Fprintln(a.out, "Fetching all remotes without changing the current worktree…")
	if err := a.repos.Fetch(ctx, "."); err != nil {
		return err
	}
	a.presenter.Success("Remote-tracking branches and tags are up to date.")
	fmt.Fprintln(a.out)
	return a.status(ctx)
}

func (a App) yeet(ctx context.Context, args []string) error {
	yes := false
	messageParts := make([]string, 0, len(args))
	for _, arg := range args {
		if arg == "--yes" || arg == "-y" {
			yes = true
			continue
		}
		messageParts = append(messageParts, arg)
	}
	message := strings.TrimSpace(strings.Join(messageParts, " "))
	if message == "" {
		return errors.New("a commit message is required: bgst yeet \"your message\"")
	}

	info, err := a.repos.Inspect(ctx, ".")
	if err != nil {
		return err
	}
	if info.Worktree.Clean() {
		return errors.New("nothing to commit; the worktree is clean")
	}
	if info.Remote == nil {
		return errors.New("cannot yeet without a Git remote")
	}

	a.presenter.Changes(info.Worktree)
	if err := a.repos.AddAll(ctx, info.Root); err != nil {
		return fmt.Errorf("could not stage changes: %w", err)
	}
	hash, err := a.repos.Commit(ctx, info.Root, message)
	if err != nil {
		return fmt.Errorf("could not commit changes: %w", err)
	}
	a.presenter.Success(fmt.Sprintf("Committed everything as %s (%s).", hash, message))

	target := info.Remote.Name + "/" + info.DefaultBranch
	if !yes {
		confirmed, err := a.confirmPush(ctx, target)
		if err != nil {
			return err
		}
		if !confirmed {
			a.presenter.Note("Commit kept locally; nothing was pushed.")
			return nil
		}
	}

	if err := a.repos.PushToDefault(ctx, info.Root, info.Remote.Name, info.DefaultBranch); err != nil {
		return fmt.Errorf("commit was created, but push to %s failed: %w", target, err)
	}
	a.presenter.Success("Pushed HEAD directly to " + target + ".")
	return nil
}

func (a App) update(ctx context.Context) error {
	fmt.Fprintln(a.out, "Checking for a newer bgst release…")
	result, err := a.updater.InstallLatest(ctx, a.buildInfo.Version)
	if err != nil {
		return fmt.Errorf("update failed: %w; reinstall with curl -fsSL https://davis7dotsh.github.io/bgst/install.sh | sh", err)
	}
	if !result.Updated {
		a.presenter.Success("Already on the latest release (" + result.Latest + ").")
		return nil
	}
	a.presenter.Success(fmt.Sprintf("Updated bgst from %s to %s.", result.Current, result.Latest))
	return nil
}

func (a App) confirmPush(ctx context.Context, target string) (bool, error) {
	input, inputOK := a.in.(*os.File)
	output, outputOK := a.out.(*os.File)
	if !inputOK || !outputOK || !term.IsTerminal(input.Fd()) || !term.IsTerminal(output.Fd()) {
		return false, errors.New("cannot prompt outside an interactive terminal; commit was kept locally (use --yes to push)")
	}

	confirmed := false
	prompt := huh.NewConfirm().
		Title("Push HEAD directly to " + target + "?").
		Description("This updates the remote default branch. The safe default is No.").
		Affirmative("Yes, push it").
		Negative("No, keep it local").
		Value(&confirmed)
	form := huh.NewForm(huh.NewGroup(prompt)).WithInput(input).WithOutput(output)
	if err := form.RunWithContext(ctx); err != nil {
		return false, fmt.Errorf("push confirmation failed: %w", err)
	}
	return confirmed, nil
}

func (a App) printVersion() {
	version := a.buildInfo.Version
	if version == "" {
		version = "dev"
	}
	fmt.Fprintf(a.out, "bgst %s", version)
	if a.buildInfo.Commit != "" {
		fmt.Fprintf(a.out, " (%s)", a.buildInfo.Commit)
	}
	if a.buildInfo.Date != "" {
		fmt.Fprintf(a.out, " built %s", a.buildInfo.Date)
	}
	fmt.Fprintf(a.out, " · %s/%s\n", runtime.GOOS, runtime.GOARCH)
}

func (a App) printHelp() {
	fmt.Fprint(a.out, `bgst gives you a fast overview of the current Git repository.

Usage:
  bgst                         Show repository and pull request status
  bgst pull                    Fetch every remote, then show status
  bgst yeet "commit message"   Commit every local change and offer to push to main
  bgst update                  Install the latest release
  bgst version                 Show version and platform

Options for yeet:
  -y, --yes                       Skip the push confirmation

The pull command never checks out, merges, rebases, or moves your worktree.
`)
}
