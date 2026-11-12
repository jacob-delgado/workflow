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

// deliverPending delivers pending through seams under ctx: again, from the
// claim an earlier session took, when it is one to post again.
func deliverPending(ctx context.Context, seams loop.DeliverSeams, pending pendingAnnouncement) error {
	if pending.again {
		return loop.Redeliver(ctx, seams, pending.memory, pending.delivery, pending.seen)
	}

	return loop.Deliver(ctx, seams, pending.memory, pending.delivery)
}

// deliverAnnouncement posts pending, pull's announcement, under ctx — again,
// from the claim an earlier session took, when it is one to post again — and
// says how it went, as postOutcome does.
func deliverAnnouncement(
	ctx context.Context, notes console.Notes, seams announceSeams, pending pendingAnnouncement, pull string,
) error {
	return postOutcome(notes, seams, pull, pending.delivery.Moment, deliverPending(ctx, seams.Deliver, pending))
}

// postOutcome says how a post of pull's announcement at moment went, err its
// outcome: where it went, and, for one posted that the store could not
// confirm, the warning every surface says, then why; or that another session
// claimed it first, which posts nothing and is no failure. Any other outcome
// is the failure notAnnounced words.
func postOutcome(notes console.Notes, seams announceSeams, pull string, moment messaging.Moment, err error) error {
	if errors.Is(err, loop.ErrAnnouncedAlready) {
		notes.Note("Already announced from another session; not posted again.")

		return nil
	}

	why, unconfirmed := loop.UnconfirmedReason(err)
	if err != nil && !unconfirmed {
		return notAnnounced(err, pull, moment, seams.Messaging.Service())
	}

	notes.Note("Announced to " + seams.Messaging.Target() + ".")

	if unconfirmed {
		notes.Warn(loop.UnconfirmedWarning + " " + sanitize.Line(why))
	}

	return nil
}

// notAnnounced words a post of pull's announcement at moment to service that
// failed with err: a check the gate saw fail, named with the way on; a post
// the service refused; one that may have gone out, which only announce asked
// again posts; or any other failure, as it is.
func notAnnounced(err error, pull string, moment messaging.Moment, service string) error {
	refusal, refused := errors.AsType[loop.GateRefusalError](err)

	switch {
	case refused && errors.Is(err, loop.ErrCIFailed):
		return checkFailed(pull, refusal.CI, moment)
	case errors.Is(err, loop.ErrNotDelivered):
		return forgeWordedError{err, service + " refused the post (" + sanitize.Line(err.Error()) +
			"), so it was not announced"}
	case errors.Is(err, loop.ErrPostUncertain):
		return forgeWordedError{err, service + " did not say whether it took the post (" + sanitize.Line(err.Error()) +
			"); workflow announce asks before posting it again"}
	default:
		return fmt.Errorf("announcing to %s: %w", service, err)
	}
}
