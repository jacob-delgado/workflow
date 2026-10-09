// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"

	"github.com/jacob-delgado/workflow/internal/jira"
	"github.com/jacob-delgado/workflow/internal/taskwarrior"
)

// addCommand is what a line that adds a task is typed after.
const addCommand = "task add"

// taskLine is one line in Taskwarrior's own grammar being typed for a task
// command — add, annotate or modify — and how sending it is going.
type taskLine struct {
	// title is the overlay's: "Add a task", "Track PROJ-42", "Annotate 12".
	title string
	// command is what the line is typed after — "task add", "task 12 annotate"
	// — which the prompt shows, and a dry run says it would have run.
	command string
	input   textinput.Model
	// write sends the line to Taskwarrior; it runs inside the command.
	write func(line string) taskLineSent
	// after is what follows once Taskwarrior takes the line; nil where nothing
	// does. heldBack is how a dry run tells it: " then annotate it with <url>".
	after    taskFollow
	heldBack string
	// tracks is the issue the line tracks, which counts as tracked from the
	// add's answer until a read begun after it lands; empty for any other line.
	tracks  jira.Key
	problem error
	sending sendState
}

var _ failable[taskLine] = taskLine{}

var (
	_ failable[taskLine] = taskLine{}
	_ pasteable          = taskLine{}
)

// openTaskLine opens a line to type for a task command, starting from prefill.
// The input is sized to the overlay before prefill is set, so a line wider than
// the overlay opens scrolled to the cursor at its end.
func (m Model) openTaskLine(line taskLine, prefill string) (Model, tea.Cmd) {
	line.input = sizedInput(newInput(""), m.detailWidth())
	line.input.SetValue(prefill)
	m.overlay = line

	return m, nil
}

// sizedInput is input fitted to width, less its prompt and the cursor's cell.
// Sizing alone scrolls nothing: an input scrolls to keep its cursor in view only
// when a key it reads, SetValue or SetCursor moves it, and then only as wide as
// it knows it is. So it is sized before each key it reads, and where it is drawn
// it is sized and its cursor set again, as the terminal may have narrowed after
// the last key.
func sizedInput(input textinput.Model, width int) textinput.Model {
	input.SetWidth(max(1, width-len(input.Prompt)-1))

	return input
}

// addTaskLine is a line whose words become a new task.
func addTaskLine(add func(line string) (string, error)) taskLine {
	return taskLine{title: "Add a task", command: addCommand, write: addLine(add)}
}

// openSelectedTaskLine opens a line for a command on the selected task: title
// names it, verb is the command, did how it is told once done, and write sends
// the line bound to the task's uuid.
func (m Model) openSelectedTaskLine(title, verb, did string, write func(uuid, line string) error) (Model, tea.Cmd) {
	task, ok := m.currentTask()
	if !ok {
		return m, nil
	}

	name := taskName(task)
	send := func(line string) taskLineSent {
		return taskLineSent{verb: did, uuid: task.UUID, id: task.ID, err: write(task.UUID, line), after: nil}
	}

	return m.openTaskLine(taskLine{title: title + " " + name, command: "task " + name + " " + verb, write: send}, "")
}

// addLine is the write an add line sends: the line to task add, answered with
// the new task's uuid.
func addLine(add func(line string) (string, error)) func(line string) taskLineSent {
	return func(line string) taskLineSent {
		uuid, err := add(line)

		return taskLineSent{verb: "added task", uuid: uuid, id: 0, err: err, after: nil}
	}
}

// view draws the command, the line being typed, and how sending it is going.
func (l taskLine) view(kit renderKit, width, _ int) (string, string) {
	l.input = sizedInput(l.input, width)
	l.input.SetCursor(l.input.Position())

	lines := append([]string{l.command + " " + kit.marks.ellipsis, l.input.View()}, l.outcome(kit, width)...)

	return l.title, strings.Join(lines, "\n")
}

// outcome says how sending is going, or that the line is still empty. A
// refusal is wrapped rather than cut, so Taskwarrior's own words are all seen.
func (l taskLine) outcome(kit renderKit, width int) []string {
	switch {
	case l.sending.sending:
		return []string{"", "sending" + kit.marks.ellipsis}
	case l.sending.err != nil:
		return []string{"", wrap(kit.failureLine(l.sending.err), width)}
	case l.problem != nil:
		return []string{"", kit.failureLine(l.problem)}
	default:
		return nil
	}
}

