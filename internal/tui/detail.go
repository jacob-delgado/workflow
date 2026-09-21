// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"cmp"
	"fmt"
	"strconv"
	"strings"
	"time"

	"charm.land/bubbles/v2/key"
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

// issueDetail is the selected issue in full, as far as it has loaded.
type issueDetail struct {
	key    jira.Key
	loaded bool
	err    error
	detail jira.IssueDetail
}

// detailLoaded carries an issue read in full.
type detailLoaded struct {
	key    jira.Key
	detail jira.IssueDetail
	err    error
}

// apply records the issue, unless the selection has moved on from it.
//
// Two reads of the SAME key in flight at once — a load and a reload after a
// comment, say — are last-to-arrive-wins: the message carries no sequence
// number, so a stale read landing after a fresh one would overwrite it. Every
// request is bounded by the request timeout, which keeps that window to a few
// seconds; a sequence guard is not worth threading through every load for it.
func (msg detailLoaded) apply(m Model) (Model, tea.Cmd) {
	if msg.key != m.detail.key {
		return m, nil
	}

	m.detail = issueDetail{key: msg.key, loaded: true, err: msg.err, detail: msg.detail}

	return m, nil
}

// detailDue is the selection having rested on an issue long enough to read it.
type detailDue struct {
	key jira.Key
}

// apply reads the issue, if the selection is still on it.
func (msg detailDue) apply(m Model) (Model, tea.Cmd) {
	selected, ok := m.issues.current()
	if !ok || selected.Key != msg.key {
		return m, nil
	}

	return m.loadDetail()
}

// searchIssues is the command that fills, or refreshes, the Issues pane with its
// first page.
func (m Model) searchIssues() tea.Cmd {
	return m.searchPage(0)
}

// searchPage is the command that reads one page of issues, from startAt.
func (m Model) searchPage(startAt int) tea.Cmd {
	search := m.deps.Jira.Search
	if search == nil {
		return nil
	}

	jql := m.activeView().jql

	return func() tea.Msg {
		found, err := search(jql, startAt)

		return issuesLoaded{found: found, err: err, startAt: startAt}
	}
}

// loadMoreIssues reads the next page when the list is truncated and one is not
// already on its way.
func (m Model) loadMoreIssues() (Model, tea.Cmd) {
	if m.issues.loading || !m.issues.hasMore() || m.deps.Jira.Search == nil {
		return m, nil
	}

	m.issues.loading = true

	return m, m.searchPage(len(m.issues.found.Issues))
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
func (m Model) reloadDetail(issueKey jira.Key) tea.Cmd {
	if m.deps.Jira.Issue == nil || m.detail.key != issueKey {
		return nil
	}

	return m.fetchDetail(issueKey)
}

// fetchDetail is the command that reads an issue in full.
func (m Model) fetchDetail(issueKey jira.Key) tea.Cmd {
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
	branchKey, named := m.branchIssue()
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
	return m.issues.render(m.marks, m.styles, rows)
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

// issuesKeys offers the verbs for the selected issue, when there is one, and the
// view switch when there is more than one view to move between.
func (m Model) issuesKeys() []key.Binding {
	selected, ok := m.issues.current()
	if !ok {
		return append(m.viewKeys(), m.keys.refresh)
	}

	keys := make([]key.Binding, 0, len(m.views)+7) //nolint:mnd // the verbs, links, more and refresh, beside the views.
	branchFor := relabel(m.keys.branchForIssue, "branch for "+string(selected.Key))
	keys = append(keys, m.keys.changeStatus, m.keys.comment, branchFor)
	keys = append(keys, m.viewKeys()...)
	keys = append(keys, m.linkKeys(m.issueURL())...)

	if m.issues.hasMore() {
		keys = append(keys, m.keys.loadMore)
	}

	return append(keys, m.keys.refresh)
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

// viewKeys offers the view switch when there is more than one view.
func (m Model) viewKeys() []key.Binding {
	if len(m.views) <= 1 {
		return nil
	}

	return []key.Binding{m.keys.nextView}
}

// moveIssue moves the selection, reads the newly selected issue once the
// selection rests, and pulls the next page when it reaches the end of a
// truncated list.
func (m Model) moveIssue(step int) (Model, tea.Cmd) {
	m.issues = m.issues.move(step)
	m.issues.moved = true

	m, page := m.pageIfAtEnd()
	m, detail := m.soonDetail()

	return m, tea.Batch(page, detail)
}

// pageIfAtEnd reads the next page once the selection reaches the last loaded
// issue of a truncated list.
func (m Model) pageIfAtEnd() (Model, tea.Cmd) {
	if m.issues.selected < len(m.issues.found.Issues)-1 {
		return m, nil
	}

	return m.loadMoreIssues()
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
	if m.issues.err != nil {
		return m.failureWithin(m.issues.err, width) + "\n\n" + m.status()
	}

	selected, ok := m.issues.current()
	if !ok {
		return m.status()
	}

	lines := []string{m.styles.strong.Render(string(selected.Key)) + " " + selected.Summary, m.facts(selected)}

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
func (m Model) fullDetail(issueKey jira.Key, width int) []string {
	switch {
	case m.detail.key != issueKey || !m.detail.loaded:
		return []string{"", m.styles.label.Render("loading the description and comments" + m.marks.ellipsis)}
	case m.detail.err != nil:
		return []string{"", m.failureWithin(m.detail.err, width), "press r to try again"}
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

	shown := detail.Comments[max(0, len(detail.Comments)-cmp.Or(m.cfg.UI.CommentsShown, defaultCommentsShown)):]
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
