// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"strconv"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/jacob-delgado/workflow/internal/forge"
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
	check, pull, head := m.deps.Forge.CheckStatus, m.review.pull, m.branch.branch.Head
	if check == nil || !m.review.found || pull.State != forge.StateOpen {
		return nil
	}

	return func() tea.Msg {
		ci, err := check(pull, head)

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

	ring := m.ciFinishNotice(was, msg.ci.State)

	m, post := m.postIfGreen()

	return m.keepPolling(tea.Batch(post, ring))
}

// ciFinishNotice rings the terminal once when CI has just gone from running to a
// settled result, if the developer asked to be told and the interface can reach
// the terminal to ring it. It fires only on the change, not on later checks that
// find CI already finished.
func (m Model) ciFinishNotice(was, now forge.CIState) tea.Cmd {
	settled := now == forge.CIPassed || now == forge.CIFailed
	if !m.cfg.UI.Notify || m.deps.Notify == nil || was != forge.CIRunning || !settled {
		return nil
	}

	notify := m.deps.Notify

	return func() tea.Msg {
		notify()

		return nil
	}
}

// keepPolling schedules the next check while CI runs or a post waits, unless one
// is already scheduled or the last check failed. A failed check would only fail
// again at the same rate, so it stops until the next refresh rather than asking
// for as long as the program runs.
func (m Model) keepPolling(then tea.Cmd) (Model, tea.Cmd) {
	waiting := m.review.ci.State == forge.CIRunning || m.messaging.pending.waiting()
	if !waiting || m.review.ciErr != nil || m.review.polling {
		return m, then
	}

	m.review.polling = true
	poll := ciPoll{review: m.reviewsBegun}

	return m, tea.Batch(then, m.deps.after(m.pollInterval(), func(time.Time) tea.Msg { return poll }))
}

// notifyPollInterval is how often CI is asked about when the developer only
// wants to be told it finished: a slower beat than the post-when-green path,
// because a notification the developer stepped away for is not in a hurry.
const notifyPollInterval = 3 * time.Minute

// pollInterval is how long to wait before asking about CI again. A post waiting
// on CI wants a prompt answer, and a configured interval is always honored; a
// bare notification, with no interval set, is content with a slower beat.
func (m Model) pollInterval() time.Duration {
	if m.cfg.UI.Notify && !m.messaging.pending.waiting() && m.deps.CIInterval <= 0 {
		return notifyPollInterval
	}

	return m.deps.ciInterval()
}

// ciPoll is time to ask about CI again, for the review it was scheduled in,
// counted among the reviews begun. A poll from a review since replaced is stale.
type ciPoll struct {
	review int
}

var _ applier = ciPoll{}

// apply asks again, unless it belongs to a review that has since been replaced,
// in which case it does nothing and its chain ends here.
func (msg ciPoll) apply(m Model) (Model, tea.Cmd) {
	if msg.review != m.reviewsBegun {
		return m, nil
	}

	m.review.polling = false

	return m, m.checkCI()
}

// ciGlyph is how the branch's CI stands, by shape.
func (m Model) ciGlyph() string {
	return m.kit().ciGlyph(m.review.ci.State)
}

// ciSummary says how CI stands in words.
func (m Model) ciSummary() string {
	reported := m.review.ci

	switch {
	case m.review.ciErr != nil:
		return m.kit().failureSummary(m.review.ciErr)
	case !m.review.checked:
		return "reading" + m.marks.ellipsis
	case reported.State == forge.CINone:
		return m.ciGlyph() + " no checks reported" + m.checkedAtSuffix()
	}

	state := reported.State.Word()

	if reported.Total > 0 {
		state += " (" + strconv.Itoa(reported.Done) + " of " + strconv.Itoa(reported.Total) + " finished)"
	}

	return m.ciGlyph() + " " + state + m.checkedAtSuffix()
}

// checkedAtSuffix says when CI was last read, for a line that already says how
// it stands.
func (m Model) checkedAtSuffix() string {
	if m.review.checkedAt.IsZero() {
		return ""
	}

	return m.marks.separator + "checked " + m.review.checkedAt.Format(ciCheckedFormat)
}
