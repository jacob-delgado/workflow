// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package pr

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/jacob-delgado/workflow/internal/armed"
	"github.com/jacob-delgado/workflow/internal/cli/cmdkit"
	"github.com/jacob-delgado/workflow/internal/cli/console"
	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/loop"
	"github.com/jacob-delgado/workflow/internal/sanitize"
)

// announceOpened previews the announcement of the pull request report says
// was opened, on notes, and waits for its CI to pass to announce it, as
// workflow announce --when-green does: --announce was the consent, so it asks
// nothing. A dry run says it would, and a pull request not opened announces
// nothing.
func announceOpened(
	ctx context.Context, notes console.Notes, seams announceSeams, report prReport, opts cmdkit.WriteOptions,
) error {
	if opts.DryRun {
		notes.Note("dry run: would announce it to " + seams.Messaging.Target() + " once its CI passes")

		return nil
	}

	if !report.opened.Opened() {
		return nil
	}

	composed := loop.Composed{
		Announcement: loop.ComposeAnnouncementOf(
			ctx, seams.Compose, report.branch, report.opened, seams.Messaging, seams.Project, seams.Kind,
		),
		Pull: report.opened, Branch: report.branch,
	}
	pending := previewAnnouncement(ctx, cmdkit.Output{Artifact: notes.Writer(), Notes: notes}, seams, composed)

	return deliverWhenGreen(ctx, seams, pending, seams.Kind.Sigil()+strconv.Itoa(report.opened.Number), pullOpenedNow)
}

// pullAge is how lately the pull request a wait for CI announces was opened.
type pullAge int

const (
	// pullFoundBefore was found on the forge before the wait began, which
	// lists it.
	pullFoundBefore pullAge = iota
	// pullOpenedNow was opened a moment ago, which the forge may not list yet.
	pullOpenedNow
)

// ErrOnlyReadyWaits refuses to wait for the CI of a pull request that is not
// ready for review: one merged has no CI left to wait for.
var ErrOnlyReadyWaits = errors.New("only a ready-for-review announcement waits for CI")

// onceGreen is announce's prompt for an announcement that waits for the CI of
// pull to pass: its question and its dry run say so.
func onceGreen(prompt cmdkit.WritePrompt, pull string) cmdkit.WritePrompt {
	prompt.Question = strings.TrimSuffix(prompt.Question, "?") + " once CI on " + pull + " passes?"

	if strings.HasPrefix(prompt.DryRun, "dry run: would announce to ") {
		prompt.DryRun += " once CI on " + pull + " passes"
	}

	return prompt
}

// deliverWhenGreen waits in the foreground, under ctx, for the CI of pending's
// pull request, named pull, to pass, and then delivers it, through one
// armed.Hold: the pull request found by the branch it was armed on, never the
// checkout's; through a failed run, and a post the gate refuses as a check
// fails meanwhile; past reads and posts that fail, for armed.UnheardPatience;
// and, for one opened a moment ago, as age says, until the forge lists it. It
// says what it waits on as that changes, above a live line of how many of its
// checks have finished while a person watches the terminal, and how the hold
// ended. ctrl+c stops it, announcing nothing.
func deliverWhenGreen(
	ctx context.Context, seams announceSeams, pending pendingAnnouncement, pull string, age pullAge,
) error {
	return seams.Watch(func(live console.Wait) error {
		return awaitThenDeliver(ctx, live, seams, pending, pull, age)
	})
}

// awaitThenDeliver is deliverWhenGreen's wait, telling live what it waits on.
func awaitThenDeliver(
	ctx context.Context, live console.Wait, seams announceSeams, pending pendingAnnouncement, pull string,
	age pullAge,
) error {
	lines := &waitLines{live: live, pull: pull, target: seams.Messaging.Target(), last: ""}

	live.Doing("CI on " + pull)

	clock := armed.Clock{Now: time.Now, Pause: pause, Interval: seams.Interval}

	final, err := armed.Drive(ctx, seams.armedSeams(pending), armed.Start(time.Now(), pending.arming(age)), clock,
		lines.tell)
	if err != nil {
		live.Notes().Note("Stopped waiting; nothing was announced.")

		return err
	}

	return heldOutcome(live.Notes(), seams, pull, final)
}

