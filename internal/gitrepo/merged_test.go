// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package gitrepo_test

import (
	"context"
	"errors"
	"maps"
	"slices"
	"strings"
	"testing"

	"github.com/jacob-delgado/workflow/internal/gitrepo"
	"github.com/jacob-delgado/workflow/internal/proc"
)

// mergedCommit is the commit a branch's pull request merged at, baseBranch
// the base remote's branch it merged into, and the commands that ask whether
// this clone holds each, how many commits the branch has past the merged one,
// how many of them the base does not hold, and the newest of those.
const (
	mergedCommit   = "4b825dc642cb6eb9a060e54bf8d69288fbee4904"
	baseBranch     = "origin/main"
	holdsMerged    = "git -C /work rev-parse --verify --quiet " + mergedCommit + "^{commit}"
	holdsBase      = "git -C /work rev-parse --verify --quiet refs/remotes/" + baseBranch + "^{commit}"
	countAfter     = "git -C /work rev-list --count " + mergedCommit + "..HEAD"
	countOwn       = countAfter + " ^refs/remotes/" + baseBranch
	logAfterMerged = "git -C /work log -z --max-count=5 --format=%h%x00%s " + mergedCommit + "..HEAD"
	logOwn         = logAfterMerged + " ^refs/remotes/" + baseBranch
)

// counted is git's count of n commits.
func counted(n string) reply {
	return reply{out: []byte(n + "\n")}
}

// errBadObject is git failing the look for a commit with exit 128, which says
// nothing of whether the clone holds it: only exit 1 does.
var errBadObject = &proc.ExitError{Program: gitProgram, Code: 128}

// errFetchDown is the fetch of the merged commit failing.
var errFetchDown = errors.New("fatal: unable to access the remote")

// notHeld is git's own "no such commit" to the look for the merged commit,
// exit 1 with nothing said, as it answers for a HEAD not made yet; any other
// failure, errNotARepository among them, says nothing either way.
func notHeld() reply {
	return noHead()
}

// commitsAfter is git's log of two commits made after the merged one.
func commitsAfter() reply {
	return reply{out: []byte("c0ffee1\x00feat: y\x00c0ffee0\x00fix: z\x00")}
}

// fetching is a fetch that answers err and counts how often it ran.
func fetching(err error) (func(context.Context) error, *int) {
	var ran int

	return func(context.Context) error {
		ran++

		return err
	}, &ran
}

// sequenced answers each command line with its replies in turn, the last
// repeated, and fails the test on any other: a probe asked twice can answer
// differently each time.
func sequenced(t *testing.T, replies map[string][]reply) gitrepo.Runner {
	t.Helper()

	asked := map[string]int{}

	return func(_ context.Context, name string, args ...string) ([]byte, error) {
		key := strings.Join(append([]string{name}, args...), " ")

		answers, ok := replies[key]
		if !ok {
			t.Errorf("git ran an unexpected command: %q", key)

			return nil, errUnexpectedCommand
		}

		answer := answers[min(asked[key], len(answers)-1)]
		asked[key]++

		return answer.out, answer.err
	}
}

func TestMadeAfterNamesTheNewestCommitsPastTheMergedOne(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := gitrepo.At(fakeRunner(t, map[string]reply{
		holdsMerged: {}, countAfter: counted("2"), holdsBase: {}, countOwn: counted("2"), logOwn: commitsAfter(),
	}), workDir)
	fetch, fetched := fetching(nil)

	// Act
	got, err := repo.MadeAfter(t.Context(), mergedCommit, baseBranch, fetch)

	// Assert
	want := []gitrepo.Commit{{Hash: "c0ffee1", Subject: "feat: y"}, {Hash: "c0ffee0", Subject: "fix: z"}}
	if err != nil || !slices.Equal(got.Latest, want) || got.Own != 2 || got.OnBase != 0 || *fetched != 0 {
		t.Errorf("MadeAfter = %+v, %v, fetching %d times; want %v, two of its own, fetching nothing",
			got, err, *fetched, want)
	}
}

