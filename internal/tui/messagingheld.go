// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"errors"
	"fmt"
	"strconv"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"

	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/loop"
)

// errAnnouncementDropped is an announcement that waited for CI and was given
// up on: a failure, told with the service it was for.
var errAnnouncementDropped = errors.New("dropped the announcement")

// queuedPost is a message waiting for one pull request's CI to pass. It names
// the pull request it was written for, because the one on screen can change
// while it waits, and a message is only ever sent for the one its writer saw.
// watch is how long its CI has reported no check, and standing how it stood
// on the last read.
type queuedPost struct {
	pull     int
	post     outgoingAnnouncement
	watch    loop.CIWatch
	standing loop.Armed
}

// waiting reports a post that has not been sent or given up on.
func (q queuedPost) waiting() bool {
	return q.post.text != ""
}

// waitingOn is what the standing line adds of what the post waits on: checks
// to be reported, or a fix for one that failed; nothing while CI runs.
func (q queuedPost) waitingOn(kit renderKit) string {
	// A map, not a switch, so there is no arm for the states a waiting post
	// is never left standing in.
	return map[loop.Armed]string{
		loop.ArmedAwaitingChecks: kit.marks.separator + "no checks reported yet",
		loop.ArmedAwaitingFix:    kit.marks.separator + "a check failed; waiting on a fix",
		loop.ArmedWaiting:        "",
		loop.ArmedDue:            "",
		loop.ArmedNoCI:           "",
	}[q.standing]
}

// queued is the post out waits for its pull request's CI to pass, armed now
// and standing on the CI as last read, when it was read.
func (m Model) queued(out outgoingAnnouncement) queuedPost {
	held := queuedPost{
		pull: m.review.pull.Number, post: out, watch: loop.WatchFrom(m.deps.now()), standing: loop.ArmedWaiting,
	}

	if m.review.checked && m.review.ciErr == nil {
		held.watch, held.standing = held.watch.Read(m.review.ci, m.deps.now())
	}

	return held
}

// quitGuard asks before quitting while a post is waiting for CI, which quitting
// would silently lose.
type quitGuard struct{}

var _ overlay = quitGuard{}

// view says what quitting would cost.
func (quitGuard) view(_ renderKit, width, _ int) (string, string) {
	return "Quit", wrap("An announcement is waiting for CI and will be lost.", width)
}

// footer offers quitting anyway or staying.
func (quitGuard) footer(keys keyMap) []key.Binding {
	return []key.Binding{relabel(keys.confirm, "quit"), relabel(keys.closeOverlay, escStay)}
}

// handleKey answers a key while the guard is shown.
func (quitGuard) handleKey(m Model, msg tea.KeyPressMsg) (Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.confirm):
		return m, tea.Quit
	case key.Matches(msg, m.keys.closeOverlay):
		return m.closeOverlay(), nil
	default:
		return m, nil
	}
}

// which names the quit guard.
func (quitGuard) which() overlayKind { return overlayQuitGuard }

// stopWaiting drops the post waiting for CI, unposted, at the user's word.
func (m Model) stopWaiting() (Model, tea.Cmd) {
	m.messaging.pending = queuedPost{}

	return m.noticed("stopped waiting; nothing was announced"), nil
}

// withoutQueuedPost gives up on a post waiting for CI, saying so, because the
// pull request it was written for is no longer the one on screen.
func (m Model) withoutQueuedPost() Model {
	if !m.messaging.pending.waiting() {
		return m
	}

	dropped := m.messaging.pending.pull
	m.messaging.pending = queuedPost{}

	return m.noticedFailure(fmt.Errorf("%w to %s waiting for %s%d, which is no longer this branch's %s",
		errAnnouncementDropped, m.cfg.Messaging.Service(), m.vocab.sigil, dropped, m.vocab.noun))
}

// withoutStrandedPost gives up on the post waiting for CI, saying why, once
// the branch's pull request, as found, is no longer the one it was written
// for: another, none, one closed without merging, or the same one merged
// first, whose CI is never read again.
func (m Model) withoutStrandedPost(pull forge.PullRequest, found bool) Model {
	if !m.messaging.pending.waiting() {
		return m
	}

	number := m.messaging.pending.pull

	err := loop.StillArmed(number, pull, found)
	if errors.Is(err, loop.ErrMergedFirst) {
		return m.givenUp(fmt.Sprintf("%s%d merged before its CI passed", m.vocab.sigil, number))
	}

	if err != nil {
		return m.withoutQueuedPost()
	}

	return m
}

