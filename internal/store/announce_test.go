// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package store_test

import (
	"testing"

	"github.com/jacob-delgado/workflow/internal/store"
)

func TestAnAnnouncementRoundTrips(t *testing.T) {
	t.Parallel()

	// Arrange
	kept := store.New(t.TempDir(), false)
	announce := store.Announce{Pull: 42, Moment: 1}

	// Act
	err := kept.RecordAnnounce(t.Context(), repo, announce, theTime())
	if err != nil {
		t.Fatalf("RecordAnnounce returned %v, want nil", err)
	}

	got, err := kept.Announces(t.Context(), repo)

	// Assert
	if err != nil || len(got) != 1 || got[0] != announce {
		t.Errorf("Announces = %v, %v; want the recorded announcement", got, err)
	}
}

func TestAnAnnouncementIsKeptPerMoment(t *testing.T) {
	t.Parallel()

	// Arrange
	// The same pull request announced at two moments — opened, then merged — is
	// two records, so neither is offered twice.
	kept := store.New(t.TempDir(), false)
	_ = kept.RecordAnnounce(t.Context(), repo, store.Announce{Pull: 42, Moment: 1}, theTime())
	_ = kept.RecordAnnounce(t.Context(), repo, store.Announce{Pull: 42, Moment: 3}, theTime())

	// Act
	got, err := kept.Announces(t.Context(), repo)

	// Assert
	if err != nil || len(got) != 2 {
		t.Errorf("Announces = %v, %v; want both moments recorded", got, err)
	}
}

func TestADisabledStoreRecordsNoAnnouncement(t *testing.T) {
	t.Parallel()

	// Arrange
	off := store.New(t.TempDir(), true)

	// Act
	err := off.RecordAnnounce(t.Context(), repo, store.Announce{Pull: 42, Moment: 1}, theTime())
	got, _ := off.Announces(t.Context(), repo)

	// Assert
	if err != nil || len(got) != 0 {
		t.Errorf("a disabled store returned %v, %v; want it to no-op", got, err)
	}
}
