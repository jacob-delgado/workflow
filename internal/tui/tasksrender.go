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

// tasksView is what the Tasks pane draws with beside its own state: the
// glyphs and styles, the clock, whether there is a Taskwarrior to ask and,
// when not, whether it was turned off, the issues the Issues pane lists, which
// group the tasks, and the checked-out branch's issue and work, drawn beside
// a task for that issue.
type tasksView struct {
	kit       renderKit
	now       time.Time
	installed bool
	disabled  bool
	issues    issueList
	// branchIssue is the issue the checked-out branch names, empty where it
	// names none, and branchWork the branch and its pull request.
	branchIssue jira.Key
	branchWork  []string
}

// tasksView is the Tasks pane's view of the rest of the interface.
func (m Model) tasksView() tasksView {
	view := tasksView{
		kit: m.kit(), now: m.deps.now(), installed: m.deps.Tasks.Install != nil,
		disabled: m.cfg.Taskwarrior.Disabled, issues: m.issues,
	}

	if branchIssue, named := m.branchIssue(); named {
		view.branchIssue, view.branchWork = branchIssue, m.branchAndPull()
	}

	return view
}

// groups is the pending tasks as the pane lists them, as listed now.
func (s tasksState) groups(view tasksView) taskGroups {
	return s.groupsBy(s.listing, view.issues, view.now)
}

// rail summarizes the pane: how many tasks are pending and active, and the
// context that narrows them, or why there are none to count, or that a write
// is on its way.
func (s tasksState) rail(view tasksView) string {
	switch {
	case !view.installed:
		return view.unavailable()
	case !s.loaded:
		return view.kit.marks.reading()
	case s.err != nil:
		return view.kit.failureSummary(s.err)
	case s.writing:
		return view.kit.marks.inFlight + " sending" + view.kit.marks.ellipsis
	}

	// The rail counts the pane's tasks whatever narrows the list, as the Reviews
	// rail counts the whole queue.
	listed := s.groupsBy(taskListing{}, view.issues, view.now).listed()
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

	if s.context != "" {
		counts = append(counts, "ctx "+s.context)
	}

	return strings.Join(counts, view.kit.marks.separator)
}

// unavailable is why there is no Taskwarrior to ask, briefly: the
// integration is turned off, or no task program was found — which is not set
// up, and told as guidance.
func (v tasksView) unavailable() string {
	if v.disabled {
		return "turned off"
	}

	return v.kit.failureSummary(taskwarrior.ErrNotInstalled)
}

// unavailableDetail is unavailable in full, for the detail.
func (v tasksView) unavailableDetail(width int) string {
	if v.disabled {
		return wrap("Turned off by taskwarrior.disabled.", width)
	}

	return wrap(v.kit.failureLine(taskwarrior.ErrNotInstalled), width)
}

// detail lists the pending tasks, then describes the selected one.
func (s tasksState) detail(view tasksView, width int) string {
	groups := s.groups(view)

	switch {
	case !view.installed:
		return view.unavailableDetail(width)
	case !s.loaded:
		return view.kit.marks.reading()
	case s.err != nil:
		return view.kit.failureBlock(s.err, width)
	case len(groups.listed()) == 0 && len(s.pending) > 0 && s.listing.narrows():
		return strings.Join(append(s.rows(view, groups, width), "No task matches the filters."), "\n")
	case len(groups.listed()) == 0:
		return strings.Join(append([]string{"No pending tasks."}, waitingRow(view.kit, groups)...), "\n")
	}

	return strings.Join(append(s.rows(view, groups, width), "", s.selectedDetail(view, groups, width)), "\n")
}

// rows draws the listed tasks one a row: how the list is listed, when that
// is not most urgent first, then those for listed issues, a faint heading over
// the others when there are both, then how many wait unlisted.
func (s tasksState) rows(view tasksView, groups taskGroups, width int) []string {
	kit := view.kit
	selected := groups.indexOf(s.selected)
	rows := make([]string, 0, groups.lines()+1)

	if groups.lead > 0 {
		heading := ansi.Truncate(sanitize.Line(s.listing.heading(kit.marks)), width, kit.marks.ellipsis)
		rows = append(rows, kit.styles.label.Render(heading), "")
	}

	for index, task := range groups.listed() {
		if groups.headed() && index == len(groups.forIssues) {
			rows = append(rows, kit.styles.label.Render("Other"))
		}

		rows = append(rows, s.row(kit, task, index == selected, view.now))
	}

	return append(rows, waitingRow(kit, groups)...)
}

