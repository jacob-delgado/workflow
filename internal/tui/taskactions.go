// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"cmp"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"

	"github.com/jacob-delgado/workflow/internal/jira"
	"github.com/jacob-delgado/workflow/internal/seams"
	"github.com/jacob-delgado/workflow/internal/taskwarrior"
)

// shortUUID is how many characters of a uuid name a task that has no id yet, as
// Taskwarrior's own short uuids do.
const shortUUID = 8

// addCommand is what a line that adds a task is typed after.
const addCommand = "task add"

// errTaskWriteInFlight is why a change of a task waits: Taskwarrior takes one
// write at a time.
var errTaskWriteInFlight = errors.New("another Taskwarrior change is still being sent; try again once it answers")

// tasksKeys offers what the Tasks pane answers right now: the verbs on the
// selected task, adding and undoing, going to its issue and opening its page,
// reading again, and syncing where the taskrc names a backend. Until Taskwarrior
// has answered, only reading again; while a write is on its way, no verb.
// Moving through the list is a global affordance, shown in the help rather than
// the footer, as the other list panes have it.
func (m Model) tasksKeys() []key.Binding {
	switch {
	case m.deps.Tasks.Install == nil:
		return nil
	case !m.tasks.answered():
		return []key.Binding{m.keys.refresh}
	}

	task, selected := m.currentTask()
	keys := m.taskVerbKeys(task, selected)

	if m.issueListed(task) {
		keys = append(keys, relabel(m.keys.confirm, "go to issue"))
	}

	keys = append(keys, m.linkKeys(m.taskIssueURL())...)
	keys = append(keys, m.taskListKeys()...)

	return append(append(keys, m.keys.refresh), m.syncKeys()...)
}

// taskVerbKeys are the keys that change tasks: those on the selected task where
// one is, and adding and undoing; none while a write is on its way.
func (m Model) taskVerbKeys(task taskwarrior.Task, selected bool) []key.Binding {
	switch {
	case m.tasks.writing:
		return nil
	case !selected:
		return []key.Binding{m.keys.addTask, m.keys.undoTask}
	}

	return []key.Binding{
		relabel(m.keys.startStop, startOrStop(task)), m.keys.markDone, m.keys.addTask, m.keys.annotateTask,
		m.keys.modifyTask, m.keys.undoTask,
	}
}

// startOrStop is what the start/stop key does to a task: stops it when it is
// started, and starts it otherwise.
func startOrStop(task taskwarrior.Task) string {
	if task.Active() {
		return "stop"
	}

	return verbStart
}

// verbStart names starting a task, on its key, in its last look and in a dry
// run's notice alike.
const verbStart = "start"

// syncKeys offers syncing, where the taskrc names a backend to sync with and no
// write is on its way.
func (m Model) syncKeys() []key.Binding {
	if !m.tasks.install.SyncConfigured || m.tasks.writing {
		return nil
	}

	return []key.Binding{m.keys.syncTasks}
}

// handleTasksKey answers the Tasks pane's own keys: its verbs, once Taskwarrior
// has answered and while no write is on its way, then moving, going to the
// selected task's issue, opening and copying the issue's page, and reading
// again.
func (m Model) handleTasksKey(msg tea.KeyPressMsg) (Model, tea.Cmd) {
	if m.tasks.answered() && !m.tasks.writing {
		if next, cmd, handled := m.handleTaskVerbKey(msg); handled {
			return next, cmd
		}
	}

	if next, handled := m.handleTaskListKey(msg); handled {
		return next, nil
	}

	switch {
	case key.Matches(msg, m.keys.up, m.keys.down):
		return m.moveTaskSelection(msg), nil
	case key.Matches(msg, m.keys.confirm):
		return m.goToTaskIssue()
	case key.Matches(msg, m.keys.openLink):
		return m.openLink(m.taskIssueURL())
	case key.Matches(msg, m.keys.copyLink):
		return m.copyLink(m.taskIssueURL())
	case key.Matches(msg, m.keys.refresh):
		return m.refreshPane(paneTasks)
	}

	return m, nil
}