// arming is what pending is armed with: asked for, its tags its own, found by
// the branch it was composed for, listed already or opened a moment ago as
// age says, and, asked for again, re-claimed from the claim an earlier session
// took.
func (p pendingAnnouncement) arming(age pullAge) armed.Arming {
	arming := armed.Arming{
		Delivery: p.delivery, Branch: p.branch, Opened: age == pullOpenedNow, Unasked: false, Automatic: false, Seen: 0,
	}
	if p.again {
		arming.Seen = p.seen
	}

	return arming
}

// armedSeams are what a hold of pending reads and posts through: the forge's
// find of a branch's pull request and its read of CI, the post and its gate,
// and the memory that claims the post.
func (s announceSeams) armedSeams(pending pendingAnnouncement) armed.Seams {
	return armed.Seams{
		FindPull: s.Compose.FindPull, CheckCI: s.Deliver.CheckCI, Deliver: s.Deliver, Memory: pending.memory,
		UnaskedTags: nil,
	}
}

// pause waits for wait to pass, or for ctx to end, answering its error.
func pause(ctx context.Context, wait time.Duration) error {
	timer := time.NewTimer(wait)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return fmt.Errorf("waiting for CI: %w", ctx.Err())
	case <-timer.C:
		return nil
	}
}

// heldOutcome says how final, the hold of pull's announcement, ended as the
// wait for its CI did: posted, posted from elsewhere, or a post that failed,
// each as a post asked for now says it; given up on, for why. Drive ends a
// hold only so.
func heldOutcome(notes console.Notes, seams announceSeams, pull string, final armed.Hold) error {
	if final.Stage() == armed.StageDropped && gaveUpWaiting(final.Reason()) {
		return gaveUp(final.Reason(), pull, seams.Messaging.Service())
	}

	return postOutcome(notes, seams, pull, final.Delivery().Moment, final.Reason())
}

// gaveUpWaiting reports whether reason, why a hold was dropped, is one the
// wait gave up on before anything was posted: no check reported, CI that ran
// unchanged for too long, the pull request no longer the one, or nothing
// heard for too long.
func gaveUpWaiting(reason error) bool {
	return errors.Is(reason, armed.ErrNoCI) || errors.Is(reason, armed.ErrRunningTooLong) ||
		errors.Is(reason, armed.ErrPullChanged) || errors.Is(reason, armed.ErrMergedFirst) ||
		errors.Is(reason, armed.ErrUnheard)
}

// gaveUp words why a wait for the CI of pull gave up without announcing to
// service: no check reported, CI that ran unchanged for too long, the pull
// request no longer the one to announce, or nothing heard of it for too long.
func gaveUp(reason error, pull, service string) error {
	patience := strconv.Itoa(int(armed.NoCIPatience.Minutes())) + "m"

	switch {
	case errors.Is(reason, armed.ErrNoCI):
		return forgeWordedError{armed.ErrNoCI, "no checks were reported on " + pull + " within " + patience +
			"; if the repository has no CI, announce it by hand with workflow announce"}
	case errors.Is(reason, armed.ErrRunningTooLong):
		return forgeWordedError{armed.ErrRunningTooLong, "CI on " + pull + " has been running for " +
			strconv.Itoa(int(armed.RunningPatience.Hours())) + "h; not announced"}
	case errors.Is(reason, armed.ErrMergedFirst):
		return forgeWordedError{armed.ErrMergedFirst, pull + " merged before its CI passed, so it was not announced"}
	case errors.Is(reason, armed.ErrPullChanged):
		return forgeWordedError{armed.ErrPullChanged, pull + " is no longer this branch's, so it was not announced"}
	default:
		return unheardFor(reason, pull, service)
	}
}

