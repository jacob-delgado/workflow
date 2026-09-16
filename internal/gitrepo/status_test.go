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

func TestStatusReadsEveryKindOfChange(t *testing.T) {
	t.Parallel()

	// Real porcelain output: a staged rename that was then edited, a deletion
	// not yet staged, and an untracked file with a space in its name.
	replies := map[string]reply{
		statusCommand: {out: []byte("RM b.txt\x00a.txt\x00 D gone.txt\x00?? new file.txt\x00")},
	}

	changes, err := gitrepo.Status(t.Context(), fakeRunner(t, replies), workDir)
	if err != nil {
		t.Fatalf("Status returned %v, want nil", err)
	}

	want := []gitrepo.Change{
		{Path: renamedTo, OriginalPath: renamedFrom, Staged: 'R', Unstaged: 'M'},
		{Path: "gone.txt", OriginalPath: "", Staged: ' ', Unstaged: 'D'},
		{Path: "new file.txt", OriginalPath: "", Staged: '?', Unstaged: '?'},
	}

	if !slices.Equal(changes, want) {
		t.Errorf("Status = %+v, want %+v", changes, want)
	}
}

func TestStatusOfACleanTreeIsEmpty(t *testing.T) {
	t.Parallel()

	changes, err := gitrepo.Status(t.Context(), fakeRunner(t, map[string]reply{statusCommand: {}}), workDir)
	if err != nil || len(changes) != 0 {
		t.Errorf("Status = %+v, %v, want nothing", changes, err)
	}
}

func TestStatusReportsGitsFailure(t *testing.T) {
	t.Parallel()

	_, err := gitrepo.Status(t.Context(), fakeRunner(t, map[string]reply{statusCommand: {err: errIndexLocked}}), workDir)
	if !errors.Is(err, errIndexLocked) {
		t.Errorf("Status returned %v, want git's error", err)
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

			got := [3]bool{tt.change.IsStaged(), tt.change.HasUnstaged(), tt.change.Conflicted()}
			if want := [3]bool{tt.staged, tt.unstaged, tt.conflicted}; got != want {
				t.Errorf("staged, unstaged, conflicted = %v, want %v", got, want)
			}
		})
	}
}

func TestStageAddsEveryPathTheChangeTouches(t *testing.T) {
	t.Parallel()

	replies := map[string]reply{
		"git -C /work add --all -- b.txt a.txt": {},
		"git -C /work add --all -- gone.txt":    {},
	}
	run := fakeRunner(t, replies)

	// A rename is two paths: staging only the new one would leave the old one's
	// deletion behind.
	err := gitrepo.Stage(t.Context(), run, workDir, gitrepo.Change{Path: renamedTo, OriginalPath: renamedFrom})
	if err != nil {
		t.Errorf("Stage(rename) returned %v, want nil", err)
	}

	err = gitrepo.Stage(t.Context(), run, workDir, gitrepo.Change{Path: "gone.txt", Unstaged: 'D'})
	if err != nil {
		t.Errorf("Stage(deletion) returned %v, want nil", err)
	}
}

func TestUnstageRestoresTheIndexWithoutTouchingTheWorkTree(t *testing.T) {
	t.Parallel()

	replies := map[string]reply{
		verifyHead: {out: []byte("abc123\n")},
		"git -C /work restore --staged -- b.txt a.txt": {},
	}

	rename := gitrepo.Change{Path: renamedTo, OriginalPath: renamedFrom}

	err := gitrepo.Unstage(t.Context(), fakeRunner(t, replies), workDir, rename)
	if err != nil {
		t.Errorf("Unstage returned %v, want nil", err)
	}
}

func TestUnstageBeforeTheFirstCommitTakesTheFileOutOfTheIndex(t *testing.T) {
	t.Parallel()

	// With no commit there is nothing to restore from: restore --staged fails.
	replies := map[string]reply{
		verifyHead: {err: errDetachedRead},
		"git -C /work rm --cached --quiet -- new.go": {},
	}

	err := gitrepo.Unstage(t.Context(), fakeRunner(t, replies), workDir, gitrepo.Change{Path: "new.go"})
	if err != nil {
		t.Errorf("Unstage returned %v, want nil", err)
	}
}

func TestStagingReportsGitsFailure(t *testing.T) {
	t.Parallel()

	replies := map[string]reply{
		"git -C /work add --all -- x.go":        {err: errIndexLocked},
		verifyHead:                              {out: []byte("abc\n")},
		"git -C /work restore --staged -- x.go": {err: errIndexLocked},
	}
	run := fakeRunner(t, replies)

	err := gitrepo.Stage(t.Context(), run, workDir, gitrepo.Change{Path: "x.go"})
	if !errors.Is(err, errIndexLocked) {
		t.Errorf("Stage returned %v, want git's error", err)
	}

	err = gitrepo.Unstage(t.Context(), run, workDir, gitrepo.Change{Path: "x.go"})
	if !errors.Is(err, errIndexLocked) {
		t.Errorf("Unstage returned %v, want git's error", err)
	}
}
