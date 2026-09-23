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
	marks    glyphs
	styles   styles
	checks   []forge.Check
	selected int
	outcome  string
	err      error
}

var _ overlay = checkList{}

// openChecks lists the checks reported on the pull request. Its caller offers it
// only when canOpenChecks reports there are checks and an opener for their pages.
func (m Model) openChecks() (Model, tea.Cmd) {
	m.overlay = checkList{marks: m.marks, styles: m.styles, checks: m.review.ci.Checks}

	return m, nil
}

// canOpenChecks reports that there are checks to list and an opener to reach
// their pages.
func (m Model) canOpenChecks() bool {
	return len(m.review.ci.Checks) > 0 && m.deps.OpenURL != nil
}

// view lists the checks, each by its state and name.
func (c checkList) view(_, rows int) (string, string) {
	lines := make([]string, 0, len(c.checks)+headerAndOutcomeRows)
	lines = append(lines, "Open a check's page with enter.", "")
	lines = append(lines, c.rows(rows-len(lines)-outcomeRows)...)
	lines = append(lines, c.outcomeLines()...)

	return checksTitle, strings.Join(lines, "\n")
}

// headerAndOutcomeRows is the two intro lines above the checks and the blank
// line plus outcome below them, so the slice is sized without a regrow.
const headerAndOutcomeRows = 4

// rows draws as many checks as fit, scrolled so the selection stays on screen.
func (c checkList) rows(space int) []string {
	first, last := window(c.selected, len(c.checks), space)
	lines := make([]string, 0, last-first)

	for index := first; index < last; index++ {
		check := c.checks[index]
		lines = append(lines, c.marks.marker(index == c.selected)+c.stateGlyph(check.State)+" "+check.Name)
	}

	return lines
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

// footer offers moving through the checks and opening one.
func (c checkList) footer(keys keyMap) []key.Binding {
	return keys.listKeys()
}

// handleKey answers a key while the checks are listed.
func (c checkList) handleKey(m Model, msg tea.KeyPressMsg) (Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.closeOverlay):
		return m.closeOverlay(), nil
	case key.Matches(msg, m.keys.down):
		c.selected = max(0, min(c.selected+1, len(c.checks)-1))
	case key.Matches(msg, m.keys.up):
		c.selected = max(0, c.selected-1)
	case key.Matches(msg, m.keys.confirm):
		return c.open(m)
	}

	m.overlay = c

	return m, nil
}

// open opens the selected check's page, leaving the list up so another can be
// opened after it. The list is never empty: it opens only over reported checks.
func (c checkList) open(m Model) (Model, tea.Cmd) {
	check := c.checks[c.selected]
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
	return m.review.found && m.review.pull.State == forge.StateOpen &&
		m.review.ci.State == forge.CIFailed && m.deps.Forge.Rerun != nil
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
		body: "re-run the failed checks on " + m.vocab.sigil + strconv.Itoa(pull.Number) + " " + pull.Title,
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
		refusal := writeRefusal(msg.err)
		if asked {
			look.send = look.send.failed(refusal)
			m.overlay = look
		}

		return m.noticedFailureLedBy("re-run failed: ", refusal), nil
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
