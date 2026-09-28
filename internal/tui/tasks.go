// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/jacob-delgado/workflow/internal/jira"
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
// the linked tasks, or why one of them could not be read.
type tasksLoaded struct {
	install taskwarrior.Install
	pending taskwarrior.List
	linked  []taskwarrior.Task
	err     error
}

var _ applier = tasksLoaded{}

// apply records Taskwarrior's answer, keeping the selection on the same task
// across a refresh, as the Reviews pane keeps its pull request — or on the first
// task, where that one is no longer listed.
func (msg tasksLoaded) apply(m Model) (Model, tea.Cmd) {
	m.tasks = tasksState{
		install: msg.install, pending: msg.pending.Tasks, linked: msg.linked, context: msg.pending.Context,
		loaded: true, err: msg.err, selected: m.tasks.selected,
	}

	groups := m.taskGroups()
	m.tasks.selected = groups.at(m.tasks.selected).UUID
	m.tasks = m.tasks.following(groups, m.detailRows())

	return m, nil
}

// loadTasks is the command that asks Taskwarrior which it is, then its pending
// tasks, then the tasks linked to issues, in one command; nil when there is no
// task program to ask.
func (m Model) loadTasks() tea.Cmd {
	install, pending, linked := m.deps.Tasks.Install, m.deps.Tasks.Pending, m.deps.Tasks.Linked
	if install == nil {
		return nil
	}

	return func() tea.Msg {
		found, err := install()
		if err != nil {
			return tasksLoaded{err: err}
		}

		list, err := pending()
		if err != nil {
			return tasksLoaded{install: found, err: err}
		}

		tasks, err := linked()

		return tasksLoaded{install: found, pending: list, linked: tasks, err: err}
	}
}

// taskGroups is the pending tasks as the pane lists them: those for an issue the
// Issues pane has loaded, then the others, each most urgent first, and how many
// wait unlisted.
type taskGroups struct {
	forIssues, others []taskwarrior.Task
	waiting           int
}

// taskGroups sorts the pending tasks into the pane's groups.
func (m Model) taskGroups() taskGroups {
	now := m.deps.now()

	var groups taskGroups

	for _, task := range m.tasks.pending {
		_, listed := m.issues.find(jira.Key(task.IssueKey))

		switch {
		case task.Waiting(now):
			groups.waiting++
		case task.Linked() && listed:
			groups.forIssues = append(groups.forIssues, task)
		default:
			groups.others = append(groups.others, task)
		}
	}

	return groups
}

// listed is every task the pane lists, in the order it draws them.
func (g taskGroups) listed() []taskwarrior.Task {
	return slices.Concat(g.forIssues, g.others)
}

// headed reports a heading drawn over the other tasks, which is only where
// there are tasks for listed issues above them.
func (g taskGroups) headed() bool {
	return len(g.forIssues) > 0 && len(g.others) > 0
}

// lines is how many lines the list takes: a row per task, and the heading.
func (g taskGroups) lines() int {
	if g.headed() {
		return len(g.listed()) + 1
	}

	return len(g.listed())
}

// lineOf is the line of the list a listed task is drawn on, past the heading.
func (g taskGroups) lineOf(index int) int {
	if g.headed() && index >= len(g.forIssues) {
		return index + 1
	}

	return index
}

// indexAt is the listed task drawn on a line of the list, and whether a task is
// drawn there rather than the heading or nothing.
func (g taskGroups) indexAt(line int) (int, bool) {
	if !g.headed() || line < len(g.forIssues) {
		return line, line >= 0 && line < len(g.listed())
	}

	return line - 1, line > len(g.forIssues) && line-1 < len(g.listed())
}

// indexOf is where the listed task with a uuid sits, or the first task where
// none has it.
func (g taskGroups) indexOf(uuid string) int {
	listed := g.listed()
	index := slices.IndexFunc(listed, func(candidate taskwarrior.Task) bool { return candidate.UUID == uuid })

	return max(0, min(index, len(listed)-1))
}

// at is the listed task with a uuid, or the first where none has it, or no task
// where none is listed.
func (g taskGroups) at(uuid string) taskwarrior.Task {
	listed := g.listed()
	if len(listed) == 0 {
		return taskwarrior.Task{}
	}

	return listed[g.indexOf(uuid)]
}

