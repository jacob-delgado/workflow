// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package armed

import (
	"errors"
	"strconv"
)

var (
	// ErrNoCI gives up on an armed announcement whose pull request reported no
	// check within NoCIPatience: it is announced by hand.
	ErrNoCI = errors.New("no checks were reported on the pull request")
	// ErrRunningTooLong gives up on an armed announcement whose pull
	// request's CI ran, unchanged, for RunningPatience: it is announced by
	// hand.
	ErrRunningTooLong = errors.New("the pull request's CI has been running, unchanged, for too long")
	// ErrPullChanged gives up on an armed announcement whose branch's pull
	// request is another one, or none.
	ErrPullChanged = errors.New("the branch's pull request is no longer the one announced")
	// ErrMergedFirst gives up on an armed announcement whose pull request
	// merged before its CI passed.
	ErrMergedFirst = errors.New("the pull request merged before its CI passed")
	// ErrMergingFirst gives up on an armed announcement whose pull request
	// the forge began merging before its CI passed: no longer ready for
	// review, it is announced, if at all, once it has merged.
	ErrMergingFirst = errors.New("the forge began merging the pull request before its CI passed")
	// ErrNotListedYet is a find that does not see a pull request just opened,
	// which a forge may list only a moment later: a read to make again.
	ErrNotListedYet = errors.New("the forge does not list the pull request yet")
	// ErrNoHead is a find that answered a pull request without the commit its
	// branch is at, whose CI cannot be read: a read that failed.
	ErrNoHead = errors.New("the forge did not say which commit the pull request is at")
	// ErrUnheard gives up on an armed announcement whose reads, or posts,
	// failed for UnheardPatience, or whose pull request the forge did not
	// list within it. An UnheardError carries it with the last failure.
	ErrUnheard = errors.New("nothing could be heard of the pull request for too long")
	// ErrStopped is an armed announcement stopped as asked, or replaced by
	// another asked for.
	ErrStopped = errors.New("the announcement was stopped before it was posted")
	// ErrLost is an armed announcement whose session or server ended while it
	// waited.
	ErrLost = errors.New("the announcement was lost, unposted, when its session ended")
	// ErrBeingPosted refuses to arm, or to stop, an announcement while its
	// post is under way.
	ErrBeingPosted = errors.New("the announcement is being posted")
	// ErrNothingWaiting refuses to stop an announcement that is not waiting.
	ErrNothingWaiting = errors.New("no announcement is waiting")
	// ErrArmedOther refuses to arm an announcement on its own over another
	// pull request's that waits.
	ErrArmedOther = errors.New("another pull request's announcement is waiting")
)

// OtherHeldError is ErrArmedOther, naming the pull request whose
// announcement waits.
type OtherHeldError struct {
	Held int
}

var _ error = OtherHeldError{}

// Error says which pull request's announcement waits.
func (e OtherHeldError) Error() string {
	return ErrArmedOther.Error() + " (" + strconv.Itoa(e.Held) + ")"
}

// Unwrap lets errors.Is match ErrArmedOther.
func (OtherHeldError) Unwrap() error {
	return ErrArmedOther
}

// UnheardError is ErrUnheard with the last failure before it gave up: a read
// or a post that failed, or ErrNotListedYet.
type UnheardError struct {
	Last error
}

var _ error = UnheardError{}

// Error says that nothing was heard for too long, and what failed last.
func (e UnheardError) Error() string {
	return ErrUnheard.Error() + ": " + e.Last.Error()
}

// Unwrap lets errors.Is match ErrUnheard and the last failure.
func (e UnheardError) Unwrap() []error {
	return []error{ErrUnheard, e.Last}
}
