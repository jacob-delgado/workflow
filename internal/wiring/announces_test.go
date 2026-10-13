// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package wiring_test

import (
	"slices"
	"testing"

	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/loop"
	"github.com/jacob-delgado/workflow/internal/messaging"
	"github.com/jacob-delgado/workflow/internal/wiring"
)

// retiredMoment is the moment an earlier build stored for a red CI, which
// this build no longer makes.
const retiredMoment messaging.Moment = 2

func TestRecordedAnnouncementsKeepOnlyTheMomentsThisBuildMakes(t *testing.T) {
	// Arrange
	// A store an earlier build wrote holds a moment this one does not make; a
	// later moment added without a schemaVersion bump would read the same.
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_STATE_HOME", "")

	where := wiring.Workspace{Root: t.TempDir(), Remote: "https://github.com/org/repo.git"}
	store := wired(t, config.Default(), where, nil).Store
	stored := []loop.Announced{
		{Pull: 7, Moment: messaging.MomentReady},
		{Pull: 7, Moment: messaging.MomentMerged},
		{Pull: 7, Moment: retiredMoment},
	}

	for _, made := range stored {
		err := store.RecordAnnounce(t.Context(), made)
		if err != nil {
			t.Fatalf("recording %+v: %v", made, err)
		}
	}

	// Act
	read := store.Announced(t.Context())

	// Assert
	if len(read) != 2 || !slices.Contains(read, stored[0]) || !slices.Contains(read, stored[1]) {
		t.Errorf("read back %+v, want only the ready and merged announcements", read)
	}
}
