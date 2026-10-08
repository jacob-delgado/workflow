// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"path/filepath"
	"strings"
	"unicode"
	"unicode/utf8"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"

	"github.com/jacob-delgado/workflow/internal/activity"
	"github.com/jacob-delgado/workflow/internal/sanitize"
	"github.com/jacob-delgado/workflow/internal/workdirs"
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
		return m.noticed(m.marks.done + " saved " + m.shownDir(next.saved.path) + "; reopened with it")
	}

	return m.noticed(m.marks.done + " switched to " + m.shownDir(next.Dir))
}

// StayedAfter is the interface reopened where it was after a switch to
// next.Dir, or a reopen with a save, could not be made, saying why.
func (m Model) StayedAfter(next Next, err error) Model {
	m = m.carryIn(next.carried)
	if next.saved.path != "" {
		return m.noticedFailureLedBy("saved "+m.shownDir(next.saved.path)+" but could not reopen with it: ", err)
	}

	return m.noticedFailureLedBy("could not switch to "+m.shownDir(next.Dir)+": ", err)
}

// arrivedSetUp is the interface reopened with the file a first run wrote, on
// the Issues pane it was set up from.
func (m Model) arrivedSetUp(saved reopening) Model {
	said := m.marks.done + " set up with " + m.shownDir(saved.path)
	if saved.notIgnored {
		said = m.marks.done + " set up; add it to .gitignore, since it holds credentials: " + m.shownDir(saved.path)
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
		body: leaveQuestion("Switch to "+m.shownDir(dir)+"?\n\nEvery pane is read again there.",
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
	question := "Saved " + m.shownDir(saved.path) + ". Reopen workflow here, so it applies now?\n\n" +
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
	return m.marks.done + " saved " + m.shownDir(path) + "; it applies once workflow reopens"
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
	// opened is the count of overlays opened when this one opened, so a look
	// answering after esc finds no prompt, even one opened since.
	opened int
}

var (
	_ overlay   = dirPrompt{}
	_ pasteable = dirPrompt{}
)

// openDirPrompt opens the go-to prompt, empty, from where you work.
func (m Model) openDirPrompt() (Model, tea.Cmd) {
	input := newInput("")
	input.Placeholder = m.shownDir(m.deps.Repositories.Here.Dir)
	m, opened := m.opening()

	m.overlay = dirPrompt{
		input: input,
		base:  m.deps.Repositories.Here.Dir, home: m.deps.Repositories.Home, opened: opened,
	}

	return m, nil
}

// view draws the path being typed, what tab found, and why a path could not
// be gone to. The path and the names tab found are as they are on disk, so
// they are neutralized only as they are drawn.
func (p dirPrompt) view(kit renderKit, width, _ int) (string, string) {
	p.input.SetWidth(max(1, width-len(p.input.Prompt)-1))

	lines := []string{"Type a path: from where you work, or from your home after ~.", "", drawnField(p.input)}
	if len(p.choices) > 0 {
		shown := make([]string, 0, len(p.choices))
		for _, choice := range p.choices {
			shown = append(shown, sanitize.Line(choice))
		}

		lines = append(lines, "", kit.styles.label.Render(strings.Join(shown, "  ")))
	}

	switch {
	case p.looking:
		lines = append(lines, "", "reading"+kit.marks.ellipsis)
	case p.problem != nil:
		lines = append(lines, "", failureLine(kit.styles, kit.marks, p.problem))
	case p.note != "":
		lines = append(lines, "", p.note)
	}

	return "Go to a directory", wrap(strings.Join(lines, "\n"), width)
}

// footer offers going, completing, and canceling.
func (dirPrompt) footer(keys keyMap) []key.Binding {
	return []key.Binding{
		relabel(keys.confirm, "go"), relabel(keys.nextField, "complete"), relabel(keys.closeOverlay, escCancel),
	}
}

// handleKey types every key but the three the footer names, so a path's j
// or q is typed rather than moving or quitting.
func (p dirPrompt) handleKey(m Model, msg tea.KeyPressMsg) (Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.closeOverlay):
		// Even while a path is looked at: a mount that does not answer is not
		// waited on, and its answer, when it comes, finds the prompt gone, or
		// another opened since, which it leaves alone.
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

// dirLooked is a typed path checked, and where it leads or why not, for the
// prompt opened as opened.
type dirLooked struct {
	dir    string
	here   bool
	err    error
	opened int
}

var _ applier = dirLooked{}

// apply goes to the directory checked, or says why it cannot, when the prompt
// that asked is still open.
func (msg dirLooked) apply(m Model) (Model, tea.Cmd) {
	prompt, open := m.overlay.(dirPrompt)
	if !open || prompt.opened != msg.opened {
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
	base, opened := p.base, p.opened
	p.looking = true
	m.overlay = p

	return m, func() tea.Msg {
		place, err := look(dir)
		here := place.Dir == base || workdirs.Same(place.Dir, base)

		return dirLooked{dir: place.Dir, here: here, err: err, opened: opened}
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
			names = append(names, entry.Name)
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

// drawnField is a text field as it is drawn, with each character in it that
// has no shape of its own — a direction mark, a zero-width space — shown as
// U+FFFD, as sanitize.Line shows a name. The field's own styling is kept: its
// value holds no other control, since the field drops each one as it is typed
// or set.
func drawnField(input textinput.Model) string {
	return strings.Map(func(character rune) rune {
		if unicode.In(character, unicode.Cf, unicode.Zl, unicode.Zp) {
			return utf8.RuneError
		}

		return character
	}, input.View())
}
