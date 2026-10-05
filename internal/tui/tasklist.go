// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"slices"
	"time"

	"github.com/jacob-delgado/workflow/internal/jira"
	"github.com/jacob-delgado/workflow/internal/taskwarrior"
)

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
