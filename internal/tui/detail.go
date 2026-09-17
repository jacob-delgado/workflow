// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/jacob-delgado/workflow/internal/convention"
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

// commentsShown is how many of an issue's most recent comments are drawn.
const commentsShown = 5

// issueDetail is the selected issue in full, as far as it has loaded.
type issueDetail struct {
	key    string
	loaded bool
	err    error
	detail jira.IssueDetail
}

// detailLoaded carries an issue read in full.
type detailLoaded struct {
	key    string
	detail jira.IssueDetail
	err    error
}

// apply records the issue, unless the selection has moved on from it.
func (msg detailLoaded) apply(m Model) (Model, tea.Cmd) {
	if msg.key != m.detail.key {
		return m, nil
	}

	m.detail = issueDetail{key: msg.key, loaded: true, err: msg.err, detail: msg.detail}

	return m, nil
}

// detailDue is the selection having rested on an issue long enough to read it.
type detailDue struct {
	key string
}

// apply reads the issue, if the selection is still on it.
func (msg detailDue) apply(m Model) (Model, tea.Cmd) {
	selected, ok := m.issues.current()
	if !ok || selected.Key != msg.key {
		return m, nil
	}

	return m.loadDetail()
}

// searchIssues is the command that fills, or refreshes, the Issues pane.
func (m Model) searchIssues() tea.Cmd {
	search := m.deps.Jira.Search
	if search == nil {
		return nil
	}

	return func() tea.Msg {
		found, err := search()

		return issuesLoaded{found: found, err: err}
	}
}

// loadDetail reads the selected issue in full, unless it already is.
func (m Model) loadDetail() (Model, tea.Cmd) {
	selected, ok := m.issues.current()
	if !ok || m.deps.Jira.Issue == nil || m.detail.key == selected.Key {
		return m, nil
	}

	m.detail = issueDetail{key: selected.Key, loaded: false, err: nil, detail: jira.IssueDetail{}}

	return m, m.fetchDetail(selected.Key)
}

// reloadDetail reads an issue in full again, when it is the one shown — after a
// comment or a change of status. What is shown stays until the answer arrives.
func (m Model) reloadDetail(issueKey string) tea.Cmd {
	if m.deps.Jira.Issue == nil || m.detail.key != issueKey {
		return nil
	}

	return m.fetchDetail(issueKey)
}

// fetchDetail is the command that reads an issue in full.
func (m Model) fetchDetail(issueKey string) tea.Cmd {
	read := m.deps.Jira.Issue

	return func() tea.Msg {
		detail, err := read(issueKey)

		return detailLoaded{key: issueKey, detail: detail, err: err}
	}
}

// resumeIssue selects the issue the checked-out branch is for, when the list
// has it and nobody has chosen an issue yet: coming back to work in progress
// starts where it was left.
func (m Model) resumeIssue() Model {
	branchKey, named := convention.IssueKey(m.branch.branch.Name)
	if m.issues.moved || !named {
		return m
	}

	if _, listed := m.issues.find(branchKey); listed {
		m.issues = m.issues.selectKey(branchKey)
	}

	return m
}

// issuesRail is the Issues pane's list.
func (m Model) issuesRail(rows int) string {
	return m.issues.render(m.marks, rows)
}

// issuesKeys offers the verbs for the selected issue, when there is one.
func (m Model) issuesKeys() []key.Binding {
	selected, ok := m.issues.current()
	if !ok {
		return []key.Binding{m.keys.refresh}
	}

	return []key.Binding{m.keys.changeStatus, m.keys.comment, relabel(m.keys.branchForIssue, "branch for "+selected.Key)}
}

// handleIssuesKey answers the Issues pane's own keys.
func (m Model) handleIssuesKey(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.down):
		return m.moveIssue(1)
	case key.Matches(msg, m.keys.up):
		return m.moveIssue(-1)
	case key.Matches(msg, m.keys.changeStatus):
		return m.openStatusPicker()
	case key.Matches(msg, m.keys.comment):
		return m.startComment()
	case key.Matches(msg, m.keys.branchForIssue):
		return m.openBranchCreator()
	case key.Matches(msg, m.keys.refresh):
		return m.refreshIssues()
	default:
		return m, nil
	}
}

