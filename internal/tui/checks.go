// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"errors"
	"strconv"
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"

	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/loop"
)

// checksTitle titles the detail pane while the checks are listed.
const checksTitle = "Checks"

// errNoCheckPage reports a check the forge gave no page to open.
var errNoCheckPage = errors.New("this check reports no page to open")

// errNothingToRerun reports a failure the forge has no re-runnable job for.
var errNothingToRerun = errors.New("nothing to re-run: this failure has no job to restart")

// checkList lists the pull request's checks, so which one failed is plain, and
// opens the page of whichever is selected.
type checkList struct {
	marks   glyphs
	styles  styles
	checks  pickList[forge.Check]
	outcome string
	err     error
}

var (
	_ overlay   = checkList{}
	_ steppable = checkList{}
)

// openChecks lists the checks reported on the pull request. Its caller offers it
// only when canOpenChecks reports there are checks and an opener for their pages.
func (m Model) openChecks() (Model, tea.Cmd) {
	m.overlay = checkList{marks: m.marks, styles: m.styles, checks: pickList[forge.Check]{items: m.review.ci.Checks}}

	return m, nil
}

// canOpenChecks reports that there are checks to list and an opener to reach
// their pages.
func (m Model) canOpenChecks() bool {
	return len(m.review.ci.Checks) > 0 && m.deps.OpenURL != nil
}

// view lists the checks, each by its state and name.
func (c checkList) view(_, rows int) (string, string) {
	lines := make([]string, 0, len(c.checks.items)+headerAndOutcomeRows)
	lines = append(lines, "Open a check's page with enter.", "")
	lines = append(lines, c.checks.rows(c.marks, rows-len(lines)-outcomeRows, c.checkRow)...)
	lines = append(lines, c.outcomeLines()...)

	return checksTitle, strings.Join(lines, "\n")
}

// headerAndOutcomeRows is the two intro lines above the checks and the blank
// line plus outcome below them, so the slice is sized without a regrow.
const headerAndOutcomeRows = 4

// checkRow names a check by how it stands and its name.
func (c checkList) checkRow(check forge.Check) string {
	return c.stateGlyph(check.State) + " " + check.Name
}

// stateGlyph is how a check stands, by shape.
func (c checkList) stateGlyph(state forge.CIState) string {
	switch state {
	case forge.CIPassed:
		return c.marks.done
	case forge.CIFailed:
		return failedGlyph(c.styles, c.marks)
	case forge.CIRunning:
		return c.marks.inFlight
	case forge.CINone:
		return c.marks.unknown
	}

	return c.marks.unknown
}

// outcomeLines say how the last open went, if one was tried.
func (c checkList) outcomeLines() []string {
	switch {
	case c.err != nil:
		return []string{"", failureLine(c.styles, c.marks, c.err)}
	case c.outcome != "":
		return []string{"", c.outcome}
	default:
		return nil
	}
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
	switch {
	case key.Matches(msg, m.keys.closeOverlay):
		return m.closeOverlay(), nil
	case key.Matches(msg, m.keys.cursorKeys()...):
		return c.step(m, m.keys.stepOf(msg)), nil
	case key.Matches(msg, m.keys.confirm):
		return c.open(m)
	case key.Matches(msg, m.keys.showLog):
		return c.readLog(m)
	}

	m.overlay = c

	return m, nil
}

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
		c.err, c.outcome = errNoCheckPage, ""
		m.overlay = c

		return m, nil
	}

	open := m.deps.OpenURL
	m.overlay = c

	return m, func() tea.Msg { return checkOpened{name: check.Name, err: open(check.URL)} }
}

// checkOpened reports how opening a check's page went.
type checkOpened struct {
	name string
	err  error
}

// apply records the outcome on the open list, or does nothing when it has since
// closed.
func (msg checkOpened) apply(m Model) (Model, tea.Cmd) {
	list, open := m.overlay.(checkList)
	if !open {
		return m, nil
	}

	if msg.err != nil {
		list.err, list.outcome = msg.err, ""
	} else {
		list.err, list.outcome = nil, m.marks.done+" opened "+msg.name
	}

	m.overlay = list

	return m, nil
}

// canRerun reports a failed pull request whose checks can be re-run. A pull
// request that has merged is left alone even if a stale CI read still reads as
// failed: there is nothing to re-run once it is in.
func (m Model) canRerun() bool {
	return m.review.found && m.deps.Forge.Rerun != nil && loop.CanRerun(m.review.pull, m.review.ci)
}

// previewRerun holds the re-run of the failed checks for a last look, naming the
// pull request it restarts CI on: a forge write, which goes only once confirmed.
func (m Model) previewRerun() (Model, tea.Cmd) {
	if !m.canRerun() {
		return m, nil
	}

	pull, head := m.review.pull, m.branch.branch.Head
	m.overlay = lastLook{
		marks: m.marks, styles: m.styles, title: "Re-run checks",
		body: "Re-run the failed checks on " + m.vocab.sigil + strconv.Itoa(pull.Number) + " " + pull.Title + "?",
		verb: "re-run", doing: "re-running",
		proceed: func(m Model) (Model, tea.Cmd) { return m.rerunChecks(pull, head) },
	}

	return m, nil
}

