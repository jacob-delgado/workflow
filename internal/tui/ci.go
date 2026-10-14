// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"strconv"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/messaging"
)

// ciCheckedFormat stamps the CI line with when it was last read.
const ciCheckedFormat = "15:04"

// checkingCI asks how CI stands, with then; with no CI to read, a Review
// refresh waits on nothing more.
func (m Model) checkingCI(then tea.Cmd) (Model, tea.Cmd) {
	check := m.checkCI()
	if check == nil {
		m.review.loading = false
	}

	return m, tea.Batch(check, then)
}

// checkCI is the command that asks how CI stands on the pull request.
func (m Model) checkCI() tea.Cmd {
	ctx, check, pull, head := m.session, m.deps.Forge.CheckStatus, m.review.pull, m.branch.branch.Head
	if check == nil || !m.review.found || pull.State != forge.StateOpen {
		return nil
	}

	return func() tea.Msg {
		ci, err := check(ctx, pull, head)

		return ciChecked{number: pull.Number, ci: ci, err: err}
	}
}

// ciChecked carries how CI stands.
type ciChecked struct {
	number int
	ci     forge.CI
	err    error
}

var _ applier = ciChecked{}

// apply records CI, posts a message that was waiting for it, rings the terminal
// if CI has just finished, and keeps asking while there is something to wait for.
func (msg ciChecked) apply(m Model) (Model, tea.Cmd) {
	if !m.review.found || msg.number != m.review.pull.Number {
		return m, nil
	}

	was := m.review.ci.State
	m.review.ci, m.review.ciErr, m.review.checked = msg.ci, msg.err, true
	m.review.loading = false
	m.review.checkedAt = m.deps.now()

	ring := ciFinishNotice(m.cfg, m.deps, was, msg.ci.State)

	m, post := m.settleHeld()

	return m.keepPolling(tea.Batch(post, ring))
}

// ciFinishNotice rings the terminal once when CI has just gone from running to a
// settled result, if the developer asked to be told and the interface can reach
// the terminal to ring it. It fires only on the change, not on later checks that
// find CI already finished.
func ciFinishNotice(cfg config.Config, deps Deps, was, now forge.CIState) tea.Cmd {
	settled := now == forge.CIPassed || now == forge.CIFailed
	if !cfg.UI.Notify || deps.Notify == nil || was != forge.CIRunning || !settled {
		return nil
	}

	notify := deps.Notify

	return func() tea.Msg {
		notify()

		return nil
	}
}

// keepPolling schedules the next check while CI runs or a post waits, unless one
// is already scheduled. A failed check only stops it while no post waits: it
// would fail again at the same rate, so it stops until the next refresh rather
// than asking for as long as the program runs; but a post waiting for CI is
// what the user asked for, so a read that failed is read past.
func (m Model) keepPolling(then tea.Cmd) (Model, tea.Cmd) {
	waiting := m.review.ci.State == forge.CIRunning || m.messaging.pending.waiting()
	givenUp := m.review.ciErr != nil && !m.messaging.pending.waiting()

	if !waiting || givenUp || m.review.polling {
		return m, then
	}

	m.review.polling = true
	poll := ciPoll{review: m.reviewsBegun}

	wait := m.messaging.pollInterval(m.cfg.UI, m.deps)

	return m, tea.Batch(then, m.deps.after(wait, func(time.Time) tea.Msg { return poll }))
}

// notifyPollInterval is how often CI is asked about when the developer only
// wants to be told it finished: a slower beat than the post-when-green path,
// because a notification the developer stepped away for is not in a hurry.
const notifyPollInterval = 3 * time.Minute

// pollInterval is how long to wait before asking about CI again. A post waiting
// on CI wants a prompt answer, and a configured interval is always honored; a
// bare notification, asked for in ui, with no interval set, is content with a
// slower beat.
func (s messagingState) pollInterval(ui config.UI, deps Deps) time.Duration {
	if ui.Notify && !s.pending.waiting() && deps.CIInterval <= 0 {
		return notifyPollInterval
	}

	return deps.ciInterval()
}

// ciPoll is time to ask about CI again, for the review it was scheduled in,
// counted among the reviews begun. A poll from a review since replaced is stale.
type ciPoll struct {
	review int
}

var _ applier = ciPoll{}

// apply asks again, unless it belongs to a review that has since been replaced,
// in which case it does nothing and its chain ends here. While a post waits
// for CI, it reads the branch again first, so the pull request and the head
// its CI is read at are current.
func (msg ciPoll) apply(m Model) (Model, tea.Cmd) {
	if msg.review != m.reviewsBegun {
		return m, nil
	}

	m.review.polling = false

	if m.messaging.pending.waiting() {
		return m, m.rereadForHeld()
	}

	return m, m.checkCI()
}

// ciRead is how the pull request's CI stood on the last read that answered,
// and whether one had.
type ciRead struct {
	state forge.CIState
	read  bool
}

// lastRead is how CI stood on the last read, when that read answered.
func (s reviewState) lastRead() ciRead {
	return ciRead{state: s.ci.State, read: s.checked && s.ciErr == nil}
}

// readFor is the last read that answered, for an announcement at moment:
// none for a merge, whose CI the pane no longer follows, so a reading from
// while it was open says nothing of it now; the gate reads it as it goes.
func (s reviewState) readFor(moment messaging.Moment) ciRead {
	if moment == messaging.MomentMerged {
		return ciRead{}
	}

	return s.lastRead()
}

// failed reports a read that found a check failed.
func (r ciRead) failed() bool {
	return r.read && r.state == forge.CIFailed
}

// ciGlyph is how the branch's CI stands, by shape.
func (s reviewState) ciGlyph(kit renderKit) string {
	return kit.ciGlyph(s.ci.State)
}

// ciSummary says how CI stands in words.
func (s reviewState) ciSummary(kit renderKit) string {
	reported := s.ci

	switch {
	case s.ciErr != nil:
		return kit.failureSummary(s.ciErr)
	case !s.checked:
		return "reading" + kit.marks.ellipsis
	case reported.State == forge.CINone:
		return s.ciGlyph(kit) + " no checks reported" + s.checkedAtSuffix(kit)
	}

	state := reported.State.Word()

	if reported.Total > 0 {
		state += " (" + strconv.Itoa(reported.Done) + " of " + strconv.Itoa(reported.Total) + " finished)"
	}

	return s.ciGlyph(kit) + " " + state + s.checkedAtSuffix(kit)
}

// ciBarWidth is how many columns the Review detail's bar of checks finished
// takes.
const ciBarWidth = 24

// ciBar is a bar of the share of CI's checks finished, on its way there, in
// the forge's hue and lined up under the CI line's words; nothing where the
// forge reports no checks one by one.
func (m Model) ciBar() []string {
	if !m.review.checked || m.review.ci.Total == 0 {
		return nil
	}

	return []string{strings.Repeat(" ", len("CI     ")) + m.kit().bar(m.motion.checks.shown, ciBarWidth, m.styles.forge)}
}

// checkedAtSuffix says when CI was last read, for a line that already says
// how it stands.
func (s reviewState) checkedAtSuffix(kit renderKit) string {
	if s.checkedAt.IsZero() {
		return ""
	}

	return kit.marks.separator + "checked " + s.checkedAt.Format(ciCheckedFormat)
}