// handleTaskVerbKey answers the keys that change tasks, reporting whether it
// claimed the key, so the caller can fall through to the list's keys.
func (m Model) handleTaskVerbKey(msg tea.KeyPressMsg) (Model, tea.Cmd, bool) {
	var act func() (Model, tea.Cmd)

	switch {
	case key.Matches(msg, m.keys.startStop):
		act = m.toggleTask
	case key.Matches(msg, m.keys.markDone):
		act = m.markDone
	case key.Matches(msg, m.keys.addTask):
		act = m.openAddLine
	case key.Matches(msg, m.keys.annotateTask):
		act = m.openAnnotateLine
	case key.Matches(msg, m.keys.modifyTask):
		act = m.openModifyLine
	case key.Matches(msg, m.keys.undoTask):
		act = m.undoTasks
	case key.Matches(msg, m.keys.syncTasks) && m.tasks.install.SyncConfigured:
		act = m.syncTasks
	default:
		return m, nil, false
	}

	next, cmd := act()

	return next, cmd, true
}

// currentTask is the task the cursor is on, once Taskwarrior has answered and
// lists one.
func (m Model) currentTask() (taskwarrior.Task, bool) {
	groups := m.taskGroups()
	if !m.tasks.answered() || len(groups.listed()) == 0 {
		return taskwarrior.Task{}, false
	}

	return groups.at(m.tasks.selected), true
}

// taskIssueURL is the page of the selected task's issue, or empty when the task
// is for no issue or there is no way to build one.
func (m Model) taskIssueURL() string {
	task, ok := m.currentTask()
	if !ok || !task.Linked() || m.deps.Jira.BrowseURL == nil {
		return ""
	}

	return m.deps.Jira.BrowseURL(jira.Key(task.IssueKey))
}

// issueListed reports whether a task is for an issue the Issues pane lists: only
// then is there a row to go to.
func (m Model) issueListed(task taskwarrior.Task) bool {
	_, listed := m.issues.find(jira.Key(task.IssueKey))

	return task.Linked() && listed
}

// goToTaskIssue focuses the Issues pane on the selected task's issue, and reads
// it, when the pane lists it.
func (m Model) goToTaskIssue() (Model, tea.Cmd) {
	task, ok := m.currentTask()
	if !ok || !m.issueListed(task) {
		return m, nil
	}

	m = m.focusOn(paneIssues)
	m.issues = m.issues.selectKey(jira.Key(task.IssueKey))
	m.issues.moved = true

	return m.loadDetail()
}

// toggleTask stops the selected task when it is started, and starts it
// otherwise.
func (m Model) toggleTask() (Model, tea.Cmd) {
	task, ok := m.currentTask()

	switch {
	case !ok:
		return m, nil
	case task.Active():
		return m.actOnTask(taskChange{verb: "stop", did: "stopped", after: ""}, m.deps.Tasks.Stop)
	default:
		return m.actOnTask(taskChange{verb: verbStart, did: "started", after: ""}, m.deps.Tasks.Start)
	}
}

// markDone marks the selected task done.
func (m Model) markDone() (Model, tea.Cmd) {
	return m.actOnTask(taskChange{verb: "mark", did: "marked", after: " done"}, m.deps.Tasks.Done)
}

// taskChange names one change of a task in its own verb: verb is the change,
// did how it is told once made, and after what follows the task's name in both
// — " done" in "mark task 3 done" and "marked 3 done".
type taskChange struct {
	verb, did, after string
}

// actOnTask sends one change of the selected task to Taskwarrior or, in a dry
// run, says what it would send and sends nothing.
func (m Model) actOnTask(change taskChange, write func(uuid string) error) (Model, tea.Cmd) {
	task, ok := m.currentTask()

	switch {
	case !ok:
		return m, nil
	case m.dryRun:
		return m.noticed("dry run: would " + change.verb + " task " + taskName(task) + change.after), nil
	}

	uuid, number := task.UUID, task.ID
	m.tasks.writing = true

	return m, func() tea.Msg {
		return taskActed{verb: change.did, after: change.after, uuid: uuid, id: number, said: "", err: write(uuid)}
	}
}

// undoTasks reverts Taskwarrior's last change, or says it would in a dry run.
func (m Model) undoTasks() (Model, tea.Cmd) {
	if m.dryRun {
		return m.noticed("dry run: would undo Taskwarrior's last change"), nil
	}

	undo := m.deps.Tasks.Undo
	m.tasks.writing = true

	return m, func() tea.Msg {
		said, err := undo()
		if errors.Is(err, taskwarrior.ErrNothingChanged) {
			return nothingToUndo{}
		}

		return taskActed{verb: "undone", uuid: "", id: 0, said: said, err: err}
	}
}

