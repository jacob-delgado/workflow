// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"errors"
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"

	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/tui/latest"
)

// checksTitle titles the detail pane while the checks are listed.
const checksTitle = "Checks"

// errNoCheckPage reports a check the forge gave no page to open.
var errNoCheckPage = errors.New("this check reports no page to open")

// checkList lists the pull request's checks, so which one failed is plain, and
// opens the page of whichever is selected.
type checkList struct {
	checks pickList[forge.Check]
	// page is how the page last asked for opened.
	page pageOutcome
	// openKey is the key apply is bound to, which opens a check's page.
	openKey string
	// opened is the round the list opened in, which a log read and a page
	// opened from it carry back, so neither lands in a list opened after.
	opened int
	// log is the read of the log last asked for, the one whose answer shows.
	log latest.Read[forge.JobLog]
}

// pageOutcome is how opening a check's page went: none asked for, opened,
// or refused, by the opener or for a check with no page.
type pageOutcome struct {
	name string
	err  error
}

var (
	_ opened    = checkList{}
	_ steppable = checkList{}
)

// openChecks lists the checks reported on the pull request. Its caller offers it
// only when canOpenChecks reports there are checks and an opener for their pages.
func (m Model) openChecks() (Model, tea.Cmd) {
	m, opened := m.nextRound()
	m.overlay = checkList{
		checks:  pickList[forge.Check]{items: m.review.listed().Checks},
		openKey: m.keys.confirm.Help().Key, opened: opened,
	}

	return m, nil
}

// canOpenChecks reports that there are checks to list and an opener to reach
// their pages.
func (s reviewState) canOpenChecks(deps Deps) bool {
	return len(s.listed().Checks) > 0 && deps.OpenURL != nil
}

// view lists the checks, each by its state and name.
func (c checkList) view(kit renderKit, _, rows int) (string, string) {
	lines := make([]string, 0, len(c.checks.items)+headerAndOutcomeRows)
	lines = append(lines, "Open a check's page with "+c.openKey+".", "")
	outcome := c.outcomeLines(kit)
	space := rows - len(lines) - max(outcomeRows, len(outcome))
	lines = append(lines, c.checks.rows(kit.marks, space, func(check forge.Check) string {
		return checkRow(kit, check)
	})...)
	lines = append(lines, outcome...)

	return checksTitle, strings.Join(lines, "\n")
}

// headerAndOutcomeRows is the two intro lines above the checks and the blank
// line plus the log's and the page's lines below them, so the slice is sized
// without a regrow.
const headerAndOutcomeRows = 5

// checkRow names a check by how it stands and its name.
func checkRow(kit renderKit, check forge.Check) string {
	return checkGlyph(kit, check.State) + " " + check.Name
}

// checkGlyph is how a check stands, by shape.
func checkGlyph(kit renderKit, state forge.CIState) string {
	switch state {
	case forge.CIPassed:
		return kit.marks.done
	case forge.CIFailed:
		return kit.failedGlyph()
	case forge.CIRunning:
		return kit.marks.inFlight
	case forge.CINone:
		return kit.marks.unknown
	}

	return kit.marks.unknown
}

// outcomeLines say how the last opens went, beneath a blank: the log's
// line, then the page's, each when there is one.
func (c checkList) outcomeLines(kit renderKit) []string {
	var lines []string
	if line, told := c.logLine(kit); told {
		lines = append(lines, line)
	}

	if line, told := c.page.line(kit); told {
		lines = append(lines, line)
	}

	if len(lines) == 0 {
		return nil
	}

	return append([]string{""}, lines...)
}

// logLine is the log's line under the checks: being read, or why it could
// not be. It switches on the read by hand rather than through drawRead: a
// log never asked for says nothing, where a pane's read says it is reading,
// and a log shown is drawn by jobLogView, not here.
func (c checkList) logLine(kit renderKit) (string, bool) {
	switch {
	case c.log.Out():
		return "reading the log" + kit.marks.ellipsis, true
	case c.log.Stage() == latest.StageFailed:
		return kit.failureLine(c.log.Err()), true
	default:
		return "", false
	}
}

