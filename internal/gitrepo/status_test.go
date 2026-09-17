// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package gitrepo_test

import (
	"errors"
	"slices"
	"testing"

	"github.com/jacob-delgado/workflow/internal/gitrepo"
)

// errIndexLocked stands in for git refusing to touch a locked index.
var errIndexLocked = errors.New("fatal: Unable to create '.git/index.lock': File exists")

// renamedFrom and renamedTo are where the renamed file in these fixtures was,
// and is.
const (
	renamedFrom = "a.txt"
	renamedTo   = "b.txt"
)

// statusCommand is the exact status invocation Status runs.
const statusCommand = "git -C /work status --porcelain=v1 -z --untracked-files=all"

func TestStatusReadsTheWorkTree(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		answer  reply
		want    []gitrepo.Change
		wantErr error
	}{
		// Real porcelain output: a staged rename that was then edited, a
		// deletion not yet staged, and an untracked file with a space in its name.
		"every kind of change": {
			answer: reply{out: []byte("RM b.txt\x00a.txt\x00 D gone.txt\x00?? new file.txt\x00")},
			want: []gitrepo.Change{
				{Path: renamedTo, OriginalPath: renamedFrom, Staged: 'R', Unstaged: 'M'},
				{Path: "gone.txt", OriginalPath: "", Staged: ' ', Unstaged: 'D'},
				{Path: "new file.txt", OriginalPath: "", Staged: '?', Unstaged: '?'},
			},
		},
		"a clean tree":  {answer: reply{}, want: nil},
		"git's failure": {answer: reply{err: errIndexLocked}, want: nil, wantErr: errIndexLocked},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			run := fakeRunner(t, map[string]reply{statusCommand: tt.answer})

			// Act
			changes, err := gitrepo.Status(t.Context(), run, workDir)

			// Assert
			if !errors.Is(err, tt.wantErr) || !slices.Equal(changes, tt.want) {
				t.Errorf("Status = %+v, %v; want %+v, %v", changes, err, tt.want, tt.wantErr)
			}
		})
	}
}

func TestAChangeSaysWhereItStands(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		change                       gitrepo.Change
		staged, unstaged, conflicted bool
	}{
		"staged only":   {change: gitrepo.Change{Staged: 'M', Unstaged: ' '}, staged: true},
		"unstaged only": {change: gitrepo.Change{Staged: ' ', Unstaged: 'M'}, unstaged: true},
		"both":          {change: gitrepo.Change{Staged: 'A', Unstaged: 'M'}, staged: true, unstaged: true},
		"untracked":     {change: gitrepo.Change{Staged: '?', Unstaged: '?'}, unstaged: true},
		"both modified": {change: gitrepo.Change{Staged: 'U', Unstaged: 'U'}, conflicted: true},
		"both added":    {change: gitrepo.Change{Staged: 'A', Unstaged: 'A'}, conflicted: true},
		"both deleted":  {change: gitrepo.Change{Staged: 'D', Unstaged: 'D'}, conflicted: true},
		"deleted by us": {change: gitrepo.Change{Staged: 'D', Unstaged: 'U'}, conflicted: true},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Act
			got := [3]bool{tt.change.IsStaged(), tt.change.HasUnstaged(), tt.change.Conflicted()}

			// Assert
			if want := [3]bool{tt.staged, tt.unstaged, tt.conflicted}; got != want {
				t.Errorf("staged, unstaged, conflicted = %v, want %v", got, want)
			}
		})
	}
}

func TestStageAddsEveryPathTheChangeTouches(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		change gitrepo.Change
		want   string
	}{
		// A rename is two paths: staging only the new one would leave the old
		// one's deletion behind.
		"a rename": {
			change: gitrepo.Change{Path: renamedTo, OriginalPath: renamedFrom},
			want:   "git -C /work add --all -- b.txt a.txt",
		},
		"a deletion": {change: gitrepo.Change{Path: "gone.txt", Unstaged: 'D'}, want: "git -C /work add --all -- gone.txt"},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			run, ran := recordingRunner(t, map[string]reply{tt.want: {}})

			// Act
			err := gitrepo.Stage(t.Context(), run, workDir, tt.change)

			// Assert
			if err != nil || !slices.Equal(*ran, []string{tt.want}) {
				t.Errorf("Stage ran %q and returned %v, want %q", *ran, err, tt.want)
			}
		})
	}
}

func TestUnstageTakesTheChangeOutOfTheIndexWithoutTouchingTheWorkTree(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		head   reply
		change gitrepo.Change
		want   string
	}{
		"after a commit, the index is restored from HEAD": {
			head:   reply{out: []byte("abc123\n")},
			change: gitrepo.Change{Path: renamedTo, OriginalPath: renamedFrom},
			want:   "git -C /work restore --staged -- b.txt a.txt",
		},
		// With no commit there is nothing to restore from: restore --staged fails.
		"before the first commit, the file leaves the index": {
			head:   reply{err: errDetachedRead},
			change: gitrepo.Change{Path: "new.go"},
			want:   "git -C /work rm --cached --quiet -- new.go",
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			run, ran := recordingRunner(t, map[string]reply{verifyHead: tt.head, tt.want: {}})

			// Act
			err := gitrepo.Unstage(t.Context(), run, workDir, tt.change)

			// Assert
			if err != nil || !slices.Equal(*ran, []string{verifyHead, tt.want}) {
				t.Errorf("Unstage ran %q and returned %v, want %q after checking HEAD", *ran, err, tt.want)
			}
		})
	}
}

func TestStageReportsGitsFailure(t *testing.T) {
	t.Parallel()

	// Arrange
	run := fakeRunner(t, map[string]reply{"git -C /work add --all -- x.go": {err: errIndexLocked}})

	// Act
	err := gitrepo.Stage(t.Context(), run, workDir, gitrepo.Change{Path: "x.go"})

	// Assert
	if !errors.Is(err, errIndexLocked) {
		t.Errorf("Stage returned %v, want git's error", err)
	}
}

func TestUnstageReportsGitsFailure(t *testing.T) {
	t.Parallel()

	// Arrange
	run := fakeRunner(t, map[string]reply{
		verifyHead:                              {out: []byte("abc\n")},
		"git -C /work restore --staged -- x.go": {err: errIndexLocked},
	})

	// Act
	err := gitrepo.Unstage(t.Context(), run, workDir, gitrepo.Change{Path: "x.go"})

	// Assert
	if !errors.Is(err, errIndexLocked) {
		t.Errorf("Unstage returned %v, want git's error", err)
	}
}
