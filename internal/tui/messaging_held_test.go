// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

import (
	"testing"

	"github.com/jacob-delgado/workflow/internal/forge"
)

func TestAWaitingPostIsDroppedWhenItsPullRequestMerges(t *testing.T) {
	t.Parallel()

	// Arrange
	merging := newWorld()
	merging.ci = []forge.CI{{State: forge.CIRunning}}
	waiting := typing(t, merging.live(t, 120, 40), "5", "p", "w")
	merging.pull.State = forge.StateMerged

	// Act
	merged := typing(t, waiting, "2", "r", "5").View().Content

	// Assert
	if calls := merging.asked("post "); len(calls) != 0 {
		t.Errorf("posted %q for a pull request that merged before its CI passed", calls)
	}

	requireScreen(t, merged, "not announced: #42 merged before its CI passed")
}

func TestAWaitingPostIsDroppedWhenItsPullRequestCloses(t *testing.T) {
	t.Parallel()

	// Arrange
	closing := newWorld()
	closing.ci = []forge.CI{{State: forge.CIRunning}}
	waiting := typing(t, closing.live(t, 120, 40), "5", "p", "w")
	closing.pull.State = forge.StateClosed

	// Act
	closed := typing(t, waiting, "2", "r").View().Content

	// Assert
	if calls := closing.asked("post "); len(calls) != 0 {
		t.Errorf("posted %q for a pull request closed without merging", calls)
	}

	requireScreen(t, closed, droppedPost)
}
