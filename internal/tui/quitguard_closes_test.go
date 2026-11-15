// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

// The quit guard asks before a quit loses the post waiting for CI. Once that
// post stops waiting, there is nothing left to lose: the guard closes, and
// the interface stays, saying how the post went.

import (
	"testing"

	"github.com/jacob-delgado/workflow/internal/forge"
)

func TestTheQuitGuardClosesWhenTheWaitingPostWentOut(t *testing.T) {
	t.Parallel()

	// Arrange
	// w arms the post and begins a read, which finds CI passed; q opens the
	// guard before that read answers.
	repo := newWorld()
	repo.ci = []forge.CI{{State: forge.CIRunning}, {State: forge.CIPassed}}
	armedPost, read := pressed(t, typing(t, repo.live(t, 140, 40), "5", "p"), "w")
	guarded := typing(t, armedPost, "q")
	requireScreen(t, guarded.View().Content, "will be lost")

	// Act
	view := drain(t, guarded, read).View().Content

	// Assert
	refuseScreen(t, view, "will be lost")
	requireScreen(t, view, "announced to")

	if posts := repo.asked("post "); len(posts) != 1 {
		t.Errorf("post calls = %q, want the waiting post alone", posts)
	}
}

func TestTheQuitGuardClosesWhenTheWaitingPostIsDropped(t *testing.T) {
	t.Parallel()

	// Arrange
	// The read w begins finds the pull request closed without merging, so
	// the post is given up on while the guard is open.
	repo := newWorld()
	repo.ci = []forge.CI{{State: forge.CIRunning}}
	armedPost, read := pressed(t, typing(t, repo.live(t, 140, 40), "5", "p"), "w")
	guarded := typing(t, armedPost, "q")

	closePull(repo)

	// Act
	view := drain(t, guarded, read).View().Content

	// Assert
	refuseScreen(t, view, "will be lost")
	requireScreen(t, view, droppedPost)
}

func TestTheQuitGuardStaysOpenWhileThePostStillWaits(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := newWorld()
	repo.ci = []forge.CI{{State: forge.CIRunning}}
	armedPost, read := pressed(t, typing(t, repo.live(t, 140, 40), "5", "p"), "w")
	guarded := typing(t, armedPost, "q")

	// Act
	view := drain(t, guarded, read).View().Content

	// Assert
	requireScreen(t, view, "An announcement is waiting for CI and will be lost")
}

// closePull closes the world's pull request without merging it, from its
// next read on.
func closePull(repo *world) {
	repo.mu.Lock()
	defer repo.mu.Unlock()

	repo.pull.State = forge.StateClosed
}
