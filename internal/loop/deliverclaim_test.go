// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package loop_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/httpx"
	"github.com/jacob-delgado/workflow/internal/loop"
	"github.com/jacob-delgado/workflow/internal/messaging"
)

func TestAGateRefusalClaimsNothing(t *testing.T) {
	t.Parallel()

	// Arrange
	var sent deliveries

	failed := forge.CI{State: forge.CIFailed, Checks: []forge.Check{{Name: "lint", State: forge.CIFailed}}}
	seams := loop.DeliverSeams{Post: sent.post(nil), CheckCI: ciReading(failed, nil)}
	claims := &ledger{}

	// Act
	err := loop.Deliver(t.Context(), seams, claims.memory(), ready())

	// Assert
	var refusal loop.GateRefusalError
	if !errors.As(err, &refusal) || !errors.Is(err, loop.ErrCIFailed) || len(refusal.CI.Checks) != 1 {
		t.Errorf("Deliver = %v, want a gate refusal carrying the CI it read", err)
	}

	if claims.claims != 0 || len(sent.posted) != 0 {
		t.Errorf("claimed %d times and posted %q, want nothing claimed or posted", claims.claims, sent.posted)
	}
}

func TestAHeldClaimPostsNothing(t *testing.T) {
	t.Parallel()

	// Arrange
	// Another session claimed it first, and its post was never confirmed.
	var sent deliveries

	claims := &ledger{}
	_, _ = claims.claim(t.Context(), ready().Made())

	// Act
	err := loop.Deliver(t.Context(), passing(sent.post(nil)), claims.memory(), ready())

	// Assert
	var already loop.AnnouncedAlreadyError
	if !errors.As(err, &already) || already.Claim.Posted || !already.Claim.ClaimedAt.Equal(claimedAt()) {
		t.Errorf("Deliver = %v, want it announced already, unconfirmed since %v", err, claimedAt())
	}

	if len(sent.posted) != 0 {
		t.Errorf("posted %q over a claim another holds, want nothing", sent.posted)
	}
}

func TestAClaimTheStoreCannotTakePostsNothing(t *testing.T) {
	t.Parallel()

	// Arrange
	var sent deliveries

	claims := &ledger{claimErr: errSeam}

	// Act
	err := loop.Deliver(t.Context(), passing(sent.post(nil)), claims.memory(), ready())

	// Assert
	if !errors.Is(err, loop.ErrNotClaimed) || !errors.Is(err, errSeam) || len(sent.posted) != 0 {
		t.Errorf("Deliver = %v, posted %q; want ErrNotClaimed with why, and nothing posted", err, sent.posted)
	}
}

func TestAPostSurelyNotDeliveredGivesUpItsClaim(t *testing.T) {
	t.Parallel()

	cases := map[string]error{
		"a message refused":    messaging.PostRefusedError{Reason: "is_archived"},
		"a credential refused": fmt.Errorf("%w: invalid_auth", messaging.ErrRejected),
		"a rate limit":         httpx.ErrRateLimited,
		"no credential":        messaging.ErrNoCredential,
	}

	for name, refused := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			var sent deliveries

			claims := &ledger{}

			// Act
			err := loop.Deliver(t.Context(), passing(sent.post(refused)), claims.memory(), ready())

			// Assert
			if !errors.Is(err, loop.ErrNotDelivered) || !errors.Is(err, refused) {
				t.Errorf("Deliver = %v, want ErrNotDelivered with the service's reason", err)
			}

			if got := claims.standing(ready().Made()); got != loop.StandingNone {
				t.Errorf("after a post surely not delivered the claim stands %v, want it given up", got)
			}
		})
	}
}

func TestAPostThatMayHaveGoneOutKeepsItsClaim(t *testing.T) {
	t.Parallel()

	cases := map[string]error{
		"a server error": fmt.Errorf("%w: 502", messaging.ErrUnexpectedStatus),
		"no answer":      fmt.Errorf("%w: connection reset", messaging.ErrUnreachable),
		"its asker left": context.Canceled,
	}

	for name, unknown := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			var sent deliveries

			claims := &ledger{}

			// Act
			err := loop.Deliver(t.Context(), passing(sent.post(unknown)), claims.memory(), ready())

			// Assert
			if !errors.Is(err, loop.ErrPostUncertain) || !errors.Is(err, unknown) {
				t.Errorf("Deliver = %v, want ErrPostUncertain with the service's error", err)
			}

			if got := claims.standing(ready().Made()); got != loop.StandingUnconfirmed {
				t.Errorf("after a post that may have gone out the claim stands %v, want it kept, unconfirmed", got)
			}
		})
	}
}

