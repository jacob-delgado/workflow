// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"slices"
	"strings"
	"time"
	"unicode/utf8"

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
	return around(o, taskOrderCount).next()
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
// session: its order, what narrows it, and whether a filter is being typed.
// Changing it reads nothing again.
type taskListing struct {
	order     taskOrder
	narrowing taskwarrior.Narrowing
	filtering bool
}

// titled reports a list that says how it is listed, above its rows: any order
// but the most urgent first the pane opens on, or a narrowing.
func (l taskListing) titled() bool {
	return l.order != taskOrderUrgency || l.narrowing.Narrows() || l.filtering
}

// heading says how the list is listed: its order, the values it is narrowed
// to, and the filter, typed or applied.
func (l taskListing) heading(marks glyphs) string {
	parts := []string{l.order.title()}

	if len(l.narrowing.Picked) > 0 {
		labels := make([]string, 0, len(l.narrowing.Picked))
		for _, picked := range l.narrowing.Picked {
			labels = append(labels, picked.Label())
		}

		parts = append(parts, "filtered to "+strings.Join(labels, ", "))
	}

	if l.filtering || l.narrowing.Text != "" {
		parts = append(parts, "search: "+l.narrowing.Text)
	}

	return strings.Join(parts, marks.separator)
}

// narrows reports a listing that leaves tasks out: values picked, or a filter
// typed or being typed. An order alone leaves none out.
func (l taskListing) narrows() bool {
	return l.narrowing.Narrows() || l.filtering
}

// beginFilter starts typing a filter afresh.
func (l taskListing) beginFilter() taskListing {
	l.filtering, l.narrowing.Text = true, ""

	return l
}

// extendFilter adds text to the filter being typed.
func (l taskListing) extendFilter(text string) taskListing {
	l.narrowing.Text += text

	return l
}

// trimFilter takes the last character off the filter being typed.
func (l taskListing) trimFilter() taskListing {
	_, size := utf8.DecodeLastRuneInString(l.narrowing.Text)
	l.narrowing.Text = l.narrowing.Text[:len(l.narrowing.Text)-size]

	return l
}

// clearFilter stops typing and drops the filter; picked values stay.
func (l taskListing) clearFilter() taskListing {
	l.filtering, l.narrowing.Text = false, ""

	return l
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
		case !listing.narrowing.Matches(task, now):
			continue
		case task.Waiting(now) && !listing.narrowing.ListsWaiting():
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

	// The list as the pane would show it with nothing narrowing it, so what the
	// user has narrowed the view to never changes which task a write targets.
	groups := m.taskGroupsBy(taskListing{})
	listed := slices.IndexFunc(tracking, func(task taskwarrior.Task) bool { return groups.lists(task.UUID) })

	return tracking[max(0, listed)], true
}

// goToTrackingTask focuses the Tasks pane on the task that tracks an issue, or,
// where the pane does not list it, says which task it is and why.
func (m Model) goToTrackingTask(issueKey jira.Key, task taskwarrior.Task) Model {
	groups := m.taskGroups()
	if shown, ok := m.listedTrackingTask(issueKey, groups); ok {
		task = shown
	}

	if !groups.lists(task.UUID) {
		return m.noticed(shownKey(issueKey) + " is tracked by task " + taskName(task) + ", " + m.unlistedBecause(task))
	}

	m = m.focusOn(paneTasks)
	m.tasks.selected = task.UUID
	m.tasks = m.tasks.following(groups, m.detailRows())

	return m
}