// following is the pane scrolled so the selected task's row shows in rows lines.
func (s tasksState) following(groups taskGroups, rows int) tasksState {
	s.scroll, _ = window(groups.lineOf(groups.indexOf(s.selected)), groups.lines(), rows)

	return s
}

// selectedTask is the task the cursor is on, or no task when none is listed.
func (m Model) selectedTask() taskwarrior.Task {
	return m.taskGroups().at(m.tasks.selected)
}

// tasksRail summarizes the pane: how many tasks are pending and active, and the
// context that narrows them, or why there are none to count.
func (m Model) tasksRail(_ int) string {
	switch {
	case m.deps.Tasks.Install == nil:
		return m.withoutTaskwarrior().brief
	case !m.tasks.loaded:
		return "looking" + m.marks.ellipsis
	case m.tasks.err != nil:
		return m.failureSummary(m.tasks.err)
	}

	listed := m.taskGroups().listed()
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

// withoutTaskwarrior is why there is no Taskwarrior to ask, briefly for the rail
// and in full for the detail: the integration is turned off, or no task program
// was found.
func (m Model) withoutTaskwarrior() wording {
	if m.cfg.Taskwarrior.Disabled {
		return wording{brief: "turned off", full: "Turned off by taskwarrior.disabled."}
	}

	return wording{brief: "not installed", full: inFull(taskwarrior.ErrNotInstalled)}
}

// tasksDetail lists the pending tasks, then describes the selected one.
func (m Model) tasksDetail(width int) string {
	groups := m.taskGroups()

	switch {
	case m.deps.Tasks.Install == nil:
		return wrap(m.withoutTaskwarrior().full, width)
	case !m.tasks.loaded:
		return "looking" + m.marks.ellipsis
	case m.tasks.err != nil:
		return m.failureBlock(m.tasks.err, width)
	case len(groups.listed()) == 0:
		return strings.Join(append([]string{"No pending tasks."}, m.waitingRow(groups)...), "\n")
	}

	return strings.Join(append(m.taskRows(groups), "", m.selectedTaskDetail(width)), "\n")
}

// taskRows draws the listed tasks one a row: those for listed issues, a faint
// heading over the others when there are both, then how many wait unlisted.
func (m Model) taskRows(groups taskGroups) []string {
	now := m.deps.now()
	selected := groups.indexOf(m.tasks.selected)
	rows := make([]string, 0, groups.lines()+1)

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

	tail = append(tail, fmt.Sprintf("%.1f", task.Urgency))

	return head + noteGap + m.styles.label.Render(strings.Join(tail, m.marks.separator))
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

	if len(task.Tags) > 0 {
		facts = append(facts, "+"+strings.Join(task.Tags, " +"))
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

// moveTaskSelection moves the selection down or up, stopping at either end,
// and scrolls the detail so the selected task stays on screen.
func (m Model) moveTaskSelection(msg tea.KeyPressMsg) Model {
	groups := m.taskGroups()
	listed := groups.listed()

	if len(listed) == 0 {
		return m
	}

	index := groups.indexOf(m.tasks.selected)
	if key.Matches(msg, m.keys.down) {
		index = min(index+1, len(listed)-1)
	} else {
		index = max(0, index-1)
	}

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
// linked task is still to do — pending, waiting or recurring — active (◐), or
// every linked task completed (●).
func (m Model) taskMark(issueKey jira.Key) string {
	linked := m.linkedTo(issueKey)

	switch {
	case slices.ContainsFunc(linked, taskwarrior.Task.Active):
		return m.marks.inFlight
	case slices.ContainsFunc(linked, func(task taskwarrior.Task) bool { return task.Status != taskwarrior.Completed }):
		return m.marks.notStarted
	case len(linked) > 0:
		return m.marks.done
	default:
		return m.marks.unknown
	}
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
// one line that says T tracks it. Nothing until Taskwarrior has answered, or
// where it could not: the Tasks pane says why.
func (m Model) issueTasksBlock(issueKey jira.Key, width int) []string {
	if !m.tasks.answered() {
		return nil
	}

	lines := []string{"", m.styles.strong.Render("Tasks")}

	linked := m.linkedTo(issueKey)
	if len(linked) == 0 {
		return append(lines, m.styles.label.Render(wrap("T tracks it in Taskwarrior.", width)))
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
