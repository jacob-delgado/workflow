// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"path/filepath"
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"

	"github.com/jacob-delgado/workflow/internal/activity"
	"github.com/jacob-delgado/workflow/internal/sanitize"
	"github.com/jacob-delgado/workflow/internal/workdirs"
)

// Next is where the interface was asked to go when it ended, and what of the
// session goes with it; a zero Dir is an interface that ended by quitting.
// The command line opens the next interface there, wired to that directory,
// and hands this back to Arrived, or to StayedAfter when it could not.
//
// Trade-off TRADE-34: switching ends the program and starts another, so
// every pane is read again for the new directory, and only what belongs to
// the session rather than the repository is carried.
type Next struct {
	Dir     string
	carried carried
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

	return m.noticed(m.marks.done + " switched to " + m.shownDir(next.Dir))
}

// StayedAfter is the interface reopened where it was after a switch to
// next.Dir could not be made, saying why.
func (m Model) StayedAfter(next Next, err error) Model {
	m = m.carryIn(next.carried)

	return m.noticedFailureLedBy("could not switch to "+m.shownDir(next.Dir)+": ", err)
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

// leaveFor ends the interface for dir, once nothing is being written and
// you have agreed to lose what a switch would.
func (m Model) leaveFor(dir string) (Model, tea.Cmd) {
	if busy := m.writeInFlight(); busy != "" {
		return m.noticed("wait for " + busy + " to finish before switching"), nil
	}

	if lost := m.lostOnLeaving(); len(lost) > 0 {
		m.overlay = switchGuard{dir: dir, shown: m.shownDir(dir), lost: lost}

		return m, nil
	}

	return m.leave(dir)
}

// leave ends the interface for dir.
func (m Model) leave(dir string) (Model, tea.Cmd) {
	m.next = Next{Dir: dir, carried: m.carryOut()}

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

// switchGuard asks before a switch that would lose what this repository's
// session holds.
type switchGuard struct {
	dir, shown string
	lost       []string
}

var _ overlay = switchGuard{}

// view says what switching would lose.
func (g switchGuard) view(width, _ int) (string, string) {
	lines := make([]string, 0, len(g.lost)+2) //nolint:mnd // the heading and the blank line under it
	lines = append(lines, "Switching ends this session's work here, losing:", "")

	for _, each := range g.lost {
		lines = append(lines, "  "+each)
	}

	return "Switch to " + g.shown, wrap(strings.Join(lines, "\n"), width)
}

// footer offers switching anyway or staying.
func (switchGuard) footer(keys keyMap) []key.Binding {
	return []key.Binding{relabel(keys.confirm, "switch"), relabel(keys.closeOverlay, "stay")}
}

// handleKey switches, or stays.
func (g switchGuard) handleKey(m Model, msg tea.KeyPressMsg) (Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.confirm):
		return m.closeOverlay().leave(g.dir)
	case key.Matches(msg, m.keys.closeOverlay):
		return m.closeOverlay(), nil
	default:
		return m, nil
	}
}

// switchToSelected leaves for the directory the cursor is on, unless it is
// where you work or is not there.
func (m Model) switchToSelected() (Model, tea.Cmd) {
	row := m.selectedRepository()

	switch {
	case row.here:
		return m.noticed("you already work in " + m.shownDir(row.dir)), nil
	case row.err != nil:
		return m.noticed(m.shownDir(row.dir) + " is not there"), nil
	default:
		return m.leaveFor(row.dir)
	}
}

// dirPrompt is the go-to prompt: a path typed from where you work, or from
// your home after a ~, completed by tab from the directories there and gone
// to once it is checked to be one.
type dirPrompt struct {
	marks      glyphs
	styles     styles
	base, home string
	input      textinput.Model
	// choices are the directories that fit what was typed, when tab found
	// several; note is what tab found otherwise.
	choices []string
	note    string
	// looking is a typed path being checked; problem is why it could not be
	// gone to.
	looking bool
	problem error
}

var (
	_ overlay   = dirPrompt{}
	_ pasteable = dirPrompt{}
)

// openDirPrompt opens the go-to prompt, empty, from where you work.
func (m Model) openDirPrompt() (Model, tea.Cmd) {
	input := newInput("")
	input.Placeholder = m.shownDir(m.deps.Repositories.Here.Dir)

	m.overlay = dirPrompt{
		marks: m.marks, styles: m.styles, input: input,
		base: m.deps.Repositories.Here.Dir, home: m.deps.Repositories.Home,
	}

	return m, nil
}

// view draws the path being typed, what tab found, and why a path could not
// be gone to.
func (p dirPrompt) view(width, _ int) (string, string) {
	p.input.SetWidth(max(1, width-len(p.input.Prompt)-1))

	lines := []string{"Type a path: from where you work, or from your home after ~.", "", p.input.View()}
	if len(p.choices) > 0 {
		lines = append(lines, "", p.styles.label.Render(strings.Join(p.choices, "  ")))
	}

	switch {
	case p.looking:
		lines = append(lines, "", "looking"+p.marks.ellipsis)
	case p.problem != nil:
		lines = append(lines, "", failureLine(p.styles, p.marks, p.problem))
	case p.note != "":
		lines = append(lines, "", p.note)
	}

	return "Go to a directory", wrap(strings.Join(lines, "\n"), width)
}

