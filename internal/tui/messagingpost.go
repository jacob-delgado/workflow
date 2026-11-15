// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"context"
	"errors"
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/loop"
	"github.com/jacob-delgado/workflow/internal/messaging"
	"github.com/jacob-delgado/workflow/internal/sanitize"
)

// errMergeCheckFailed is the gate's refusal of a merge a check failed on,
// which has no CI to wait for: its failed checks are re-run instead.
var errMergeCheckFailed = errors.New("the merge was not announced")

// sendToMessaging posts an announcement now, the way every surface delivers
// one: the gate first, then the store's claim, the post and the claim's
// confirmation, with the groups it tagged when it offered any. One waiting
// for CI is left waiting until this one has gone, and then is posted in its
// place.
func (m Model) sendToMessaging(out outgoingAnnouncement) (Model, tea.Cmd) {
	ctx, seams, memory, delivery := m.session, m.deliverSeams(), m.announceMemory(out.tags), m.deliveryOf(out)
	m.messaging.send = starting()

	return m, func() tea.Msg {
		err := withFailedChecks(loop.Deliver(ctx, seams, memory, delivery))

		return messagingPosted{made: delivery.Made(), to: out.to, opened: out.opened, err: refusedAt(delivery.Moment, err)}
	}
}

// deliverSeams are what a post goes through: the service, and the read of CI
// its gate makes first, both going on past a quit.
func (m Model) deliverSeams() loop.DeliverSeams {
	return loop.DeliverSeams{Post: m.deps.Messaging.Post, CheckCI: m.deps.announceCI}
}

// announceMemory is the store's claim a post takes before it goes, its
// confirmation and its release, and, for one whose tags offered groups, the
// groups it tagged, remembered as the workspace's last choice.
func (m Model) announceMemory(tags postTags) loop.AnnounceMemory {
	store := m.deps.Store
	memory := loop.AnnounceMemory{
		Recorded: nil, Claim: store.ClaimAnnounce, Confirm: store.ConfirmAnnounce, Release: store.ReleaseAnnounce,
		Reclaim: nil, RecordGroups: nil,
	}

	if record := store.RecordGroups; tags.offersGroups && record != nil {
		memory.RecordGroups = func(ctx context.Context, ids []string) error { return record(ctx, tags.workspace, ids) }
	}

	return memory
}

// withFailedChecks is err, a post's outcome, naming the checks the gate saw
// fail where it refused the post for them.
func withFailedChecks(err error) error {
	refusal, _ := errors.AsType[loop.GateRefusalError](err)

	names := failedCheckNames(refusal.CI)
	if names == "" {
		return err
	}

	return failedChecksError{err: err, names: names}
}

// failedChecksError is a refusal for a failed check that names the checks
// that failed.
type failedChecksError struct {
	err   error
	names string
}

var _ error = failedChecksError{}

// Error is the refusal's own words, then the checks that failed.
func (e failedChecksError) Error() string {
	return e.err.Error() + e.names
}

// Unwrap lets errors.Is match what the refusal does.
func (e failedChecksError) Unwrap() error {
	return e.err
}

// failedCheckNames names the checks that failed in ci, in brackets after a
// space, or nothing where the forge named none as failed.
func failedCheckNames(ci forge.CI) string {
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

// refusedAt is err, how an announcement at moment went, with a merge's
// failed check told apart from a ready one's, since only a ready one can
// wait for a run that passes.
func refusedAt(moment messaging.Moment, err error) error {
	if moment == messaging.MomentMerged && errors.Is(err, loop.ErrCIFailed) {
		return fmt.Errorf("%w: %w", errMergeCheckFailed, err)
	}

	return err
}

// messagingPosted reports how posting went, the announcement it made — a pull
// request and the moment it marked — where it went, and the round the
// preview it was written in opened in.
type messagingPosted struct {
	made   loop.Announced
	to     string
	opened int
	err    error
}

var _ applier = messagingPosted{}

// apply records the post, or why it failed, in the pane, and in the preview it
// was written in while that is still open, and reads what was announced
// again, which its claim changed. One that went out is posted in place of the
// announcement waiting for CI of the same moment, which then goes no more;
// one that failed leaves that one waiting. One another session announced
// first closes the preview as one that went out does, saying so.
func (msg messagingPosted) apply(m Model) (Model, tea.Cmd) {
	preview, open := answerFor[messagingPreview](m, msg.opened)
	m.messaging.send = sendState{}
	m, reread := m.readingAnnounces()

	elsewhere := errors.Is(msg.err, loop.ErrAnnouncedAlready)
	if _, unconfirmed := loop.UnconfirmedReason(msg.err); msg.err != nil && !unconfirmed && !elsewhere {
		m.messaging.send = m.messaging.send.failed(msg.err)
		if open {
			m = m.withAnswered(preview.failed(msg.err))
		}

		return m.noticedFailure(msg.err), reread
	}

	if open {
		m = m.closeOverlay()
	}

	if elsewhere {
		return m.noticed(m.marks.done + " already announced from another session; not posted again"), reread
	}

	m.messaging.hold = m.messaging.hold.Superseded(msg.made)

	return m.noticedAnnounced(msg.to, msg.err), reread
}

// noticedAnnounced says an announcement went out to where it went. One the
// store could not confirm says so too, in words no pane repeats.
func (m Model) noticedAnnounced(to string, err error) Model {
	posted := m.marks.done + " announced to " + to

	note := unconfirmedNote(m.marks, err)
	if note == "" {
		return m.noticed(posted)
	}

	return m.noticedUnrepeated(posted + note)
}

// unconfirmedNote is what the notice adds of an announcement the store could
// not confirm, on its one row: the warning every surface says, then why. It
// adds nothing when the store confirmed it.
func unconfirmedNote(marks glyphs, err error) string {
	why, unconfirmed := loop.UnconfirmedReason(err)
	if !unconfirmed {
		return ""
	}

	return marks.separator + loop.UnconfirmedWarning + " " + why
}
