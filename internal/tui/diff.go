// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/jacob-delgado/workflow/internal/sanitize"
)

// diffTab is the spaces a tab is drawn as, so a tab-indented diff line keeps a
// steady width to truncate against.
const diffTab = "    "

// diffState is the diff of the file the Commits pane has selected.
type diffState struct {
	path  string
	lines []string
	err   error
}

// loadDiff reads the selected file's diff through the repository seam, unless it
// is already the one loaded. A stale answer for a file no longer selected is
// dropped when it arrives, so it is safe to call on every move.
func (m Model) loadDiff() tea.Cmd {
	read := m.deps.Git.Diff

	change, ok := m.changes.current()
	if read == nil || !ok || m.diff.path == change.Path {
		return nil
	}

	return func() tea.Msg {
		lines, err := read(change)

		return diffLoaded{path: change.Path, lines: lines, err: err}
	}
}

// diffLoaded is a file's diff back from git.
type diffLoaded struct {
	path  string
	lines []string
	err   error
}

func (msg diffLoaded) apply(m Model) (Model, tea.Cmd) {
	// A faster key may have moved the selection on; keep the diff only if it is
	// still the selected file's.
	if change, ok := m.changes.current(); !ok || change.Path != msg.path {
		return m, nil
	}

	m.diff = diffState(msg)

	return m, nil
}

// diffSection is the selected file's diff, headed by its path and marked line by
// line — added, removed, or context — so the Commits pane can show it below its
// list. An added line keeps its leading + and a removed line its -, so the mark
// reads without color too.
func (m Model) diffSection(width int) []string {
	change, ok := m.changes.current()
	if m.deps.Git.Diff == nil || !ok {
		return nil
	}

	lines := []string{"", m.styles.strong.Render("diff: " + sanitize.Line(change.Path))}

	switch {
	case m.diff.path != change.Path:
		return append(lines, m.styles.label.Render("reading the diff"+m.marks.ellipsis))
	case m.diff.err != nil:
		return append(lines, failedGlyph(m.styles, m.marks)+" "+m.diff.err.Error())
	case len(m.diff.lines) == 0:
		return append(lines, m.styles.label.Render("no textual change"))
	}

	inHunk := false

	for _, line := range m.diff.lines {
		if strings.HasPrefix(line, "@@") {
			inHunk = true
		}

		lines = append(lines, m.markDiffLine(line, width, inHunk))
	}

	return lines
}

// markDiffLine draws one diff line: tabs made even, the line clipped to width,
// then tinted by its kind — an added line green, a removed line red, a hunk
// header faint — with git's + or - left in place. A + or - only marks a change
// inside a hunk; the file header before the first hunk (its +++ and --- lines,
// a content line there could not) is drawn plain.
func (m Model) markDiffLine(line string, width int, inHunk bool) string {
	clipped := ansi.Truncate(strings.ReplaceAll(line, "\t", diffTab), width, m.marks.ellipsis)

	switch {
	case strings.HasPrefix(clipped, "@@"):
		return m.styles.label.Render(clipped)
	case !inHunk:
		return clipped
	case strings.HasPrefix(clipped, "+"):
		return m.styles.forge.Render(clipped)
	case strings.HasPrefix(clipped, "-"):
		return m.styles.failure.Render(clipped)
	default:
		return clipped
	}
}
