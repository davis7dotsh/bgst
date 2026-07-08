package repository

import "testing"

func TestParseGitHubRemote(t *testing.T) {
	tests := []struct {
		name  string
		input string
		web   string
		owner string
		repo  string
		ok    bool
	}{
		{name: "https", input: "https://github.com/davis7dotsh/bgst.git", web: "https://github.com/davis7dotsh/bgst", owner: "davis7dotsh", repo: "bgst", ok: true},
		{name: "scp ssh", input: "git@github.com:davis7dotsh/bgst.git", web: "https://github.com/davis7dotsh/bgst", owner: "davis7dotsh", repo: "bgst", ok: true},
		{name: "ssh url", input: "ssh://git@github.com/davis7dotsh/bgst.git", web: "https://github.com/davis7dotsh/bgst", owner: "davis7dotsh", repo: "bgst", ok: true},
		{name: "not github", input: "https://gitlab.com/davis/bgst.git", ok: false},
		{name: "too deep", input: "https://github.com/a/b/c.git", ok: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			web, owner, repo, ok := ParseGitHubRemote(test.input)
			if web != test.web || owner != test.owner || repo != test.repo || ok != test.ok {
				t.Fatalf("ParseGitHubRemote(%q) = %q, %q, %q, %v", test.input, web, owner, repo, ok)
			}
		})
	}
}

func TestWorktreeClean(t *testing.T) {
	if !(Worktree{}).Clean() {
		t.Fatal("empty worktree should be clean")
	}
	if (Worktree{Untracked: 1}).Clean() {
		t.Fatal("worktree with an untracked file should not be clean")
	}
}
