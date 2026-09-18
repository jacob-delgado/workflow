// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package gitrepo_test

import (
	"context"
	"errors"
	"maps"
	"strings"
	"testing"

	"github.com/jacob-delgado/workflow/internal/gitrepo"
)

// Commands several fixtures answer.
const (
	showCurrentBranch = "git -C /work branch --show-current"
	verifyHead        = "git -C /work rev-parse --verify --quiet HEAD"
	showToplevel      = "git -C /work rev-parse --show-toplevel"
)

// workDir is the directory every fixture below describes.
const workDir = "/work"

// Sentinels for the fake runner. Declared here rather than built at the point of
// failure because err113 forbids the latter, in tests as well as production.
var (
	errUnexpectedCommand = errors.New("unexpected command")
	errNotARepository    = errors.New("fatal: not a git repository")
	errNoSuchRemote      = errors.New("fatal: no such remote 'origin'")
	errDetachedRead      = errors.New("fatal: ambiguous argument 'HEAD'")
	errNotIgnored        = errors.New("exit status 1")
)

// reply is one canned answer from the fake runner.
type reply struct {
	out []byte
	err error
}

// fakeRunner answers the exact command lines it is given and fails the test on
// any other, so a change in which git commands Describe runs cannot pass
// unnoticed.
func fakeRunner(t *testing.T, replies map[string]reply) gitrepo.Runner {
	t.Helper()

	return func(_ context.Context, name string, args ...string) ([]byte, error) {
		key := strings.Join(append([]string{name}, args...), " ")

		answer, ok := replies[key]
		if !ok {
			t.Errorf("Describe ran an unexpected command: %q", key)

			return nil, errUnexpectedCommand
		}

		return answer.out, answer.err
	}
}

// recordingRunner is fakeRunner that also records each command line it ran, so
// a test can require that the command it expects actually ran rather than only
// that nothing unexpected did.
func recordingRunner(t *testing.T, replies map[string]reply) (gitrepo.Runner, *[]string) {
	t.Helper()

	var ran []string

	answer := fakeRunner(t, replies)

	return func(ctx context.Context, name string, args ...string) ([]byte, error) {
		ran = append(ran, strings.Join(append([]string{name}, args...), " "))

		return answer(ctx, name, args...)
	}, &ran
}

// with is replies with some answers changed.
func with(replies map[string]reply, changes map[string]reply) map[string]reply {
	maps.Copy(replies, changes)

	return replies
}

// onABranch is a complete, healthy repository.
func onABranch() map[string]reply {
	return map[string]reply{
		showToplevel:                         {out: []byte("/work\n")},
		showCurrentBranch:                    {out: []byte("feat/token-redaction\n")},
		"git -C /work remote get-url origin": {out: []byte("git@github.com:example/repo.git\n")},
	}
}

func TestDescribeReadsTheRepository(t *testing.T) {
	t.Parallel()

	onBranch := gitrepo.Repo{
		Root: workDir, Branch: "feat/token-redaction", Remote: "git@github.com:example/repo.git", Detached: false,
	}

	cases := map[string]struct {
		replies map[string]reply
		want    gitrepo.Repo
	}{
		"its root, branch and remote": {replies: onABranch(), want: onBranch},
		// `branch --show-current` prints nothing when HEAD is not on a branch.
		// That is not an error — the repository still reads — but a caller needs
		// to know, because there is no branch to base work on.
		"a detached head": {
			replies: with(onABranch(), map[string]reply{showCurrentBranch: {out: []byte("\n")}}),
			want:    gitrepo.Repo{Root: workDir, Branch: "", Remote: onBranch.Remote, Detached: true},
		},
		// A repository without a remote still works.
		"no origin": {
			replies: with(onABranch(), map[string]reply{"git -C /work remote get-url origin": {err: errNoSuchRemote}}),
			want:    gitrepo.Repo{Root: workDir, Branch: onBranch.Branch, Remote: "", Detached: false},
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Act
			repo, err := gitrepo.At(fakeRunner(t, tt.replies), workDir).Describe(t.Context())

			// Assert
			if err != nil || repo != tt.want {
				t.Errorf("Describe = %+v, %v; want %+v", repo, err, tt.want)
			}
		})
	}
}

func TestDescribeReportsWhatItCannotRead(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		replies        map[string]reply
		want           error
		notARepository bool
	}{
		"a directory outside any repository": {
			replies:        map[string]reply{showToplevel: {err: errNotARepository}},
			want:           gitrepo.ErrNotARepository,
			notARepository: true,
		},
		// An unreadable branch is git's error, and not mistaken for being outside
		// a repository.
		"an unreadable branch": {
			replies:        with(onABranch(), map[string]reply{showCurrentBranch: {err: errDetachedRead}}),
			want:           errDetachedRead,
			notARepository: false,
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Act
			_, err := gitrepo.At(fakeRunner(t, tt.replies), workDir).Describe(t.Context())

			// Assert
			if !errors.Is(err, tt.want) || errors.Is(err, gitrepo.ErrNotARepository) != tt.notARepository {
				t.Errorf("Describe returned %v, want %v (ErrNotARepository: %v)", err, tt.want, tt.notARepository)
			}
		})
	}
}

// Compile-time proof that proc.Run satisfies the seam Describe takes, so the
// production wiring cannot drift from what the tests exercise.
var _ gitrepo.Runner = func(_ context.Context, _ string, _ ...string) ([]byte, error) {
	return nil, nil
}