// line is the page's line under the checks: why it was refused, or that it
// opened; none before a page is asked for.
func (p pageOutcome) line(kit renderKit) (string, bool) {
	switch {
	case p.err != nil:
		return kit.failureLine(p.err), true
	case p.name != "":
		return kit.marks.done + " opened " + p.name, true
	default:
		return "", false
	}
}

// opening is the list told how a page opened, which takes the place of a
// log's failure, never of a log still being read.
func (c checkList) opening(page pageOutcome) checkList {
	c.page = page
	if !c.log.Out() {
		c.log = c.log.Cleared()
	}

	return c
}

// footer offers moving through the checks and opening one, and reading the
// selected one's log where the forge keeps one.
func (c checkList) footer(keys keyMap) []key.Binding {
	if check, ok := c.checks.chosen(); ok && check.LogAvailable {
		return append(keys.listKeys(), keys.showLog)
	}

	return keys.listKeys()
}

// handleKey answers a key while the checks are listed.
func (c checkList) handleKey(m Model, msg tea.KeyPressMsg) (Model, tea.Cmd) {
	if listed, answered := m.listKey(c, msg); answered {
		return listed, nil
	}

	switch {
	case key.Matches(msg, m.keys.confirm):
		return c.open(m)
	case key.Matches(msg, m.keys.showLog):
		return c.readLog(m)
	}

	return m, nil
}

// which names the checks list.
func (checkList) which() overlayKind { return overlayChecks }

// openedAs is the round the checks list opened in.
func (c checkList) openedAs() int { return c.opened }

// step moves the choice of check by delta.
func (c checkList) step(m Model, delta int) Model {
	c.checks = c.checks.moved(delta)
	m.overlay = c

	return m
}

// open opens the selected check's page, leaving the list up so another can be
// opened after it. The list is never empty: it opens only over reported checks.
func (c checkList) open(m Model) (Model, tea.Cmd) {
	check, _ := c.checks.chosen()
	if check.URL == "" {
		m.overlay = c.opening(pageOutcome{name: check.Name, err: errNoCheckPage})

		return m, nil
	}

	ctx, open, opened := m.session, m.deps.OpenURL, c.opened
	m.overlay = c

	return m, func() tea.Msg { return checkOpened{name: check.Name, opened: opened, err: open(ctx, check.URL)} }
}

// checkOpened reports how opening a check's page went, from the list opened
// in round opened.
type checkOpened struct {
	name   string
	opened int
	err    error
}

var _ applier = checkOpened{}

// apply records the outcome on the list that opened the page, or does nothing
// when it has since closed, even when another has opened since.
func (msg checkOpened) apply(m Model) (Model, tea.Cmd) {
	list, open := answerFor[checkList](m, msg.opened)
	if !open {
		return m, nil
	}

	return m.withAnswered(list.opening(pageOutcome{name: msg.name, err: msg.err})), nil
}

// failedChecks are the review's failed checks under its CI line: the stage
// each ran in and its name, then why it failed when the forge says, so the
// pane names what broke rather than only that something did.
func (s reviewState) failedChecks(kit renderKit) []string {
	var lines []string

	for _, check := range s.listed().Checks {
		if check.State != forge.CIFailed {
			continue
		}

		name := check.Name
		if check.Stage != "" {
			name = check.Stage + kit.marks.separator + check.Name
		}

		lines = append(lines, "  "+kit.failedGlyph()+" "+name)

		if check.Reason != "" {
			lines = append(lines, "    "+check.Reason)
		}
	}

	return lines
}

// readLog asks the forge for the selected check's log, or says the forge
// keeps none for it, on the key alone, never as CI is polled. Either way a
// log asked for before shows nothing, nor does a page opened before. A log
// the forge keeps none of is a read answered at once with forge.ErrNoLog.
func (c checkList) readLog(m Model) (Model, tea.Cmd) {
	check, _ := c.checks.chosen()
	reading, round := c.log.Begin()
	c.page = pageOutcome{}

	if !check.LogAvailable || m.deps.Forge.JobLog == nil {
		c.log, _ = reading.Answered(round, forge.JobLog{}, forge.ErrNoLog)
		m.overlay = c

		return m, nil
	}

	c.log = reading
	m.overlay = c
	ctx, read, opened := m.session, m.deps.Forge.JobLog, c.opened

	return m, func() tea.Msg {
		log, err := read(ctx, check)

		return logRead{opened: opened, round: round, check: check, log: log, err: err}
	}
}

