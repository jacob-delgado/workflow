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
)

// ciCheckedFormat stamps the CI line with when it was last read.
const ciCheckedFormat = "15:04"

// checkingCI asks how CI stands, with then; with no CI to read, a Review
// refresh waits on nothing more.
func (m Model) checkingCI(then tea.Cmd) (Model, tea.Cmd) {
	m, check := m.checkCI()
	if check == nil {
		m.review.loading = false
	}

	return m, tea.Batch(check, then)
}

// checkCI counts a read of CI begun, and is the command that asks how CI
// stands on the pull request.
func (m Model) checkCI() (Model, tea.Cmd) {
	ctx, check, pull, head := m.session, m.deps.Forge.CheckStatus, m.review.pull, m.branch.branch.Head
	if check == nil || !m.review.found || pull.State != forge.StateOpen {
		return m, nil
	}

	m.ciReads++
	read := m.ciReads

	return m, func() tea.Msg {
		ci, err := check(ctx, pull, head)

		return ciChecked{number: pull.Number, read: read, ci: ci, err: err}
	}
}

// ciChecked carries how CI stands, as the read counted read found it.
type ciChecked struct {
	number int
	read   int
	ci     forge.CI
	err    error
}

var _ applier = ciChecked{}

// apply records CI, rings the terminal if CI has just finished, and keeps
// asking while it runs. A read that failed lists no check, which may have
// moved on since, but keeps how CI last stood, so the read that answers next
// rings for a change from it; the pane does not ask again after a failed read,
// leaving a post that waits to the shared hold's own reads. A read a later one
// has superseded says nothing: its answer may land after the later one's, and
// would show CI as it stood before.
func (msg ciChecked) apply(m Model) (Model, tea.Cmd) {
	if !m.review.found || msg.number != m.review.pull.Number || msg.read != m.ciReads {
		return m, nil
	}

	was := m.review.ci.State
	m.review.ci, m.review.ciErr, m.review.checked = msg.ci, msg.err, true
	m.review.loading = false
	m.review.checkedAt = m.deps.now()

	if msg.err != nil {
		m.review.ci = forge.CI{State: was}

		return m, nil
	}

	return m.keepPolling(ciFinishNotice(m.cfg, m.deps, was, msg.ci.State))
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

// keepPolling schedules the next check while CI runs, unless one is already
// scheduled. A failed check stops it: it would fail again at the same rate,
// so it stops until the next refresh rather than asking for as long as the
// program runs. An announcement waiting for CI reads it on a beat of its own.
func (m Model) keepPolling(then tea.Cmd) (Model, tea.Cmd) {
	if m.review.ci.State != forge.CIRunning || m.review.polling {
		return m, then
	}

	m.review.polling = true
	poll := ciPoll{review: m.reviewsBegun}

	wait := pollInterval(m.cfg.UI, m.deps)

	return m, tea.Batch(then, m.deps.after(wait, func(time.Time) tea.Msg { return poll }))
}

// notifyPollInterval is how often CI is asked about when the developer only
// wants to be told it finished: a slower beat than an announcement waiting for
// CI reads it at, because a notification the developer stepped away for is
// not in a hurry.
const notifyPollInterval = 3 * time.Minute

// pollInterval is how long to wait before asking about CI again. A configured
// interval is always honored; a notification, asked for in ui, with no
// interval set, is content with a slower beat.
func pollInterval(ui config.UI, deps Deps) time.Duration {
	if ui.Notify && deps.CIInterval <= 0 {
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
// in which case it does nothing and its chain ends here.
func (msg ciPoll) apply(m Model) (Model, tea.Cmd) {
	if msg.review != m.reviewsBegun {
		return m, nil
	}

	m.review.polling = false

	return m.checkCI()
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
