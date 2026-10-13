// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

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
	ctx context.Context, notes io.Writer, seams announceSeams, report prReport, opts writeOptions,
) error {
	if opts.dryRun {
		fmt.Fprintln(notes, "dry run: would announce it to "+seams.Messaging.Target()+" once its CI passes")

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
	pending := previewAnnouncement(ctx, output{artifact: notes, notes: notes}, seams, composed)

	return deliverWhenGreen(ctx, notes, seams, pending, seams.Kind.Sigil()+strconv.Itoa(report.opened.Number),
		pullOpenedNow)
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

// errOnlyReadyWaits refuses to wait for the CI of a pull request that is not
// ready for review: one merged has no CI left to wait for.
var errOnlyReadyWaits = errors.New("only a ready-for-review announcement waits for CI")

// onceGreen is announce's prompt for an announcement that waits for the CI of
// pull to pass: its question and its dry run say so.
func onceGreen(prompt writePrompt, pull string) writePrompt {
	prompt.question = strings.TrimSuffix(prompt.question, "?") + " once CI on " + pull + " passes?"

	if strings.HasPrefix(prompt.dryRun, "dry run: would announce to ") {
		prompt.dryRun += " once CI on " + pull + " passes"
	}

	return prompt
}

// deliverWhenGreen waits in the foreground, under ctx, for the CI of pending's
// pull request, named pull, to pass — through a failed run, saying what it
// waits on as it changes — and then delivers it, unless the store says
// another session announced it meanwhile. A post the gate refuses, a check
// having failed since the read that let it go, waits again, an interval on.
// It gives up on a pull request that reports no check, is no longer the
// branch's, or merged first; one opened a moment ago, as age says, is waited
// for until the forge lists it. ctrl+c stops it, announcing nothing.
func deliverWhenGreen(
	ctx context.Context, notes io.Writer, seams announceSeams, pending pendingAnnouncement, pull string,
	age pullAge,
) error {
	heard := &waitLines{notes: notes, pull: pull, target: seams.Messaging.Target()}
	await := seams.awaitSeams(heard.hear)
	await.Opened = age == pullOpenedNow

	for {
		green, err := loop.AwaitGreen(ctx, await, pending.delivery.Pull.Number)
		if err != nil {
			return gaveUp(notes, err, pull)
		}

		pending.delivery.Pull, pending.delivery.Head = green.Pull, green.Head

		if seams.Memory.Holds(ctx, pending.delivery.Made()) {
			fmt.Fprintln(notes, "Already announced from another session; not posted again.")

			return nil
		}

		err = deliverAnnouncement(ctx, notes, seams, pending)
		if !errors.Is(err, loop.ErrCIFailed) && !errors.Is(err, loop.ErrCIUnread) {
			return err
		}

		heard.last, await.Delay = "", seams.Interval
	}
}

// awaitSeams are what a wait for the CI to pass reads through: the branch,
// its pull request and its CI, every Interval, telling heard of each read.
func (s announceSeams) awaitSeams(heard func(loop.Heard)) loop.AwaitSeams {
	return loop.AwaitSeams{
		Branch: s.Compose.Branch, FindPull: s.Compose.FindPull, CheckCI: s.Deliver.CheckCI,
		Pause: pause, Now: time.Now, Interval: s.Interval, Heard: heard,
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

// gaveUp words why a wait for CI ended without announcing: stopped, no check
// reported, or the pull request no longer the one to announce.
func gaveUp(notes io.Writer, err error, pull string) error {
	patience := strconv.Itoa(int(loop.NoCIPatience.Minutes())) + "m"

	switch {
	case errors.Is(err, context.Canceled):
		fmt.Fprintln(notes, "Stopped waiting; nothing was announced.")

		return err
	case errors.Is(err, loop.ErrNoCI):
		return forgeWordedError{loop.ErrNoCI, "no checks were reported on " + pull + " within " + patience +
			"; if the repository has no CI, announce it by hand with workflow announce"}
	case errors.Is(err, loop.ErrMergedFirst):
		return forgeWordedError{loop.ErrMergedFirst, pull + " merged before its CI passed, so it was not announced"}
	case errors.Is(err, loop.ErrPullChanged):
		return forgeWordedError{loop.ErrPullChanged, pull + " is no longer this branch's, so it was not announced"}
	default:
		return err
	}
}

// waitLines tells what a wait for the CI of pull to pass waits on, to notes,
// a line each time it changes.
type waitLines struct {
	notes        io.Writer
	pull, target string
	last         string
}

// hear tells of a read, when what it says differs from the last.
func (w *waitLines) hear(heard loop.Heard) {
	line := w.line(heard)
	if line == "" || line == w.last {
		return
	}

	w.last = line
	fmt.Fprintln(w.notes, line)
}

// line is what a read says of the wait, or "" when it says nothing new.
func (w *waitLines) line(heard loop.Heard) string {
	if heard.Err != nil {
		return "Could not read CI on " + w.pull + " (" + sanitize.Line(heard.Err.Error()) + "); trying again."
	}

	// A map, not a switch, so there is no last-case arm gobco can never see:
	// a pass and giving up say nothing here.
	return map[loop.Armed]string{
		loop.ArmedWaiting: "Waiting for CI on " + w.pull + " to pass" + finished(heard.CI) + ", then announcing to " +
			w.target + "; ctrl+c stops without announcing.",
		loop.ArmedAwaitingFix:    "A check failed on " + w.pull + failedChecks(heard.CI) + "; waiting for a run that passes.",
		loop.ArmedAwaitingChecks: "No checks reported on " + w.pull + " yet; waiting for them.",
		loop.ArmedDue:            "",
		loop.ArmedNoCI:           "",
	}[heard.Armed]
}

// finished is how many of ci's checks finished, in brackets after a space, or
// nothing where the forge counts none.
func finished(ci forge.CI) string {
	if ci.Total == 0 {
		return ""
	}

	return " (" + strconv.Itoa(ci.Done) + " of " + strconv.Itoa(ci.Total) + " finished)"
}
