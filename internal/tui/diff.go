// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/jacob-delgado/workflow/internal/gitrepo"
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

// load reads the selected file's diff through the repository seam, unless it
// is already the one loaded. A stale answer for a file no longer selected is
// dropped when it arrives, so it is safe to call on every move.
func (d diffState) load(read func(gitrepo.Change) ([]string, error), changes changeList) tea.Cmd {
	change, ok := changes.current()
	if read == nil || !ok || d.path == change.Path {
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

var _ applier = diffLoaded{}

func (msg diffLoaded) apply(m Model) (Model, tea.Cmd) {
	// A faster key may have moved the selection on; keep the diff only if it is
	// still the selected file's.
	if change, ok := m.changes.current(); !ok || change.Path != msg.path {
		return m, nil
	}

	m.diff = diffState(msg)

	return m, nil
}

// section is the selected file's diff, headed by its path and marked line by
// line — added, removed, or context — so the Commits pane can show it below its
// list. An added line keeps its leading + and a removed line its -, so the mark
// reads without color too.
func (d diffState) section(kit renderKit, change gitrepo.Change, width int) []string {
	lines := []string{"", kit.styles.strong.Render("diff: " + sanitize.Line(change.Path))}

	switch {
	case d.path != change.Path:
		return append(lines, kit.styles.label.Render("reading the diff"+kit.marks.ellipsis))
	case d.err != nil:
		return append(lines, kit.failureLine(d.err))
	case len(d.lines) == 0:
		return append(lines, kit.styles.label.Render("no textual change"))
	}

	inHunk := false

	for _, line := range d.lines {
		if strings.HasPrefix(line, "@@") {
			inHunk = true
		}

		lines = append(lines, markDiffLine(kit, line, width, inHunk))
	}

	return lines
}

// markDiffLine draws one diff line: tabs made even, the line clipped to width,
// then tinted by its kind — an added line green, a removed line red, a hunk
// header faint — with git's + or - left in place. A + or - only marks a change
// inside a hunk; the file header before the first hunk (its +++ and --- lines,
// a content line there could not) is drawn plain.
func markDiffLine(kit renderKit, line string, width int, inHunk bool) string {
	clipped := ansi.Truncate(strings.ReplaceAll(line, "\t", diffTab), width, kit.marks.ellipsis)

	switch {
	case strings.HasPrefix(clipped, "@@"):
		return kit.styles.label.Render(clipped)
	case !inHunk:
		return clipped
	case strings.HasPrefix(clipped, "+"):
		return kit.styles.forge.Render(clipped)
	case strings.HasPrefix(clipped, "-"):
		return kit.styles.failure.Render(clipped)
	default:
		return clipped
	}
}

// discardPreviewLines is how much of a file's diff the discard's last look
// shows: enough to recognize the change, not to read it all.
const discardPreviewLines = 12

// previewDiscard holds discarding the selected file's changes for a last look
// at the file and the start of its diff, since a discard cannot be undone.
func (m Model) previewDiscard() (Model, tea.Cmd) {
	change, _ := m.changes.current()
	path := sanitize.Line(change.Path)
	body := "Discard the " + change.Kind() + " file " + path + ", staged and not?"

	if m.diff.path == change.Path {
		body += "\n\n" + strings.Join(m.diff.lines[:min(len(m.diff.lines), discardPreviewLines)], "\n")
	}

	m.overlay = lastLook{
		title: "Discard changes", verb: "discard", doing: "discarding",
		body:    body + "\n\nThis cannot be undone.",
		proceed: func(m Model) (Model, tea.Cmd) { return m.discardChange(change) },
	}

	return m, nil
}

// discardChange drops change from the index and the work tree, the look open
// and in flight until git answers.
func (m Model) discardChange(change gitrepo.Change) (Model, tea.Cmd) {
	if m.dryRun {
		return m.closeOverlay().noticed("dry run: would discard " + sanitize.Line(change.Path)), nil
	}

	discard := m.deps.Git.Discard

	return m, func() tea.Msg { return discarded{path: change.Path, err: discard(change)} }
}

// discarded reports how a discard went.
type discarded struct {
	path string
	err  error
}

var _ applier = discarded{}

// apply closes the look and says so, or keeps it open with git's refusal, and
// reads the status again either way.
func (msg discarded) apply(m Model) (Model, tea.Cmd) {
	m = m.answerLook(msg.err)

	if msg.err != nil {
		m = m.noticedFailure(msg.err)
	} else {
		m = m.noticed(m.marks.done + " discarded " + sanitize.Line(msg.path))
	}

	return m, loadChanges(m.deps)
}
