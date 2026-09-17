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

// The base branches the fixtures name.
const (
	originMain = "origin/main"
	localMain  = "main"
)

// Commands the branch fixtures answer.
const (
	readUpstream = "git -C /work rev-parse --abbrev-ref --symbolic-full-name @{upstream}"
	countAhead   = "git -C /work rev-list --left-right --count @{upstream}...HEAD"
	logFromMain  = "git -C /work log --reverse --max-count=200 --format=%h%x1f%s%x1e origin/main..HEAD"
	readHooksDir = "git -C /work rev-parse --git-path hooks"
)

// featureCommits are the commits featureBranch has on top of its base.
func featureCommits() []gitrepo.Commit {
	return []gitrepo.Commit{
		{Hash: "1a2b3c4", Subject: "fix(config): redact tokens"},
		{Hash: "5d6e7f8", Subject: "test: cover the empty token"},
	}
}

func TestReadBranchReadsWhereTheBranchStands(t *testing.T) {
	t.Parallel()

	// Act
	branch, err := gitrepo.ReadBranch(t.Context(), fakeRunner(t, featureBranch()), workDir)
	if err != nil {
		t.Fatalf("ReadBranch returned %v, want nil", err)
	}

	// Assert
	want := gitrepo.Branch{
		Name:     "fix/PROJ-412-token-redaction",
		Detached: false,
		Head:     "9f0e3885f06a",
		Upstream: "origin/fix/PROJ-412-token-redaction",
		Ahead:    2,
		Behind:   1,
		Base:     originMain,
		Commits:  featureCommits(),
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

func TestABranchIsPushedOnlyWhenItsOwnUpstreamHasEverything(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		replies  map[string]reply
		upstream string
		pushed   bool
	}{
		"never pushed, so no upstream": {
			replies: with(featureBranch(), map[string]reply{readUpstream: {err: errNoUpstream}}),
		},
		"pushed, with nothing new": {
			replies:  with(featureBranch(), map[string]reply{countAhead: {out: []byte("0\t0\n")}}),
			upstream: "origin/fix/PROJ-412-token-redaction",
			pushed:   true,
		},
		// Created with git switch -c from origin/main and no --no-track, a branch
		// tracks origin/main: nothing ahead, and still not on the remote at all.
		// Reproduced on this repository's own feature branch.
		"tracking some other branch": {
			replies: with(featureBranch(), map[string]reply{
				readUpstream: {out: []byte("origin/main\n")},
				countAhead:   {out: []byte("0\t0\n")},
			}),
			upstream: originMain,
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Act
			branch, err := gitrepo.ReadBranch(t.Context(), fakeRunner(t, tt.replies), workDir)

			// Assert
			if err != nil || branch.Upstream != tt.upstream || branch.Pushed() != tt.pushed {
				t.Errorf("ReadBranch = %+v, %v; want upstream %q, pushed %v", branch, err, tt.upstream, tt.pushed)
			}
		})
	}
}

func TestTheBaseFallsBackWhenOriginNamesNoDefault(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		present []string
		want    string
	}{
		"origin/main exists":   {present: []string{"refs/remotes/origin/main"}, want: originMain},
		"origin/master exists": {present: []string{"refs/remotes/origin/master"}, want: "origin/master"},
		"only a local main":    {present: []string{"refs/heads/main"}, want: localMain},
		"only a local master":  {present: []string{"refs/heads/master"}, want: "master"},
		"nothing to call base": {present: nil, want: ""},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
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

			delete(replies, logFromMain)
			replies["git -C /work log --reverse --max-count=200 --format=%h%x1f%s%x1e "+tt.want+"..HEAD"] = reply{}

			// Act
			branch, err := gitrepo.ReadBranch(t.Context(), fakeRunner(t, replies), workDir)

			// Assert
			if err != nil || branch.Base != tt.want {
				t.Errorf("Base = %q, %v, want %q", branch.Base, err, tt.want)
			}
		})
	}
}

// emptyRepository is a repository with no commit yet: HEAD does not resolve,
// so there is no range to log.
func emptyRepository() map[string]reply {
	return map[string]reply{
		showCurrentBranch:             {out: []byte("main\n")},
		"git -C /work rev-parse HEAD": {err: errDetachedRead},
		readUpstream:                  {err: errNoUpstream},
		"git -C /work symbolic-ref --quiet --short refs/remotes/origin/HEAD": {out: []byte("origin/main\n")},
	}
}

func TestAnEmptyRepositoryStillReads(t *testing.T) {
	t.Parallel()

	// Act
	branch, err := gitrepo.ReadBranch(t.Context(), fakeRunner(t, emptyRepository()), workDir)

	// Assert
	if err != nil || branch.Name != localMain || branch.Head != "" || len(branch.Commits) != 0 {
		t.Errorf("ReadBranch = %+v, %v, want main with no head and no commits", branch, err)
	}
}

func TestADetachedHeadStillReads(t *testing.T) {
	t.Parallel()

	// Arrange
	replies := with(emptyRepository(), map[string]reply{showCurrentBranch: {out: []byte("\n")}})

	// Act
	branch, err := gitrepo.ReadBranch(t.Context(), fakeRunner(t, replies), workDir)

	// Assert
	if err != nil || !branch.Detached {
		t.Errorf("ReadBranch = %+v, %v, want it detached with no branch checked out", branch, err)
	}
}

