// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"fmt"
	"slices"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/jacob-delgado/workflow/internal/jira"
)

// issuesLoaded carries the search's answer back into the update loop.
type issuesLoaded struct {
	found jira.SearchResult
	err   error
}

// apply records the answer, and asks for the selected issue in full.
func (msg issuesLoaded) apply(m Model) (Model, tea.Cmd) {
	m.issues = m.issues.settle(msg)
	m = m.resumeIssue()

	return m.loadDetail()
}

// issueList is the Issues pane's state. Loading, failed, empty and listing are
// derived from these fields by guard clauses rather than held as an enum, so
// there is no switch with a final arm that can never be taken.
type issueList struct {
	found    jira.SearchResult
	err      error
	settled  bool
	selected int
	// moved records that someone chose an issue, after which nothing selects
	// one for them.
	moved bool
}

// settle records the search's answer. The selection follows its issue rather
// than its row: the list is ordered by update time, so a refresh after changing
// an issue's status brings that issue to the top.
func (l issueList) settle(answer issuesLoaded) issueList {
	previous, _ := l.current()
	l.found, l.err, l.settled = answer.found, answer.err, true

	return l.selectKey(previous.Key)
}

// selectKey selects the issue with a key, or keeps the row, clamped, when the
// issue is not listed — done, reassigned, or never selected.
func (l issueList) selectKey(issueKey string) issueList {
	index := slices.IndexFunc(l.found.Issues, func(candidate jira.Issue) bool {
		return candidate.Key == issueKey
	})
	if index < 0 {
		return l.move(0)
	}

	l.selected = index

	return l
}

// current is the selected issue, if there is one.
func (l issueList) current() (jira.Issue, bool) {
	if len(l.found.Issues) == 0 {
		return jira.Issue{}, false
	}

	return l.found.Issues[l.selected], true
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
	last := len(l.found.Issues) - 1
	l.selected = max(0, min(l.selected+step, last))

	return l
}

// render draws as many rows as fit, scrolled so the selection stays on screen.
func (l issueList) render(marks glyphs, rows int) string {
	switch {
	case !l.settled:
		return "loading" + marks.ellipsis
	case l.err != nil:
		return marks.failed + " failed" + marks.separator + "see detail"
	case len(l.found.Issues) == 0:
		return "no open issues assigned to you"
	}

	first, last := window(l.selected, len(l.found.Issues), rows)
	lines := make([]string, 0, last-first)

	for index := first; index < last; index++ {
		issue := l.found.Issues[index]
		lines = append(lines, marks.marker(index == l.selected)+marks.status(issue.StatusCategory)+" "+
			issue.Key+" "+issue.Summary)
	}

	return strings.Join(lines, "\n")
}

// rowAt is the index of the issue drawn on a line of render's output.
func (l issueList) rowAt(line, rows int) (int, bool) {
	first, last := window(l.selected, len(l.found.Issues), rows)
	index := first + line

	return index, line >= 0 && index < last
}

// failure describes a search that failed, above what the pane falls back to.
func (l issueList) failure(fallback string) (string, bool) {
	if l.err == nil {
		return "", false
	}

	return "issues: " + l.err.Error() + "\n\n" + fallback, true
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