func TestAClaimThatCannotBeGivenUpLeavesThePostUncertain(t *testing.T) {
	t.Parallel()

	// Arrange
	var sent deliveries

	claims := &ledger{releaseErr: errSeam}
	refused := messaging.PostRefusedError{Reason: "is_archived"}

	// Act
	err := loop.Deliver(t.Context(), passing(sent.post(refused)), claims.memory(), ready())

	// Assert
	if !errors.Is(err, loop.ErrPostUncertain) || !errors.Is(err, refused) || !errors.Is(err, errSeam) {
		t.Errorf("Deliver = %v, want ErrPostUncertain with the refusal and why the claim stayed", err)
	}
}

func TestAPostTheStoreCannotConfirmIsUnconfirmed(t *testing.T) {
	t.Parallel()

	// Arrange
	var sent deliveries

	claims := &ledger{confirmErr: errSeam}

	// Act
	err := loop.Deliver(t.Context(), passing(sent.post(nil)), claims.memory(), ready())

	// Assert
	why, unconfirmed := loop.UnconfirmedReason(fmt.Errorf("announcing: %w", err))
	if !errors.Is(err, loop.ErrUnconfirmed) || !unconfirmed || why != errSeam.Error() || len(sent.posted) != 1 {
		t.Errorf("Deliver = %v, posted %q; want it posted once, then ErrUnconfirmed with why", err, sent.posted)
	}

	if got := claims.standing(ready().Made()); got != loop.StandingUnconfirmed {
		t.Errorf("the claim stands %v, want it unconfirmed, so it is not offered again", got)
	}
}

func TestAClaimNeverConfirmedReadsUnconfirmed(t *testing.T) {
	t.Parallel()

	// Arrange
	// The claim is taken, and the process dies before its post is confirmed.
	claims := &ledger{}
	_, _ = claims.claim(t.Context(), ready().Made())
	records, _ := claims.recorded(t.Context())

	// Act
	got := loop.StandingOf(records, openPull(), true)

	// Assert
	if got != loop.StandingUnconfirmed {
		t.Errorf("StandingOf a claim never confirmed = %v, want it unconfirmed", got)
	}
}

func TestTwoDeliveriesOfOneAnnouncementPostOnce(t *testing.T) {
	t.Parallel()

	// Arrange
	var sent deliveries

	claims := &ledger{}
	first := loop.Deliver(t.Context(), passing(sent.post(nil)), claims.memory(), ready())

	// Act
	second := loop.Deliver(t.Context(), passing(sent.post(nil)), claims.memory(), ready())

	// Assert
	if first != nil || !errors.Is(second, loop.ErrAnnouncedAlready) || len(sent.posted) != 1 {
		t.Errorf("Deliver twice = %v, %v, posted %q; want one post, then announced already", first, second, sent.posted)
	}
}

func TestRedeliverPostsAgainFromTheGenerationSeen(t *testing.T) {
	t.Parallel()

	// Arrange
	// It was posted; two sessions both saw that and both ask to post again.
	var sent deliveries

	claims := &ledger{}
	_ = loop.Deliver(t.Context(), passing(sent.post(nil)), claims.memory(), ready())
	again := loop.Redeliver(t.Context(), passing(sent.post(nil)), claims.memory(), ready(), 1)

	// Act
	twice := loop.Redeliver(t.Context(), passing(sent.post(nil)), claims.memory(), ready(), 1)

	// Assert
	if again != nil || !errors.Is(twice, loop.ErrAnnouncedAlready) || len(sent.posted) != 2 {
		t.Errorf("Redeliver twice = %v, %v, posted %q; want one post again, then announced already",
			again, twice, sent.posted)
	}
}