// rerunChecks asks the forge to re-run the failed CI on the pull request the
// look named, the look open and in flight until the forge answers.
func (m Model) rerunChecks(pull forge.PullRequest, head string) (Model, tea.Cmd) {
	if m.dryRun {
		return m.closeOverlay().noticed("dry run: would re-run the failed checks"), nil
	}

	rerun := m.deps.Forge.Rerun

	return m, func() tea.Msg {
		reran, err := rerun(pull, head)

		return rerunRequested{reran: reran, err: err}
	}
}

// rerunRequested is the outcome of asking the forge to re-run the failed checks.
type rerunRequested struct {
	reran bool
	err   error
}

// apply returns the pane to "running" and restarts the poll once a re-run has
// started, or says nothing could be re-run — a failure the forge has no
// re-runnable job for, so the pane must not claim one — closing the look either
// way. A refusal stays pinned in the look, and says why in the notice.
func (msg rerunRequested) apply(m Model) (Model, tea.Cmd) {
	// The look that asked is the one open and in flight; nothing else is touched.
	look, open := m.overlay.(lastLook)
	asked := open && look.send.sending

	if msg.err != nil {
		if asked {
			m = keepOpenWith[lastLook](m, msg.err)
		}

		return m.noticedFailureLedBy("re-run failed: ", msg.err), nil
	}

	if asked {
		m = m.closeOverlay()
	}

	if !msg.reran {
		return m.noticedFailure(errNothingToRerun), nil
	}

	m.review.ci = forge.CI{State: forge.CIRunning}
	m.review.ciErr = nil

	// The re-run has only just started, so a check now would still read the old
	// failure; let the poll the set-to-running schedules read it once it moves.
	return m.keepPolling(nil)
}

// failedChecks are the review's failed checks under its CI line: the stage
// each ran in and its name, then why it failed when the forge says, so the
// pane names what broke rather than only that something did.
func (m Model) failedChecks() []string {
	var lines []string

	for _, check := range m.review.ci.Checks {
		if check.State != forge.CIFailed {
			continue
		}

		name := check.Name
		if check.Stage != "" {
			name = check.Stage + m.marks.separator + check.Name
		}

		lines = append(lines, "  "+failedGlyph(m.styles, m.marks)+" "+name)

		if check.Reason != "" {
			lines = append(lines, "    "+check.Reason)
		}
	}

	return lines
}

// readLog asks the forge for the selected check's log, or says the forge
// keeps none for it. It is asked for only here, on the key, never as CI is
// polled.
func (c checkList) readLog(m Model) (Model, tea.Cmd) {
	check, _ := c.checks.chosen()
	if !check.LogAvailable || m.deps.Forge.JobLog == nil {
		c.err, c.outcome = forge.ErrNoLog, ""
		m.overlay = c

		return m, nil
	}

	c.err, c.outcome = nil, "reading the log"+c.marks.ellipsis
	m.overlay = c
	read := m.deps.Forge.JobLog

	return m, func() tea.Msg {
		log, err := read(check)

		return logRead{check: check, log: log, err: err}
	}
}

// logRead carries a check's log back into the update loop.
type logRead struct {
	check forge.Check
	log   forge.JobLog
	err   error
}

var _ applier = logRead{}

// apply shows the log in place of the checks, or the reason it could not be
// read beneath them; nothing when the checks have since closed.
func (msg logRead) apply(m Model) (Model, tea.Cmd) {
	list, open := m.overlay.(checkList)
	if !open {
		return m, nil
	}

	if msg.err != nil {
		list.err, list.outcome = msg.err, ""
		m.overlay = list

		return m, nil
	}

	list.outcome = ""
	m.overlay = jobLogView{marks: m.marks, check: msg.check, log: msg.log, back: list}

	return m, nil
}

// jobLogView is the end of a failed check's log, scrolled to its last lines,
// where the failure is; esc goes back to the checks.
type jobLogView struct {
	marks  glyphs
	check  forge.Check
	log    forge.JobLog
	back   checkList
	scroll int
}

var _ overlay = jobLogView{}

// lines is the log as drawn, under a mark saying where the forge cut it short.
func (v jobLogView) lines() []string {
	lines := strings.Split(v.log.Text, "\n")
	if v.log.Truncated {
		lines = append([]string{v.marks.ellipsis + " earlier lines are not shown"}, lines...)
	}

	return lines
}

// topScroll is the scroll that shows the log's first line at the top of a
// full window of rows: scrolling further would only drop lines from the end.
func (v jobLogView) topScroll(rows int) int {
	return max(0, len(v.lines())-max(1, rows))
}

// view draws as many of the log's last lines as fit, scrolled up by scroll.
func (v jobLogView) view(_, rows int) (string, string) {
	lines := v.lines()
	end := len(lines) - min(v.scroll, v.topScroll(rows))
	start := max(0, end-max(1, rows))

	return v.check.Name + v.marks.separator + "log", strings.Join(lines[start:end], "\n")
}

// footer offers scrolling the log and going back.
func (v jobLogView) footer(keys keyMap) []key.Binding {
	return []key.Binding{keys.up, keys.down, relabel(keys.closeOverlay, escBack)}
}

// handleKey scrolls the log, or goes back to the checks.
func (v jobLogView) handleKey(m Model, msg tea.KeyPressMsg) (Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.closeOverlay):
		m.overlay = v.back

		return m, nil
	case key.Matches(msg, m.keys.cursorKeys()...):
		// The log is scrolled up from its end, so a step down the log is one
		// fewer row scrolled.
		v.scroll = max(0, min(v.scroll-m.keys.stepOf(msg), v.topScroll(m.detailRows())))
	}

	m.overlay = v

	return m, nil
}
