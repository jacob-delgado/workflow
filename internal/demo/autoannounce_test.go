// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package demo_test

import (
	"testing"

	"github.com/jacob-delgado/workflow/internal/demo"
)

func TestWithAutoAnnounceOffOpeningThePullRequestArmsNothing(t *testing.T) {
	t.Parallel()

	// Arrange
	served := servedWeb(t, demo.HoldCI(), demo.AutoAnnounceOff())

	// Act
	opened := served.opened()

	// Assert
	if opened.Announcement != nil || opened.NotArmed != nil {
		t.Errorf("opening armed %+v, not armed %v; want neither said", opened.Announcement, opened.NotArmed)
	}

	if queued := served.frame().QueuedAnnouncement; queued != nil {
		t.Errorf("the frame holds %+v queued, want no announcement waiting", queued)
	}
}
