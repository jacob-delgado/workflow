// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"cmp"
	"errors"
	"strconv"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"

	"github.com/jacob-delgado/workflow/internal/jira"
	"github.com/jacob-delgado/workflow/internal/taskwarrior"
)

// shortUUID is how many characters of a uuid name a task that has no id yet, as
// Taskwarrior's own short uuids do.
const shortUUID = 8

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
	keys := m.tasks.verbKeys(m.keys, task, selected)

	if m.issueListed(task) {
		keys = append(keys, relabel(m.keys.confirm, "go to issue"))
	}

	keys = append(keys, m.linkKeys(m.taskIssueURL())...)
	keys = append(keys, m.tasks.listKeys(m.keys)...)

	return append(append(keys, m.keys.refresh), m.tasks.syncKeys(m.keys)...)
}

// verbKeys are the keys that change tasks: those on the selected task where
// one is, and adding and undoing; none while a write is on its way.
func (s tasksState) verbKeys(keys keyMap, task taskwarrior.Task, selected bool) []key.Binding {
	switch {
	case s.writing:
		return nil
	case !selected:
		return []key.Binding{keys.addTask, keys.undoTask}
	}

	return []key.Binding{
		relabel(keys.startStop, startOrStop(task)), keys.markDone, keys.addTask, keys.annotateTask,
		keys.modifyTask, keys.undoTask,
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
func (s tasksState) syncKeys(keys keyMap) []key.Binding {
	if !s.install.SyncConfigured || s.writing {
		return nil
	}

	return []key.Binding{keys.syncTasks}
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
		return m.moveTaskBy(m.keys.stepOf(msg)), nil
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
		return m.actOnTask(taskChange{verb: "stop", did: "stopped"}, m.deps.Tasks.Stop)
	default:
		return m.actOnTask(taskChange{verb: verbStart, did: "started"}, m.deps.Tasks.Start)
	}
}

// markDone asks, through a last look, to mark the selected task done.
func (m Model) markDone() (Model, tea.Cmd) {
	task, ok := m.currentTask()
	if !ok {
		return m, nil
	}

	m.overlay = doneAsked(m.deps, task)

	return m, nil
}

// taskChange names one change of a task in its own verb: verb is the change,
// and did how it is told once made.
type taskChange struct {
	verb, did string
}

// actOnTask sends one change of the selected task to Taskwarrior or, in a dry
// run, says what it would send and sends nothing.
func (m Model) actOnTask(change taskChange, write func(uuid string) error) (Model, tea.Cmd) {
	task, ok := m.currentTask()

	switch {
	case !ok:
		return m, nil
	case m.dryRun:
		return m.noticed("dry run: would " + change.verb + " task " + taskName(task)), nil
	}

	uuid, number := task.UUID, task.ID
	m.tasks.writing = true

	return m, func() tea.Msg {
		return taskActed{verb: change.did, after: "", uuid: uuid, id: number, said: "", err: write(uuid)}
	}
}

// undoTasks asks, through a last look, to revert Taskwarrior's last change:
// Taskwarrior has no redo, so an undo cannot itself be taken back.
func (m Model) undoTasks() (Model, tea.Cmd) {
	undo := m.deps.Tasks.Undo

	m.overlay = asking(lastLook{
		title: "Undo in Taskwarrior", verb: "undo", doing: "undoing",
		body: "Undo Taskwarrior's last change?\n\nTaskwarrior has no redo.",
	}, "undo Taskwarrior's last change", func() tea.Msg {
		said, err := undo()
		if errors.Is(err, taskwarrior.ErrNothingChanged) {
			return nothingToUndo{}
		}

		return offerAnswered{acted: taskActed{verb: "undone", uuid: "", id: 0, said: said, err: err}, then: nil}
	})

	return m, nil
}

// nothingToUndo reports an undo Taskwarrior had nothing to revert for: no task
// was written, so the sentence for a write that changed nothing would mislead.
type nothingToUndo struct{}

var _ applier = nothingToUndo{}

// apply closes the undo's look and says there was nothing to undo.
func (nothingToUndo) apply(m Model) (Model, tea.Cmd) {
	m.tasks.writing = false

	return m.answerLook(nil).noticed("nothing to undo in Taskwarrior"), nil
}

// syncTasks asks, through a last look, to sync Taskwarrior with its server,
// which sends the tasks off the machine.
func (m Model) syncTasks() (Model, tea.Cmd) {
	sync := m.deps.Tasks.Sync

	m.overlay = asking(lastLook{
		title: "Sync Taskwarrior", verb: "sync", doing: "syncing",
		body: "Sync Taskwarrior with its server?\n\nYour tasks are sent there, and its changes taken.",
	}, "sync Taskwarrior", func() tea.Msg {
		said, err := sync()

		return offerAnswered{acted: taskActed{verb: "synced", uuid: "", id: 0, said: said, err: err}, then: nil}
	})

	return m, nil
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

var _ applier = taskActed{}

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
