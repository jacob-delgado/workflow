// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package demo_test

import (
	"testing"

	"github.com/jacob-delgado/workflow/internal/loop"
	"github.com/jacob-delgado/workflow/internal/messaging"
	"github.com/jacob-delgado/workflow/internal/seams"
)

// readyPull is the announcement the store tests claim: #128, made ready.
func readyPull() loop.Announced {
	return loop.Announced{Pull: 128, Moment: messaging.MomentReady}
}

// otherPull is an announcement claimed beside readyPull: #129, made ready.
func otherPull() loop.Announced {
	return loop.Announced{Pull: 129, Moment: messaging.MomentReady}
}

// claimed is the demo's store with readyPull claimed, its post not yet
// confirmed.
func claimed(t *testing.T) seams.Store {
	t.Helper()

	store := standingStill().Deps().Store

	claim, err := store.ClaimAnnounce(t.Context(), readyPull())
	if err != nil || claim.Kind != loop.ClaimTaken {
		t.Fatalf("claiming %+v = %+v, %v; want it taken", readyPull(), claim, err)
	}

	return store
}

// claimedBoth is the demo's store with readyPull and then otherPull claimed,
// neither post confirmed.
func claimedBoth(t *testing.T) seams.Store {
	t.Helper()

	store := claimed(t)

	claim, err := store.ClaimAnnounce(t.Context(), otherPull())
	if err != nil || claim.Kind != loop.ClaimTaken {
		t.Fatalf("claiming %+v = %+v, %v; want it taken", otherPull(), claim, err)
	}

	return store
}

// postedRecord reports whether the demo's records hold made, and whether
// they say its post was confirmed.
func postedRecord(t *testing.T, store seams.Store, made loop.Announced) (bool, bool) {
	t.Helper()

	records, err := store.Announced(t.Context())
	if err != nil {
		t.Fatalf("reading what was announced: %v", err)
	}

	for _, record := range records {
		if record.Made == made {
			return true, record.Posted
		}
	}

	return false, false
}

func TestADemoClaimReadsAsNotPostedUntilConfirmed(t *testing.T) {
	t.Parallel()

	// Arrange
	store := standingStill().Deps().Store

	// Act
	_, err := store.ClaimAnnounce(t.Context(), readyPull())

	// Assert
	held, posted := postedRecord(t, store, readyPull())
	if err != nil || !held || posted {
		t.Errorf("after a claim: %v, held %t, posted %t; want it held, not posted", err, held, posted)
	}
}

func TestADemoClaimReadsAsPostedOnceConfirmed(t *testing.T) {
	t.Parallel()

	// Arrange
	store := claimed(t)

	// Act
	err := store.ConfirmAnnounce(t.Context(), readyPull(), 1)

	// Assert
	held, posted := postedRecord(t, store, readyPull())
	if err != nil || !held || !posted {
		t.Errorf("after a confirm: %v, held %t, posted %t; want it held and posted", err, held, posted)
	}
}

func TestASecondDemoClaimSaysTheFirstsPostIsNotConfirmed(t *testing.T) {
	t.Parallel()

	// Arrange
	store := claimed(t)

	// Act
	second, err := store.ClaimAnnounce(t.Context(), readyPull())

	// Assert
	if err != nil || second.Kind != loop.ClaimHeld || second.Posted {
		t.Errorf("a second claim before a confirm = %+v, %v; want it held, not posted", second, err)
	}
}

func TestASecondDemoClaimSaysTheFirstsPostIsConfirmed(t *testing.T) {
	t.Parallel()

	// Arrange
	store := claimed(t)

	err := store.ConfirmAnnounce(t.Context(), readyPull(), 1)
	if err != nil {
		t.Fatalf("confirming: %v", err)
	}

	// Act
	second, err := store.ClaimAnnounce(t.Context(), readyPull())

	// Assert
	if err != nil || second.Kind != loop.ClaimHeld || !second.Posted {
		t.Errorf("a second claim after a confirm = %+v, %v; want it held and posted", second, err)
	}
}

func TestConfirmingADemoAnnouncementNeverClaimedRecordsNothing(t *testing.T) {
	t.Parallel()

	// Arrange
	store := standingStill().Deps().Store

	// Act
	err := store.ConfirmAnnounce(t.Context(), readyPull(), 1)

	// Assert
	held, _ := postedRecord(t, store, readyPull())
	if err != nil || held {
		t.Errorf("a confirm with no claim: %v, held %t; want nothing recorded", err, held)
	}
}

func TestConfirmingOneDemoClaimLeavesTheOthersUnposted(t *testing.T) {
	t.Parallel()

	// Arrange
	store := claimedBoth(t)

	// Act
	err := store.ConfirmAnnounce(t.Context(), otherPull(), 1)

	// Assert
	readyHeld, readyPosted := postedRecord(t, store, readyPull())
	otherHeld, otherPosted := postedRecord(t, store, otherPull())

	if err != nil || !readyHeld || readyPosted || !otherHeld || !otherPosted {
		t.Errorf("after confirming #129: %v; #128 held %t, posted %t; #129 held %t, posted %t; "+
			"want both held and #129 alone posted", err, readyHeld, readyPosted, otherHeld, otherPosted)
	}
}

func TestGivingUpOneDemoClaimKeepsTheOthers(t *testing.T) {
	t.Parallel()

	// Arrange
	store := claimedBoth(t)

	// Act
	err := store.ReleaseAnnounce(t.Context(), readyPull(), 1)

	// Assert
	readyHeld, _ := postedRecord(t, store, readyPull())
	otherHeld, _ := postedRecord(t, store, otherPull())

	if err != nil || readyHeld || !otherHeld {
		t.Errorf("after giving up #128: %v; #128 held %t, #129 held %t; want #129 alone held",
			err, readyHeld, otherHeld)
	}
}

func TestGivingUpAConfirmedDemoClaimKeepsItPosted(t *testing.T) {
	t.Parallel()

	// Arrange
	store := claimed(t)

	err := store.ConfirmAnnounce(t.Context(), readyPull(), 1)
	if err != nil {
		t.Fatalf("confirming: %v", err)
	}

	// Act
	err = store.ReleaseAnnounce(t.Context(), readyPull(), 1)

	// Assert
	held, posted := postedRecord(t, store, readyPull())
	if err != nil || !held || !posted {
		t.Errorf("giving up a confirmed claim: %v, held %t, posted %t; want it held and posted",
			err, held, posted)
	}
}
