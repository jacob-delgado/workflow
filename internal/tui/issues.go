// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"fmt"
	"slices"
	"strings"
	"unicode/utf8"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/jacob-delgado/workflow/internal/jira"
)

// issuesLoaded carries the search's answer back into the update loop. startAt is
// where the page began: past zero, it is a further page to append rather than a
// fresh list to replace.
type issuesLoaded struct {
	found   jira.SearchResult
	err     error
	startAt int
}

// apply records the answer, asks for the selected issue in full, and saves a
// fresh first page to the cache so the next start shows it at once.
func (msg issuesLoaded) apply(m Model) (Model, tea.Cmd) {
	m.issues = m.issues.settle(msg)
	m = m.resumeIssue()

	updated, detail := m.loadDetail()
	if msg.startAt == 0 && msg.err == nil {
		return updated, tea.Batch(detail, updated.saveIssues())
	}

	return updated, detail
}

// seed shows the last saved issue list at once, before Jira answers, so the
// pane is useful on the first frame. An empty or absent cache changes nothing.
func (l issueList) seed(load func() []jira.Issue) issueList {
	if load == nil {
		return l
	}

	cached := load()
	if len(cached) == 0 {
		return l
	}

	l.found = jira.SearchResult{Issues: cached, Total: len(cached)}
	l.settled = true

	return l
}

// saveIssues persists the loaded list for the next start, best effort.
func (m Model) saveIssues() tea.Cmd {
	save := m.deps.Cache.SaveIssues
	if save == nil {
		return nil
	}

	issues := m.issues.found.Issues

	return func() tea.Msg {
		save(issues)

		return nil
	}
}

// issueList is the Issues pane's state. Loading, failed, empty and listing are
// derived from these fields by guard clauses rather than held as an enum, so
// there is no switch with a final arm that can never be taken.
type issueList struct {
	found    jira.SearchResult
	err      error
	settled  bool
	loading  bool
	selected int
	// moved records that someone chose an issue, after which nothing selects
	// one for them.
	moved bool
	// viewing records that, in the collapsed layout, the selected issue is being
	// read in full rather than the list being scanned.
	viewing bool
	// filter narrows the list to issues whose key or summary contains it, over
	// what is already loaded; filtering records that keystrokes are building it.
	filter    string
	filtering bool
}

// visible is the issues the filter admits, or all of them when it is empty. The
// selection and every row operation work over this, so filtering narrows what
// can be scanned without touching what was loaded.
func (l issueList) visible() []jira.Issue {
	if l.filter == "" {
		return l.found.Issues
	}

	needle := strings.ToLower(l.filter)

	var matching []jira.Issue

	for _, issue := range l.found.Issues {
		if strings.Contains(strings.ToLower(issue.Key+" "+issue.Summary), needle) {
			matching = append(matching, issue)
		}
	}

	return matching
}

// settle records the search's answer. The selection follows its issue rather
// than its row: the list is ordered by update time, so a refresh after changing
// an issue's status brings that issue to the top.
func (l issueList) settle(answer issuesLoaded) issueList {
	l.settled, l.loading = true, false

	// A further page appends to the list; a failed page leaves what is already
	// there rather than replacing the pane with an error.
	if answer.startAt > 0 {
		if answer.err == nil {
			l.found.Issues = append(l.found.Issues, answer.found.Issues...)
			l.found.Total = answer.found.Total
		}

		return l
	}

	previous, _ := l.current()
	l.found, l.err = answer.found, answer.err

	return l.selectKey(previous.Key)
}

// hasMore reports that the list is truncated: fewer issues are loaded than
// matched, so there is a further page to read.
func (l issueList) hasMore() bool {
	return len(l.found.Issues) < l.found.Total
}

// selectKey selects the issue with a key, or keeps the row, clamped, when the
// issue is not listed — done, reassigned, or never selected.
func (l issueList) selectKey(issueKey string) issueList {
	index := slices.IndexFunc(l.visible(), func(candidate jira.Issue) bool {
		return candidate.Key == issueKey
	})
	if index < 0 {
		return l.move(0)
	}

	l.selected = index

	return l
}