func TestMadeAfterTellsItsOwnCommitsFromThoseTheBaseAlreadyHolds(t *testing.T) {
	t.Parallel()

	// Arrange
	// Squash-merged, then moved onto the newer base and committed to: past
	// the merged commit are the squash, the base's commits since, and one of
	// its own.
	own := reply{out: []byte("c0ffee9\x00feat: my new work\x00")}
	repo := gitrepo.At(fakeRunner(t, map[string]reply{
		holdsMerged: {}, countAfter: counted("8"), holdsBase: {}, countOwn: counted("1"), logOwn: own,
	}), workDir)
	fetch, _ := fetching(nil)

	// Act
	got, err := repo.MadeAfter(t.Context(), mergedCommit, baseBranch, fetch)

	// Assert
	want := gitrepo.AfterMerge{
		Latest: []gitrepo.Commit{{Hash: "c0ffee9", Subject: "feat: my new work"}}, Own: 1, OnBase: 7, Base: baseBranch,
	}
	if err != nil || !slices.Equal(got.Latest, want.Latest) || got.Own != want.Own || got.OnBase != want.OnBase ||
		got.Base != want.Base {
		t.Errorf("MadeAfter = %+v, %v; want %+v", got, err, want)
	}
}

func TestMadeAfterListsNothingWhereTheBaseHoldsEveryCommitPastTheMerge(t *testing.T) {
	t.Parallel()

	// Arrange
	// No log is asked for: fakeRunner fails on a command it was not given.
	repo := gitrepo.At(fakeRunner(t, map[string]reply{
		holdsMerged: {}, countAfter: counted("3"), holdsBase: {}, countOwn: counted("0"),
	}), workDir)
	fetch, _ := fetching(nil)

	// Act
	got, err := repo.MadeAfter(t.Context(), mergedCommit, baseBranch, fetch)

	// Assert
	if err != nil || len(got.Latest) != 0 || got.Own != 0 || got.OnBase != 3 || !got.Any() {
		t.Errorf("MadeAfter = %+v, %v; want three, all on the base, none listed", got, err)
	}
}

func TestMadeAfterCountsEveryCommitItsOwnWithoutABaseToTellThemApart(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		base    string
		replies map[string]reply
	}{
		"no base named": {replies: map[string]reply{}},
		"no such branch in this clone": {
			base: baseBranch, replies: map[string]reply{holdsBase: notHeld()},
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			replies := map[string]reply{holdsMerged: {}, countAfter: counted("2"), logAfterMerged: commitsAfter()}
			maps.Copy(replies, tt.replies)
			repo := gitrepo.At(fakeRunner(t, replies), workDir)
			fetch, _ := fetching(nil)

			// Act
			got, err := repo.MadeAfter(t.Context(), mergedCommit, tt.base, fetch)

			// Assert
			if err != nil || len(got.Latest) != 2 || got.Own != 2 || got.OnBase != 0 || got.Base != "" {
				t.Errorf("MadeAfter = %+v, %v; want two of its own, told from no base", got, err)
			}
		})
	}
}

func TestMadeAfterFindsNoneOnABranchAtOrBehindTheMergedCommit(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := gitrepo.At(fakeRunner(t, map[string]reply{holdsMerged: {}, countAfter: counted("0")}), workDir)
	fetch, _ := fetching(nil)

	// Act
	got, err := repo.MadeAfter(t.Context(), mergedCommit, baseBranch, fetch)

	// Assert
	if err != nil || got.Any() {
		t.Errorf("MadeAfter = %+v, %v; want none", got, err)
	}
}

func TestMadeAfterFetchesAMergedCommitThisCloneLacksFirst(t *testing.T) {
	t.Parallel()

	// Arrange
	// A commit pushed from elsewhere, which this clone never fetched.
	repo := gitrepo.At(sequenced(t, map[string][]reply{
		holdsMerged:    {notHeld(), {}},
		countAfter:     {counted("2")},
		holdsBase:      {notHeld()},
		logAfterMerged: {commitsAfter()},
	}), workDir)
	fetch, fetched := fetching(nil)

	// Act
	got, err := repo.MadeAfter(t.Context(), mergedCommit, baseBranch, fetch)

	// Assert
	if err != nil || len(got.Latest) != 2 || *fetched != 1 {
		t.Errorf("MadeAfter = %+v, %v, fetching %d times; want two commits, fetching once", got, err, *fetched)
	}
}

func TestMadeAfterKnowsOfNoneWhereTheForgeNamedNoCommit(t *testing.T) {
	t.Parallel()

	cases := map[string]string{
		"no commit named":    "",
		"a name, not a hash": "--orphan",
	}

	for name, merged := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			run, ran := recordingRunner(t, map[string]reply{})
			fetch, fetched := fetching(nil)

			// Act
			got, err := gitrepo.At(run, workDir).MadeAfter(t.Context(), merged, baseBranch, fetch)

			// Assert
			if err != nil || got.Any() || len(*ran) != 0 || *fetched != 0 {
				t.Errorf("MadeAfter = %v, %v, having run %q and fetched %d times; want none, nothing run",
					got, err, *ran, *fetched)
			}
		})
	}
}