// nothingToUndo reports an undo Taskwarrior had nothing to revert for: no task
// was written, so the sentence for a write that changed nothing would mislead.
type nothingToUndo struct{}

// apply says there was nothing to undo.
func (nothingToUndo) apply(m Model) (Model, tea.Cmd) {
	m.tasks.writing = false

	return m.noticed("Taskwarrior has nothing to undo."), nil
}

// syncTasks syncs Taskwarrior with its backend, or says it would in a dry run.
func (m Model) syncTasks() (Model, tea.Cmd) {
	if m.dryRun {
		return m.noticed("dry run: would sync Taskwarrior"), nil
	}

	sync := m.deps.Tasks.Sync
	m.tasks.writing = true

	return m, func() tea.Msg {
		said, err := sync()

		return taskActed{verb: "synced", uuid: "", id: 0, said: said, err: err}
	}
}

// taskActed reports how a change of a task went: what was done, to which task —
// none for an undo, a sync, or stops the verb names — what Taskwarrior said of
// it, and why it failed.
type taskActed struct {
	verb string
	// after follows the task's name in the notice, as " done" in "marked 3 done".
	after string
	uuid  string
	id    int
	said  string
	err   error
}

// apply says what was done, or why it failed, then reads the tasks again, which
// the change may have moved. The write has answered, so the next may go.
func (msg taskActed) apply(m Model) (Model, tea.Cmd) {
	m.tasks.writing = false

	if msg.err != nil {
		return m.noticedFailure(msg.err), m.loadTasks()
	}

	note := taskNote(msg.verb, msg.id, msg.uuid) + msg.after
	if msg.said != "" {
		note += ": " + msg.said
	}

	return m.noticed(m.marks.done + " " + note), m.loadTasks()
}

// taskNote says what was done to a task: by its id, or by the start of its uuid
// where it has no id yet, or — for an undo, a sync, or stops the verb names,
// which change no one task — the verb alone.
func taskNote(verb string, taskID int, uuid string) string {
	switch {
	case taskID != 0:
		return verb + " " + strconv.Itoa(taskID)
	case uuid != "":
		return verb + " " + shortened(uuid)
	default:
		return verb
	}
}

// taskName names a task as the screen shows it: by its id, or by the start of
// its uuid where it has none.
func taskName(task taskwarrior.Task) string {
	return cmp.Or(taskNumber(task), shortened(task.UUID))
}

// shortened is the start of a uuid, as Taskwarrior's own short uuids are.
func shortened(uuid string) string {
	return uuid[:min(len(uuid), shortUUID)]
}

