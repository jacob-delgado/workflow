// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"

	"github.com/jacob-delgado/workflow/internal/activity"
)

// Next is where the interface was asked to go when it ended, and what of the
// session goes with it; a zero Dir is an interface that ended by quitting.
// A save in Settings asks for the directory it works in, so the interface
// opened next is wired with what was saved.
// The command line opens the next interface there, wired to that directory,
// and hands this back to Arrived, or to StayedAfter when it could not.
//
// Trade-off TRADE-34: switching ends the program and starts another, so
// every pane is read again for the new directory, and only what belongs to
// the session rather than the repository is carried.
type Next struct {
	Dir     string
	carried carried
	// saved is the configuration file a save in Settings or a first run's
	// setup wrote, when that is why the interface ended.
	saved reopening
}

// reopening is a configuration file written that workflow reopens with: a
// save in Settings, or a first run's setup, which arrives on the Issues pane
// it was offered on and says when git would let the file be committed.
type reopening struct {
	path       string
	firstRun   bool
	notIgnored bool
}

// carried is the session's own state, kept across a switch: the comments
// written on Jira issues, which are the same issues wherever the same Jira
// is configured; how
// the Tasks list is seen; and the Summary's period. A forge issue's comment,
// a commit message or a pull request belongs to the repository left, and
// switchGuard names it as lost.
type carried struct {
	// commentDrafts are for issues on jiraURL's instance, and dropped on
	// arriving where another is configured: there a key names someone
	// else's issue.
	commentDrafts commentDrafts
	jiraURL       string
	listing       taskListing
	period        activity.Period
	periodChosen  bool
}

// Destination is where the interface was asked to go when it ended, or a
// zero Next when it was not.
func (m Model) Destination() Next {
	return m.next
}

// Arrived is the interface opened after a switch: the session's state
// carried in, the Repositories pane in focus, and where it is now said.
func (m Model) Arrived(next Next) Model {
	m = m.carryIn(next.carried)
	if next.saved.firstRun {
		return m.arrivedSetUp(next.saved)
	}

	if next.saved.path != "" {
		return m.noticed(m.marks.done + " saved " + shownDir(m.deps, next.saved.path) + "; reopened with it")
	}

	return m.noticed(m.marks.done + " switched to " + shownDir(m.deps, next.Dir))
}

// StayedAfter is the interface reopened where it was after a switch to
// next.Dir, or a reopen with a save, could not be made, saying why.
func (m Model) StayedAfter(next Next, err error) Model {
	m = m.carryIn(next.carried)
	if next.saved.path != "" {
		return m.noticedFailureLedBy("saved "+shownDir(m.deps, next.saved.path)+" but could not reopen with it: ", err)
	}

	return m.noticedFailureLedBy("could not switch to "+shownDir(m.deps, next.Dir)+": ", err)
}

// arrivedSetUp is the interface reopened with the file a first run wrote, on
// the Issues pane it was set up from.
func (m Model) arrivedSetUp(saved reopening) Model {
	said := m.marks.done + " set up with " + shownDir(m.deps, saved.path)
	if saved.notIgnored {
		said = m.marks.done + " set up; add it to .gitignore, since it holds credentials: " + shownDir(m.deps, saved.path)
	}

	return m.focusOn(paneIssues).noticed(said)
}

// carryIn takes what a switch carried, and focuses the pane it was made from,
// which Init then reads.
func (m Model) carryIn(session carried) Model {
	if session.jiraURL == m.cfg.Jira.BaseURL {
		m.commentDrafts = session.commentDrafts
	}

	m.tasks.listing = session.listing
	m.summary.period, m.summary.chosen = session.period, session.periodChosen
	// Init reads the pane it opens on.
	m.refreshed[paneRepositories] = m.deps.now()
	m.repositories.loading = true

	return m.focusOn(paneRepositories)
}

// carryOut is what of the session goes with a switch.
func (m Model) carryOut() carried {
	var jiraDrafts commentDrafts

	for _, draft := range m.commentDrafts {
		if !isForgeKey(draft.issue) {
			jiraDrafts = append(jiraDrafts, draft)
		}
	}

	return carried{
		commentDrafts: jiraDrafts, jiraURL: m.cfg.Jira.BaseURL, listing: m.tasks.listing,
		period: m.summary.period, periodChosen: m.summary.chosen,
	}
}

// verbSwitch names switching, a directory or a task, on the key that does it
// and on its last look alike.
const verbSwitch = "switch"

// leaveFor asks, through a last look, to end the interface for dir, once
// nothing is being written: a switch changes what every pane reads and writes,
// and the look names what it would lose.
func (m Model) leaveFor(dir string) (Model, tea.Cmd) {
	if busy := m.writeInFlight(); busy != "" {
		return m.noticed("wait for " + busy + " to finish before switching"), nil
	}

	m.overlay = m.switchLook(dir)

	return m, nil
}