func TestAStoreThatKeepsNothingPostsUnguarded(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		post error
		want error
	}{
		"a post that goes":   {post: nil, want: nil},
		"a post refused":     {post: messaging.ErrNoCredential, want: messaging.ErrNoCredential},
		"a post that may go": {post: messaging.ErrUnreachable, want: messaging.ErrUnreachable},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			var sent deliveries

			unkept := loop.AnnounceMemory{
				Claim: func(context.Context, loop.Announced) (loop.Claim, error) {
					return loop.Claim{Kind: loop.ClaimNotKept}, nil
				},
			}

			// Act
			err := loop.Deliver(t.Context(), passing(sent.post(tt.post)), unkept, ready())

			// Assert
			if !errors.Is(err, tt.want) || errors.Is(err, loop.ErrNotDelivered) || errors.Is(err, loop.ErrPostUncertain) ||
				len(sent.posted) != 1 {
				t.Errorf("Deliver = %v, posted %q; want it posted once, answering %v as the service said it",
					err, sent.posted, tt.want)
			}
		})
	}
}

func TestStandingOfReadsThePullRequestAtTheMomentItIsAt(t *testing.T) {
	t.Parallel()

	ready := loop.Announced{Pull: 9, Moment: messaging.MomentReady}
	posted := []loop.AnnounceRecord{{Made: ready, Generation: 1, ClaimedAt: claimedAt(), Posted: true}}
	begun := []loop.AnnounceRecord{{Made: ready, Generation: 1, ClaimedAt: claimedAt(), Posted: false}}
	closed := openPull()
	closed.State = forge.StateClosed

	cases := map[string]struct {
		records []loop.AnnounceRecord
		pull    forge.PullRequest
		found   bool
		want    loop.Standing
	}{
		"announced as ready":               {records: posted, pull: openPull(), found: true, want: loop.StandingPosted},
		"a post begun and never confirmed": {records: begun, pull: openPull(), found: true, want: loop.StandingUnconfirmed},
		"merged since it was announced": {
			records: posted, pull: forge.PullRequest{Number: 9, State: forge.StateMerged}, found: true,
		},
		"closed without merging, announced": {records: posted, pull: closed, found: true},
		"another pull request":              {records: posted, pull: forge.PullRequest{Number: 10}, found: true},
		"no pull request found":             {records: posted, pull: openPull()},
		"nothing announced":                 {pull: openPull(), found: true},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Act & Assert
			if got := loop.StandingOf(tt.records, tt.pull, tt.found); got != tt.want {
				t.Errorf("StandingOf(%+v, %t) = %v, want %v", tt.pull, tt.found, got, tt.want)
			}
		})
	}
}

func TestUnconfirmedReasonIsNoneForAnyOtherOutcome(t *testing.T) {
	t.Parallel()

	cases := map[string]error{
		"confirmed":           nil,
		"a post that fails":   errSeam,
		"announced elsewhere": loop.AnnouncedAlreadyError{},
	}

	for name, err := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Act
			why, unconfirmed := loop.UnconfirmedReason(err)

			// Assert
			if unconfirmed || why != "" {
				t.Errorf("UnconfirmedReason = %q, %t; want nothing: the store was not what failed", why, unconfirmed)
			}
		})
	}
}

func TestRecordsWithNoWayToReadThemAreNone(t *testing.T) {
	t.Parallel()

	// Act
	records, err := loop.AnnounceMemory{}.Records(t.Context())

	// Assert
	if err != nil || len(records) != 0 {
		t.Errorf("Records = %+v, %v; want none, and no error", records, err)
	}
}

func TestAnnouncedAlreadySaysWhetherThePostWasConfirmed(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		claim loop.Claim
		want  string
	}{
		"confirmed": {
			claim: loop.Claim{Kind: loop.ClaimHeld, Generation: 1, ClaimedAt: claimedAt(), Posted: true},
			want:  "the announcement was already made",
		},
		"begun and never confirmed": {
			claim: loop.Claim{Kind: loop.ClaimHeld, Generation: 1, ClaimedAt: claimedAt(), Posted: false},
			want:  "the announcement was already made: a post began at 2026-11-11T09:00:00Z and was not confirmed",
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Act
			said := loop.AnnouncedAlreadyError{Claim: tt.claim}.Error()

			// Assert
			if said != tt.want {
				t.Errorf("AnnouncedAlreadyError says %q, want %q", said, tt.want)
			}
		})
	}
}
