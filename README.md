# Better Git Status (`bgst`)

One command for the useful state of a GitHub repo: branch and worktree changes, last commit, remote drift, draft PRs, and open PR checks.

```sh
curl -fsSL https://davis7dotsh.github.io/bgst/install.sh | sh
```

```text
bgst                        repo + PR overview
bgst pull                   fetch everything, then show the overview
bgst yeet "message"         commit everything, optionally push to main
bgst update                 install the latest release
bgst version                print version and platform
```

Requires Git and an authenticated [GitHub CLI](https://cli.github.com/). `bgst pull` never checks out, merges, rebases, or resets anything.

MIT licensed.