// logRead carries a check's log back into the update loop, for the list
// opened as opened, from the log read it began in round.
type logRead struct {
	opened int
	round  latest.Round
	check  forge.Check
	log    forge.JobLog
	err    error
}

var _ applier = logRead{}

// apply shows the log in place of the checks, or the reason it could not be
// read beneath them; nothing when the checks it was read from have since
// closed, even when others have opened, or when a log was asked for since.
func (msg logRead) apply(m Model) (Model, tea.Cmd) {
	list, open := answerFor[checkList](m, msg.opened)
	if !open {
		return m, nil
	}

	answered, taken := list.log.Answered(msg.round, msg.log, msg.err)
	if !taken {
		return m, nil
	}

	list.log = answered
	if msg.err != nil {
		return m.withAnswered(list), nil
	}

	// The log takes the list's place, so a page opened while it was read is
	// not told again once the log is closed.
	list.page = pageOutcome{}
	m.overlay = jobLogView{check: msg.check, log: msg.log, back: list}

	return m, nil
}

// jobLogView is the end of a failed check's log, scrolled to its last lines,
// where the failure is; esc goes back to the checks.
type jobLogView struct {
	check  forge.Check
	log    forge.JobLog
	back   checkList
	scroll int
}

var (
	_ overlay    = jobLogView{}
	_ scrollable = jobLogView{}
	_ steppable  = jobLogView{}
)

// lines is the log as drawn, under a mark saying where the forge cut it short.
func (v jobLogView) lines(marks glyphs) []string {
	lines := strings.Split(v.log.Text, "\n")
	if v.log.Truncated {
		lines = append([]string{marks.ellipsis + " earlier lines are not shown"}, lines...)
	}

	return lines
}

// height is how many lines the log draws, the mark of a log cut short among
// them.
func (v jobLogView) height() int {
	height := strings.Count(v.log.Text, "\n") + 1
	if v.log.Truncated {
		height++
	}

	return height
}

// topScroll is the scroll that shows the log's first line at the top of a
// full window of rows: scrolling further would only drop lines from the end.
func (v jobLogView) topScroll(rows int) int {
	return max(0, v.height()-max(1, rows))
}

// view draws as many of the log's last lines as fit, scrolled up by scroll.
func (v jobLogView) view(kit renderKit, _, rows int) (string, string) {
	lines := v.lines(kit.marks)
	end := len(lines) - min(v.scroll, v.topScroll(rows))
	start := max(0, end-max(1, rows))

	return v.check.Name + kit.marks.separator + "log", strings.Join(lines[start:end], "\n")
}

// scrolls reports a log taller than the pane, so the scroll keys move it.
func (v jobLogView) scrolls(_, rows int) bool {
	return v.height() > rows
}

// footer offers stepping through the log and going back. A log taller than
// the pane adds the scroll keys after these.
func (v jobLogView) footer(keys keyMap) []key.Binding {
	return []key.Binding{keys.up, keys.down, relabel(keys.closeOverlay, escBack)}
}

// handleKey steps or pages through the log, or goes back to the checks.
func (v jobLogView) handleKey(m Model, msg tea.KeyPressMsg) (Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.closeOverlay):
		m.overlay = v.back

		return m, nil
	case key.Matches(msg, m.keys.scrollDown):
		return v.step(m, m.halfPage()), nil
	case key.Matches(msg, m.keys.scrollUp):
		return v.step(m, -m.halfPage()), nil
	case key.Matches(msg, m.keys.cursorKeys()...):
		return v.step(m, m.keys.stepOf(msg)), nil
	}

	return m, nil
}

// which names a job's log.
func (jobLogView) which() overlayKind { return overlayJobLog }

// step moves delta lines down the log, as up and down do, stopping at either
// end. The log is scrolled up from its end, so a step down the log is one
// fewer row scrolled.
func (v jobLogView) step(m Model, delta int) Model {
	v.scroll = max(0, min(v.scroll-delta, v.topScroll(m.detailRows())))
	m.overlay = v

	return m
}
