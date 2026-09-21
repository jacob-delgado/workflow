// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package gitrepo_test

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/jacob-delgado/workflow/internal/gitrepo"
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
		"git -C /work rev-list --left-right --count @{upstream}...HEAD": {out: []byte("1\t2\n")},
		originsHead: {out: []byte("origin/main\n")},
		// Real log output under -z: a NUL after the hash, and one after each
		// subject where git would otherwise put a newline.
		logFromMain: {
			out: []byte(firstHash + "\x00fix(config): redact tokens\x00" + secondHash + "\x00test: cover the empty token\x00"),
		},
		baseAge + originMain: {out: []byte(baseUpdatedISO + "\n")},
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
	logCommits   = "git -C /work log -z --reverse --format=%h%x00%s "
	logFromMain  = logCommits + "origin/main..HEAD"
	readHooksDir = "git -C /work rev-parse --git-path hooks"
	originsHead  = "git -C /work symbolic-ref --quiet --short refs/remotes/origin/HEAD"
	baseAge      = "git -C /work log -1 --format=%cI "
)

// baseUpdatedISO is when featureBranch's base last moved, as git writes %cI.
const baseUpdatedISO = "2026-09-13T16:00:00Z"

// firstHash and secondHash are the abbreviated hashes of featureBranch's
// commits, and secondSubject a subject the log cases give the second one.
const (
	firstHash     = "1a2b3c4"
	secondHash    = "5d6e7f8"
	secondSubject = "test: two"
)

// featureCommits are the commits featureBranch has on top of its base.
func featureCommits() []gitrepo.Commit {
	return []gitrepo.Commit{
		{Hash: firstHash, Subject: "fix(config): redact tokens"},
		{Hash: secondHash, Subject: "test: cover the empty token"},
	}
}

func TestReadBranchReadsWhereTheBranchStands(t *testing.T) {
	t.Parallel()

	// Act
	branch, err := gitrepo.At(fakeRunner(t, featureBranch()), workDir).ReadBranch(t.Context())
	if err != nil {
		t.Fatalf("ReadBranch returned %v, want nil", err)
	}

	// Assert
	want := gitrepo.Branch{
		Name:        "fix/PROJ-412-token-redaction",
		Detached:    false,
		Head:        "9f0e3885f06a",
		Upstream:    "origin/fix/PROJ-412-token-redaction",
		Ahead:       2,
		Behind:      1,
		Base:        originMain,
		BaseUpdated: baseUpdatedTime(t),
		Commits:     featureCommits(),
	}

	if !equalBranches(branch, want) {
		t.Errorf("ReadBranch = %+v, want %+v", branch, want)
	}

	if branch.Pushed() {
		t.Error("Pushed() = true with two commits not yet on the upstream")
	}
}

// commitCap mirrors the unexported commitLimit: how many commits ReadBranch
// keeps.
const commitCap = 200

// A branch longer than the cap keeps its OLDEST commits, so the first — the one
// a pull request titles itself with — survives. git applies a --max-count
// before it reverses, which would keep the newest instead.
func TestReadBranchKeepsItsFirstCommitPastTheLimit(t *testing.T) {
	t.Parallel()

	// Arrange
	first := gitrepo.Commit{Hash: "abc1230", Subject: "feat: the branch's first commit"}
	longLog := with(featureBranch(), map[string]reply{logFromMain: {out: manyCommits(first, commitCap+50)}})

	// Act
	branch, err := gitrepo.At(fakeRunner(t, longLog), workDir).ReadBranch(t.Context())
	if err != nil {
		t.Fatalf("ReadBranch returned %v, want nil", err)
	}

	// Assert
	if len(branch.Commits) != commitCap {
		t.Errorf("kept %d commits, want the cap of %d", len(branch.Commits), commitCap)
	}

	if len(branch.Commits) == 0 || branch.Commits[0] != first {
		t.Errorf("first commit = %+v, want the branch's first %+v", branch.Commits, first)
	}

	if !branch.Truncated {
		t.Error("Truncated = false, want true for a branch past the cap")
	}
}

// manyCommits renders count commits as git's -z log, oldest first, starting with
// first and filling the rest with placeholders.
func manyCommits(first gitrepo.Commit, count int) []byte {
	var out strings.Builder

	out.WriteString(first.Hash + "\x00" + first.Subject + "\x00")

	for index := 1; index < count; index++ {
		fmt.Fprintf(&out, "c%06d\x00chore: commit %d\x00", index, index)
	}

	return []byte(out.String())
}

// baseUpdatedTime is baseUpdatedISO parsed, so a test can compare against the
// time ReadBranch reads from git's %cI.
func baseUpdatedTime(t *testing.T) time.Time {
	t.Helper()

	parsed, err := time.Parse(time.RFC3339, baseUpdatedISO)
	if err != nil {
		t.Fatalf("parsing baseUpdatedISO: %v", err)
	}

	return parsed
}

