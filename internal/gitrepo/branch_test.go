// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package gitrepo_test

import (
	"errors"
	"slices"
	"testing"

	"github.com/jacob-delgado/workflow/internal/gitrepo"
	"github.com/jacob-delgado/workflow/internal/proc"
)

// errNoUpstream is what git says for a branch that was never pushed.
var errNoUpstream = errors.New("fatal: no upstream configured for branch")

// errNoRef is what rev-parse --verify --quiet and symbolic-ref --quiet exit with.
var errNoRef = errors.New("exit status 1")

// featureBranch is a pushed feature branch two commits ahead of its upstream.
func featureBranch() map[string]reply {
	return map[string]reply{
		showCurrentBranch:             {out: []byte("fix/PROJ-412-token-redaction\n")},
		"git -C /work rev-parse HEAD": {out: []byte("9f0e3885f06a\n")},
		"git -C /work rev-parse --abbrev-ref --symbolic-full-name @{upstream}": {
			out: []byte("origin/fix/PROJ-412-token-redaction\n"),
		},
		"git -C /work rev-list --left-right --count @{upstream}...HEAD":      {out: []byte("1\t2\n")},
		"git -C /work symbolic-ref --quiet --short refs/remotes/origin/HEAD": {out: []byte("origin/main\n")},
		// Real log output: a unit separator between the fields, a record
		// separator after each commit, and a newline git adds between them.
		"git -C /work log --reverse --max-count=200 --format=%h%x1f%s%x1e origin/main..HEAD": {
			out: []byte("1a2b3c4\x1ffix(config): redact tokens\x1e\n5d6e7f8\x1ftest: cover the empty token\x1e\n"),
		},
	}
}

func TestReadBranchReadsWhereTheBranchStands(t *testing.T) {
	t.Parallel()

	branch, err := gitrepo.ReadBranch(t.Context(), fakeRunner(t, featureBranch()), workDir)
	if err != nil {
		t.Fatalf("ReadBranch returned %v, want nil", err)
	}

	want := gitrepo.Branch{
		Name:     "fix/PROJ-412-token-redaction",
		Detached: false,
		Head:     "9f0e3885f06a",
		Upstream: "origin/fix/PROJ-412-token-redaction",
		Ahead:    2,
		Behind:   1,
		Base:     "origin/main",
		Commits: []gitrepo.Commit{
			{Hash: "1a2b3c4", Subject: "fix(config): redact tokens"},
			{Hash: "5d6e7f8", Subject: "test: cover the empty token"},
		},
	}

	if !equalBranches(branch, want) {
		t.Errorf("ReadBranch = %+v, want %+v", branch, want)
	}

	if branch.Pushed() {
		t.Error("Pushed() = true with two commits not yet on the upstream")
	}
}

// equalBranches compares two branches field by field.
func equalBranches(got, want gitrepo.Branch) bool {
	commits := got.Commits
	got.Commits, want.Commits = nil, slices.Clone(want.Commits)

	return got.Name == want.Name && got.Detached == want.Detached && got.Head == want.Head &&
		got.Upstream == want.Upstream && got.Ahead == want.Ahead && got.Behind == want.Behind &&
		got.Base == want.Base && slices.Equal(commits, want.Commits)
}

func TestABranchNeverPushedHasNoUpstream(t *testing.T) {
	t.Parallel()

	replies := featureBranch()
	replies["git -C /work rev-parse --abbrev-ref --symbolic-full-name @{upstream}"] = reply{err: errNoUpstream}

	branch, err := gitrepo.ReadBranch(t.Context(), fakeRunner(t, replies), workDir)
	if err != nil {
		t.Fatalf("ReadBranch returned %v, want nil", err)
	}

	if branch.Upstream != "" || branch.Ahead != 0 || branch.Pushed() {
		t.Errorf("ReadBranch = %+v, want no upstream and not pushed", branch)
	}
}

func TestAPushedBranchWithNothingNewIsPushed(t *testing.T) {
	t.Parallel()

	replies := featureBranch()
	replies["git -C /work rev-list --left-right --count @{upstream}...HEAD"] = reply{out: []byte("0\t0\n")}

	branch, err := gitrepo.ReadBranch(t.Context(), fakeRunner(t, replies), workDir)
	if err != nil || !branch.Pushed() {
		t.Errorf("ReadBranch = %+v, %v, want it pushed", branch, err)
	}
}