// footer offers sending or canceling; while sending, only quitting.
func (l taskLine) footer(keys keyMap) []key.Binding {
	if l.sending.sending {
		return []key.Binding{keys.interrupt}
	}

	return []key.Binding{relabel(keys.confirm, "send"), relabel(keys.closeOverlay, escCancel)}
}

// handleKey answers a key while the line has the keyboard.
func (l taskLine) handleKey(m Model, msg tea.KeyPressMsg) (Model, tea.Cmd) {
	switch {
	case l.sending.sending:
		return m, nil
	case key.Matches(msg, m.keys.closeOverlay):
		return m.closeOverlay(), nil
	case key.Matches(msg, m.keys.confirm):
		return l.confirm(m)
	default:
		l.input, _ = sizedInput(l.input, m.detailWidth()).Update(msg)
		l.problem = nil
		m.overlay = l

		return m, nil
	}
}

// which names a task form.
func (taskLine) which() overlayKind { return overlayTaskLine }

// pasted types a paste into the line, as typing it would.
func (l taskLine) pasted(m Model, paste tea.PasteMsg) (Model, tea.Cmd) {
	if l.sending.sending {
		return m, nil
	}

	l.input, _ = sizedInput(l.input, m.detailWidth()).Update(paste)
	l.problem = nil
	m.overlay = l

	return m, nil
}

// confirm sends the typed line, refusing an empty one in place, and holding it
// back in a dry run.
func (l taskLine) confirm(m Model) (Model, tea.Cmd) {
	line := strings.TrimSpace(l.input.Value())
	if line == "" {
		// Clear any earlier send failure, so "needs a value" is what shows rather
		// than a stale reason from the last attempt.
		l.sending, l.problem = sendState{}, errNeedsValue
		m.overlay = l

		return m, nil
	}

	if m.dryRun {
		return m.closeOverlay().noticed("dry run: " + l.command + " " + line + l.heldBack), nil
	}

	if m.tasks.writing {
		l.sending, l.problem = sendState{}, errTaskWriteInFlight
		m.overlay = l

		return m, nil
	}

	l.sending = starting()
	m.overlay = l
	m.tasks.writing = true
	write, after, stub := l.write, l.after, l.stub(line)

	return m, func() tea.Msg {
		sent := write(line)
		sent.after, sent.stub = after, stub

		return sent
	}
}

// stub is the task a track line adds, as known before Taskwarrior is read
// again: pending, linked to the issue, and described by the line's words
// after its --. The zero task for a line that tracks no issue.
func (l taskLine) stub(line string) taskwarrior.Task {
	if l.tracks == "" {
		return taskwarrior.Task{}
	}

	_, words, _ := strings.Cut(line, " -- ")

	return taskwarrior.Task{IssueKey: string(l.tracks), Description: strings.TrimSpace(words), Status: taskwarrior.Pending}
}

// failed is the line kept open with the reason Taskwarrior refused it.
func (l taskLine) failed(err error) taskLine {
	l.sending = l.sending.failed(err)

	return l
}

// taskLineSent reports how a task line went: what was done, to which task, why
// it failed, what follows, and the stub of the task it adds for an issue, if it
// tracks one.
type taskLineSent struct {
	verb  string
	uuid  string
	id    int
	err   error
	after taskFollow
	stub  taskwarrior.Task
}

var _ applier = taskLineSent{}

// apply keeps the line open with Taskwarrior's words when it refused it, or
// closes it, says what was done, counts the issue it tracks as tracked, and goes
// on to what follows, a write whose answer lets the next go — or, where nothing
// follows, reads the tasks again.
func (msg taskLineSent) apply(m Model) (Model, tea.Cmd) {
	if msg.err != nil {
		m.tasks.writing = false

		return keepOpenWith[taskLine](m, msg.err), nil
	}

	m = m.closeOverlay().noticed(m.marks.done + " " + taskNote(msg.verb, msg.id, msg.uuid))
	if msg.stub.Linked() {
		added := msg.stub
		added.UUID = msg.uuid
		m.tasks = m.tasks.justAdded(added)
		m = m.withTaskWords()
	}

	if msg.after != nil {
		return m, msg.after(msg.uuid)
	}

	m.tasks.writing = false

	return m, m.tasks.load(m.deps)
}
