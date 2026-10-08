// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"slices"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/jacob-delgado/workflow/internal/jira"
	"github.com/jacob-delgado/workflow/internal/places"
	"github.com/jacob-delgado/workflow/internal/taskwarrior"
)

// tasksState is the Tasks pane: the pending tasks of the active context, most
// urgent first, every task linked to an issue whatever its context, the
// Taskwarrior that answered, the task the cursor is on, and how far the detail
// is scrolled.
type tasksState struct {
	install taskwarrior.Install
	pending []taskwarrior.Task
	linked  []taskwarrior.Task
	context string
	loaded  bool
	err     error
	// selected is the uuid of the task the cursor is on, not its row: the rows
	// regroup whenever the Issues pane's list changes, and the cursor stays on
	// the task.
	selected string
	scroll   int
	// writing is a write sent to Taskwarrior and not answered yet. One goes at a
	// time: Taskwarrior commits writes in the order they finish, so an undo sent
	// while a done waits on a hook would revert the change before the done.
	writing bool
	// tracked counts the tracks whose add has answered. stubs are the tasks
	// those adds made that no read begun since has replaced, each linked from
	// its add's answer.
	tracked int
	stubs   []trackStub
	// listing is how the user has chosen to see the list, kept for the session
	// across every read.
	listing taskListing
	// loading is a refresh begun and not yet answered.
	loading bool
}

// trackStub is a task a track added, known from its add's answer and its
// line, and which track it was, counting from the first.
type trackStub struct {
	task  taskwarrior.Task
	track int
}

// justAdded is s with a task a track has just added, linked as the next track.
func (s tasksState) justAdded(task taskwarrior.Task) tasksState {
	s.tracked++
	s.stubs = append(slices.Clip(s.stubs), trackStub{task: task, track: s.tracked})
	s.linked = append(slices.Clip(s.linked), task)

	return s
}

// stubbed reports whether the task with a uuid is known only from its add's
// answer: no read begun since has held it.
func (s tasksState) stubbed(uuid string) bool {
	return slices.ContainsFunc(s.stubs, func(stub trackStub) bool { return stub.task.UUID == uuid })
}

// relinked is what a read begun once tracked tracks had answered links, with
// the stubs it cannot have held — a later track's, not among its linked tasks
// — and those stubs, which stay until a later read replaces them.
func (s tasksState) relinked(tracked int, linked []taskwarrior.Task) ([]taskwarrior.Task, []trackStub) {
	all := slices.Clip(linked)

	var kept []trackStub

	for _, stub := range s.stubs {
		held := slices.ContainsFunc(linked, func(task taskwarrior.Task) bool { return task.UUID == stub.task.UUID })
		if stub.track > tracked && !held {
			kept = append(kept, stub)
			all = append(all, stub.task)
		}
	}

	return all, kept
}

// answered reports whether Taskwarrior has told what it holds: it has been
// asked, and did not fail.
func (s tasksState) answered() bool {
	return s.loaded && s.err == nil
}

// tasksLoaded carries Taskwarrior's answer: the install, the pending list and
// the linked tasks, or why one of them could not be read; and how many tracks
// had answered when the read was begun.
type tasksLoaded struct {
	install taskwarrior.Install
	pending taskwarrior.List
	linked  []taskwarrior.Task
	err     error
	tracked int
}

var _ applier = tasksLoaded{}

// apply records Taskwarrior's answer, keeping the selection on the same task
// across a refresh, as the Reviews pane keeps its pull request — or on the first
// task, where that one is no longer listed — and each task a track added since
// the read was begun.
func (msg tasksLoaded) apply(m Model) (Model, tea.Cmd) {
	linked, stubs := m.tasks.relinked(msg.tracked, msg.linked)
	m.tasks = tasksState{
		install: msg.install, pending: msg.pending.Tasks, linked: linked, context: msg.pending.Context,
		loaded: true, err: msg.err, selected: m.tasks.selected, writing: m.tasks.writing,
		tracked: m.tasks.tracked, stubs: stubs, listing: m.tasks.listing,
	}

	groups := m.taskGroups()
	m.tasks.selected = groups.at(m.tasks.selected).UUID
	m.tasks = m.tasks.following(groups, m.detailRows())

	// Marks that changed can move the selection off an issue the list no longer
	// admits, onto one whose detail is not read yet.
	return m.withTaskWords().loadDetail()
}

