// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package armed

import (
	"context"
	"time"

	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/loop"
	"github.com/jacob-delgado/workflow/internal/messaging"
)

// Seams are what a hold reads and posts through: the find of a branch's pull
// request and the read of its CI at a head; the post, and the read its gate
// makes first; the memory that claims a post before it goes; and, for a hold
// whose tags are proposed as it posts, whom it tags, asked of no one. A nil
// UnaskedTags tags whom the delivery already does.
type Seams struct {
	FindPull    func(ctx context.Context, branch string) (forge.PullRequest, bool, error)
	CheckCI     func(ctx context.Context, pull forge.PullRequest, head string) (forge.CI, error)
	Deliver     loop.DeliverSeams
	Memory      loop.AnnounceMemory
	UnaskedTags func(ctx context.Context, delivery loop.Delivery) messaging.Mentions
}

// Read reads the held pull request under ctx, as the hold's round, at the
// time now tells once it is done: the branch's pull request, found by the
// branch armed, never the checkout's, and, where it is open and the one
// held, its CI at the commit the forge says it is at. A read that failed is
// the Failure, its Err set; else the Reading. One with no head commit to read
// CI at failed, with ErrNoHead.
func (s Seams) Read(ctx context.Context, hold Hold, now func() time.Time) (Reading, Failure) {
	pull, found, err := s.FindPull(ctx, hold.Branch())
	if err != nil {
		return Reading{}, Failure{Round: hold.Round(), Err: err, At: now()}
	}

	reading := Reading{Round: hold.Round(), Pull: pull, Found: found, CI: forge.CI{}, At: time.Time{}}

	if found && pull.IsOpen() && pull.Number == hold.Pull() {
		if pull.HeadCommit == "" {
			return Reading{}, Failure{Round: hold.Round(), Err: ErrNoHead, At: now()}
		}

		reading.CI, err = s.CheckCI(ctx, pull, pull.HeadCommit)
		if err != nil {
			return Reading{}, Failure{Round: hold.Round(), Err: err, At: now()}
		}
	}

	reading.At = now()

	return reading, Failure{}
}

// DeliverHeld posts the held delivery under ctx through loop.Deliver — its
// tags proposed first, asked of no one, for a hold armed so — and answers
// its outcome, as the hold's round, at the time now tells once it is done.
func (s Seams) DeliverHeld(ctx context.Context, hold Hold, now func() time.Time) Outcome {
	delivery := hold.Delivery()
	if hold.arming.Unasked && s.UnaskedTags != nil {
		delivery.Mentions = s.UnaskedTags(ctx, delivery)
	}

	err := loop.Deliver(ctx, s.Deliver, s.Memory, delivery)

	return Outcome{Round: hold.Round(), Err: err, At: now()}
}
