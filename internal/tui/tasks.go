// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"context"
	"slices"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/jacob-delgado/workflow/internal/jira"
	"github.com/jacob-delgado/workflow/internal/places"
	"github.com/jacob-delgado/workflow/internal/taskwarrior"
	"github.com/jacob-delgado/workflow/internal/tui/latest"
)

// tasksState is the Tasks pane: what the latest read of Taskwarrior found,
// the task the cursor is on, and how far the detail is scrolled.
type tasksState struct {
	read latest.Read[taskList]
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
	// justDone are the uuids of the tasks dones have marked since the latest
	// read that succeeded: each is still listed as the read before its done had
	// it, and offers neither a start nor a done until a read describes it
	// again. A done's answer begins the read after it, so the next read whose
	// answer is taken was begun after every done, and describes them all.
	justDone []string
	// listing is how the user has chosen to see the list, kept for the session
	// across every read.
	listing taskListing
}

// taskList is what one read of Taskwarrior found: which Taskwarrior
// answered, the pending tasks of the active context, most urgent first, every
// task linked to an issue whatever its context, and that context.
type taskList struct {
	install taskwarrior.Install
	pending []taskwarrior.Task
	linked  []taskwarrior.Task
	context string
}

// trackStub is a task a track added, known from its add's answer and its
// line, and which track it was, counting from the first.
type trackStub struct {
	task  taskwarrior.Task
	track int
}

// held is the tasks the latest successful read found, kept across a read
// that failed since; none before one has.
func (s tasksState) held() taskList {
	list, _ := s.read.Value()

	return list
}

// holdsList reports whether a read has found the tasks, to list and move
// through even after a refresh that failed.
func (s tasksState) holdsList() bool {
	_, held := s.read.Value()

	return held
}

// pending is the pending tasks held.
func (s tasksState) pending() []taskwarrior.Task {
	return s.held().pending
}

// linked is every task held linked to an issue, then each a track added that
// no read begun since has held.
func (s tasksState) linked() []taskwarrior.Task {
	linked := slices.Clip(s.held().linked)

	for _, stub := range s.stubs {
		linked = append(linked, stub.task)
	}

	return linked
}

// install is the Taskwarrior that answered the read held.
func (s tasksState) install() taskwarrior.Install {
	return s.held().install
}

// context is the context that narrowed the pending tasks held, or empty.
func (s tasksState) context() string {
	return s.held().context
}

// answeredWrite is s once the write out has answered, so the next may go.
func (s tasksState) answeredWrite() tasksState {
	s.writing = false

	return s
}

// markedDone is s with the task a done has just marked.
func (s tasksState) markedDone(uuid string) tasksState {
	s.justDone = append(slices.Clip(s.justDone), uuid)

	return s
}

// justMarkedDone reports whether a done has marked the task with a uuid that
// no read begun since has answered.
func (s tasksState) justMarkedDone(uuid string) bool {
	return slices.Contains(s.justDone, uuid)
}

// justAdded is s with a task a track has just added, linked as the next track.
func (s tasksState) justAdded(task taskwarrior.Task) tasksState {
	s.tracked++
	s.stubs = append(slices.Clip(s.stubs), trackStub{task: task, track: s.tracked})

	return s
}

// stubbed reports whether the task with a uuid is known only from its add's
// answer: no read begun since has held it.
func (s tasksState) stubbed(uuid string) bool {
	return slices.ContainsFunc(s.stubs, func(stub trackStub) bool { return stub.task.UUID == uuid })
}

// described is s once a read has found linked, begun when the count of
// tracks answered was tracked: the stubs it cannot have held — a later
// track's, not among its linked tasks — stay, and every done's mark goes,
// since the read was begun after every done. A track's add, unlike a done,
// can answer before the read after it is begun: its line may annotate or
// start the task first.
func (s tasksState) described(tracked int, linked []taskwarrior.Task) tasksState {
	s.stubs = slices.DeleteFunc(slices.Clone(s.stubs), func(stub trackStub) bool {
		held := slices.ContainsFunc(linked, func(task taskwarrior.Task) bool { return task.UUID == stub.task.UUID })

		return stub.track <= tracked || held
	})
	s.justDone = nil

	return s
}