func TestABranchTrackingSomeOtherBranchIsNotPushed(t *testing.T) {
	t.Parallel()

	// Created with git switch -c from origin/main and no --no-track, a branch
	// tracks origin/main: nothing ahead, and still not on the remote at all.
	// Reproduced on this repository's own feature branch.
	replies := featureBranch()
	replies["git -C /work rev-parse --abbrev-ref --symbolic-full-name @{upstream}"] = reply{out: []byte("origin/main\n")}
	replies["git -C /work rev-list --left-right --count @{upstream}...HEAD"] = reply{out: []byte("0\t0\n")}

	branch, err := gitrepo.ReadBranch(t.Context(), fakeRunner(t, replies), workDir)
	if err != nil || branch.Pushed() {
		t.Errorf("ReadBranch = %+v, %v, want a branch tracking origin/main to read as not pushed", branch, err)
	}
}

func TestTheBaseFallsBackWhenOriginNamesNoDefault(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		present []string
		want    string
	}{
		"origin/main exists":   {present: []string{"refs/remotes/origin/main"}, want: "origin/main"},
		"origin/master exists": {present: []string{"refs/remotes/origin/master"}, want: "origin/master"},
		"only a local main":    {present: []string{"refs/heads/main"}, want: "main"},
		"only a local master":  {present: []string{"refs/heads/master"}, want: "master"},
		"nothing to call base": {present: nil, want: ""},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			replies := featureBranch()
			replies["git -C /work symbolic-ref --quiet --short refs/remotes/origin/HEAD"] = reply{err: errNoRef}

			for _, ref := range []string{
				"refs/remotes/origin/main", "refs/remotes/origin/master", "refs/heads/main", "refs/heads/master",
			} {
				answer := reply{err: errNoRef}
				if slices.Contains(tt.present, ref) {
					answer = reply{out: []byte("abc\n")}
				}

				replies["git -C /work rev-parse --verify --quiet "+ref] = answer
			}

			delete(replies, "git -C /work log --reverse --max-count=200 --format=%h%x1f%s%x1e origin/main..HEAD")
			replies["git -C /work log --reverse --max-count=200 --format=%h%x1f%s%x1e "+tt.want+"..HEAD"] = reply{}

			branch, err := gitrepo.ReadBranch(t.Context(), fakeRunner(t, replies), workDir)
			if err != nil || branch.Base != tt.want {
				t.Errorf("Base = %q, %v, want %q", branch.Base, err, tt.want)
			}
		})
	}
}

func TestADetachedOrEmptyRepositoryStillReads(t *testing.T) {
	t.Parallel()

	// No commit yet: HEAD does not resolve, so there is no range to log.
	replies := map[string]reply{
		showCurrentBranch:             {out: []byte("main\n")},
		"git -C /work rev-parse HEAD": {err: errDetachedRead},
		"git -C /work rev-parse --abbrev-ref --symbolic-full-name @{upstream}": {err: errNoUpstream},
		"git -C /work symbolic-ref --quiet --short refs/remotes/origin/HEAD":   {out: []byte("origin/main\n")},
	}

	branch, err := gitrepo.ReadBranch(t.Context(), fakeRunner(t, replies), workDir)
	if err != nil || branch.Head != "" || len(branch.Commits) != 0 {
		t.Errorf("ReadBranch = %+v, %v, want an empty branch with no commits", branch, err)
	}

	replies[showCurrentBranch] = reply{out: []byte("\n")}

	branch, _ = gitrepo.ReadBranch(t.Context(), fakeRunner(t, replies), workDir)
	if !branch.Detached {
		t.Error("Detached = false with no branch checked out")
	}
}

func TestReadBranchReportsAnUnreadableRepository(t *testing.T) {
	t.Parallel()

	replies := map[string]reply{showCurrentBranch: {err: errNotARepository}}

	_, err := gitrepo.ReadBranch(t.Context(), fakeRunner(t, replies), workDir)
	if !errors.Is(err, errNotARepository) {
		t.Errorf("ReadBranch returned %v, want git's error", err)
	}
}

func TestAnUnreadableAheadCountOrLogIsLeftEmpty(t *testing.T) {
	t.Parallel()

	replies := featureBranch()
	replies["git -C /work rev-list --left-right --count @{upstream}...HEAD"] = reply{out: []byte("garbage\n")}
	replies["git -C /work log --reverse --max-count=200 --format=%h%x1f%s%x1e origin/main..HEAD"] = reply{err: errNoRef}

	branch, err := gitrepo.ReadBranch(t.Context(), fakeRunner(t, replies), workDir)
	if err != nil || branch.Ahead != 0 || branch.Behind != 0 || len(branch.Commits) != 0 {
		t.Errorf("ReadBranch = %+v, %v, want counts and commits left empty", branch, err)
	}
}

