// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package loop_test

import (
	"context"
	"testing"

	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/httpx"
	"github.com/jacob-delgado/workflow/internal/loop"
	"github.com/jacob-delgado/workflow/internal/messaging"
)

// unkeptMemory is a store that keeps nothing: every claim is ClaimNotKept.
func unkeptMemory() loop.AnnounceMemory {
	return loop.AnnounceMemory{
		Claim: func(context.Context, loop.Announced) (loop.Claim, error) {
			return loop.Claim{Kind: loop.ClaimNotKept}, nil
		},
	}
}

// claimedAs is a memory whose every claim is of kind.
func claimedAs(kind loop.ClaimKind) loop.AnnounceMemory {
	return loop.AnnounceMemory{
		Claim: func(context.Context, loop.Announced) (loop.Claim, error) {
			return loop.Claim{Kind: kind, Generation: 1}, nil
		},
	}
}

// deliveredCase is one way a delivery ends: the seams it goes out through,
// the memory it claims in, and the kind Deliver names.
type deliveredCase struct {
	seams  func(post func(context.Context, string, string) error) loop.DeliverSeams
	post   error
	memory func() loop.AnnounceMemory
	want   loop.Delivered
}

// deliveredCases are every return Deliver makes, each naming its kind.
func deliveredCases() map[string]deliveredCase {
	ledgerWith := func(l *ledger) func() loop.AnnounceMemory { return l.memory }
	failedCI := forge.CI{State: forge.CIFailed}
	gated := func(ci forge.CI, err error) func(func(context.Context, string, string) error) loop.DeliverSeams {
		return func(post func(context.Context, string, string) error) loop.DeliverSeams {
			return loop.DeliverSeams{Post: post, CheckCI: ciReading(ci, err)}
		}
	}
	unposted := func(func(context.Context, string, string) error) loop.DeliverSeams {
		return loop.DeliverSeams{Post: nil, CheckCI: ciReading(forge.CI{State: forge.CIPassed}, nil)}
	}

	return map[string]deliveredCase{
		"no way to post":  {seams: unposted, memory: ledgerWith(&ledger{}), want: loop.DeliveredFailed},
		"a check failing": {seams: gated(failedCI, nil), memory: ledgerWith(&ledger{}), want: loop.DeliveredCIFailed},
		"CI could not be read": {
			seams: gated(forge.CI{}, errSeam), memory: ledgerWith(&ledger{}), want: loop.DeliveredCIUnread,
		},
		"a claim not taken": {
			seams: passing, memory: ledgerWith(&ledger{claimErr: errSeam}), want: loop.DeliveredNotClaimed,
		},
		"a claim of no kind known": {
			seams: passing, memory: func() loop.AnnounceMemory { return claimedAs(loop.ClaimKind(99)) },
			want: loop.DeliveredNotClaimed,
		},
		"a claim held elsewhere": {
			seams: passing, memory: func() loop.AnnounceMemory { return claimedAs(loop.ClaimHeld) },
			want: loop.DeliveredElsewhere,
		},
		"posted": {seams: passing, memory: ledgerWith(&ledger{}), want: loop.DeliveredPosted},
		"posted, not confirmed": {
			seams: passing, memory: ledgerWith(&ledger{confirmErr: errSeam}), want: loop.DeliveredUnconfirmed,
		},
		"posted unguarded": {seams: passing, memory: unkeptMemory, want: loop.DeliveredPosted},
		"refused by the service": {
			seams: passing, post: messaging.PostRefusedError{Reason: "channel_not_found"}, memory: ledgerWith(&ledger{}),
			want: loop.DeliveredRefused,
		},
		"rate limited": {
			seams: passing, post: httpx.ErrRateLimited, memory: ledgerWith(&ledger{}), want: loop.DeliveredDeferred,
		},
		"rate limited, its claim kept for want of a release": {
			seams: passing, post: httpx.ErrRateLimited, memory: ledgerWith(&ledger{releaseErr: errSeam}),
			want: loop.DeliveredUncertain,
		},
		"may have gone out": {
			seams: passing, post: messaging.ErrUnreachable, memory: ledgerWith(&ledger{}), want: loop.DeliveredUncertain,
		},
		"refused, unguarded": {
			seams: passing, post: messaging.ErrNoCredential, memory: unkeptMemory, want: loop.DeliveredFailed,
		},
		"rate limited, unguarded": {
			seams: passing, post: httpx.ErrRateLimited, memory: unkeptMemory, want: loop.DeliveredDeferredUnclaimed,
		},
	}
}

func TestDeliverNamesHowEachDeliveryWent(t *testing.T) {
	t.Parallel()

	for name, tt := range deliveredCases() {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			var sent deliveries

			seams := tt.seams(sent.post(tt.post))

			// Act
			got, err := loop.Deliver(t.Context(), seams, tt.memory(), ready())

			// Assert
			if got != tt.want {
				t.Errorf("Deliver = %d (%v), want %d", got, err, tt.want)
			}
		})
	}
}

func TestRedeliverNamesAPostRateLimitedUnderItsClaimDeferred(t *testing.T) {
	t.Parallel()

	// Arrange
	// Taken again at generation 2, which a release does not give up: the
	// claim stays, and the next try re-claims from it.
	var sent deliveries

	claims := &ledger{}
	_, _ = loop.Deliver(t.Context(), passing(sent.post(nil)), claims.memory(), ready())

	// Act
	got, err := loop.Redeliver(t.Context(), passing(sent.post(httpx.ErrRateLimited)), claims.memory(), ready(), 1)

	// Assert
	if got != loop.DeliveredDeferred {
		t.Errorf("Redeliver = %d (%v), want DeliveredDeferred", got, err)
	}
}

// everyDelivered is every kind a delivery can come to, by name.
func everyDelivered() map[string]loop.Delivered {
	return map[string]loop.Delivered{
		"failed": loop.DeliveredFailed, "posted": loop.DeliveredPosted, "unconfirmed": loop.DeliveredUnconfirmed,
		"a check failing": loop.DeliveredCIFailed, "CI unread": loop.DeliveredCIUnread,
		"not claimed": loop.DeliveredNotClaimed, "elsewhere": loop.DeliveredElsewhere,
		"refused": loop.DeliveredRefused, "deferred": loop.DeliveredDeferred,
		"deferred, unclaimed": loop.DeliveredDeferredUnclaimed, "uncertain": loop.DeliveredUncertain,
	}
}

func TestADeliveryWentOnlyPostedConfirmedOrNot(t *testing.T) {
	t.Parallel()

	for name, kind := range everyDelivered() {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			want := kind == loop.DeliveredPosted || kind == loop.DeliveredUnconfirmed

			// Act & Assert
			if got := kind.Went(); got != want {
				t.Errorf("Went() = %t, want %t", got, want)
			}
		})
	}
}
