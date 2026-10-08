// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/jacob-delgado/workflow/internal/jira"
	"github.com/jacob-delgado/workflow/internal/places"
	"github.com/jacob-delgado/workflow/internal/sanitize"
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

// noteGap sets a task's faint note apart from what it is.
const noteGap = "  "

// taskIndent sets an issue's tasks in under their heading.
const taskIndent = "  "

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

// refreshTasks asks Taskwarrior again.
func (m Model) refreshTasks() (Model, tea.Cmd) {
	read := m.loadTasks()
	m.tasks.loading = read != nil

	return m, read
}

// loadTasks is the command that asks Taskwarrior which it is, then its pending
// tasks, then the tasks linked to issues, in one command; nil when there is no
// task program to ask.
func (m Model) loadTasks() tea.Cmd {
	install, pending, linked := m.deps.Tasks.Install, m.deps.Tasks.Pending, m.deps.Tasks.Linked
	if install == nil {
		return nil
	}

	tracked := m.tasks.tracked

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

// selectedTask is the task the cursor is on, or no task when none is listed.
func (m Model) selectedTask() taskwarrior.Task {
	return m.taskGroups().at(m.tasks.selected)
}

// tasksRail summarizes the pane: how many tasks are pending and active, and the
// context that narrows them, or why there are none to count, or that a write is
// on its way.
func (m Model) tasksRail(_ int) string {
	switch {
	case m.deps.Tasks.Install == nil:
		return m.withoutTaskwarriorRail()
	case !m.tasks.loaded:
		return m.marks.reading()
	case m.tasks.err != nil:
		return m.failureSummary(m.tasks.err)
	case m.tasks.writing:
		return m.marks.inFlight + " sending" + m.marks.ellipsis
	}

	// The rail counts the pane's tasks whatever narrows the list, as the Reviews
	// rail counts the whole queue.
	listed := m.taskGroupsBy(taskListing{}).listed()
	counts := []string{strconv.Itoa(len(listed)) + " pending"}

	active := 0

	for _, task := range listed {
		if task.Active() {
			active++
		}
	}

	if active > 0 {
		counts = append(counts, strconv.Itoa(active)+" active")
	}

	if m.tasks.context != "" {
		counts = append(counts, "ctx "+m.tasks.context)
	}

	return strings.Join(counts, m.marks.separator)
}

// withoutTaskwarriorRail is why there is no Taskwarrior to ask, briefly: the
// integration is turned off, or no task program was found — which is not set
// up, and told as guidance.
func (m Model) withoutTaskwarriorRail() string {
	if m.cfg.Taskwarrior.Disabled {
		return "turned off"
	}

	return m.failureSummary(taskwarrior.ErrNotInstalled)
}

// withoutTaskwarriorDetail is withoutTaskwarriorRail in full, for the detail.
func (m Model) withoutTaskwarriorDetail(width int) string {
	if m.cfg.Taskwarrior.Disabled {
		return wrap("Turned off by taskwarrior.disabled.", width)
	}

	return wrap(m.failureLine(taskwarrior.ErrNotInstalled), width)
}

// tasksDetail lists the pending tasks, then describes the selected one.
func (m Model) tasksDetail(width int) string {
	groups := m.taskGroups()

	switch {
	case m.deps.Tasks.Install == nil:
		return m.withoutTaskwarriorDetail(width)
	case !m.tasks.loaded:
		return m.marks.reading()
	case m.tasks.err != nil:
		return m.failureBlock(m.tasks.err, width)
	case len(groups.listed()) == 0 && len(m.tasks.pending) > 0 && m.tasks.listing.narrows():
		return strings.Join(append(m.taskRows(groups, width), "No task matches the filters."), "\n")
	case len(groups.listed()) == 0:
		return strings.Join(append([]string{"No pending tasks."}, m.waitingRow(groups)...), "\n")
	}

	return strings.Join(append(m.taskRows(groups, width), "", m.selectedTaskDetail(width)), "\n")
}

// taskRows draws the listed tasks one a row: how the list is listed, when that
// is not most urgent first, then those for listed issues, a faint heading over
// the others when there are both, then how many wait unlisted.
func (m Model) taskRows(groups taskGroups, width int) []string {
	now := m.deps.now()
	selected := groups.indexOf(m.tasks.selected)
	rows := make([]string, 0, groups.lines()+1)

	if groups.lead > 0 {
		heading := ansi.Truncate(sanitize.Line(m.tasks.listing.heading(m.marks)), width, m.marks.ellipsis)
		rows = append(rows, m.styles.label.Render(heading), "")
	}

	for index, task := range groups.listed() {
		if groups.headed() && index == len(groups.forIssues) {
			rows = append(rows, m.styles.label.Render("Other"))
		}

		rows = append(rows, m.taskRow(task, index == selected, now))
	}

	return append(rows, m.waitingRow(groups)...)
}

// waitingRow is how many pending tasks wait unlisted, on a faint row, or no row
// when none does.
func (m Model) waitingRow(groups taskGroups) []string {
	if groups.waiting == 0 {
		return nil
	}

	return []string{m.styles.label.Render(strconv.Itoa(groups.waiting) + " waiting")}
}

// taskRow draws one task: the cursor, whether it is started, its id and what it
// is, then a faint tail of the issue it is for, when it is due and how urgent
// it is. Taskwarrior's client has already neutralized every value.
func (m Model) taskRow(task taskwarrior.Task, selected bool, now time.Time) string {
	head := m.marks.marker(selected) + m.taskGlyph(task) + " " + fmt.Sprintf("%3s", taskNumber(task)) + " " +
		task.Description

	var tail []string

	if task.Linked() {
		tail = append(tail, task.IssueKey)
	}

	if !task.Due.IsZero() {
		tail = append(tail, dueIn(task.Due, now))
	}

	if task.Waiting(now) {
		tail = append(tail, "waits until "+task.Wait.In(now.Location()).Format(time.DateOnly))
	}

	if sortedBy := m.taskSortKey(task, now); sortedBy != "" {
		tail = append(tail, sortedBy)
	}

	tail = append(tail, fmt.Sprintf("%.1f", task.Urgency))

	return head + noteGap + m.styles.label.Render(strings.Join(tail, m.marks.separator))
}

// taskSortKey is what a row adds to its tail so the order it is listed in can
// be read off it: its priority, or its tags, when the list is sorted by that,
// as the filter names them.
func (m Model) taskSortKey(task taskwarrior.Task, now time.Time) string {
	switch m.tasks.listing.order {
	case taskOrderPriority:
		return facetLabels(task, taskwarrior.FacetPriority, now)
	case taskOrderTag:
		return facetLabels(task, taskwarrior.FacetTag, now)
	case taskOrderUrgency, taskOrderState, taskOrderID, taskOrderIssue:
		return ""
	}

	return ""
}

// facetLabels names the values a task holds in one kind, as the filter does.
func facetLabels(task taskwarrior.Task, kind taskwarrior.FacetKind, now time.Time) string {
	var labels []string

	for _, facet := range task.Facets(now) {
		if facet.Kind == kind {
			labels = append(labels, facet.Label())
		}
	}

	return strings.Join(labels, " ")
}

// selectedTaskDetail describes the selected task: what it is, its facts, the
// issue it is for, and its annotations. A styled line is wrapped before it is
// styled, so each of its rows opens and closes the style rather than one row
// running it on into the border and the rail beside the next.
func (m Model) selectedTaskDetail(width int) string {
	task := m.selectedTask()
	lines := []string{
		m.styles.strong.Render(wrap(task.Description, width)),
		m.styles.label.Render(wrap(m.taskFacts(task), width)),
	}

	if task.Linked() {
		lines = append(lines, m.taskIssueLine(task))
	}

	return wrap(strings.Join(append(lines, m.taskAnnotations(task)...), "\n"), width)
}

// taskFacts is a task's facts on one line: how long ago it was started, its
// project, tags, due date, urgency and id, leaving out any it does not have. The
// due date is the day in the clock's zone, not in UTC, which Taskwarrior keeps:
// a task due at midnight east of UTC is due that day, not the day before.
func (m Model) taskFacts(task taskwarrior.Task) string {
	now := m.deps.now()

	var facts []string

	if task.Active() {
		facts = append(facts, "started "+elapsed(task.Start, now)+" ago")
	}

	if task.Project != "" {
		facts = append(facts, task.Project)
	}

	if task.Priority != "" {
		facts = append(facts, facetLabels(task, taskwarrior.FacetPriority, now))
	}

	if len(task.Tags) > 0 {
		facts = append(facts, facetLabels(task, taskwarrior.FacetTag, now))
	}

	if !task.Due.IsZero() {
		facts = append(facts, "due "+task.Due.In(now.Location()).Format(time.DateOnly))
	}

	facts = append(facts, fmt.Sprintf("urgency %.1f", task.Urgency))

	if number := taskNumber(task); number != "" {
		facts = append(facts, "#"+number)
	}

	return strings.Join(facts, m.marks.separator)
}

// taskIssueLine is the issue a task is for, with its summary and status where
// the Issues pane lists it, and — when the checked-out branch names it — that
// branch and its pull request.
func (m Model) taskIssueLine(task taskwarrior.Task) string {
	issueKey := jira.Key(task.IssueKey)
	parts := []string{task.IssueKey}

	if issue, listed := m.issues.find(issueKey); listed {
		parts = []string{task.IssueKey + " " + issue.Summary, issue.Status}
	}

	if branchKey, named := m.branchIssue(); named && branchKey == issueKey {
		parts = append(parts, m.branchAndPull()...)
	}

	return strings.Join(parts, m.marks.separator)
}

// branchAndPull is the checked-out branch and, when the Review pane holds one,
// its pull request's number and how its CI stands.
func (m Model) branchAndPull() []string {
	parts := []string{m.branch.branch.Name}

	if m.review.found {
		parts = append(parts, m.vocab.sigil+strconv.Itoa(m.review.pull.Number)+" "+m.ciStateGlyph(m.review.ci.State))
	}

	return parts
}

// taskAnnotations is a task's annotations under their heading, each after the
// day it was written in the clock's zone, or nothing for a task with none.
func (m Model) taskAnnotations(task taskwarrior.Task) []string {
	if len(task.Annotations) == 0 {
		return nil
	}

	zone := m.deps.now().Location()
	lines := []string{"", m.styles.strong.Render("Annotations")}

	for _, note := range task.Annotations {
		lines = append(lines, m.styles.label.Render(note.Entry.In(zone).Format(time.DateOnly))+" "+note.Description)
	}

	return lines
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

// taskMarks is how the Issues rows mark each issue's task state, or nil, so the
// column is not drawn, until Taskwarrior has answered: without one, while it is
// asked, and where it could not answer — go-task on PATH, a failed read — a
// column of "no task" would claim what is not known.
func (m Model) taskMarks() func(jira.Key) string {
	if !m.tasks.answered() {
		return nil
	}

	return m.taskMark
}

// taskMark is an issue's task state by shape: nothing (·), tracked (○) while a
// linked task is still to do, active (◐), or every linked task completed (●).
func (m Model) taskMark(issueKey jira.Key) string {
	glyph, _ := m.taskStanding(issueKey)

	return glyph
}

// taskWord is an issue's task state in the words its place goes by, or empty
// with no task linked to it.
func (m Model) taskWord(issueKey jira.Key) string {
	_, word := m.taskStanding(issueKey)

	return word
}

// taskStanding is how an issue's linked tasks stand, by shape and in words.
func (m Model) taskStanding(issueKey jira.Key) (string, string) {
	linked := m.linkedTo(issueKey)

	switch {
	case slices.ContainsFunc(linked, taskwarrior.Task.Active):
		return m.marks.inFlight, places.TaskActive
	case slices.ContainsFunc(linked, stillToDo):
		return m.marks.notStarted, places.Tracked
	case len(linked) > 0:
		return m.marks.done, places.TaskDone
	default:
		return m.marks.unknown, ""
	}
}

// stillToDo reports a task not completed — pending, waiting or recurring — which
// is what tracks the issue it is linked to, for the issue's mark and its track
// key alike.
func stillToDo(task taskwarrior.Task) bool {
	return task.Status != taskwarrior.Completed
}

// linkedTo is every task linked to an issue.
func (m Model) linkedTo(issueKey jira.Key) []taskwarrior.Task {
	var linked []taskwarrior.Task

	for _, task := range m.tasks.linked {
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
	linked := m.linkedTo(issueKey)
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
		lines = append(lines, m.issueTaskRows(task, now, width)...)
	}

	return lines
}

// issueTaskRows is one of an issue's tasks, wrapped on its own under a two-cell
// indent, so a long one still reads as an item of the block: its glyph, id and
// description, then its faint note.
func (m Model) issueTaskRows(task taskwarrior.Task, now time.Time, width int) []string {
	room := max(1, width-len(taskIndent))

	number := taskNumber(task)
	if number != "" {
		number = "#" + number + " "
	}

	rows := strings.Split(wrap(m.taskGlyph(task)+" "+number+task.Description, room), "\n")
	rows = m.withNote(rows, linkedTaskNote(task, now), room)

	for index, row := range rows {
		rows[index] = taskIndent + row
	}

	return rows
}

// withNote is rows of text followed by a faint note: on the last row where it
// fits, or wrapped onto rows of its own, each styled on its own so the style
// never runs on past a row's end. An empty note adds nothing.
func (m Model) withNote(rows []string, note string, width int) []string {
	last := len(rows) - 1

	switch {
	case note == "":
		return rows
	case ansi.StringWidth(rows[last]+noteGap+note) <= width:
		return append(rows[:last:last], rows[last]+noteGap+m.styles.label.Render(note))
	default:
		return append(rows, strings.Split(m.styles.label.Render(wrap(note, width)), "\n")...)
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

// tasksSuffix names the active context beside the Tasks pane's title, when one
// narrows the list.
func (m Model) tasksSuffix(p pane) string {
	if p != paneTasks || m.tasks.context == "" {
		return ""
	}

	return m.marks.separator + m.tasks.context
}

// activeTask is the first started task, pending or linked, if one is — and none
// until Taskwarrior has answered, as the task marks and an issue's Tasks block
// have it: after a failed read the Tasks pane shows the failure, not the task.
func (m Model) activeTask() (taskwarrior.Task, bool) {
	if !m.tasks.answered() {
		return taskwarrior.Task{}, false
	}

	tasks := slices.Concat(m.tasks.pending, m.tasks.linked)

	index := slices.IndexFunc(tasks, taskwarrior.Task.Active)
	if index < 0 {
		return taskwarrior.Task{}, false
	}

	return tasks[index], true
}

// taskGlyph is how far a task has got, by shape: started, completed, or not
// started yet.
func (m Model) taskGlyph(task taskwarrior.Task) string {
	switch {
	case task.Active():
		return m.marks.inFlight
	case task.Status == taskwarrior.Completed:
		return m.marks.done
	default:
		return m.marks.notStarted
	}
}

// taskNumber is a task's working-set id, or "" where the id means nothing: only
// a pending or waiting task has one, and reads skip the garbage collection that
// would clear a finished task's stale id.
func taskNumber(task taskwarrior.Task) string {
	if task.ID == 0 || (task.Status != taskwarrior.Pending && task.Status != taskwarrior.Waiting) {
		return ""
	}

	return strconv.Itoa(task.ID)
}

// dueIn says how soon a task is due, or that it is overdue.
func dueIn(due, now time.Time) string {
	if !due.After(now) {
		return "overdue"
	}

	return "due in " + elapsed(now, due)
}