// beginFilter starts a fresh filter, capturing keystrokes until it is confirmed
// or canceled.
func (l issueList) beginFilter() issueList {
	l.filter, l.filtering = "", true

	return l
}

// extendFilter adds typed text to the filter and keeps the selection in range.
func (l issueList) extendFilter(text string) issueList {
	l.filter += text

	return l.clampSelection()
}

// trimFilter removes the last character of the filter.
func (l issueList) trimFilter() issueList {
	if l.filter != "" {
		_, size := utf8.DecodeLastRuneInString(l.filter)
		l.filter = l.filter[:len(l.filter)-size]
	}

	return l.clampSelection()
}

// confirmFilter stops capturing keystrokes but keeps the filter applied, so the
// narrowed list can be navigated.
func (l issueList) confirmFilter() issueList {
	l.filtering = false

	return l
}

// clearFilter cancels the filter and restores the whole list, keeping the
// selection on the issue that was selected.
func (l issueList) clearFilter() issueList {
	selected, ok := l.current()
	l.filter, l.filtering = "", false

	if ok {
		return l.selectKey(selected.Key)
	}

	return l.move(0)
}

// clampSelection keeps the selection within the visible list after it narrows.
func (l issueList) clampSelection() issueList {
	l.selected = max(0, min(l.selected, len(l.visible())-1))

	return l
}

// current is the selected issue, if there is one.
func (l issueList) current() (jira.Issue, bool) {
	visible := l.visible()
	if l.selected < 0 || l.selected >= len(visible) {
		return jira.Issue{}, false
	}

	return visible[l.selected], true
}

// find is the listed issue with a key, if it is listed.
func (l issueList) find(issueKey string) (jira.Issue, bool) {
	index := slices.IndexFunc(l.found.Issues, func(candidate jira.Issue) bool {
		return candidate.Key == issueKey
	})
	if index < 0 {
		return jira.Issue{}, false
	}

	return l.found.Issues[index], true
}

// move shifts the selection, stopping at either end rather than wrapping: in a
// list, wrapping from the last row to the first reads as a jump.
func (l issueList) move(step int) issueList {
	last := len(l.visible()) - 1
	l.selected = max(0, min(l.selected+step, last))
	l.viewing = false

	return l
}

// render draws as many rows as fit, scrolled so the selection stays on screen.
func (l issueList) render(marks glyphs, sty styles, rows int) string {
	visible := l.visible()

	switch {
	case !l.settled:
		return "loading" + marks.ellipsis
	case l.err != nil:
		return failedGlyph(sty, marks) + " failed" + marks.separator + "see detail"
	case len(l.found.Issues) == 0:
		return "no issues in this view"
	case len(visible) == 0:
		return "no issue matches the filter"
	}

	first, last := window(l.selected, len(visible), rows)
	lines := make([]string, 0, last-first)

	for index := first; index < last; index++ {
		issue := visible[index]
		lines = append(lines, marks.marker(index == l.selected)+marks.status(issue.StatusCategory)+" "+
			issue.Key+" "+issue.Summary)
	}

	return strings.Join(lines, "\n")
}

// rowAt is the index of the issue drawn on a line of render's output.
func (l issueList) rowAt(line, rows int) (int, bool) {
	first, last := window(l.selected, len(l.visible()), rows)
	index := first + line

	return index, line >= 0 && index < last
}

// capped says how much of a capped list is shown, or nothing for a whole one.
func (l issueList) capped() string {
	if l.found.Total <= len(l.found.Issues) {
		return ""
	}

	return fmt.Sprintf("showing %d of %d", len(l.found.Issues), l.found.Total)
}

// window is the slice of a list of count rows that fits in rows lines, scrolled
// so the selected row is on screen. At least one row always shows.
func window(selected, count, rows int) (int, int) {
	rows = max(1, rows)
	first := max(0, selected-rows+1)

	return first, min(count, first+rows)
}
