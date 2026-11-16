// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package loop_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/jacob-delgado/workflow/internal/loop"
)

// scriptedClaims is a memory whose claims, first or taken again, answer the
// kinds of its script in turn, each at generation 1, and which notes the
// generation every confirmation was made at.
type scriptedClaims struct {
	script    []loop.ClaimKind
	claims    int
	reclaims  int
	confirmed []int
}

// next is the claim the script answers next.
func (s *scriptedClaims) next() loop.Claim {
	kind := s.script[min(s.claims+s.reclaims, len(s.script)-1)]

	return loop.Claim{Kind: kind, Generation: 1, ClaimedAt: claimedAt(), Posted: false}
}

// memory is the script as the memory a delivery claims and confirms through.
func (s *scriptedClaims) memory() loop.AnnounceMemory {
	return loop.AnnounceMemory{
		Claim: func(context.Context, loop.Announced) (loop.Claim, error) {
			claim := s.next()
			s.claims++

			return claim, nil
		},
		Reclaim: func(context.Context, loop.Announced, int) (loop.Claim, error) {
			claim := s.next()
			s.reclaims++

			return claim, nil
		},
		Confirm: func(_ context.Context, _ loop.Announced, generation int) error {
			s.confirmed = append(s.confirmed, generation)

			return nil
		},
	}
}

func TestAClaimGivenUpBeforeItWasReadIsClaimedOnceMore(t *testing.T) {
	t.Parallel()

	// Arrange
	// Another's claim met by the INSERT is given up before the SELECT reads
	// it; the claim asked once more is this caller's to take.
	var sent deliveries

	claims := &scriptedClaims{script: []loop.ClaimKind{loop.ClaimGone, loop.ClaimTaken}}

	// Act
	kind, err := loop.Deliver(t.Context(), passing(sent.post(nil)), claims.memory(), ready())

	// Assert
	if len(sent.posted) != 1 {
		t.Errorf("posted %d times, want 1: the claim taken the second time posts", len(sent.posted))
	}

	if kind != loop.DeliveredPosted || err != nil || fmt.Sprint(claims.confirmed) != "[1]" {
		t.Errorf("Deliver = %d (%v), confirmed at %v; want it posted, confirmed at generation 1",
			kind, err, claims.confirmed)
	}
}

func TestAClaimGivenUpTwicePostsNothing(t *testing.T) {
	t.Parallel()

	// Arrange
	var sent deliveries

	claims := &scriptedClaims{script: []loop.ClaimKind{loop.ClaimGone, loop.ClaimGone}}

	// Act
	kind, err := loop.Deliver(t.Context(), passing(sent.post(nil)), claims.memory(), ready())

	// Assert
	if kind != loop.DeliveredNotClaimed || !errors.Is(err, loop.ErrNotClaimed) || len(sent.posted) != 0 ||
		claims.claims != 2 {
		t.Errorf("Deliver = %d (%v), posted %d, claimed %d times; want it not claimed, nothing posted, "+
			"claimed twice", kind, err, len(sent.posted), claims.claims)
	}
}

func TestARedeliveryThatMeetsAClaimGivenUpPostsNothing(t *testing.T) {
	t.Parallel()

	// Arrange
	// A redelivery compares the generation alone, so claimed a second time
	// it could take over a fresh claim: it does not ask again.
	var sent deliveries

	claims := &scriptedClaims{script: []loop.ClaimKind{loop.ClaimGone, loop.ClaimTaken}}

	// Act
	kind, err := loop.Redeliver(t.Context(), passing(sent.post(nil)), claims.memory(), ready(), 1)

	// Assert
	if kind != loop.DeliveredNotClaimed || !errors.Is(err, loop.ErrNotClaimed) || len(sent.posted) != 0 ||
		claims.reclaims != 1 {
		t.Errorf("Redeliver = %d (%v), posted %d, claimed again %d times; want it not claimed, nothing posted, "+
			"claimed once", kind, err, len(sent.posted), claims.reclaims)
	}
}
