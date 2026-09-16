// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"cmp"
	"fmt"
	"strings"

	"github.com/jacob-delgado/workflow/internal/jira"
)

// failedGlyph marks a pane whose load failed. It is the one place the interface
// says "this broke", so it carries its own shape rather than relying on color.
const failedGlyph = "✗"

// issuesLoaded carries the search's answer back into the update loop.
type issuesLoaded struct {
	found jira.SearchResult
	err   error
}

// issueList is the Issues pane's state. Loading, failed, empty and listing are
// derived from these fields by guard clauses rather than held as an enum, so
// there is no switch with a final arm that can never be taken.
type issueList struct {
	found    jira.SearchResult
	err      error
	settled  bool
	selected int
}

// settle records the search's answer.
func (l issueList) settle(answer issuesLoaded) issueList {
	l.found, l.err, l.settled, l.selected = answer.found, answer.err, true, 0

	return l
}

// move shifts the selection, stopping at either end rather than wrapping: in a
// list, wrapping from the last row to the first reads as a jump.
func (l issueList) move(step int) issueList {
	last := len(l.found.Issues) - 1
	l.selected = max(0, min(l.selected+step, last))

	return l
}

// render draws as many rows as fit, scrolled so the selection stays on screen.
func (l issueList) render(rows int) string {
	switch {
	case !l.settled:
		return "loading…"
	case l.err != nil:
		return failedGlyph + " failed · see detail"
	case len(l.found.Issues) == 0:
		return "no open issues assigned to you"
	}

	first := max(0, l.selected-rows+1)
	last := min(len(l.found.Issues), first+rows)
	lines := make([]string, 0, max(0, last-first))

	for index := first; index < last; index++ {
		lines = append(lines, l.row(index))
	}

	return strings.Join(lines, "\n")
}

// row is one issue: a selection marker, a status glyph, the key and the summary.
// The frame clips it with an ellipsis, so it is never measured here.
func (l issueList) row(index int) string {
	issue := l.found.Issues[index]

	marker := "  "
	if index == l.selected {
		marker = "▸ "
	}

	return marker + statusGlyph(issue.StatusCategory) + " " + issue.Key + " " + issue.Summary
}

// detail describes the selected issue, or explains why there is nothing to
// describe. fallback is what the pane shows when there is no issue to talk
// about, which keeps the configuration summary on screen through a failure.
func (l issueList) detail(fallback string) string {
	if l.err != nil {
		return "issues: " + l.err.Error() + "\n\n" + fallback
	}

	if !l.settled || len(l.found.Issues) == 0 {
		return fallback
	}

	selected := l.found.Issues[l.selected]
	lines := []string{selected.Key + " " + selected.Summary, "", "status  " + selected.Status}

	if l.found.Total > len(l.found.Issues) {
		lines = append(lines, "", fmt.Sprintf("showing %d of %d", len(l.found.Issues), l.found.Total))
	}

	return strings.Join(lines, "\n")
}

// statusGlyph carries an issue's status category by fill: not started, in
// flight, done. A category an instance invented gets a neutral dot rather than
// a guess.
func statusGlyph(category string) string {
	glyphs := map[string]string{"new": "○", "indeterminate": "◐", "done": "●"}

	return cmp.Or(glyphs[category], "·")
}
