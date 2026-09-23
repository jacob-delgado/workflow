// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package loop_test

import (
	"errors"
	"testing"

	"github.com/jacob-delgado/workflow/internal/gitrepo"
	"github.com/jacob-delgado/workflow/internal/loop"
)

// stagedEdit is a change in the index, waiting to be committed.
func stagedEdit() gitrepo.Change {
	return gitrepo.Change{Path: "main.go", Staged: 'M', Unstaged: ' '}
}

// unstagedEdit is a change in the work tree that is not yet in the index.
func unstagedEdit() gitrepo.Change {
	return gitrepo.Change{Path: "main.go", Staged: ' ', Unstaged: 'M'}
}

// untrackedFile is a new file git does not track yet.
func untrackedFile() gitrepo.Change {
	return gitrepo.Change{Path: "notes.txt", Staged: '?', Unstaged: '?'}
}

func TestRefuseDirtyNamesTheGuard(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		changes []gitrepo.Change
		want    error
	}{
		"a clean tree":          {changes: nil, want: nil},
		"a staged change":       {changes: []gitrepo.Change{stagedEdit()}, want: loop.ErrDirtyTree},
		"an unstaged change":    {changes: []gitrepo.Change{unstagedEdit()}, want: loop.ErrDirtyTree},
		"only an untracked one": {changes: []gitrepo.Change{untrackedFile()}, want: loop.ErrDirtyTree},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Act
			err := loop.RefuseDirty(tt.changes)

			// Assert
			if !errors.Is(err, tt.want) {
				t.Errorf("RefuseDirty = %v, want %v", err, tt.want)
			}
		})
	}
}

func TestRefuseNothingStagedNamesTheGuard(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		changes []gitrepo.Change
		want    error
	}{
		"a clean tree": {changes: nil, want: loop.ErrNothingStaged},
		"only unstaged changes": {
			changes: []gitrepo.Change{unstagedEdit(), untrackedFile()}, want: loop.ErrNothingStaged,
		},
		"a staged change among them": {changes: []gitrepo.Change{unstagedEdit(), stagedEdit()}, want: nil},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Act
			err := loop.RefuseNothingStaged(tt.changes)

			// Assert
			if !errors.Is(err, tt.want) {
				t.Errorf("RefuseNothingStaged = %v, want %v", err, tt.want)
			}
		})
	}
}
