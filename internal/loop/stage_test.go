// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package loop_test

import (
	"errors"
	"slices"
	"testing"

	"github.com/jacob-delgado/workflow/internal/gitrepo"
	"github.com/jacob-delgado/workflow/internal/loop"
)

// errLocked is a stage git refuses, as it does while another git holds the
// index lock.
var errLocked = errors.New("fatal: Unable to create '.git/index.lock'")

// partlyStagedEdit is a file with some of its edits in the index and more in
// the work tree.
func partlyStagedEdit() gitrepo.Change {
	return gitrepo.Change{Path: "log.go", Staged: 'M', Unstaged: 'M'}
}

// conflictedFile is a file a merge left unresolved, which staging marks
// resolved.
func conflictedFile() gitrepo.Change {
	return gitrepo.Change{Path: "conflict.go", Staged: 'U', Unstaged: 'U'}
}

// workTree is one of each kind of change stage-all tells apart, a wholly staged
// file first.
func workTree() []gitrepo.Change {
	whollyStaged := gitrepo.Change{Path: "redact.go", Staged: 'M', Unstaged: ' '}

	return []gitrepo.Change{whollyStaged, partlyStagedEdit(), unstagedEdit(), untrackedFile(), conflictedFile()}
}

// recordingStage is a stage that records each path it was asked to stage and
// fails for the paths in refused.
func recordingStage(staged *[]string, refused ...string) func(gitrepo.Change) error {
	return func(change gitrepo.Change) error {
		*staged = append(*staged, change.Path)

		if slices.Contains(refused, change.Path) {
			return errLocked
		}

		return nil
	}
}

// stageablePaths is where workTree's stageable changes are, in its order: the
// rest of a partly staged file, an edit, an untracked file and a conflict.
func stageablePaths() []string {
	return []string{partlyStagedEdit().Path, unstagedEdit().Path, untrackedFile().Path, conflictedFile().Path}
}

func TestStageableIsEveryChangeTheIndexDoesNotHoldYet(t *testing.T) {
	t.Parallel()

	// Act
	pending := loop.Stageable(workTree())

	// Assert
	paths := make([]string, 0, len(pending))
	for _, change := range pending {
		paths = append(paths, change.Path)
	}

	if want := stageablePaths(); !slices.Equal(paths, want) {
		t.Errorf("Stageable = %q, want %q, and not what is wholly staged", paths, want)
	}
}

func TestStageAllStagesEveryStageableChange(t *testing.T) {
	t.Parallel()

	// Arrange
	var staged []string

	// Act
	err := loop.StageAll(workTree(), recordingStage(&staged))

	// Assert
	if want := stageablePaths(); err != nil || !slices.Equal(staged, want) {
		t.Errorf("StageAll = %v, staged %q; want %q staged and no error", err, staged, want)
	}
}

func TestStageAllGoesOnPastAFileGitRefuses(t *testing.T) {
	t.Parallel()

	// Arrange
	var staged []string

	// Act
	err := loop.StageAll(workTree(), recordingStage(&staged, unstagedEdit().Path))

	// Assert
	if !errors.Is(err, errLocked) {
		t.Errorf("StageAll = %v, want the refusal carried back", err)
	}

	if want := stageablePaths(); !slices.Equal(staged, want) {
		t.Errorf("staged %q, want %q: every file tried, past the refused one", staged, want)
	}
}

func TestStageAllWithoutAStageSeamIsUnavailable(t *testing.T) {
	t.Parallel()

	// Act
	err := loop.StageAll(workTree(), nil)

	// Assert
	if !errors.Is(err, loop.ErrStagingUnavailable) {
		t.Errorf("StageAll = %v, want %v", err, loop.ErrStagingUnavailable)
	}
}