// taskLine is one line in Taskwarrior's own grammar being typed for a task
// command — add, annotate or modify — and how sending it is going.
type taskLine struct {
	marks  glyphs
	styles styles
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

var (
	_ failable[taskLine] = taskLine{}
	_ pasteable          = taskLine{}
)

// openTaskLine opens a line to type for a task command, starting from prefill.
// The input is sized to the overlay before prefill is set, so a line wider than
// the overlay opens scrolled to the cursor at its end.
func (m Model) openTaskLine(line taskLine, prefill string) (Model, tea.Cmd) {
	line.marks, line.styles = m.marks, m.styles
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

// openAddLine opens a line whose words become a new task.
func (m Model) openAddLine() (Model, tea.Cmd) {
	return m.openTaskLine(taskLine{title: "Add a task", command: addCommand, write: addLine(m.deps.Tasks.Add)}, "")
}

// openAnnotateLine opens a line whose words are added to the selected task as an
// annotation.
func (m Model) openAnnotateLine() (Model, tea.Cmd) {
	return m.openSelectedTaskLine("Annotate", "annotate", "annotated", m.deps.Tasks.Annotate)
}

// openModifyLine opens a line whose words change the selected task.
func (m Model) openModifyLine() (Model, tea.Cmd) {
	return m.openSelectedTaskLine("Modify", "modify", "modified", m.deps.Tasks.Modify)
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
func (l taskLine) view(width, _ int) (string, string) {
	l.input = sizedInput(l.input, width)
	l.input.SetCursor(l.input.Position())

	lines := append([]string{l.command + " " + l.marks.ellipsis, l.input.View()}, l.outcome(width)...)

	return l.title, strings.Join(lines, "\n")
}

// outcome says how sending is going, or that the line is still empty. A
// refusal is wrapped rather than cut, so Taskwarrior's own words are all seen.
func (l taskLine) outcome(width int) []string {
	switch {
	case l.sending.sending:
		return []string{"", "sending" + l.marks.ellipsis}
	case l.sending.err != nil:
		return []string{"", wrap(failureLine(l.styles, l.marks, l.sending.err), width)}
	case l.problem != nil:
		return []string{"", failureLine(l.styles, l.marks, l.problem)}
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

	return m, m.loadTasks()
}

// taskFollow is what follows once Taskwarrior has taken a task line: a command,
// given the task's uuid.
type taskFollow func(uuid string) tea.Cmd

// trackIssue is what follows adding a task for an issue: the issue's page as an
// annotation, where the tracker gives one, and the task started, when the line
// was offered as work on the issue began.
type trackIssue struct {
	url       string
	thenStart bool
}

// follow annotates the added task with the issue's page, then starts it when
// asked to, in one command whose answer reads the tasks again. A failed
// annotation keeps the task, and says how to add the page by hand.
func (t trackIssue) follow(tasks seams.Tasks) taskFollow {
	annotate, start := tasks.Annotate, tasks.Start
	verb := "added task"

	if t.thenStart {
		verb = "added and started task"
	}

	return func(uuid string) tea.Cmd {
		return func() tea.Msg {
			err := t.annotation(annotate, uuid)
			if t.thenStart {
				err = errors.Join(err, start(uuid))
			}

			return taskActed{verb: verb, uuid: uuid, id: 0, said: "", err: err}
		}
	}
}

// heldBack is how a dry run tells what would follow adding the task: annotating
// it with the issue's page, where the tracker gives one, and starting it, when
// asked to.
func (t trackIssue) heldBack() string {
	var steps []string

	if t.url != "" {
		steps = append(steps, "annotate it with "+t.url)
	}

	if t.thenStart {
		steps = append(steps, "start it")
	}

	if len(steps) == 0 {
		return ""
	}

	return " then " + strings.Join(steps, " and ")
}

// annotation adds the issue's page to the task, when there is one: an issue with
// no page adds nothing, which Taskwarrior would refuse.
func (t trackIssue) annotation(annotate func(uuid, text string) error, uuid string) error {
	if t.url == "" {
		return nil
	}

	err := annotate(uuid, t.url)
	if err != nil {
		return fmt.Errorf("%w: %w", taskwarrior.ErrAnnotateFailed, err)
	}

	return nil
}

// trackSelectedIssue goes to the task that tracks the selected issue, or says
// why the Tasks pane does not list it, and otherwise opens the line that tracks
// the issue in Taskwarrior, prefilled in its grammar.
func (m Model) trackSelectedIssue() (Model, tea.Cmd) {
	selected, ok := m.issues.current()
	if !ok || !m.canTrack(selected.Key) {
		return m, nil
	}

	if task, tracked := m.trackingTask(selected.Key); tracked {
		return m.goToTrackingTask(selected.Key, task), nil
	}

	track := trackIssue{url: m.issueURL(), thenStart: false}
	line := taskwarrior.TrackLine(taskwarrior.IssueLink{
		Key: string(selected.Key), Summary: selected.Summary, URL: track.url, Priority: selected.Priority,
	})

	return m.openTaskLine(taskLine{
		title: "Track " + string(selected.Key), command: addCommand, write: addLine(m.deps.Tasks.Add),
		after: track.follow(m.deps.Tasks), heldBack: track.heldBack(), tracks: selected.Key,
	}, line)
}

// canTrack reports whether the Issues pane can track an issue, or go to the task
// that does: a task can be added, and Taskwarrior has answered, so which issues
// are tracked already is known — and, for an issue no task tracks, no write is
// on its way, since one goes at a time.
func (m Model) canTrack(issueKey jira.Key) bool {
	if m.deps.Tasks.Add == nil || !m.tasks.answered() {
		return false
	}

	_, tracked := m.trackingTask(issueKey)

	return tracked || !m.tasks.writing
}

// trackKey is the track key as the Issues footer offers it: going to the task
// where one tracks the issue already.
func (m Model) trackKey(issueKey jira.Key) key.Binding {
	if _, tracked := m.trackingTask(issueKey); tracked {
		return relabel(m.keys.trackIssue, "go to task")
	}

	return m.keys.trackIssue
}