// listedTrackingTask is a task that tracks an issue and that the pane lists as
// it is narrowed now, so going to the issue's task lands on one in view
// whenever there is one.
func (m Model) listedTrackingTask(issueKey jira.Key, groups taskGroups) (taskwarrior.Task, bool) {
	for _, task := range m.linkedTo(issueKey) {
		if stillToDo(task) && groups.lists(task.UUID) {
			return task, true
		}
	}

	return taskwarrior.Task{}, false
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
	case m.taskGroupsBy(taskListing{}).lists(task.UUID):
		return "which the Tasks pane's filter hides; " + m.keys.filterTasks.Help().Key + " there changes it"
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

	// A narrowing that lists nothing leaves the cursor's task as it was, so
	// the cursor is back on it once the narrowing lets it through again.
	if len(groups.listed()) > 0 {
		m.tasks.selected = groups.at(m.tasks.selected).UUID
	}

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
	case key.Matches(msg, m.keys.searchTasks) && len(m.tasks.pending) > 0:
		m.tasks.listing = m.tasks.listing.beginFilter()

		return m.relistTasks(), true
	case key.Matches(msg, m.keys.filterTasks) && len(m.tasks.pending) > 0:
		return m.openTaskNarrowing(), true
	default:
		return m, false
	}
}

// taskListKeys are the footer's keys that change how the list is listed.
func (m Model) taskListKeys() []key.Binding {
	if len(m.tasks.pending) == 0 {
		return []key.Binding{m.keys.sortTasks}
	}

	return []key.Binding{m.keys.searchTasks, m.keys.filterTasks, m.keys.sortTasks}
}

// filteringTasks reports the Tasks pane capturing keystrokes into its filter,
// which takes every key, q and the digits included, until it closes.
func (m Model) filteringTasks() bool {
	return m.focus == paneTasks && m.tasks.listing.filtering
}

// handleTaskFilterKey types the Tasks filter as the Issues filter is typed:
// enter keeps it, esc clears it, the arrows move the cursor, and every other
// key extends it. It reads enter and esc themselves, so a printable key ui.keys
// moved onto either still types.
func (m Model) handleTaskFilterKey(msg tea.KeyPressMsg) (Model, tea.Cmd) {
	switch msg.Code {
	case tea.KeyEscape:
		m.tasks.listing = m.tasks.listing.clearFilter()
	case tea.KeyEnter:
		m.tasks.listing.filtering = false
	case tea.KeyDown:
		return m.moveTaskBy(1), nil
	case tea.KeyUp:
		return m.moveTaskBy(-1), nil
	case tea.KeyBackspace:
		m.tasks.listing = m.tasks.listing.trimFilter()
	default:
		m.tasks.listing = m.tasks.listing.extendFilter(typedText(msg))
	}

	return m.relistTasks(), nil
}

// withoutTaskFilter drops a typed filter when focus leaves the pane, as the
// Issues filter is dropped; picked values stay for the session.
func (m Model) withoutTaskFilter() Model {
	if !m.tasks.listing.filtering && m.tasks.listing.narrowing.Text == "" {
		return m
	}

	m.tasks.listing = m.tasks.listing.clearFilter()

	return m.relistTasks()
}

var (
	_ overlay   = checklist[taskwarrior.Facet]{}
	_ clickable = checklist[taskwarrior.Facet]{}
	_ steppable = checklist[taskwarrior.Facet]{}
)

// openTaskNarrowing opens the checklist of values the pending tasks hold, with
// those already picked checked.
func (m Model) openTaskNarrowing() Model {
	picked := m.tasks.listing.narrowing.Picked
	choices := taskwarrior.Choices(m.tasks.pending, picked, m.deps.now())
	offers := make([]offered[taskwarrior.Facet], 0, len(choices))

	for _, choice := range choices {
		offers = append(offers, offered[taskwarrior.Facet]{value: choice.Facet, count: choice.Count})
	}

	m.overlay = checklist[taskwarrior.Facet]{
		title: filterTitle, none: "no task to filter",
		choices: pickList[offered[taskwarrior.Facet]]{items: offers}, chosen: slices.Clone(picked),
		label: taskwarrior.Facet.Label,
		apply: func(m Model, chosen []taskwarrior.Facet) (Model, tea.Cmd) {
			m.tasks.listing.narrowing.Picked = chosen

			return m.relistTasks(), nil
		},
	}

	return m
}