// equalBranches compares two branches field by field.
func equalBranches(got, want gitrepo.Branch) bool {
	commits := got.Commits
	got.Commits, want.Commits = nil, slices.Clone(want.Commits)

	return got.Name == want.Name && got.Detached == want.Detached && got.Head == want.Head &&
		got.Upstream == want.Upstream && got.Ahead == want.Ahead && got.Behind == want.Behind &&
		got.Base == want.Base && got.BaseUpdated.Equal(want.BaseUpdated) && slices.Equal(commits, want.Commits)
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
			branch, err := gitrepo.At(fakeRunner(t, tt.replies), workDir).ReadBranch(t.Context())

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
			replies[originsHead] = reply{err: errNoRef}

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
			replies[logCommits+tt.want+"..HEAD"] = reply{}

			if tt.want != "" {
				replies[baseAge+tt.want] = reply{out: []byte(baseUpdatedISO + "\n")}
			}

			// Act
			branch, err := gitrepo.At(fakeRunner(t, replies), workDir).ReadBranch(t.Context())

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
		originsHead:                   {out: []byte("origin/main\n")},
	}
}

func TestAnEmptyRepositoryStillReads(t *testing.T) {
	t.Parallel()

	// Act
	branch, err := gitrepo.At(fakeRunner(t, emptyRepository()), workDir).ReadBranch(t.Context())

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
	branch, err := gitrepo.At(fakeRunner(t, replies), workDir).ReadBranch(t.Context())

	// Assert
	if err != nil || !branch.Detached {
		t.Errorf("ReadBranch = %+v, %v, want it detached with no branch checked out", branch, err)
	}
}

func TestReadBranchReportsAnUnreadableRepository(t *testing.T) {
	t.Parallel()

	// Arrange
	replies := map[string]reply{
		showCurrentBranch: {err: errNotARepository},
		showToplevel:      {out: []byte("/work\n")},
	}

	// Act
	_, err := gitrepo.At(fakeRunner(t, replies), workDir).ReadBranch(t.Context())

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
				logFromMain: {out: []byte(firstHash + "\x00fix: \x1b]0;owned\x07subject\x00")},
			}),
			ahead:   2,
			behind:  1,
			commits: []gitrepo.Commit{{Hash: firstHash, Subject: "fix: subject"}},
		},
		// A subject may hold any byte but NUL, the separators this once read by
		// included, and it stays one subject of one commit.
		"a subject is one subject whatever bytes it holds": {
			replies: with(featureBranch(), map[string]reply{
				logFromMain: {
					out: []byte(firstHash + "\x00fix: one\x1e\nfacade\x1fsubject\x00" + secondHash + "\x00" + secondSubject + "\x00"),
				},
			}),
			ahead:  2,
			behind: 1,
			commits: []gitrepo.Commit{
				{Hash: firstHash, Subject: "fix: one\uFFFD\nfacade\uFFFDsubject"},
				{Hash: secondHash, Subject: secondSubject},
			},
		},
		// An abbreviated hash is hex. A record that opens with anything else
		// is not one git wrote, and is left out.
		"a record whose hash is not one is left out": {
			replies: with(featureBranch(), map[string]reply{
				logFromMain: {out: []byte("not a hash\x00fix: one\x00" + secondHash + "\x00" + secondSubject + "\x00")},
			}),
			ahead:   2,
			behind:  1,
			commits: []gitrepo.Commit{{Hash: secondHash, Subject: secondSubject}},
		},
		"a record with no hash at all is left out": {
			replies: with(featureBranch(), map[string]reply{
				logFromMain: {out: []byte("\x00fix: one\x00" + secondHash + "\x00" + secondSubject + "\x00")},
			}),
			ahead:   2,
			behind:  1,
			commits: []gitrepo.Commit{{Hash: secondHash, Subject: secondSubject}},
		},
		"a hash with no subject after it is left out": {
			replies: with(featureBranch(), map[string]reply{
				logFromMain: {out: []byte(firstHash + "\x00fix: one\x00" + secondHash)},
			}),
			ahead:   2,
			behind:  1,
			commits: []gitrepo.Commit{{Hash: firstHash, Subject: "fix: one"}},
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Act
			branch, err := gitrepo.At(fakeRunner(t, tt.replies), workDir).ReadBranch(t.Context())

			// Assert
			if err != nil || branch.Ahead != tt.ahead || branch.Behind != tt.behind ||
				!slices.Equal(branch.Commits, tt.commits) {
				t.Errorf("ReadBranch = %+v, %v; want ahead %d, behind %d, commits %+v",
					branch, err, tt.ahead, tt.behind, tt.commits)
			}
		})
	}
}

func TestReadBranchReportsADirectoryOutsideARepository(t *testing.T) {
	t.Parallel()

	// Arrange
	// The branch read fails and the work-tree probe fails too: this is no
	// repository, so the sentinel says so however git worded it.
	replies := map[string]reply{
		showCurrentBranch: {err: errNotARepository},
		showToplevel:      {err: errNotARepository},
	}

	// Act
	_, err := gitrepo.At(fakeRunner(t, replies), workDir).ReadBranch(t.Context())

	// Assert
	if !errors.Is(err, gitrepo.ErrNotARepository) {
		t.Errorf("ReadBranch returned %v, want ErrNotARepository", err)
	}
}

func TestBaseNameStripsTheRemote(t *testing.T) {
	t.Parallel()

	cases := map[string]struct{ base, want string }{
		"a remote branch": {base: originMain, want: localMain},
		"a nested branch": {base: "origin/release/1.0", want: "release/1.0"},
		"a local base":    {base: localMain, want: localMain},
		"no base at all":  {base: "", want: ""},
		"another remote":  {base: "upstream/develop", want: "develop"},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Act & Assert
			if got := (gitrepo.Branch{Base: tt.base}).BaseName(); got != tt.want {
				t.Errorf("BaseName(%q) = %q, want %q", tt.base, got, tt.want)
			}
		})
	}
}
