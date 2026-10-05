// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"slices"
	"time"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"

	"github.com/jacob-delgado/workflow/internal/jira"
	"github.com/jacob-delgado/workflow/internal/taskwarrior"
)

// taskOrder is how the Tasks pane orders the tasks within each of its groups.
type taskOrder int

const (
	taskOrderUrgency taskOrder = iota
	taskOrderState
	taskOrderID
	taskOrderTag
	taskOrderIssue
	taskOrderPriority
)

// taskOrderCount is untyped on purpose: typed as taskOrder, the exhaustive
// linter would count it as an order and demand a case for it.
const taskOrderCount = 6

// next is the order after o, back to the first after the last.
func (o taskOrder) next() taskOrder {
	return (o + 1) % taskOrderCount
}

// title names the order where the list says how it is sorted.
func (o taskOrder) title() string {
	titles := [taskOrderCount]string{"most urgent first", "by state", "by id", "by tag", "by issue", "by priority"}

	return titles[o]
}

// sorted is tasks in the order, judged at now where the order is by state.
func (o taskOrder) sorted(tasks []taskwarrior.Task, now time.Time) []taskwarrior.Task {
	orders := [taskOrderCount]func([]taskwarrior.Task) []taskwarrior.Task{
		taskwarrior.ByUrgency,
		func(tasks []taskwarrior.Task) []taskwarrior.Task { return taskwarrior.ByState(tasks, now) },
		taskwarrior.ByID, taskwarrior.ByTag, taskwarrior.ByIssue, taskwarrior.ByPriority,
	}

	return orders[o](tasks)
}

// taskListing is how the user has chosen to see the Tasks list for the
// session: its order. Changing it reads nothing again.
type taskListing struct {
	order taskOrder
}

// titled reports a list that says how it is listed, above its rows: any order
// but the most urgent first the pane opens on.
func (l taskListing) titled() bool {
	return l.order != taskOrderUrgency
}

// heading says how the list is listed.
func (l taskListing) heading() string {
	return l.order.title()
}

// taskGroups is the pending tasks as the pane lists them: those for an issue the
// Issues pane has loaded, then the others, each in the listing's order, and how
// many wait unlisted. lead is how many lines the list's heading takes above the
// rows.
type taskGroups struct {
	forIssues, others []taskwarrior.Task
	waiting           int
	lead              int
}

// taskGroups sorts the pending tasks into the pane's groups, as listed now.
func (m Model) taskGroups() taskGroups {
	return m.taskGroupsBy(m.tasks.listing)
}

// taskGroupsBy sorts the pending tasks into the pane's groups, in listing's
// order.
func (m Model) taskGroupsBy(listing taskListing) taskGroups {
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

	groups.forIssues = listing.order.sorted(groups.forIssues, now)
	groups.others = listing.order.sorted(groups.others, now)

	if listing.titled() {
		groups.lead = 2
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

// lines is how many lines the list takes: the list's heading, a row per task,
// and the heading over the other tasks.
func (g taskGroups) lines() int {
	if g.headed() {
		return g.lead + len(g.listed()) + 1
	}

	return g.lead + len(g.listed())
}

// lineOf is the line of the list a listed task is drawn on, past the headings.
func (g taskGroups) lineOf(index int) int {
	if g.headed() && index >= len(g.forIssues) {
		return g.lead + index + 1
	}

	return g.lead + index
}

// indexAt is the listed task drawn on a line of the list, and whether a task is
// drawn there rather than a heading or nothing.
func (g taskGroups) indexAt(line int) (int, bool) {
	line -= g.lead
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

// lists reports whether the pane lists the task with a uuid.
func (g taskGroups) lists(uuid string) bool {
	return slices.ContainsFunc(g.listed(), func(task taskwarrior.Task) bool { return task.UUID == uuid })
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

// trackingTask is a task that tracks an issue — linked to it and still to do, as
// the issue's mark counts it — one the Tasks pane lists where there is one, and
// whether any task tracks it.
func (m Model) trackingTask(issueKey jira.Key) (taskwarrior.Task, bool) {
	tracking := slices.DeleteFunc(m.linkedTo(issueKey), func(task taskwarrior.Task) bool { return !stillToDo(task) })
	if len(tracking) == 0 {
		return taskwarrior.Task{}, false
	}

	groups := m.taskGroups()
	listed := slices.IndexFunc(tracking, func(task taskwarrior.Task) bool { return groups.lists(task.UUID) })

	return tracking[max(0, listed)], true
}

// goToTrackingTask focuses the Tasks pane on the task that tracks an issue, or,
// where the pane does not list it, says which task it is and why.
func (m Model) goToTrackingTask(issueKey jira.Key, task taskwarrior.Task) Model {
	groups := m.taskGroups()
	if !groups.lists(task.UUID) {
		return m.noticed(string(issueKey) + " is tracked by task " + taskName(task) + ", " + m.unlistedBecause(task))
	}

	m = m.focusOn(paneTasks)
	m.tasks.selected = task.UUID
	m.tasks = m.tasks.following(groups, m.detailRows())

	return m
}

// unlistedBecause is why the Tasks pane does not list a task still to do: it
// waits, until a day in the clock's zone; it is the template a recurring task's
// instances are made from; a track has just added it, and no read since has
// held it; the active context hides it; or, with no context to hide it, it
// changed between the pending read and the linked one, which reading them
// again settles.
func (m Model) unlistedBecause(task taskwarrior.Task) string {
	now := m.deps.now()

	switch {
	case task.Waiting(now):
		return "which waits until " + task.Wait.In(now.Location()).Format(time.DateOnly)
	case task.Status == taskwarrior.Recurring:
		return "a recurring template"
	case m.tasks.stubbed(task.UUID):
		return "just added; " + m.keys.refresh.Help().Key + " in the Tasks pane reads it"
	case m.tasks.context == "":
		return "not among the tasks just read; " + m.keys.refresh.Help().Key + " in the Tasks pane reads them again"
	default:
		return "outside context " + m.tasks.context
	}
}

// relistTasks lists the tasks again as the listing now says, keeping the
// cursor on its task while that task is listed, and the task in view.
func (m Model) relistTasks() Model {
	groups := m.taskGroups()
	m.tasks.selected = groups.at(m.tasks.selected).UUID
	m.tasks = m.tasks.following(groups, m.detailRows())

	return m
}

// sortTasks moves the Tasks list on to its next order.
func (m Model) sortTasks() Model {
	m.tasks.listing.order = m.tasks.listing.order.next()

	return m.relistTasks()
}

// handleTaskListKey answers the keys that change how the list is listed, once
// Taskwarrior has answered: none of them writes, so a write on its way does
// not hold them back. It reports whether it claimed the key.
func (m Model) handleTaskListKey(msg tea.KeyPressMsg) (Model, bool) {
	if !m.tasks.answered() {
		return m, false
	}

	switch {
	case key.Matches(msg, m.keys.sortTasks):
		return m.sortTasks(), true
	default:
		return m, false
	}
}
