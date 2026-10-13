// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package loop_test

import (
	"context"
	"errors"
	"testing"

	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/loop"
	"github.com/jacob-delgado/workflow/internal/messaging"
)

// ciReading is a CheckCI seam answering ci and err.
func ciReading(ci forge.CI, err error) func(context.Context, forge.PullRequest, string) (forge.CI, error) {
	return func(context.Context, forge.PullRequest, string) (forge.CI, error) { return ci, err }
}

// passing is the seams a delivery goes out through: post, after a CI read
// that says it passed.
func passing(post func(ctx context.Context, channel, text string) error) loop.DeliverSeams {
	return loop.DeliverSeams{Post: post, CheckCI: ciReading(forge.CI{State: forge.CIPassed}, nil)}
}

// readyText is what a ready-for-review delivery these tests send says.
const readyText = "ready"

// ready is the ready-for-review delivery of the open pull request, to #dev.
func ready() loop.Delivery {
	return loop.Delivery{
		Channel: devChannel, Text: readyText, Pull: openPull(), Head: headCommit, Moment: messaging.MomentReady,
	}
}

func TestDeliverRefusesAPullRequestACheckFailedOn(t *testing.T) {
	t.Parallel()

	// Arrange
	var sent deliveries

	seams := loop.DeliverSeams{Post: sent.post(nil), CheckCI: ciReading(forge.CI{State: forge.CIFailed}, nil)}

	// Act
	err := loop.Deliver(t.Context(), seams, sent.memory(), ready())

	// Assert
	if !errors.Is(err, loop.ErrCIFailed) || len(sent.posted) != 0 || len(sent.recorded) != 0 {
		t.Errorf("Deliver = %v, posted %q, recorded %+v; want ErrCIFailed and nothing posted or recorded",
			err, sent.posted, sent.recorded)
	}
}

func TestDeliverRefusesWhatItCannotConfirm(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		check func(context.Context, forge.PullRequest, string) (forge.CI, error)
		cause error
	}{
		"a CI read that fails": {check: ciReading(forge.CI{State: forge.CIPassed}, errSeam), cause: errSeam},
		"no way to read CI":    {check: nil, cause: loop.ErrCIUnread},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			var sent deliveries

			seams := loop.DeliverSeams{Post: sent.post(nil), CheckCI: tt.check}

			// Act
			err := loop.Deliver(t.Context(), seams, sent.memory(), ready())

			// Assert
			if !errors.Is(err, loop.ErrCIUnread) || !errors.Is(err, tt.cause) || len(sent.posted) != 0 {
				t.Errorf("Deliver = %v, posted %q; want ErrCIUnread wrapping %v, nothing posted", err, sent.posted, tt.cause)
			}
		})
	}
}

func TestDeliverPostsWhileNothingHasFailed(t *testing.T) {
	t.Parallel()

	for _, state := range []forge.CIState{forge.CINone, forge.CIRunning, forge.CIPassed} {
		t.Run(state.Word(), func(t *testing.T) {
			t.Parallel()

			// Arrange
			var sent deliveries

			seams := loop.DeliverSeams{Post: sent.post(nil), CheckCI: ciReading(forge.CI{State: state}, nil)}

			// Act
			err := loop.Deliver(t.Context(), seams, sent.memory(), ready())

			// Assert
			if err != nil || len(sent.posted) != 1 || len(sent.recorded) != 1 {
				t.Errorf("Deliver = %v, posted %q, recorded %+v; want it posted and recorded once",
					err, sent.posted, sent.recorded)
			}
		})
	}
}

func TestDeliverReadsTheCIOfThePullRequestAtTheHeadItAnnounces(t *testing.T) {
	t.Parallel()

	// Arrange
	var (
		sent      deliveries
		askedPull int
		askedHead string
	)

	seams := loop.DeliverSeams{Post: sent.post(nil), CheckCI: func(
		_ context.Context, pull forge.PullRequest, head string,
	) (forge.CI, error) {
		askedPull, askedHead = pull.Number, head

		return forge.CI{State: forge.CIPassed}, nil
	}}

	// Act
	err := loop.Deliver(t.Context(), seams, sent.memory(), ready())

	// Assert
	if err != nil || askedPull != 9 || askedHead != headCommit {
		t.Errorf("Deliver = %v, CI read of #%d at %q; want #9 at %q", err, askedPull, askedHead, headCommit)
	}
}

func TestDeliverGatesAMergeToo(t *testing.T) {
	t.Parallel()

	// Arrange
	var sent deliveries

	seams := loop.DeliverSeams{Post: sent.post(nil), CheckCI: ciReading(forge.CI{State: forge.CIFailed}, nil)}

	// Act
	err := loop.Deliver(t.Context(), seams, sent.memory(), merged())

	// Assert
	if !errors.Is(err, loop.ErrCIFailed) || len(sent.posted) != 0 {
		t.Errorf("Deliver = %v, posted %q; want a merge with a failed check refused", err, sent.posted)
	}
}

func TestClearToAnnounceAnswersTheCIItRead(t *testing.T) {
	t.Parallel()

	// Arrange
	running := forge.CI{State: forge.CIRunning, Done: 1, Total: 3}

	// Act
	ci, err := loop.ClearToAnnounce(t.Context(), ciReading(running, nil), openPull(), headCommit)

	// Assert
	if err != nil || ci.State != running.State || ci.Done != running.Done {
		t.Errorf("ClearToAnnounce = %+v, %v; want %+v cleared", ci, err, running)
	}
}