// waitingRow is how many pending tasks wait unlisted, on a faint row, or no row
// when none does.
func waitingRow(kit renderKit, groups taskGroups) []string {
	if groups.waiting == 0 {
		return nil
	}

	return []string{kit.styles.label.Render(strconv.Itoa(groups.waiting) + " waiting")}
}

// row draws one task: the cursor, whether it is started, its id and what it
// is, then a faint tail of the issue it is for, when it is due and how urgent
// it is. Taskwarrior's client has already neutralized every value.
func (s tasksState) row(kit renderKit, task taskwarrior.Task, selected bool, now time.Time) string {
	head := kit.marks.marker(selected) + taskGlyph(kit.marks, task) + " " + fmt.Sprintf("%3s", taskNumber(task)) +
		" " + task.Description

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

	if sortedBy := s.sortKey(task, now); sortedBy != "" {
		tail = append(tail, sortedBy)
	}

	tail = append(tail, fmt.Sprintf("%.1f", task.Urgency))

	return head + noteGap + kit.styles.label.Render(strings.Join(tail, kit.marks.separator))
}

// sortKey is what a row adds to its tail so the order it is listed in can
// be read off it: its priority, or its tags, when the list is sorted by that,
// as the filter names them.
func (s tasksState) sortKey(task taskwarrior.Task, now time.Time) string {
	switch s.listing.order {
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

// selectedDetail describes the selected task: what it is, its facts, the
// issue it is for, and its annotations. A styled line is wrapped before it is
// styled, so each of its rows opens and closes the style rather than one row
// running it on into the border and the rail beside the next.
func (s tasksState) selectedDetail(view tasksView, groups taskGroups, width int) string {
	task := groups.at(s.selected)
	lines := []string{
		view.kit.styles.strong.Render(wrap(task.Description, width)),
		view.kit.styles.label.Render(wrap(taskFacts(view.kit.marks, task, view.now), width)),
	}

	if task.Linked() {
		lines = append(lines, view.issueLine(task))
	}

	return wrap(strings.Join(append(lines, taskAnnotations(view.kit, task, view.now.Location())...), "\n"), width)
}

// taskFacts is a task's facts on one line: how long ago it was started, its
// project, tags, due date, urgency and id, leaving out any it does not have. The
// due date is the day in the clock's zone, not in UTC, which Taskwarrior keeps:
// a task due at midnight east of UTC is due that day, not the day before.
func taskFacts(marks glyphs, task taskwarrior.Task, now time.Time) string {
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

	return strings.Join(facts, marks.separator)
}

// issueLine is the issue a task is for, with its summary and status where the
// Issues pane lists it, and — when the checked-out branch names it — that
// branch and its pull request.
func (v tasksView) issueLine(task taskwarrior.Task) string {
	issueKey := jira.Key(task.IssueKey)
	parts := []string{task.IssueKey}

	if issue, listed := v.issues.find(issueKey); listed {
		parts = []string{task.IssueKey + " " + issue.Summary, issue.Status}
	}

	if v.branchIssue != "" && v.branchIssue == issueKey {
		parts = append(parts, v.branchWork...)
	}

	return strings.Join(parts, v.kit.marks.separator)
}

// branchAndPull is the checked-out branch and, when the Review pane holds one,
// its pull request's number and how its CI stands.
func (m Model) branchAndPull() []string {
	parts := []string{m.branch.branch.Name}

	if m.review.found {
		parts = append(parts, m.vocab.sigil+strconv.Itoa(m.review.pull.Number)+" "+m.kit().ciGlyph(m.review.ci.State))
	}

	return parts
}

// taskAnnotations is a task's annotations under their heading, each after the
// day it was written in zone, or nothing for a task with none.
func taskAnnotations(kit renderKit, task taskwarrior.Task, zone *time.Location) []string {
	if len(task.Annotations) == 0 {
		return nil
	}

	lines := []string{"", kit.styles.strong.Render("Annotations")}

	for _, note := range task.Annotations {
		lines = append(lines, kit.styles.label.Render(note.Entry.In(zone).Format(time.DateOnly))+" "+note.Description)
	}

	return lines
}

// taskGlyph is how far a task has got, by shape: started, completed, or not
// started yet.
func taskGlyph(marks glyphs, task taskwarrior.Task) string {
	switch {
	case task.Active():
		return marks.inFlight
	case task.Status == taskwarrior.Completed:
		return marks.done
	default:
		return marks.notStarted
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
