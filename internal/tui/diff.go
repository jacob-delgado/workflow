// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/jacob-delgado/workflow/internal/gitrepo"
	"github.com/jacob-delgado/workflow/internal/sanitize"
	"github.com/jacob-delgado/workflow/internal/tui/latest"
)

// diffTab is the spaces a tab is drawn as, so a tab-indented diff line keeps a
// steady width to truncate against.
const diffTab = "    "

// diffState is the diff of the file the Commits pane has selected: the file
// its latest read is of, and what that read found.
type diffState struct {
	path string
	read latest.Read[[]string]
}

// rereadDiff forgets the diff held, and any read of it still out, and reads
// the selected file's again: the status answered, and a stage or an edit
// behind a refresh changes a file's diff without changing its path.
func (m Model) rereadDiff() (Model, tea.Cmd) {
	m.diff = diffState{read: m.diff.read.Cleared()}

	return m.loadDiff()
}

// loadDiff begins a read of the selected file's diff through the repository
// seam, superseding any still out, unless the read held or out is already
// that file's and has not failed; so it is safe to call on every move, and a
// move back onto a file whose diff failed reads it again. A read begins from
// nothing held, so the pane says it is reading, not the failure it retries.
func (m Model) loadDiff() (Model, tea.Cmd) {
	ctx, diff := m.session, m.deps.Git.Diff

	change, ok := m.changes.current()
	if diff == nil || !ok || m.diff.standsFor(change.Path) {
		return m, nil
	}

	read, round := m.diff.read.Cleared().Begin()
	m.diff = diffState{path: change.Path, read: read}

	return m, func() tea.Msg {
		lines, err := diff(ctx, change)

		return diffLoaded{lines: lines, err: err, round: round}
	}
}

// standsFor reports a read of path's diff out, or answered and shown: one a
// read begun now would only repeat.
func (d diffState) standsFor(path string) bool {
	return d.path == path && (d.read.Out() || d.read.Stage() == latest.StageShown)
}

// diffLoaded is a file's diff back from git, as the read begun in round
// found it.
type diffLoaded struct {
	lines []string
	err   error
	round latest.Round
}

var _ applier = diffLoaded{}

// apply shows the diff unless a later read has begun since: a move to another
// file, or a status answered, which a stage brings, begins one, so the answer
// of a read for a file no longer selected, or of one read before the stage,
// is dropped.
func (msg diffLoaded) apply(m Model) (Model, tea.Cmd) {
	if read, taken := m.diff.read.Answered(msg.round, msg.lines, msg.err); taken {
		m.diff.read = read
	}

	return m, nil
}

// section is the selected file's diff, headed by its path and marked line by
// line — added, removed, or context — so the Commits pane can show it below its
// list. An added line keeps its leading + and a removed line its -, so the mark
// reads without color too.
func (d diffState) section(kit renderKit, change gitrepo.Change, width int) []string {
	reading := func() []string { return []string{kit.styles.label.Render("reading the diff" + kit.marks.ellipsis)} }
	body := map[latest.Stage]func() []string{
		latest.StageNeverRead: reading,
		latest.StageReading:   reading,
		latest.StageShown:     func() []string { return d.marked(kit, width) },
		latest.StageFailed:    func() []string { return []string{kit.failureLine(d.read.Err())} },
	}[d.read.Stage()]()

	return append([]string{"", kit.styles.strong.Render("diff: " + sanitize.Line(change.Path))}, body...)
}

// marked is the diff held, each line marked as markDiffLine marks it, or
// what says it holds no textual change.
func (d diffState) marked(kit renderKit, width int) []string {
	diff, _ := d.read.Value()
	if len(diff) == 0 {
		return []string{kit.styles.label.Render("no textual change")}
	}

	lines := make([]string, 0, len(diff))
	inHunk := false

	for _, line := range diff {
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

	if diff, held := m.diff.read.Value(); held {
		body += "\n\n" + strings.Join(diff[:min(len(diff), discardPreviewLines)], "\n")
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

	ctx, discard := m.session, m.deps.Git.Discard

	return m, func() tea.Msg { return discarded{path: change.Path, err: discard(ctx, change)} }
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

	return m.readingChanges()
}
