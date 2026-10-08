// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"slices"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/jacob-delgado/workflow/internal/jira"
)

// detailDelay is how long the selection must rest on an issue before it is read
// in full, so holding j down does not send Jira a request per row.
const detailDelay = 150 * time.Millisecond

// day and month are the spans past which an age is given in days, then as a
// date.
const (
	day   = 24 * time.Hour
	month = 30 * day
)

// defaultCommentsShown is how many of an issue's most recent comments are drawn
// when ui.comments_shown is not set.
const defaultCommentsShown = 5

// issueDetail is the selected issue in full, as far as it has loaded, and how
// far the Issues pane's detail is scrolled — which another issue starts at the
// top.
type issueDetail struct {
	key    jira.Key
	loaded bool
	err    error
	detail jira.IssueDetail
	scroll int
}

// detailLoaded carries an issue read in full, and which read it answers.
type detailLoaded struct {
	key    jira.Key
	read   int
	detail jira.IssueDetail
	err    error
}

var _ applier = detailLoaded{}

// apply records the issue, unless the selection has moved on from it or a later
// read has started — a load and then a reload after a comment, say — whose
// answer is the one to show, whichever of the two arrives last.
func (msg detailLoaded) apply(m Model) (Model, tea.Cmd) {
	if msg.key != m.detail.key || msg.read != m.detailReads {
		return m, nil
	}

	m.detail.loaded, m.detail.err, m.detail.detail = true, msg.err, msg.detail

	return m, nil
}

// detailDue is the selection having rested on an issue long enough to read it.
type detailDue struct {
	key jira.Key
}

var _ applier = detailDue{}

// apply reads the issue, if the selection is still on it.
func (msg detailDue) apply(m Model) (Model, tea.Cmd) {
	selected, ok := m.issues.current()
	if !ok || selected.Key != msg.key {
		return m, nil
	}

	return m.loadDetail()
}

// loadDetail reads the selected issue in full, unless it already is.
func (m Model) loadDetail() (Model, tea.Cmd) {
	selected, ok := m.issues.current()
	if !ok || m.deps.Jira.Issue == nil || m.detail.key == selected.Key {
		return m, nil
	}

	m.detail = issueDetail{key: selected.Key, loaded: false, err: nil, detail: jira.IssueDetail{}}

	return m.readDetail()
}

// reloadDetail reads an issue in full again, when it is the one shown — after a
// comment or a change of status. What is shown stays until the answer arrives.
func (m Model) reloadDetail(issueKey jira.Key) (Model, tea.Cmd) {
	if m.deps.Jira.Issue == nil || m.detail.key != issueKey {
		return m, nil
	}

	return m.readDetail()
}

// readDetail starts a read of the issue shown, numbered so that only the latest
// read's answer is shown. Every read goes through here.
func (m Model) readDetail() (Model, tea.Cmd) {
	m.detailReads++
	read, issueKey, number := m.deps.Jira.Issue, m.detail.key, m.detailReads

	return m, func() tea.Msg {
		detail, err := read(issueKey)

		return detailLoaded{key: issueKey, read: number, detail: detail, err: err}
	}
}

// resumeIssue selects the issue the checked-out branch is for, when the list
// has it and nobody has chosen an issue yet: coming back to work in progress
// starts where it was left.
func (m Model) resumeIssue() Model {
	branchKey, named := m.branchIssue()
	if m.issues.moved || !named {
		return m
	}

	if _, listed := m.issues.find(branchKey); listed {
		m.issues = m.issues.selectKey(branchKey)
	}

	return m
}

// issuesRail is the Issues pane's list, each issue marked with its task state
// when there is a Taskwarrior to ask.
func (m Model) issuesRail(rows int) string {
	return m.issues.render(m.marks, m.styles, rows, m.taskMarks())
}

// issuesNarrow is the collapsed Issues view: the full issue — or the reason
// there is none, and the setup steps — when there is nothing to scan or enter
// asked to read one, otherwise the list.
func (m Model) issuesNarrow(rows int) string {
	_, ok := m.issues.current()
	if !ok || m.issues.viewing {
		return m.issueDetailView(m.detailWidth())
	}

	return m.issuesRail(rows)
}

// issueURL is the selected issue's browse URL, or empty when there is no issue
// selected or no way to build one.
func (m Model) issueURL() string {
	selected, ok := m.issues.current()
	if !ok || m.deps.Jira.BrowseURL == nil {
		return ""
	}

	return m.deps.Jira.BrowseURL(selected.Key)
}

// moveIssue moves the selection, reads the newly selected issue once the
// selection rests, and pulls the next page when it reaches the end of a
// truncated list.
func (m Model) moveIssue(step int) (Model, tea.Cmd) {
	m.issues = m.issues.move(step)
	m.issues.moved = true

	m, page := m.pageIfAtEnd()

	return m, tea.Batch(page, m.soonDetail())
}

// soonDetail reads the selected issue after detailDelay, unless it is already
// shown.
func (m Model) soonDetail() tea.Cmd {
	selected, ok := m.issues.current()
	if !ok || m.detail.key == selected.Key {
		return nil
	}

	return m.deps.after(detailDelay, func(time.Time) tea.Msg { return detailDue{key: selected.Key} })
}