// load is the command that asks Taskwarrior which it is, then its pending
// tasks, then the tasks linked to issues, in one command; nil when there is no
// task program to ask.
func (s tasksState) load(deps Deps) tea.Cmd {
	install, pending, linked := deps.Tasks.Install, deps.Tasks.Pending, deps.Tasks.Linked
	if install == nil {
		return nil
	}

	tracked := s.tracked

	return func() tea.Msg {
		found, err := install()
		if err != nil {
			return tasksLoaded{err: err, tracked: tracked}
		}

		list, err := pending()
		if err != nil {
			return tasksLoaded{install: found, err: err, tracked: tracked}
		}

		tasks, err := linked()

		return tasksLoaded{install: found, pending: list, linked: tasks, err: err, tracked: tracked}
	}
}

// moveTaskBy moves the cursor delta rows down the listed tasks, or up for a
// negative delta, stopping at either end, and scrolls the detail so the
// selected task stays on screen.
func (m Model) moveTaskBy(delta int) Model {
	groups := m.taskGroups()
	listed := groups.listed()

	if len(listed) == 0 {
		return m
	}

	index := max(0, min(groups.indexOf(m.tasks.selected)+delta, len(listed)-1))
	m.tasks.selected = listed[index].UUID
	m.tasks = m.tasks.following(groups, m.detailRows())

	return m
}

// pickTask selects the task on a clicked line of the detail. The heading over
// the other tasks, and a failure drawn where the list would be, select nothing.
func (m Model) pickTask(line, _ int, inRail bool) (Model, tea.Cmd) {
	drawn, onScreen := m.detailLineAt(line)
	if inRail || !onScreen || m.tasks.err != nil {
		return m, nil
	}

	groups := m.taskGroups()

	index, listed := groups.indexAt(drawn)
	if !listed {
		return m, nil
	}

	m.tasks.selected = groups.listed()[index].UUID

	return m, nil
}

// issueMarks is how the Issues rows mark each issue's task state, or nil, so the
// column is not drawn, until Taskwarrior has answered: without one, while it is
// asked, and where it could not answer — go-task on PATH, a failed read — a
// column of "no task" would claim what is not known.
func (s tasksState) issueMarks(kit renderKit) func(jira.Key) string {
	if !s.answered() {
		return nil
	}

	return func(issueKey jira.Key) string { return s.issueMark(kit, issueKey) }
}

// issueMark is an issue's task state by shape: nothing (·), tracked (○) while a
// linked task is still to do, active (◐), or every linked task completed (●).
func (s tasksState) issueMark(kit renderKit, issueKey jira.Key) string {
	glyph, _ := s.standing(kit, issueKey)

	return glyph
}

// issueWord is an issue's task state in the words its place goes by, or empty
// with no task linked to it.
func (s tasksState) issueWord(kit renderKit, issueKey jira.Key) string {
	_, word := s.standing(kit, issueKey)

	return word
}

// standing is how an issue's linked tasks stand, by shape and in words.
func (s tasksState) standing(kit renderKit, issueKey jira.Key) (string, string) {
	linked := s.linkedTo(issueKey)

	switch {
	case slices.ContainsFunc(linked, taskwarrior.Task.Active):
		return kit.marks.inFlight, places.TaskActive
	case slices.ContainsFunc(linked, stillToDo):
		return kit.marks.notStarted, places.Tracked
	case len(linked) > 0:
		return kit.marks.done, places.TaskDone
	default:
		return kit.marks.unknown, ""
	}
}

// stillToDo reports a task not completed — pending, waiting or recurring — which
// is what tracks the issue it is linked to, for the issue's mark and its track
// key alike.
func stillToDo(task taskwarrior.Task) bool {
	return task.Status != taskwarrior.Completed
}

// linkedTo is every task linked to an issue.
func (s tasksState) linkedTo(issueKey jira.Key) []taskwarrior.Task {
	var linked []taskwarrior.Task

	for _, task := range s.linked {
		if task.IssueKey == string(issueKey) {
			linked = append(linked, task)
		}
	}

	return linked
}

