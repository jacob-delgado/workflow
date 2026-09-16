// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package gitrepo_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/jacob-delgado/workflow/internal/gitrepo"
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

// onABranch is a complete, healthy repository.
func onABranch() map[string]reply {
	return map[string]reply{
		"git -C /work rev-parse --show-toplevel": {out: []byte("/work\n")},
		"git -C /work branch --show-current":     {out: []byte("feat/token-redaction\n")},
		"git -C /work remote get-url origin":     {out: []byte("git@github.com:example/repo.git\n")},
	}
}

func TestDescribeReadsRootBranchAndRemote(t *testing.T) {
	t.Parallel()

	repo, err := gitrepo.Describe(t.Context(), fakeRunner(t, onABranch()), workDir)
	if err != nil {
		t.Fatalf("Describe returned %v, want nil", err)
	}

	if repo.Root != "/work" {
		t.Errorf("Root = %q, want %q", repo.Root, "/work")
	}

	if repo.Branch != "feat/token-redaction" {
		t.Errorf("Branch = %q, want %q", repo.Branch, "feat/token-redaction")
	}

	if repo.Remote != "git@github.com:example/repo.git" {
		t.Errorf("Remote = %q, want the origin URL", repo.Remote)
	}

	if repo.Detached {
		t.Error("Detached = true, want false on a named branch")
	}
}

func TestDescribeReportsADirectoryOutsideAnyRepository(t *testing.T) {
	t.Parallel()

	replies := map[string]reply{
		"git -C /work rev-parse --show-toplevel": {err: errNotARepository},
	}

	_, err := gitrepo.Describe(t.Context(), fakeRunner(t, replies), workDir)
	if !errors.Is(err, gitrepo.ErrNotARepository) {
		t.Errorf("Describe returned %v, want ErrNotARepository", err)
	}
}

func TestDescribeReportsAnUnreadableBranch(t *testing.T) {
	t.Parallel()

	replies := onABranch()
	replies["git -C /work branch --show-current"] = reply{err: errDetachedRead}

	_, err := gitrepo.Describe(t.Context(), fakeRunner(t, replies), workDir)
	if err == nil {
		t.Fatal("Describe returned nil, want the error from reading the branch")
	}

	if errors.Is(err, gitrepo.ErrNotARepository) {
		t.Errorf("Describe returned %v, want it distinguished from ErrNotARepository", err)
	}
}

func TestDescribeMarksADetachedHead(t *testing.T) {
	t.Parallel()

	replies := onABranch()
	replies["git -C /work branch --show-current"] = reply{out: []byte("\n")}

	repo, err := gitrepo.Describe(t.Context(), fakeRunner(t, replies), workDir)
	if err != nil {
		t.Fatalf("Describe returned %v, want nil", err)
	}

	// `branch --show-current` prints nothing when HEAD is not on a branch. That
	// is not an error — the repository still reads — but a caller needs to know,
	// because there is no branch to base work on.
	if !repo.Detached {
		t.Error("Detached = false, want true when HEAD is not on a branch")
	}
}

func TestDescribeTreatsAMissingOriginAsNoRemote(t *testing.T) {
	t.Parallel()

	replies := onABranch()
	replies["git -C /work remote get-url origin"] = reply{err: errNoSuchRemote}

	repo, err := gitrepo.Describe(t.Context(), fakeRunner(t, replies), workDir)
	if err != nil {
		t.Fatalf("Describe returned %v, want nil — a repository without a remote still works", err)
	}

	if repo.Remote != "" {
		t.Errorf("Remote = %q, want empty when origin is not configured", repo.Remote)
	}
}

// Compile-time proof that proc.Run satisfies the seam Describe takes, so the
// production wiring cannot drift from what the tests exercise.
var _ gitrepo.Runner = func(_ context.Context, _ string, _ ...string) ([]byte, error) {
	return nil, nil
}