// answered reports whether Taskwarrior has told what it holds: the latest
// read answered, and did not fail. Only then does the pane write from the
// tasks it lists.
func (s tasksState) answered() bool {
	return s.read.Stage() == latest.StageShown
}

// tasksLoaded carries Taskwarrior's answer to the read begun in round: the
// install, the pending list and the linked tasks, or why one of them could
// not be read; and how many tracks had answered when it was begun.
type tasksLoaded struct {
	list    taskList
	err     error
	tracked int
	round   latest.Round
}

var _ applier = tasksLoaded{}

// apply records Taskwarrior's answer, keeping the selection on the same task
// across a refresh, as the Reviews pane keeps its pull request — or on the first
// task, where that one is no longer listed — and each task a track added
// since the read was begun. The answer of a read a later one
// superseded is dropped: a write's answer begins the read after it, so an
// earlier read would list the tasks as they stood before the write. A failed
// refresh keeps the tasks listed, beside its failure.
func (msg tasksLoaded) apply(m Model) (Model, tea.Cmd) {
	read, taken := m.tasks.read.Answered(msg.round, msg.list, msg.err)
	if !taken {
		return m, nil
	}

	m.tasks.read = read
	if msg.err == nil {
		m.tasks = m.tasks.described(msg.tracked, msg.list.linked)
	}

	groups := m.taskGroups()
	m.tasks.selected = groups.at(m.tasks.selected).UUID
	m.tasks = m.tasks.following(groups, m.detailRows())

	// Marks that changed can move the selection off an issue the list no longer
	// admits, onto one whose detail is not read yet.
	return m.withTaskWords().followDetail()
}

// readingTasks begins a read of Taskwarrior, superseding any still out, and
// is the command that reads it; with no task program to ask, it begins
// nothing.
func (m Model) readingTasks() (Model, tea.Cmd) {
	if m.deps.Tasks.Install == nil {
		return m, nil
	}

	read, round := m.tasks.read.Begin()
	m.tasks.read = read

	return m, m.tasks.load(m.session, m.deps, round)
}