// pickIssue selects the issue on a clicked line of the list wherever it is
// drawn: in the rail, or in the collapsed detail when no issue is read there.
func (m Model) pickIssue(line, rows int, inRail bool) (Model, tea.Cmd) {
	if !inRail && (!m.shape().Collapsed() || m.issues.viewing) {
		return m, nil
	}

	index, ok := m.issues.rowAt(line, rows)
	if !ok {
		return m, nil
	}

	m.issues.selected, m.issues.moved = index, true

	return m, m.soonDetail()
}

// issueDetailView describes the selected issue in full, or explains why there
// is nothing to describe — keeping the configuration summary on screen through a
// failure.
func (m Model) issueDetailView(width int) string {
	if m.issues.err != nil {
		return m.failureBlock(m.issues.err, width) + "\n\n" + m.status(width)
	}

	selected, ok := m.issues.current()
	if !ok {
		return m.status(width)
	}

	lines := []string{m.styles.strong.Render(shownKey(selected.Key)) + " " + selected.Summary, m.facts(selected)}

	if capped := m.issues.capped(); capped != "" {
		lines = append(lines, m.styles.label.Render(capped))
	}

	// A blank row sets the Tasks block apart from who reported the issue, unless
	// the full read, still loading or failed, opens with one of its own.
	tasks, full := m.issueTasksBlock(selected.Key, width), m.fullDetail(selected.Key, width)
	if len(tasks) > 0 && full[0] != "" {
		tasks = append(tasks, "")
	}

	return wrap(strings.Join(slices.Concat(lines, tasks, full), "\n"), width)
}

// facts is an issue's type, priority and status on one line, leaving out any
// the instance does not use.
func (m Model) facts(issue jira.Issue) string {
	var parts []string

	for _, part := range []string{issue.Type, issue.Priority, issue.Status} {
		if part != "" {
			parts = append(parts, part)
		}
	}

	return m.styles.label.Render(strings.Join(parts, m.marks.separator))
}

// fullDetail is the part of an issue only a full read has: the reporter, the
// description, and the most recent comments.
func (m Model) fullDetail(issueKey jira.Key, width int) []string {
	switch {
	case m.detail.key != issueKey || !m.detail.loaded:
		return []string{"", m.styles.label.Render("reading the description and comments" + m.marks.ellipsis)}
	case m.detail.err != nil:
		return []string{"", m.failureBlock(m.detail.err, width), "press " + m.keys.refresh.Help().Key + " to try again"}
	}

	detail := m.detail.detail
	lines := []string{m.styles.label.Render("reported by " + detail.Reporter)}
	lines = append(lines, m.issuePeopleAndTags(detail)...)
	lines = append(lines, "")

	if strings.TrimSpace(detail.Description) == "" {
		lines = append(lines, m.styles.label.Render("no description"))
	} else {
		lines = append(lines, wrap(detail.Description, width))
	}

	lines = append(lines, m.issueRelations(detail)...)

	return append(lines, m.comments(detail)...)
}

// issuePeopleAndTags is the assignee and the issue's tags — labels, components,
// fix versions, and its parent — each line drawn only when the issue has it.
func (m Model) issuePeopleAndTags(detail jira.IssueDetail) []string {
	var lines []string

	if detail.Assignee != "" {
		lines = append(lines, m.styles.label.Render("assigned to "+detail.Assignee))
	}

	lines = m.tagLine(lines, "labels", detail.Labels)
	lines = m.tagLine(lines, "components", detail.Components)
	lines = m.tagLine(lines, "fix versions", detail.FixVersions)

	if detail.Parent.Key != "" {
		lines = append(lines, m.styles.label.Render("parent "+detail.Parent.Key+" "+detail.Parent.Summary))
	}

	return lines
}

// tagLine adds a labeled, comma-joined line for a list of tags, or nothing when
// the list is empty.
func (m Model) tagLine(lines []string, name string, values []string) []string {
	if len(values) == 0 {
		return lines
	}

	return append(lines, m.styles.label.Render(name+" "+strings.Join(values, ", ")))
}

// issueRelations is the issue's subtasks and its links to other issues, each
// block drawn only when there is one.
func (m Model) issueRelations(detail jira.IssueDetail) []string {
	var lines []string

	if len(detail.Subtasks) > 0 {
		lines = append(lines, "", m.styles.strong.Render("Subtasks"))
		for _, sub := range detail.Subtasks {
			lines = append(lines, m.styles.label.Render("  "+sub.Key+" "+sub.Summary+m.marks.separator+sub.Status))
		}
	}

	if len(detail.IssueLinks) > 0 {
		lines = append(lines, "", m.styles.strong.Render("Links"))
		for _, link := range detail.IssueLinks {
			lines = append(lines, m.styles.label.Render("  "+link.Relation+" "+link.Issue.Key+" "+
				link.Issue.Summary+m.marks.separator+link.Issue.Status))
		}
	}

	return lines
}