func TestMadeAfterRefusesAMergedCommitItCannotFetch(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		holds    []reply
		fetchErr error
	}{
		"the fetch failed":                {holds: []reply{notHeld()}, fetchErr: errFetchDown},
		"the fetch did not bring it here": {holds: []reply{notHeld()}},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			repo := gitrepo.At(sequenced(t, map[string][]reply{holdsMerged: tt.holds}), workDir)
			fetch, _ := fetching(tt.fetchErr)

			// Act
			_, err := repo.MadeAfter(t.Context(), mergedCommit, baseBranch, fetch)

			// Assert
			if !errors.Is(err, gitrepo.ErrNotFetched) || !strings.Contains(err.Error(), mergedCommit[:12]) {
				t.Errorf("MadeAfter = %v, want ErrNotFetched naming %s", err, mergedCommit[:12])
			}

			if tt.fetchErr != nil && !errors.Is(err, tt.fetchErr) {
				t.Errorf("MadeAfter = %v, want the fetch's own failure kept", err)
			}
		})
	}
}

func TestMadeAfterSaysWhenGitCouldNotAnswer(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		replies map[string][]reply
		want    error
	}{
		// Not "no such commit": git said nothing either way, so nothing is
		// fetched on a guess.
		"the look for the merged commit failed": {
			replies: map[string][]reply{holdsMerged: {{err: errNotARepository}}, showToplevel: {{out: []byte("/work\n")}}},
			want:    errNotARepository,
		},
		"the look for the merged commit failed otherwise than not finding it": {
			replies: map[string][]reply{
				holdsMerged: {{err: errBadObject}}, showToplevel: {{out: []byte("/work\n")}},
			},
			want: errBadObject,
		},
		"the look for the merged commit timed out": {
			replies: map[string][]reply{holdsMerged: {{err: proc.ErrTimedOut}}}, want: proc.ErrTimedOut,
		},
		"the look after the fetch failed": {
			replies: map[string][]reply{
				holdsMerged: {notHeld(), {err: errNotARepository}}, showToplevel: {{out: []byte("/work\n")}},
			},
			want: errNotARepository,
		},
		"the commits could not be counted": {
			replies: map[string][]reply{holdsMerged: {{}}, countAfter: {{err: errStepFailed}}}, want: errStepFailed,
		},
		"git's count was not a number": {
			replies: map[string][]reply{holdsMerged: {{}}, countAfter: {counted("many")}}, want: gitrepo.ErrNotACount,
		},
		"the look for the base failed": {
			replies: map[string][]reply{
				holdsMerged: {{}}, countAfter: {counted("2")}, holdsBase: {{err: proc.ErrTimedOut}},
			},
			want: proc.ErrTimedOut,
		},
		"its own commits could not be counted": {
			replies: map[string][]reply{
				holdsMerged: {{}}, countAfter: {counted("2")}, holdsBase: {{}}, countOwn: {{err: errStepFailed}},
			},
			want: errStepFailed,
		},
		"its own commits could not be listed": {
			replies: map[string][]reply{
				holdsMerged: {{}}, countAfter: {counted("2")}, holdsBase: {{}}, countOwn: {counted("2")},
				logOwn: {{err: errStepFailed}},
			},
			want: errStepFailed,
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			repo := gitrepo.At(sequenced(t, tt.replies), workDir)
			fetch, _ := fetching(nil)

			// Act
			_, err := repo.MadeAfter(t.Context(), mergedCommit, baseBranch, fetch)

			// Assert
			if !errors.Is(err, tt.want) || errors.Is(err, gitrepo.ErrNotFetched) {
				t.Errorf("MadeAfter = %v, want %v, and not taken for a commit it could not fetch", err, tt.want)
			}
		})
	}
}

func TestFetchRefCommandFetchesOneRefWritingNoneHere(t *testing.T) {
	t.Parallel()

	// Act
	command := gitrepo.FetchRefCommand(workDir, "upstream", "refs/pull/7/head")

	// Assert
	want := []string{"fetch", "--no-tags", "--no-write-fetch-head", "--", "upstream", "refs/pull/7/head"}
	if command.Dir != workDir || command.Name != gitProgram || !slices.Equal(command.Args, want) ||
		!slices.Contains(command.Env, "GIT_TERMINAL_PROMPT=0") {
		t.Errorf("FetchRefCommand = %+v, want git %q in %s with prompts off", command, want, workDir)
	}
}