// load is the command that asks Taskwarrior, under ctx, which it is, then its
// pending tasks, then the tasks linked to issues, in one command, as the read
// begun in round; nil when there is no task program to ask.
func (s tasksState) load(ctx context.Context, deps Deps, round latest.Round) tea.Cmd {
	install, pending, linked := deps.Tasks.Install, deps.Tasks.Pending, deps.Tasks.Linked
	if install == nil {
		return nil
	}

	begun := tasksLoaded{tracked: s.tracked, round: round}

	return func() tea.Msg {
		answer := begun

		answer.list.install, answer.err = install(ctx)
		if answer.err != nil {
			return answer
		}

		var list taskwarrior.List

		list, answer.err = pending(ctx)
		answer.list.pending, answer.list.context = list.Tasks, list.Context

		if answer.err != nil {
			return answer
		}

		answer.list.linked, answer.err = linked(ctx)

		return answer
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
// the other tasks, and a failure drawn where no list is held, select nothing.
func (m Model) pickTask(line, _ int, inRail bool) (Model, tea.Cmd) {
	drawn, onScreen := m.detailLineAt(line)
	if inRail || !onScreen {
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

// issueMarks is how the Issues rows mark each issue's task state at now, as
// the tasks held have it, or nil, so the column is not drawn, until a read has
// found them: without Taskwarrior, while it is first asked, and where it could
// not answer — go-task on PATH, a first read that failed — a column of "no
// task" would claim what is not known. A refresh that fails since keeps the
// marks, as the Tasks pane keeps the tasks listed.
func (s tasksState) issueMarks(kit renderKit, now time.Time) func(jira.Key) string {
	if !s.holdsList() {
		return nil
	}

	return func(issueKey jira.Key) string { return s.issueMark(kit, issueKey, now) }
}

// issueMark is an issue's task state by shape: nothing (·), tracked (○) while a
// linked task is still to do, active (◐), or every linked task completed (●).
func (s tasksState) issueMark(kit renderKit, issueKey jira.Key, now time.Time) string {
	glyph, _ := s.standing(kit, issueKey, now)

	return glyph
}

// issueWord is an issue's task state in the words its place goes by, or empty
// with no task linked to it.
func (s tasksState) issueWord(kit renderKit, issueKey jira.Key, now time.Time) string {
	_, word := s.standing(kit, issueKey, now)

	return word
}

// standing is how an issue's linked tasks stand at now, by shape and in words:
// a started task that waits is still to do, as the Tasks list shows it.
func (s tasksState) standing(kit renderKit, issueKey jira.Key, now time.Time) (string, string) {
	linked := s.linkedTo(issueKey)

	switch {
	case slices.ContainsFunc(linked, startedAt(now)):
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

	for _, task := range s.linked() {
		if task.IssueKey == string(issueKey) {
			linked = append(linked, task)
		}
	}

	return linked
}

// issueTasksBlock is the Tasks block of an issue's detail, wrapped to width:
// each linked task held with its glyph, id, description and started/due note,
// or the one line that names the key that tracks it. Nothing until a read has
// found the tasks, or where it could not: the Tasks pane says why; nor for an
// issue with no task where none can be added now — no task program to add
// one, or a refresh that failed since, which holds the track key back.
func (m Model) issueTasksBlock(issueKey jira.Key, width int) []string {
	linked := m.tasks.linkedTo(issueKey)
	if !m.tasks.holdsList() || (len(linked) == 0 && (m.deps.Tasks.Add == nil || !m.tasks.answered())) {
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

	rows := strings.Split(wrap(taskGlyph(kit.marks, task, now)+" "+number+task.Description, room), "\n")
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
	case task.Started(now):
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
	if p != paneTasks || s.context() == "" {
		return ""
	}

	return kit.marks.separator + s.context()
}

// active is the first task held started at now, pending or linked, if one is
// — and none until a read has found the tasks, as the task marks and an
// issue's Tasks block have it. A refresh that fails since keeps it, as the
// Tasks pane keeps the task listed: only the verbs wait for a read that
// succeeds.
func (s tasksState) active(now time.Time) (taskwarrior.Task, bool) {
	if !s.holdsList() {
		return taskwarrior.Task{}, false
	}

	tasks := slices.Concat(s.pending(), s.linked())

	index := slices.IndexFunc(tasks, startedAt(now))
	if index < 0 {
		return taskwarrior.Task{}, false
	}

	return tasks[index], true
}

// startedAt is whether a task is started as the lists show it at now, for
// finding one among many.
func startedAt(now time.Time) func(taskwarrior.Task) bool {
	return func(task taskwarrior.Task) bool { return task.Started(now) }
}

// tasksBehavior is the Tasks pane's behavior.
func tasksBehavior() behavior {
	return behavior{
		rail:   func(m Model, _ int) string { return m.tasks.rail(m.tasksView()) },
		detail: func(m Model, width int) string { return m.tasks.detail(m.tasksView(), width) }, narrow: nil,
		keys: Model.tasksKeys, handle: Model.handleTasksKey, pick: Model.pickTask, move: commandless(Model.moveTaskBy),
		// refresh asks Taskwarrior again.
		refresh: Model.readingTasks,
		loading: func(m Model) bool { return m.tasks.read.Refreshing() },
		scroll:  func(m *Model) *int { return &m.tasks.scroll }, listInDetail: true,
		answers: []string{
			"start-stop", actionMarkDone, "add-task", "annotate-task", "modify-task", actionUndoTask, actionSyncTasks,
			"search-tasks", "filter-tasks", "sort-tasks", actionOpenLink, actionCopyLink, actionRefresh,
		},
	}
}
