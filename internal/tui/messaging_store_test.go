// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

import (
	"testing"

	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/loop"
	"github.com/jacob-delgado/workflow/internal/messaging"
)

func TestAHeldPostIsNotSentWhenAnotherSessionAnnouncedIt(t *testing.T) {
	t.Parallel()

	// Arrange
	// A post waits for CI; meanwhile workflow announce, or the web, announces
	// the same moment, and the store records it.
	repo := newWorld()
	repo.ci = []forge.CI{{State: forge.CIRunning}, {State: forge.CIPassed}}
	queued, checkCI := pressed(t, typing(t, repo.live(t, 120, 40), "5", "p"), "w")
	repo.storedAnnounces = []loop.Announced{{Pull: 42, Moment: messaging.MomentReady}}

	// Act
	passed := drain(t, queued, checkCI).View().Content

	// Assert
	if calls := repo.asked("post "); len(calls) != 0 {
		t.Errorf("post calls = %q, want nothing posted a second time", calls)
	}

	requireScreen(t, passed, "already announced from another session; not posted again", "● announced")
}
