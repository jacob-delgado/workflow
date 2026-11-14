// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"strconv"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/jacob-delgado/workflow/internal/armed"
	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/loop"
	"github.com/jacob-delgado/workflow/internal/tui/latest"
)

// ciCheckedFormat stamps the CI line with when it was last read.
const ciCheckedFormat = "15:04"

// checkCI begins a read of CI, superseding any still out, and is the
// command that asks how CI stands on the pull request; with no forge to
// ask, or a pull request held that is not open, it begins nothing. Every
// caller holds a pull request found: a find's answer that found one, one
// opened here, and a poll, whose review a find finding none replaces.
func (m Model) checkCI() (Model, tea.Cmd) {
	ctx, check, pull, head := m.session, m.deps.Forge.CheckStatus, m.review.pull(), m.branch.checkedOut().Head
	if check == nil || pull.State != forge.StateOpen {
		return m, nil
	}

	read, round := m.review.ci.Begin()
	m.review.ci = read

	return m, func() tea.Msg {
		ci, err := check(ctx, pull, head)

		return ciChecked{round: round, ci: ci, err: err}
	}
}

// ciChecked carries how CI stands, to the read begun in round.
type ciChecked struct {
	round latest.Round
	ci    forge.CI
	err   error
}

var _ applier = ciChecked{}

// apply records CI, rings the terminal if CI has just finished, and keeps
// asking while it runs. The answer of a read a later one superseded, or one
// begun before a re-run was asked for or the pull request was replaced, is
// dropped: it would show CI as it stood before. A read that failed lists no
// check, which may have moved on since, but keeps how CI last stood, so the
// read that answers next rings for a change from it; the pane does not ask
// again after a failed read, leaving a post that waits to the shared hold's
// own reads.
func (msg ciChecked) apply(m Model) (Model, tea.Cmd) {
	now := m.deps.now()
	if msg.err == nil && m.review.awaitingRerun(msg.ci, now) {
		return m.heldForRerun(msg.round, now)
	}

	awaited := m.review.awaitedFinish()

	read, taken := m.review.ci.Answered(msg.round, msg.ci, msg.err)
	if !taken {
		return m, nil
	}

	m.review.ci, m.review.checkedAt = read, now

	if msg.err != nil {
		m.review.quiet = m.review.quiet.Unheard()

		return m, nil
	}

	m.review.reran, m.review.quiet = nil, m.review.quiet.Read(m.review.pull(), msg.ci, now)

	return m.keepPolling(ciFinishNotice(m.cfg, m.deps, awaited, msg.ci.State))
}

// heldForRerun ends round's read, made at now, without showing it: it still
// lists a run a re-run was asked for, so CI stays running, for a while, as
// the re-run told it: the forge has not started the re-run yet.
func (m Model) heldForRerun(round latest.Round, now time.Time) (Model, tea.Cmd) {
	read, taken := m.review.ci.Settled(round)
	if !taken {
		return m, nil
	}

	m.review.ci, m.review.checkedAt = read, now

	return m.keepPolling(nil)
}

// awaitedFinish reports CI the last read, or a re-run since, left still to
// come: running, or reporting no check yet for a pull request just opened. A
// read that then finds it settled is CI finishing.
func (s reviewState) awaitedFinish() bool {
	return s.checked() && s.ciToCome(s.checkedAt)
}

// checked reports CI read, or told by a re-run: whether or not the read
// failed, the pane no longer says it is reading.
func (s reviewState) checked() bool {
	stage := s.ci.Stage()

	return stage == latest.StageShown || stage == latest.StageFailed
}

// listed is CI as the pane lists it: as the latest read found it, or as a
// re-run told it, or after a failed read, how CI last stood and no check,
// since the checks may have moved on since.
func (s reviewState) listed() forge.CI {
	ci, _ := s.ci.Value()
	if s.ci.Stage() != latest.StageShown {
		return forge.CI{State: ci.State}
	}

	return ci
}

// ciFinishNotice rings the terminal once when CI awaited has just settled, if
// the developer asked to be told and the interface can reach the terminal to
// ring it. It fires only on the change, not on later checks that find CI
// already finished, nor on the first read, which finds CI as it already was.
func ciFinishNotice(cfg config.Config, deps Deps, awaited bool, now forge.CIState) tea.Cmd {
	settled := now == forge.CIPassed || now == forge.CIFailed
	if !cfg.UI.Notify || deps.Notify == nil || !awaited || !settled {
		return nil
	}

	notify := deps.Notify

	return func() tea.Msg {
		notify()

		return nil
	}
}

// keepPolling schedules the next check while CI is still to come, unless one
// is already scheduled. A failed check stops it: it would fail again at the
// same rate, so it stops until the next refresh rather than asking for as long
// as the program runs. An announcement waiting for CI reads it on a beat of
// its own.
func (m Model) keepPolling(then tea.Cmd) (Model, tea.Cmd) {
	if !m.review.ciToCome(m.deps.now()) || m.review.polling {
		return m, then
	}

	m.review.polling = true
	poll := ciPoll{review: m.reviewsBegun}

	wait := pollInterval(m.cfg.UI, m.deps)

	return m, tea.Batch(then, m.deps.after(wait, func(time.Time) tea.Msg { return poll }))
}

// ciToCome reports CI still to come: running, or, read cleanly, reporting no
// check for a pull request this session opened within armed.NoCIPatience — a
// forge lists a new pull request's checks a moment after it opens, and an
// announcement armed as it opened waits that long for them too — or for one
// whose merge waits on its head's quiet to be offered.
func (s reviewState) ciToCome(now time.Time) bool {
	state := s.listed().State
	if state == forge.CIRunning {
		return true
	}

	if state != forge.CINone || s.ci.Stage() == latest.StageFailed {
		return false
	}

	justOpened := !s.opened.IsZero() && now.Sub(s.opened) < armed.NoCIPatience

	return justOpened || s.mergeAwaitsQuiet()
}

// mergeAwaitsQuiet reports a pull request ready to merge in every way but
// its head's quiet: no check reported on it, for less than armed.NoCIPatience
// of the reads heard.
func (s reviewState) mergeAwaitsQuiet() bool {
	return loop.CanMerge(s.pull(), s.listed()) && !s.quiet.CanMerge(s.pull(), s.listed())
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
	return kit.ciGlyph(s.listed().State)
}

// ciSummary says how CI stands in words: that it is being read, how the
// latest read found it, or why that read failed.
func (s reviewState) ciSummary(kit renderKit) string {
	// Beside "CI", reading needs no mark of its own.
	reading := func() string { return "reading" + kit.marks.ellipsis }

	return map[latest.Stage]func() string{
		latest.StageNeverRead: reading,
		latest.StageReading:   reading,
		latest.StageShown:     func() string { return s.ciShown(kit) },
		latest.StageFailed:    func() string { return kit.failureSummary(s.ci.Err()) },
	}[s.ci.Stage()]()
}

// ciShown says how CI stands as the latest read found it: none reported, on
// a pull request whose merge waits on its head's quiet, as checks not
// reported yet.
func (s reviewState) ciShown(kit renderKit) string {
	reported := s.listed()

	switch {
	case reported.State == forge.CINone && s.mergeAwaitsQuiet():
		return s.ciGlyph(kit) + " no checks reported yet" + s.checkedAtSuffix(kit)
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
	if m.review.listed().Total == 0 {
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