// issueTasksBlock is the Tasks block of an issue's detail, wrapped to width:
// each linked task with its glyph, id, description and started/due note, or the
// one line that names the key that tracks it. Nothing until Taskwarrior has
// answered, or where it could not: the Tasks pane says why; nor for an issue
// with no task where no task can be added.
func (m Model) issueTasksBlock(issueKey jira.Key, width int) []string {
	linked := m.tasks.linkedTo(issueKey)
	if !m.tasks.answered() || (len(linked) == 0 && m.deps.Tasks.Add == nil) {
		return nil
	}

	lines := []string{"", m.styles.strong.Render("Tasks")}

	if len(linked) == 0 {
		hint := m.keys.trackIssue.Help().Key + " tracks it in Taskwarrior."

		return append(lines, m.styles.label.Render(wrap(hint, width)))
	}

	now := m.deps.now()

	for _, task := range linked {
		lines = append(lines, issueTaskRows(m.kit(), task, now, width)...)
	}

	return lines
}

// issueTaskRows is one of an issue's tasks, wrapped on its own under a two-cell
// indent, so a long one still reads as an item of the block: its glyph, id and
// description, then its faint note.
func issueTaskRows(kit renderKit, task taskwarrior.Task, now time.Time, width int) []string {
	room := max(1, width-len(taskIndent))

	number := taskNumber(task)
	if number != "" {
		number = "#" + number + " "
	}

	rows := strings.Split(wrap(taskGlyph(kit.marks, task)+" "+number+task.Description, room), "\n")
	rows = withNote(kit.styles, rows, linkedTaskNote(task, now), room)

	for index, row := range rows {
		rows[index] = taskIndent + row
	}

	return rows
}

// withNote is rows of text followed by a faint note: on the last row where it
// fits, or wrapped onto rows of its own, each styled on its own so the style
// never runs on past a row's end. An empty note adds nothing.
func withNote(sty styles, rows []string, note string, width int) []string {
	last := len(rows) - 1

	switch {
	case note == "":
		return rows
	case ansi.StringWidth(rows[last]+noteGap+note) <= width:
		return append(rows[:last:last], rows[last]+noteGap+sty.label.Render(note))
	default:
		return append(rows, strings.Split(sty.label.Render(wrap(note, width)), "\n")...)
	}
}

// linkedTaskNote is what the note after an issue's task says: how long ago it
// was started, or when one still to do — pending, waiting or recurring — is
// due, or nothing.
func linkedTaskNote(task taskwarrior.Task, now time.Time) string {
	switch {
	case task.Active():
		return "started " + elapsed(task.Start, now) + " ago"
	case task.Status != taskwarrior.Completed && !task.Due.IsZero():
		return dueIn(task.Due, now)
	default:
		return ""
	}
}

// suffix names the active context beside the Tasks pane's title, when one
// narrows the list.
func (s tasksState) suffix(kit renderKit, p pane) string {
	if p != paneTasks || s.context == "" {
		return ""
	}

	return kit.marks.separator + s.context
}

// active is the first started task, pending or linked, if one is — and none
// until Taskwarrior has answered, as the task marks and an issue's Tasks block
// have it: after a failed read the Tasks pane shows the failure, not the task.
func (s tasksState) active() (taskwarrior.Task, bool) {
	if !s.answered() {
		return taskwarrior.Task{}, false
	}

	tasks := slices.Concat(s.pending, s.linked)

	index := slices.IndexFunc(tasks, taskwarrior.Task.Active)
	if index < 0 {
		return taskwarrior.Task{}, false
	}

	return tasks[index], true
}

// tasksBehavior is the Tasks pane's behavior.
func tasksBehavior() behavior {
	return behavior{
		rail:   func(m Model, _ int) string { return m.tasks.rail(m.tasksView()) },
		detail: func(m Model, width int) string { return m.tasks.detail(m.tasksView(), width) }, narrow: nil,
		keys: Model.tasksKeys, handle: Model.handleTasksKey, pick: Model.pickTask, move: commandless(Model.moveTaskBy),
		// refresh asks Taskwarrior again.
		refresh: func(m Model) (Model, tea.Cmd) {
			read := m.tasks.load(m.deps)
			m.tasks.loading = read != nil

			return m, read
		},
		loading: func(m Model) bool { return m.tasks.loading },
		scroll:  func(m *Model) *int { return &m.tasks.scroll }, listInDetail: true,
		answers: []string{
			"start-stop", "mark-done", "add-task", "annotate-task", "modify-task", "undo-task", "sync-tasks",
			"search-tasks", "filter-tasks", "sort-tasks", actionOpenLink, actionCopyLink, actionRefresh,
		},
	}
}
