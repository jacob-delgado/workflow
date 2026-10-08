// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"cmp"
	"slices"
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
	branchKey, named := m.branch.issue(m.cfg.Jira.Project)
	if m.issues.moved || !named {
		return m
	}

	if _, listed := m.issues.find(branchKey); listed {
		m.issues = m.issues.selectKey(branchKey)
	}

	return m
}

// browseURL is the selected issue's browse URL, or empty when there is no issue
// selected or no way to build one.
func (l issueList) browseURL(deps Deps) string {
	selected, ok := l.current()
	if !ok || deps.Jira.BrowseURL == nil {
		return ""
	}

	return deps.Jira.BrowseURL(selected.Key)
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
		return m.kit().failureBlock(m.issues.err, width) + "\n\n" + m.configStatus().block(width)
	}

	selected, ok := m.issues.current()
	if !ok {
		return m.configStatus().block(width)
	}

	lines := []string{
		m.styles.strong.Render(shownKey(selected.Key)) + " " + selected.Summary, issueFacts(m.kit(), selected),
	}

	if capped := m.issues.capped(); capped != "" {
		lines = append(lines, m.styles.label.Render(capped))
	}

	// A blank row sets the Tasks block apart from who reported the issue, unless
	// the full read, still loading or failed, opens with one of its own.
	tasks, full := m.issueTasksBlock(selected.Key, width), m.detail.lines(m.readView(), selected.Key, width)
	if len(tasks) > 0 && full[0] != "" {
		tasks = append(tasks, "")
	}

	return wrap(strings.Join(slices.Concat(lines, tasks, full), "\n"), width)
}

// issueFacts is an issue's type, priority and status on one line, leaving out any
// the instance does not use.
func issueFacts(kit renderKit, issue jira.Issue) string {
	var parts []string

	for _, part := range []string{issue.Type, issue.Priority, issue.Status} {
		if part != "" {
			parts = append(parts, part)
		}
	}

	return kit.styles.label.Render(strings.Join(parts, kit.marks.separator))
}

// lines is the part of an issue only a full read has: the reporter, the
// description, and the most recent comments.
func (d issueDetail) lines(view readView, issueKey jira.Key, width int) []string {
	kit := view.kit

	switch {
	case d.key != issueKey || !d.loaded:
		return []string{"", kit.styles.label.Render("reading the description and comments" + kit.marks.ellipsis)}
	case d.err != nil:
		return []string{"", kit.failureBlock(d.err, width), "press " + view.keys.refresh.Help().Key + " to try again"}
	}

	detail := d.detail
	lines := []string{kit.styles.label.Render("reported by " + detail.Reporter)}
	lines = append(lines, issuePeopleAndTags(kit.styles, detail)...)
	lines = append(lines, "")

	if strings.TrimSpace(detail.Description) == "" {
		lines = append(lines, kit.styles.label.Render("no description"))
	} else {
		lines = append(lines, wrap(detail.Description, width))
	}

	lines = append(lines, issueRelations(kit, detail)...)

	return append(lines, view.comments(detail)...)
}

// readView is what an issue's detail draws with beside its own read: the
// glyphs and styles, the keys it names, the clock its comments are aged by,
// and how many of the latest comments it shows.
type readView struct {
	kit           renderKit
	keys          keyMap
	now           time.Time
	commentsShown int
}

// readView is the issue detail's view of the rest of the interface.
func (m Model) readView() readView {
	return readView{
		kit: m.kit(), keys: m.keys, now: m.deps.now(),
		commentsShown: cmp.Or(m.cfg.UI.CommentsShown, defaultCommentsShown),
	}
}

// issuePeopleAndTags is the assignee and the issue's tags — labels, components,
// fix versions, and its parent — each line drawn only when the issue has it.
func issuePeopleAndTags(sty styles, detail jira.IssueDetail) []string {
	var lines []string

	if detail.Assignee != "" {
		lines = append(lines, sty.label.Render("assigned to "+detail.Assignee))
	}

	lines = tagLine(sty, lines, "labels", detail.Labels)
	lines = tagLine(sty, lines, "components", detail.Components)
	lines = tagLine(sty, lines, "fix versions", detail.FixVersions)

	if detail.Parent.Key != "" {
		lines = append(lines, sty.label.Render("parent "+detail.Parent.Key+" "+detail.Parent.Summary))
	}

	return lines
}

// tagLine adds a labeled, comma-joined line for a list of tags, or nothing when
// the list is empty.
func tagLine(sty styles, lines []string, name string, values []string) []string {
	if len(values) == 0 {
		return lines
	}

	return append(lines, sty.label.Render(name+" "+strings.Join(values, ", ")))
}

// issueRelations is the issue's subtasks and its links to other issues, each
// block drawn only when there is one.
func issueRelations(kit renderKit, detail jira.IssueDetail) []string {
	var lines []string

	if len(detail.Subtasks) > 0 {
		lines = append(lines, "", kit.styles.strong.Render("Subtasks"))
		for _, sub := range detail.Subtasks {
			lines = append(lines, kit.styles.label.Render("  "+sub.Key+" "+sub.Summary+kit.marks.separator+sub.Status))
		}
	}

	if len(detail.IssueLinks) > 0 {
		lines = append(lines, "", kit.styles.strong.Render("Links"))
		for _, link := range detail.IssueLinks {
			lines = append(lines, kit.styles.label.Render("  "+link.Relation+" "+link.Issue.Key+" "+
				link.Issue.Summary+kit.marks.separator+link.Issue.Status))
		}
	}

	return lines
}

// issuesBehavior is the Issues pane's behavior.
func issuesBehavior() behavior {
	// The rail is the list, its rows marked by how their issues' tasks stand.
	rail := func(m Model, rows int) string { return m.issues.render(m.kit(), rows, m.tasks.issueMarks(m.kit())) }

	return behavior{
		rail: rail, detail: Model.issueDetailView,
		// With the rail gone the list and the selected issue take turns: the
		// issue once it is being read, or when there is no list to pick from.
		narrow: func(m Model, rows int) string {
			if _, ok := m.issues.current(); !ok || m.issues.viewing {
				return m.issueDetailView(m.detailWidth())
			}

			return rail(m, rows)
		},
		keys:   func(m Model) []key.Binding { return liveKeys(m.issuesOffers()) },
		handle: Model.handleIssuesKey, pick: Model.pickIssue, move: Model.moveIssue,
		// refresh reads the list again, and the issue shown in full whether or not
		// it changed, so r retries a detail load that failed. An issue the selection has
		// only just reached is left to the read its rest will start.
		refresh: func(m Model) (Model, tea.Cmd) {
			m = m.searching()

			selected, ok := m.issues.current()
			if !ok {
				return m, m.relistIssues()
			}

			m, detail := m.reloadDetail(selected.Key)

			return m, tea.Batch(m.relistIssues(), detail)
		},
		loading: func(m Model) bool { return m.issues.loading },
		scroll:  func(m *Model) *int { return &m.detail.scroll },
		answers: []string{
			"change-status", "comment", "assign", "log-work", "start-work", "track-issue", actionOpenLink,
			actionCopyLink, "search-issues", "filter-issues", "switch-view", "load-more", actionRefresh,
		},
	}
}