// refreshIssues reads the list again, and the selected issue in full whether or
// not it changed, so r retries a detail load that failed.
func (m Model) refreshIssues() (Model, tea.Cmd) {
	selected, ok := m.issues.current()
	if !ok || m.deps.Jira.Issue == nil {
		return m, m.searchIssues()
	}

	return m, tea.Batch(m.searchIssues(), m.fetchDetail(selected.Key))
}

// moveIssue moves the selection, and reads the newly selected issue once the
// selection rests.
func (m Model) moveIssue(step int) (Model, tea.Cmd) {
	m.issues = m.issues.move(step)
	m.issues.moved = true

	return m.soonDetail()
}

// soonDetail reads the selected issue after detailDelay, unless it is already
// shown.
func (m Model) soonDetail() (Model, tea.Cmd) {
	selected, ok := m.issues.current()
	if !ok || m.detail.key == selected.Key {
		return m, nil
	}

	m.scroll = 0

	return m, tea.Tick(detailDelay, func(time.Time) tea.Msg { return detailDue{key: selected.Key} })
}

// pickIssue selects the issue on a clicked line of the list, wherever the list
// is drawn: in the rail, or as the whole detail on a narrow terminal.
func (m Model) pickIssue(line, rows int, inRail bool) (Model, tea.Cmd) {
	if !inRail && !m.shape().Collapsed() {
		return m, nil
	}

	index, ok := m.issues.rowAt(line, rows)
	if !ok {
		return m, nil
	}

	m.issues.selected, m.issues.moved = index, true

	return m.soonDetail()
}

// issueDetailView describes the selected issue in full, or explains why there
// is nothing to describe — keeping the configuration summary on screen through a
// failure.
func (m Model) issueDetailView(width int) string {
	if text, failed := m.issues.failure(m.status()); failed {
		return wrap(text, width)
	}

	selected, ok := m.issues.current()
	if !ok {
		return m.status()
	}

	lines := []string{m.styles.strong.Render(selected.Key) + " " + selected.Summary, m.facts(selected)}

	if capped := m.issues.capped(); capped != "" {
		lines = append(lines, m.styles.label.Render(capped))
	}

	return wrap(strings.Join(append(lines, m.fullDetail(selected.Key, width)...), "\n"), width)
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
func (m Model) fullDetail(issueKey string, width int) []string {
	switch {
	case m.detail.key != issueKey || !m.detail.loaded:
		return []string{"", m.styles.label.Render("loading the description and comments" + m.marks.ellipsis)}
	case m.detail.err != nil:
		return []string{"", m.failure(m.detail.err), "press r to try again"}
	}

	detail := m.detail.detail
	lines := []string{m.styles.label.Render("reported by " + detail.Reporter), ""}

	if strings.TrimSpace(detail.Description) == "" {
		lines = append(lines, m.styles.label.Render("no description"))
	} else {
		lines = append(lines, wrap(detail.Description, width))
	}

	return append(lines, m.comments(detail)...)
}

// comments draws the most recent comments, oldest of them first.
func (m Model) comments(detail jira.IssueDetail) []string {
	if detail.CommentTotal == 0 {
		return nil
	}

	shown := detail.Comments[max(0, len(detail.Comments)-commentsShown):]
	heading := fmt.Sprintf("Comments %s of %s", strconv.Itoa(len(shown)), strconv.Itoa(detail.CommentTotal))
	lines := []string{"", m.styles.strong.Render(heading)}

	for _, comment := range shown {
		lines = append(lines, "",
			m.styles.label.Render(comment.Author+m.marks.separator+age(m.deps.now(), comment.Created)),
			comment.Body)
	}

	return lines
}

// age says how long ago something happened, as briefly as is still clear.
func age(now, then time.Time) string {
	elapsed := now.Sub(then)

	switch {
	case then.IsZero():
		return "some time ago"
	case elapsed < time.Minute:
		return "just now"
	case elapsed < time.Hour:
		return strconv.Itoa(int(elapsed.Minutes())) + "m ago"
	case elapsed < day:
		return strconv.Itoa(int(elapsed.Hours())) + "h ago"
	case elapsed < month:
		return strconv.Itoa(int(elapsed/day)) + "d ago"
	default:
		return then.Format(time.DateOnly)
	}
}
