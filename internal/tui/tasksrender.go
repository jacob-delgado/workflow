// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/x/ansi"

	"github.com/jacob-delgado/workflow/internal/jira"
	"github.com/jacob-delgado/workflow/internal/sanitize"
	"github.com/jacob-delgado/workflow/internal/taskwarrior"
)

// noteGap sets a task's faint note apart from what it is.
const noteGap = "  "

// taskIndent sets an issue's tasks in under their heading.
const taskIndent = "  "

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