func TestReadBranchReportsAnUnreadableRepository(t *testing.T) {
	t.Parallel()

	// Arrange
	replies := map[string]reply{showCurrentBranch: {err: errNotARepository}}

	// Act
	_, err := gitrepo.ReadBranch(t.Context(), fakeRunner(t, replies), workDir)

	// Assert
	if !errors.Is(err, errNotARepository) {
		t.Errorf("ReadBranch returned %v, want git's error", err)
	}
}

func TestReadBranchKeepsOnlyWhatGitAnsweredClearly(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		replies       map[string]reply
		ahead, behind int
		commits       []gitrepo.Commit
	}{
		"an unreadable count and log are left empty": {
			replies: with(featureBranch(), map[string]reply{
				countAhead:  {out: []byte("garbage\n")},
				logFromMain: {err: errNoRef},
			}),
		},
		"counts that are not numbers read as zero": {
			replies: with(featureBranch(), map[string]reply{countAhead: {out: []byte("one\ttwo\n")}}),
			commits: featureCommits(),
		},
		// Anyone who can get a commit onto the base branch writes its subject.
		"a commit subject cannot drive the terminal": {
			replies: with(featureBranch(), map[string]reply{
				logFromMain: {out: []byte("1a2b3c4\x1ffix: \x1b]0;owned\x07subject\x1e\n")},
			}),
			ahead:   2,
			behind:  1,
			commits: []gitrepo.Commit{{Hash: "1a2b3c4", Subject: "fix: subject"}},
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Act
			branch, err := gitrepo.ReadBranch(t.Context(), fakeRunner(t, tt.replies), workDir)

			// Assert
			if err != nil || branch.Ahead != tt.ahead || branch.Behind != tt.behind ||
				!slices.Equal(branch.Commits, tt.commits) {
				t.Errorf("ReadBranch = %+v, %v; want ahead %d, behind %d, commits %+v",
					branch, err, tt.ahead, tt.behind, tt.commits)
			}
		})
	}
}

func TestCreateBranchStartsFromTheBaseWithoutTrackingIt(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		name, start string
		command     string
		answer      reply
		wantErr     error
	}{
		// Without --no-track, a branch started from origin/main would take
		// origin/main as its upstream, and read as behind or ahead of main.
		"from a base": {
			name: "fix/PROJ-1-x", start: "origin/main",
			command: "git -C /work switch --create fix/PROJ-1-x --no-track origin/main",
		},
		"from HEAD": {name: "feat/PROJ-2-y", start: "", command: "git -C /work switch --create feat/PROJ-2-y"},
		"refused by git": {
			name: "taken", start: localMain,
			command: "git -C /work switch --create taken --no-track main",
			answer:  reply{err: errIndexLocked}, wantErr: errIndexLocked,
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			run, ran := recordingRunner(t, map[string]reply{tt.command: tt.answer})

			// Act
			err := gitrepo.CreateBranch(t.Context(), run, workDir, tt.name, tt.start)

			// Assert
			if !errors.Is(err, tt.wantErr) || !slices.Equal(*ran, []string{tt.command}) {
				t.Errorf("CreateBranch ran %q and returned %v, want %q and %v", *ran, err, tt.command, tt.wantErr)
			}
		})
	}
}

func TestPushCommandRunsInTheRepositoryWithoutPrompts(t *testing.T) {
	t.Parallel()

	// Act
	push := gitrepo.PushCommand(workDir, "fix/PROJ-1-x")

	// Assert
	if push.Dir != workDir || push.Name != "git" ||
		!slices.Equal(push.Args, []string{"push", "--set-upstream", "origin", "fix/PROJ-1-x"}) {
		t.Errorf("PushCommand = %+v", push)
	}

	// Nobody can answer a credential prompt from inside the interface, so git
	// must fail rather than wait for one.
	if !slices.Contains(push.Env, "GIT_TERMINAL_PROMPT=0") {
		t.Errorf("PushCommand environment = %q, want prompts turned off", push.Env)
	}
}

func TestCommitCommandRunsInTheRepository(t *testing.T) {
	t.Parallel()

	// Act
	commit := gitrepo.CommitCommand(workDir, "/tmp/message.txt")

	// Assert
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

			// Arrange
			replies := map[string]reply{readHooksDir: {out: []byte(tt.answer)}}

			// Act
			dir, err := gitrepo.HooksDir(t.Context(), fakeRunner(t, replies), workDir)

			// Assert
			if err != nil || dir != tt.want {
				t.Errorf("HooksDir = %q, %v, want %q", dir, err, tt.want)
			}
		})
	}
}

func TestHooksDirReportsGitsFailure(t *testing.T) {
	t.Parallel()

	// Arrange
	replies := map[string]reply{readHooksDir: {err: errNotARepository}}

	// Act
	_, err := gitrepo.HooksDir(t.Context(), fakeRunner(t, replies), workDir)

	// Assert
	if !errors.Is(err, errNotARepository) {
		t.Errorf("HooksDir returned %v, want git's error", err)
	}
}