// footer offers going, completing, and canceling.
func (dirPrompt) footer(keys keyMap) []key.Binding {
	return []key.Binding{
		relabel(keys.confirm, "go"), relabel(keys.nextField, "complete"), relabel(keys.closeOverlay, "cancel"),
	}
}

// handleKey types every key but the three the footer names, so a path's j
// or q is typed rather than moving or quitting.
func (p dirPrompt) handleKey(m Model, msg tea.KeyPressMsg) (Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.closeOverlay):
		// Even while a path is looked at: a mount that does not answer is not
		// waited on, and its answer, when it comes, finds the prompt gone.
		return m.closeOverlay(), nil
	case p.looking:
		return m, nil
	case key.Matches(msg, m.keys.confirm):
		return p.look(m)
	case key.Matches(msg, m.keys.nextField):
		return p.complete(m)
	default:
		return p.typed(m, msg)
	}
}

// pasted types a paste into the path, as typing it would.
func (p dirPrompt) pasted(m Model, paste tea.PasteMsg) (Model, tea.Cmd) {
	if p.looking {
		return m, nil
	}

	return p.typed(m, paste)
}

// typed hands a key or a paste to the path, forgetting what tab and enter
// last found.
func (p dirPrompt) typed(m Model, msg tea.Msg) (Model, tea.Cmd) {
	p.input, _ = p.input.Update(msg)
	p.choices, p.note, p.problem = nil, "", nil
	m.overlay = p

	return m, nil
}

// dirLooked is a typed path checked, and where it leads or why not.
type dirLooked struct {
	dir  string
	here bool
	err  error
}

var _ applier = dirLooked{}

// apply goes to the directory checked, or says why it cannot.
func (msg dirLooked) apply(m Model) (Model, tea.Cmd) {
	prompt, open := m.overlay.(dirPrompt)
	if !open {
		return m, nil
	}

	switch {
	case msg.err != nil:
		prompt.looking, prompt.problem = false, msg.err
		m.overlay = prompt

		return m, nil
	case msg.here:
		return m.closeOverlay().noticed("you already work in " + m.shownDir(msg.dir)), nil
	default:
		return m.closeOverlay().leaveFor(msg.dir)
	}
}

// look checks the path typed off the update loop, since it reads the disk.
func (p dirPrompt) look(m Model) (Model, tea.Cmd) {
	look := m.deps.Repositories.Look
	if look == nil {
		p.note = "cannot look at directories here"
		m.overlay = p

		return m, nil
	}

	dir := workdirs.Resolve(p.input.Value(), p.base, p.home)
	base := p.base
	p.looking = true
	m.overlay = p

	return m, func() tea.Msg {
		place, err := look(dir)

		return dirLooked{dir: place.Dir, here: place.Dir == base || workdirs.Same(place.Dir, base), err: err}
	}
}

// dirCompleted is the directories that fit what was typed when tab was
// pressed, or why there were none to read.
type dirCompleted struct {
	typed, head, partial string
	names                []string
	err                  error
}

var _ applier = dirCompleted{}

// apply completes the path when one directory fits, types what several
// share and names them, or says none does; an answer for a path since
// changed is dropped.
func (msg dirCompleted) apply(m Model) (Model, tea.Cmd) {
	prompt, open := m.overlay.(dirPrompt)
	if !open || prompt.input.Value() != msg.typed {
		return m, nil
	}

	switch len(msg.names) {
	case 0:
		prompt.note = "no directory there starts with " + sanitize.Line(msg.partial)
		if msg.err != nil {
			prompt.note = inFull(msg.err)
		}
	case 1:
		prompt.input.SetValue(msg.head + msg.names[0] + "/")
	default:
		prompt.input.SetValue(msg.head + sharedPrefix(msg.names))
		prompt.choices = msg.names
	}

	prompt.input.CursorEnd()
	m.overlay = prompt

	return m, nil
}

// complete reads the directories where the path typed so far points, off the
// update loop.
func (p dirPrompt) complete(m Model) (Model, tea.Cmd) {
	list := m.deps.Repositories.Subdirectories
	if list == nil {
		return m, nil
	}

	typed := p.input.Value()
	if typed == "~" {
		typed = "~/"
	}

	cut := strings.LastIndexAny(typed, "/"+string(filepath.Separator)) + 1
	head, partial := typed[:cut], typed[cut:]
	parent := workdirs.Resolve(head, p.base, p.home)

	return m, func() tea.Msg {
		listing, err := list(parent, partial)

		names := make([]string, 0, len(listing.Entries))
		for _, entry := range listing.Entries {
			names = append(names, sanitize.Line(entry.Name))
		}

		return dirCompleted{typed: p.input.Value(), head: head, partial: partial, names: names, err: err}
	}
}

// sharedPrefix is what every name starts with, whole letters only: v1é and
// v1è share v1, not the first byte of their last letters.
func sharedPrefix(names []string) string {
	shared := []rune(names[0])
	for _, name := range names[1:] {
		for !strings.HasPrefix(name, string(shared)) {
			shared = shared[:len(shared)-1]
		}
	}

	return string(shared)
}
