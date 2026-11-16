// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package pr

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jacob-delgado/workflow/internal/cli/console"
	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/loop"
	"github.com/jacob-delgado/workflow/internal/messaging"
	"github.com/jacob-delgado/workflow/internal/sanitize"
)

// checkFailed refuses an announcement of pull at moment, a check having failed
// in ci, naming the checks that failed and the way on.
func checkFailed(pull string, ci forge.CI, moment messaging.Moment) error {
	return forgeWordedError{loop.ErrCIFailed, "a check failed on " + pull + failedChecks(ci) + checkFailedWayOn(moment)}
}

// checkFailedWayOn is the rest of the refusal of an announcement at moment a
// check failed on, saying the way on: a pull request ready for review can
// wait for a run that passes; a merge, the other moment, has no CI to wait
// for, so its failed checks are re-run.
func checkFailedWayOn(moment messaging.Moment) string {
	if moment == messaging.MomentReady {
		return ", so it was not announced; workflow announce --when-green announces it once a run passes"
	}

	return ", so its merge was not announced; re-run the failed checks on the forge, then announce it once they pass"
}

// failedChecks names the checks that failed in ci, in brackets after a space,
// or nothing where the forge named none as failed. No forge does that today for
// CI that failed: GitHub fails CI only by a failed check, and GitLab names its
// pipeline when it can read no failed job.
func failedChecks(ci forge.CI) string {
	var names []string

	for _, check := range ci.Checks {
		if check.State == forge.CIFailed {
			names = append(names, sanitize.Line(check.Name))
		}
	}

	if len(names) == 0 {
		return ""
	}

	return " (" + strings.Join(names, ", ") + ")"
}

// postAnswer is how a post went: the kind loop.Deliver named, and the error
// that says why.
type postAnswer struct {
	kind loop.Delivered
	err  error
}

// deliverPending delivers pending through seams under ctx: again, from the
// claim an earlier session took, when it is one to post again.
func deliverPending(ctx context.Context, seams loop.DeliverSeams, pending pendingAnnouncement) postAnswer {
	if pending.again {
		kind, err := loop.Redeliver(ctx, seams, pending.memory, pending.delivery, pending.seen)

		return postAnswer{kind: kind, err: err}
	}

	kind, err := loop.Deliver(ctx, seams, pending.memory, pending.delivery)

	return postAnswer{kind: kind, err: err}
}

// deliverAnnouncement posts pending, pull's announcement, under ctx — again,
// from the claim an earlier session took, when it is one to post again — and
// says how it went, as postOutcome does.
func deliverAnnouncement(
	ctx context.Context, notes console.Notes, seams announceSeams, pending pendingAnnouncement, pull string,
) error {
	return postOutcome(notes, seams, pull, pending.delivery.Moment, deliverPending(ctx, seams.Deliver, pending))
}

// postOutcome says how a post of pull's announcement at moment went: where
// it went, and, for one posted that the store could not confirm, the warning
// every surface says, then why; or that another session claimed it first,
// which posts nothing and is no failure. Any other outcome is the failure
// notAnnounced words.
func postOutcome(
	notes console.Notes, seams announceSeams, pull string, moment messaging.Moment, answer postAnswer,
) error {
	if answer.kind == loop.DeliveredElsewhere {
		notes.Note("Already announced from another session; not posted again.")

		return nil
	}

	if !answer.kind.Went() {
		return notAnnounced(answer, pull, moment, seams.Messaging.Service())
	}

	notes.Note("Announced to " + seams.Messaging.Target() + ".")

	if answer.kind == loop.DeliveredUnconfirmed {
		why, _ := loop.UnconfirmedReason(answer.err)
		notes.Warn(loop.UnconfirmedWarning + " " + sanitize.Line(why))
	}

	return nil
}

// notAnnounced words a post of pull's announcement at moment to service that
// did not go: a check the gate saw fail, named with the way on; a post the
// service refused, or asked to wait; one that may have gone out, which only
// announce asked again posts; or any other failure, as it is.
func notAnnounced(answer postAnswer, pull string, moment messaging.Moment, service string) error {
	err := answer.err
	refusal, _ := errors.AsType[loop.GateRefusalError](err)
	refused := func() error {
		return forgeWordedError{err, service + " refused the post (" + sanitize.Line(err.Error()) +
			"), so it was not announced"}
	}
	uncertain := func() error {
		return forgeWordedError{err, service + " did not say whether it took the post (" + sanitize.Line(err.Error()) +
			"); workflow announce asks before posting it again"}
	}
	asItIs := func() error { return fmt.Errorf("announcing to %s: %w", service, err) }

	// A map, not a switch, so exhaustive names every kind a post can go.
	return map[loop.Delivered]func() error{
		loop.DeliveredCIFailed:          func() error { return checkFailed(pull, refusal.CI, moment) },
		loop.DeliveredRefused:           refused,
		loop.DeliveredDeferred:          refused,
		loop.DeliveredUncertain:         uncertain,
		loop.DeliveredFailed:            asItIs,
		loop.DeliveredCIUnread:          asItIs,
		loop.DeliveredNotClaimed:        asItIs,
		loop.DeliveredElsewhere:         asItIs,
		loop.DeliveredPosted:            asItIs,
		loop.DeliveredUnconfirmed:       asItIs,
		loop.DeliveredDeferredUnclaimed: asItIs,
	}[answer.kind]()
}