// switchLook is the last look at switching to dir, which names what the
// switch would lose.
func (m Model) switchLook(dir string) lastLook {
	return lastLook{
		title: "Switch directory", verb: verbSwitch, leave: escStay,
		body: leaveQuestion("Switch to "+shownDir(m.deps, dir)+"?\n\nEvery pane is read again there.",
			"Switching", m.lostOnLeaving()),
		proceed: func(m Model) (Model, tea.Cmd) {
			m = m.closeOverlay()
			if busy := m.writeInFlight(); busy != "" {
				return m.noticed("wait for " + busy + " to finish before switching"), nil
			}

			return m.leave(dir)
		},
	}
}

// leaveQuestion asks question, naming what leaving, as acting names it,
// would lose when anything would be.
func leaveQuestion(question, acting string, lost []string) string {
	if len(lost) == 0 {
		return question
	}

	lines := make([]string, 0, len(lost)+3) //nolint:mnd // the question, a blank line and the heading
	lines = append(lines, question, "", acting+" ends this session's work here, losing:")

	for _, each := range lost {
		lines = append(lines, "  "+each)
	}

	return strings.Join(lines, "\n")
}

// leave ends the interface for dir.
func (m Model) leave(dir string) (Model, tea.Cmd) {
	m.next = Next{Dir: dir, carried: m.carryOut()}

	return m, tea.Quit
}

// reopenWith ends the interface for where you work, so the one opened there
// is wired with the configuration saved, once nothing is being
// written and nothing would be lost; otherwise a last look asks first, as a
// switch's does. Without a directory to reopen in, the save waits for the
// next start.
func (m Model) reopenWith(saved reopening) (Model, tea.Cmd) {
	dir := m.deps.Repositories.Here.Dir
	m = m.closeOverlay()

	switch {
	case dir == "":
		return m.noticed(m.savedForLater(saved.path)), nil
	case m.writeInFlight() == "" && len(m.lostOnLeaving()) == 0:
		return m.reopen(dir, saved)
	default:
		m.overlay = m.reopenLook(dir, saved)

		return m, nil
	}
}

// reopenLook is the last look at reopening in dir with the configuration
// saved, which names what reopening would lose.
func (m Model) reopenLook(dir string, saved reopening) lastLook {
	question := "Saved " + shownDir(m.deps, saved.path) + ". Reopen workflow here, so it applies now?\n\n" +
		"Every pane is read again. Staying keeps it for when workflow next opens."

	return lastLook{
		title: "Reopen with the settings", verb: "reopen", leave: escStay, stayed: m.savedForLater(saved.path),
		body: leaveQuestion(question, "Reopening", m.lostOnLeaving()),
		proceed: func(m Model) (Model, tea.Cmd) {
			m = m.closeOverlay()
			if busy := m.writeInFlight(); busy != "" {
				return m.noticed("wait for " + busy + " to finish before reopening"), nil
			}

			return m.reopen(dir, saved)
		},
	}
}

// savedForLater says the configuration at path was saved, and applies when
// workflow next opens.
func (m Model) savedForLater(path string) string {
	return m.marks.done + " saved " + shownDir(m.deps, path) + "; it applies once workflow reopens"
}

// reopen ends the interface for dir, the directory it works in, saying the
// configuration was saved.
func (m Model) reopen(dir string, saved reopening) (Model, tea.Cmd) {
	m.next = Next{Dir: dir, carried: m.carryOut(), saved: saved}

	return m, tea.Quit
}

// writeInFlight names a write a pane has sent and not had answered, which a
// switch would leave unknown; "" when there is none. A write sent from an
// overlay holds the keyboard until it answers, so it never gets this far.
func (m Model) writeInFlight() string {
	switch {
	case m.messaging.send.sending:
		return "the announcement"
	case m.tasks.writing:
		return "the change to a task"
	default:
		return ""
	}
}

// lostOnLeaving names what is kept only for this repository and would be
// lost by a switch.
func (m Model) lostOnLeaving() []string {
	var lost []string

	if m.draft.subject != "" || m.draft.body != "" {
		lost = append(lost, "the commit message you were writing")
	}

	if m.prDraft.branch != "" && m.prDraft.edited {
		lost = append(lost, "the "+m.vocab.noun+" you were writing")
	}

	if m.messaging.pending.waiting() {
		lost = append(lost, "the announcement waiting for CI")
	}

	for _, draft := range m.commentDrafts {
		if isForgeKey(draft.issue) {
			lost = append(lost, "your comment on "+shownKey(draft.issue))
		}
	}

	return lost
}

// switchToSelected leaves for the directory the cursor is on, unless it is
// where you work or is not there.
func (m Model) switchToSelected() (Model, tea.Cmd) {
	row := m.repositories.selectedRow(m.deps.Repositories.Here)

	switch {
	case row.here:
		return m.noticed("you already work in " + shownDir(m.deps, row.dir)), nil
	case row.err != nil:
		return m.noticed(shownDir(m.deps, row.dir) + " is not there"), nil
	default:
		return m.leaveFor(row.dir)
	}
}

// footer offers going, completing, and canceling.
func (dirPrompt) footer(keys keyMap) []key.Binding {
	return []key.Binding{
		relabel(keys.confirm, "go"), relabel(keys.nextField, "complete"), relabel(keys.closeOverlay, escCancel),
	}
}
