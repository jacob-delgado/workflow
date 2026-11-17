// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package loop

import "errors"

// Delivered is how a delivery went: posted, refused before a claim, held
// elsewhere, or failed one of the ways a post can. Deliver and Redeliver name
// it at each return, beside the error that says why in words.
type Delivered int

const (
	// DeliveredFailed did not go, for a reason none of the others name: no
	// way to post, or an unguarded post the service failed. It is the zero,
	// so a kind nobody named never reads as posted.
	DeliveredFailed Delivered = iota
	// DeliveredPosted went out, its claim confirmed or none kept.
	DeliveredPosted
	// DeliveredUnconfirmed went out, and the store could not confirm its
	// claim, which still keeps it from going twice.
	DeliveredUnconfirmed
	// DeliveredCIFailed was refused by the gate: a check failed. Nothing
	// was claimed.
	DeliveredCIFailed
	// DeliveredCIUnread was refused by the gate: the CI could not be read.
	// Nothing was claimed.
	DeliveredCIUnread
	// DeliveredNotClaimed was not posted: the store could not take the
	// claim, or answered one of no kind a delivery knows.
	DeliveredNotClaimed
	// DeliveredElsewhere was not posted: another session or process holds
	// the claim.
	DeliveredElsewhere
	// DeliveredRefused was surely not delivered: the service refused it.
	// Deliver's claim was given up; Redeliver's stays at its generation.
	DeliveredRefused
	// DeliveredDeferred was surely not delivered: the service asked it to
	// wait. The claim was given up or kept, so a redelivery's seen moves on.
	DeliveredDeferred
	// DeliveredDeferredUnclaimed was surely not delivered: the service asked
	// it to wait, and no store keeps claims.
	DeliveredDeferredUnclaimed
	// DeliveredUncertain may have gone out: the service did not say, or a
	// claim on a post surely not delivered could not be given up. The claim
	// is kept, so nothing posts it again unasked.
	DeliveredUncertain
)

// Went reports whether the delivery went out: posted, confirmed or not.
func (d Delivered) Went() bool {
	return map[Delivered]bool{
		DeliveredFailed:            false,
		DeliveredPosted:            true,
		DeliveredUnconfirmed:       true,
		DeliveredCIFailed:          false,
		DeliveredCIUnread:          false,
		DeliveredNotClaimed:        false,
		DeliveredElsewhere:         false,
		DeliveredRefused:           false,
		DeliveredDeferred:          false,
		DeliveredDeferredUnclaimed: false,
		DeliveredUncertain:         false,
	}[d]
}

// posting is how a delivery goes on once its claim is answered.
type posting int

const (
	// postingRefused posts nothing: the claim is of no kind a delivery
	// knows. It is the zero, so a kind with no entry fails closed.
	postingRefused posting = iota
	// postingElsewhere posts nothing: another holds the claim.
	postingElsewhere
	// postingGuarded posts under the claim taken, which is given up if the
	// post surely did not go and confirmed once it has.
	postingGuarded
	// postingUnguarded posts with no claim kept.
	postingUnguarded
)

// posts reports whether a delivery going on so posts: guarded or not. A
// map, not a comparison, so a posting added later is placed here before it
// can post, and one with no entry refuses.
func (p posting) posts() bool {
	return map[posting]bool{
		postingRefused:   false,
		postingElsewhere: false,
		postingGuarded:   true,
		postingUnguarded: true,
	}[p]
}

// postingOn is how a delivery goes on after a claim of kind. A map, not a
// switch, so every kind is named and one with no entry refuses.
func postingOn(kind ClaimKind) posting {
	return map[ClaimKind]posting{
		claimOfNoKind: postingRefused,
		ClaimHeld:     postingElsewhere,
		ClaimTaken:    postingGuarded,
		ClaimNotKept:  postingUnguarded,
	}[kind]
}

// notDeliveredError is ErrNotDelivered carrying the service's refusal, which
// says what went wrong in the service's own words: the claim's giving up is
// for the caller to tell apart, not to say.
type notDeliveredError struct {
	why error
}

var _ error = notDeliveredError{}

func (e notDeliveredError) Error() string {
	return e.why.Error()
}

func (e notDeliveredError) Unwrap() []error {
	return []error{ErrNotDelivered, e.why}
}

// unconfirmedError is ErrUnconfirmed carrying why the store could not
// confirm the post Deliver made.
type unconfirmedError struct {
	why error
}

var _ error = unconfirmedError{}

func (e unconfirmedError) Error() string {
	return ErrUnconfirmed.Error() + ": " + e.why.Error()
}

func (e unconfirmedError) Unwrap() []error {
	return []error{ErrUnconfirmed, e.why}
}

// UnconfirmedReason reports whether err is Deliver's word that it posted an
// announcement whose claim the store could not confirm, wrapped or not, and
// why the store could not: what the terminal and workflow announce say after
// UnconfirmedWarning.
func UnconfirmedReason(err error) (string, bool) {
	var unconfirmed unconfirmedError
	if !errors.As(err, &unconfirmed) {
		return "", false
	}

	return unconfirmed.why.Error(), true
}
