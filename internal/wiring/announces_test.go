// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package wiring_test

import (
	"slices"
	"testing"

	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/loop"
	"github.com/jacob-delgado/workflow/internal/messaging"
	"github.com/jacob-delgado/workflow/internal/seams"
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
		announce(t, store, made)
	}

	// Act
	read, err := store.Announced(t.Context())

	// Assert
	made := madeOf(read)
	if err != nil || len(made) != 2 || !slices.Contains(made, stored[0]) || !slices.Contains(made, stored[1]) {
		t.Errorf("read back %+v, %v; want only the ready and merged announcements", read, err)
	}
}

// announce claims made in store and confirms its post, as a delivery does,
// failing the test when the store will not.
func announce(t *testing.T, store seams.Store, made loop.Announced) {
	t.Helper()

	claim, err := store.ClaimAnnounce(t.Context(), made)
	if err != nil || claim.Kind != loop.ClaimTaken {
		t.Fatalf("claiming %+v: %+v, %v", made, claim, err)
	}

	err = store.ConfirmAnnounce(t.Context(), made, claim.Generation)
	if err != nil {
		t.Fatalf("confirming %+v: %v", made, err)
	}
}

// madeOf is what each record made.
func madeOf(records []loop.AnnounceRecord) []loop.Announced {
	made := make([]loop.Announced, 0, len(records))
	for _, record := range records {
		made = append(made, record.Made)
	}

	return made
}