// givenUp drops the post waiting for CI, unposted, for reason, which the
// messaging pane goes on showing.
func (m Model) givenUp(reason string) Model {
	m.messaging.pending = queuedPost{}
	m.messaging.dropped = reason

	return m.noticedFailure(fmt.Errorf("%w to %s: %s", errAnnouncementDropped, m.cfg.Messaging.Service(), reason))
}

// settleHeld settles the post waiting for CI on the CI just read: posts it
// once CI passed, gives up on it once no check was reported for
// loop.NoCIPatience, and otherwise keeps it waiting — through a failed run,
// for a fix — saying what it waits on. A read that failed says nothing of the
// CI, so it is waited past; and a post for a pull request no longer on screen
// is given up on.
func (m Model) settleHeld() (Model, tea.Cmd) {
	if !m.messaging.pending.waiting() || m.review.ciErr != nil {
		return m, nil
	}

	if m.messaging.pending.pull != m.review.pull.Number {
		return m.withoutQueuedPost(), nil
	}

	pending := &m.messaging.pending
	pending.watch, pending.standing = pending.watch.Read(m.review.ci, m.deps.now())

	// A map, not a switch, so there is no last-case arm gobco can never see;
	// exhaustive keeps it complete.
	settle := map[loop.Armed]func() (Model, tea.Cmd){
		loop.ArmedDue:            m.postQueued,
		loop.ArmedNoCI:           m.giveUpOnQuietCI,
		loop.ArmedWaiting:        m.keepQueued,
		loop.ArmedAwaitingChecks: m.keepQueued,
		loop.ArmedAwaitingFix:    m.keepQueued,
	}[pending.standing]

	return settle()
}

// postQueued sends the post that waited for CI.
func (m Model) postQueued() (Model, tea.Cmd) {
	return m.sendHeld(m.messaging.pending)
}

// giveUpOnQuietCI gives up on the post that waited for a pull request that
// reported no check for loop.NoCIPatience, saying to announce it by hand.
func (m Model) giveUpOnQuietCI() (Model, tea.Cmd) {
	patience := strconv.Itoa(int(loop.NoCIPatience.Minutes())) + "m"

	return m.givenUp(fmt.Sprintf("no checks reported on %s%d within %s; announce it by hand",
		m.vocab.sigil, m.messaging.pending.pull, patience)), nil
}

// keepQueued leaves the post waiting while CI has not passed.
func (m Model) keepQueued() (Model, tea.Cmd) {
	return m, nil
}

// waitingAgain puts back the post that waited for CI, the gate having refused
// a post — it, as it went, or one made now in its place — since a check
// failed, or the CI could not be read: it waits on.
func (m Model) waitingAgain(held queuedPost, err error) Model {
	if errors.Is(err, loop.ErrCIFailed) {
		held.standing = loop.ArmedAwaitingFix
	}

	m.messaging.pending, m.messaging.send = held, sendState{}

	return m
}

// heldBack says why the post that waited for CI, put back, did not go, and
// keeps reading CI for it.
func (m Model) heldBack(err error) (Model, tea.Cmd) {
	return m.noticed(m.marks.inFlight + " not announced yet: " + m.kit().failureSummary(err) +
		"; it waits for CI to pass").keepPolling(nil)
}

// refusedByGate reports a post the gate refused: a check failed, or CI could
// not be read.
func refusedByGate(err error) bool {
	return errors.Is(err, loop.ErrCIFailed) || errors.Is(err, loop.ErrCIUnread)
}

// rereadForHeld reads the branch again for the post waiting for CI, so the
// pull request and the head its CI is read at are the ones there are now — a
// push, a merge or another pull request is seen — and the find and the CI read
// follow. A read that fails keeps the post waiting, to be read for again.
func (m Model) rereadForHeld() tea.Cmd {
	ctx, read, review := m.session, m.deps.Git.Branch, m.reviewsBegun
	if read == nil {
		return m.checkCI()
	}

	return func() tea.Msg {
		branch, err := read(ctx)
		if err != nil {
			return heldUnread{review: review}
		}

		return branchLoaded{branch: branch, err: nil}
	}
}

// heldUnread is a read of the branch for the post waiting for CI that failed,
// for the review it was asked in.
type heldUnread struct {
	review int
}

var _ applier = heldUnread{}

// apply schedules the next read for the post still waiting, unless the
// review it was asked in has since been replaced.
func (msg heldUnread) apply(m Model) (Model, tea.Cmd) {
	if msg.review != m.reviewsBegun {
		return m, nil
	}

	return m.keepPolling(nil)
}