// unheardFor words a wait for the CI of pull that heard nothing for
// armed.UnheardPatience, by what failed last: the forge not listing it, a
// post to service, or a read of CI. It exits as that failure's family does,
// and as a refused precondition where it is in none a script can try again
// on: a forge it could not reach, or one that asked it to wait, is not
// refused.
func unheardFor(reason error, pull, service string) error {
	unheard, _ := errors.AsType[armed.UnheardError](reason)
	patience := strconv.Itoa(int(armed.UnheardPatience.Minutes())) + "m"
	last := sanitize.Line(unheard.Last.Error())

	var sentinel error = unheard
	if cmdkit.IsAny(unheard.Last, cmdkit.ServicesUnreachable()...) {
		sentinel = unheard.Last
	}

	switch {
	case errors.Is(unheard.Last, armed.ErrNotListedYet):
		return forgeWordedError{sentinel, "the forge did not list " + pull + " within " + patience +
			", so it was not announced"}
	case postFailed(unheard.Last):
		return forgeWordedError{sentinel, "could not post to " + service + " for " + patience + " (" + last +
			"), so it was not announced"}
	default:
		return forgeWordedError{sentinel, "could not read CI on " + pull + " for " + patience + " (" + last +
			"), so it was not announced"}
	}
}

// postFailed reports whether err is a post's failure that a hold tries again
// past: the store could not take its claim, or the service refused it for
// now.
func postFailed(err error) bool {
	return errors.Is(err, loop.ErrNotClaimed) || errors.Is(err, loop.ErrNotDelivered)
}

// waitLines tells what a wait for the CI of pull to pass waits on: in its
// notes, a line each time where it stands, or why, changes, and on its live
// line, how many of the checks have finished.
type waitLines struct {
	live         console.Wait
	pull, target string
	last         string
}

// tell tells of the hold after a step: how many of the checks it last heard
// finished, and, when where it stands differs from the last it told, what it
// waits on.
func (w *waitLines) tell(hold armed.Hold) {
	w.live.Counted(hold.CI().Done, hold.CI().Total)

	line := w.line(hold)
	key := strconv.Itoa(int(hold.Stage())) + " " + fmt.Sprint(hold.Reason())

	if line == "" || key == w.last {
		return
	}

	w.last = key
	w.live.Notes().Note(line)
}

// line is what the hold waits on, or "" for a hold that waits on nothing:
// being posted, or ended, which the wait's end says.
func (w *waitLines) line(hold armed.Hold) string {
	heard, reason := hold.CI(), hold.Reason()
	failed := w.failedLine(reason)

	listing := "Waiting for the forge to list " + w.pull + "…"
	if reason != nil && !errors.Is(reason, armed.ErrNotListedYet) {
		listing = failed
	}

	fix := "A check failed on " + w.pull + failedChecks(heard) + "; waiting for a run that passes."
	if _, refused := errors.AsType[loop.GateRefusalError](reason); refused {
		fix = "A check failed on " + w.pull + failedChecks(heard) + " as it was to be posted; " +
			"waiting for a run that passes."
	}

	// A map, not a switch, so there is no last-case arm gobco can never see.
	return map[armed.Stage]string{
		armed.StageListing: listing,
		armed.StageRunning: "Waiting for CI on " + w.pull + " to pass" + finished(heard) + ", then announcing to " +
			w.target + "; ctrl+c stops without announcing.",
		armed.StageAwaitingFix:    fix,
		armed.StageAwaitingChecks: "No checks reported on " + w.pull + " yet; waiting for them.",
		armed.StageUnheard:        failed,
		armed.StageIdle:           "",
		armed.StageDue:            "",
		armed.StagePosted:         "",
		armed.StageElsewhere:      "",
		armed.StageDropped:        "",
		armed.StageStopped:        "",
	}[hold.Stage()]
}

// failedLine says that a read, or a post, failed with reason, and is tried
// again.
func (w *waitLines) failedLine(reason error) string {
	why := sanitize.Line(fmt.Sprint(reason))
	if postFailed(reason) {
		return "Could not post to " + w.target + " (" + why + "); trying again."
	}

	return "Could not read CI on " + w.pull + " (" + why + "); trying again."
}

// finished is how many of ci's checks finished, in brackets after a space, or
// nothing where the forge counts none.
func finished(ci forge.CI) string {
	if ci.Total == 0 {
		return ""
	}

	return " (" + strconv.Itoa(ci.Done) + " of " + strconv.Itoa(ci.Total) + " finished)"
}