func TestCountsThatAreNotNumbersReadAsZero(t *testing.T) {
	t.Parallel()

	replies := featureBranch()
	replies["git -C /work rev-list --left-right --count @{upstream}...HEAD"] = reply{out: []byte("one\ttwo\n")}

	branch, err := gitrepo.ReadBranch(t.Context(), fakeRunner(t, replies), workDir)
	if err != nil || branch.Ahead != 0 || branch.Behind != 0 {
		t.Errorf("ReadBranch = %+v, %v, want zero counts", branch, err)
	}
}

func TestACommitSubjectCannotDriveTheTerminal(t *testing.T) {
	t.Parallel()

	// Anyone who can get a commit onto the base branch writes its subject.
	replies := featureBranch()
	replies["git -C /work log --reverse --max-count=200 --format=%h%x1f%s%x1e origin/main..HEAD"] = reply{
		out: []byte("1a2b3c4\x1ffix: \x1b]0;owned\x07subject\x1e\n"),
	}

	branch, _ := gitrepo.ReadBranch(t.Context(), fakeRunner(t, replies), workDir)
	if len(branch.Commits) != 1 || branch.Commits[0].Subject != "fix: subject" {
		t.Errorf("Commits = %+v, want the sequence stripped", branch.Commits)
	}
}

func TestCreateBranchStartsFromTheBaseWithoutTrackingIt(t *testing.T) {
	t.Parallel()

	// Without --no-track, a branch started from origin/main would take
	// origin/main as its upstream, and read as behind or ahead of main.
	replies := map[string]reply{
		"git -C /work switch --create fix/PROJ-1-x --no-track origin/main": {},
		"git -C /work switch --create feat/PROJ-2-y":                       {},
		"git -C /work switch --create taken --no-track main":               {err: errIndexLocked},
	}
	run := fakeRunner(t, replies)

	err := gitrepo.CreateBranch(t.Context(), run, workDir, "fix/PROJ-1-x", "origin/main")
	if err != nil {
		t.Errorf("CreateBranch from a base returned %v, want nil", err)
	}

	err = gitrepo.CreateBranch(t.Context(), run, workDir, "feat/PROJ-2-y", "")
	if err != nil {
		t.Errorf("CreateBranch from HEAD returned %v, want nil", err)
	}

	err = gitrepo.CreateBranch(t.Context(), run, workDir, "taken", "main")
	if !errors.Is(err, errIndexLocked) {
		t.Errorf("CreateBranch returned %v, want git's error", err)
	}
}

func TestPushAndCommitRunInTheRepository(t *testing.T) {
	t.Parallel()

	push := gitrepo.PushCommand(workDir, "fix/PROJ-1-x")
	if push.Dir != workDir || push.Name != "git" ||
		!slices.Equal(push.Args, []string{"push", "--set-upstream", "origin", "fix/PROJ-1-x"}) {
		t.Errorf("PushCommand = %+v", push)
	}

	// Nobody can answer a credential prompt from inside the interface, so git
	// must fail rather than wait for one.
	if !slices.Contains(push.Env, "GIT_TERMINAL_PROMPT=0") {
		t.Errorf("PushCommand environment = %q, want prompts turned off", push.Env)
	}

	commit := gitrepo.CommitCommand(workDir, "/tmp/message.txt")

	want := proc.Command{Dir: workDir, Name: "git", Args: []string{"commit", "--file", "/tmp/message.txt"}, Env: nil}
	if commit.Dir != want.Dir || commit.Name != want.Name || !slices.Equal(commit.Args, want.Args) {
		t.Errorf("CommitCommand = %+v, want %+v", commit, want)
	}
}

func TestHooksDirIsWhereGitLooksForHooks(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		answer string
		want   string
	}{
		"relative to the repository":  {answer: ".git/hooks\n", want: "/work/.git/hooks"},
		"core.hooksPath set absolute": {answer: "/etc/githooks\n", want: "/etc/githooks"},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			replies := map[string]reply{"git -C /work rev-parse --git-path hooks": {out: []byte(tt.answer)}}

			dir, err := gitrepo.HooksDir(t.Context(), fakeRunner(t, replies), workDir)
			if err != nil || dir != tt.want {
				t.Errorf("HooksDir = %q, %v, want %q", dir, err, tt.want)
			}
		})
	}

	replies := map[string]reply{"git -C /work rev-parse --git-path hooks": {err: errNotARepository}}

	_, err := gitrepo.HooksDir(t.Context(), fakeRunner(t, replies), workDir)
	if !errors.Is(err, errNotARepository) {
		t.Errorf("HooksDir returned %v, want git's error", err)
	}
}
