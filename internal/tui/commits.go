// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/jacob-delgado/workflow/internal/gitrepo"
	"github.com/jacob-delgado/workflow/internal/proc"
	"github.com/jacob-delgado/workflow/internal/sanitize"
)

// preCommit is the hook h runs.
const preCommit = "pre-commit"

// changeList is the work tree's changed files, as far as they have loaded.
type changeList struct {
	changes  []gitrepo.Change
	loaded   bool
	err      error
	selected int
}

// changesLoaded carries the work tree's status.
type changesLoaded struct {
	changes []gitrepo.Change
	err     error
}

// apply records the changes, keeping the selection on the same file.
func (msg changesLoaded) apply(m Model) (Model, tea.Cmd) {
	previous, _ := m.changes.current()
	m.changes = changeList{changes: msg.changes, loaded: true, err: msg.err, selected: 0}

	index := slices.IndexFunc(msg.changes, func(change gitrepo.Change) bool { return change.Path == previous.Path })
	m.changes.selected = max(0, min(index, len(msg.changes)-1))

	return m, nil
}

// loadChanges is the command that reads the work tree's status.
func (m Model) loadChanges() tea.Cmd {
	read := m.deps.Git.Changes
	if read == nil {
		return nil
	}

	return func() tea.Msg {
		changes, err := read()

		return changesLoaded{changes: changes, err: err}
	}
}

// current is the selected change, if there is one.
func (l changeList) current() (gitrepo.Change, bool) {
	if len(l.changes) == 0 {
		return gitrepo.Change{}, false
	}

	return l.changes[l.selected], true
}

// staged counts the changes in the index.
func (l changeList) staged() int {
	count := 0

	for _, change := range l.changes {
		if change.IsStaged() {
			count++
		}
	}

	return count
}

// commitsRail counts what is staged and changed, and the branch's commits.
func (m Model) commitsRail(_ int) string {
	switch {
	case !m.changes.loaded:
		return "loading" + m.marks.ellipsis
	case m.changes.err != nil:
		return m.marks.failed + " status failed" + m.marks.separator + "see detail"
	}

	counts := strconv.Itoa(m.changes.staged()) + " of " + strconv.Itoa(len(m.changes.changes)) + " staged"

	return counts + "\n" + m.styles.label.Render(plural(len(m.branch.branch.Commits), "commit")+" on this branch")
}

// commitsDetail lists the changed files, then the branch's commits.
func (m Model) commitsDetail(width int) string {
	if m.outsideRepository() {
		return wrap(notInRepository, width)
	}

	if m.changes.err != nil {
		return wrap(m.failure(m.changes.err), width)
	}

	lines := m.changeRows()
	if len(lines) == 0 {
		lines = []string{m.styles.label.Render("nothing changed")}
	}

	commits := m.branch.branch.Commits
	if len(commits) > 0 {
		lines = append(lines, "", m.styles.strong.Render("On this branch"))
	}

	for _, commit := range commits {
		lines = append(lines, m.styles.label.Render(commit.Hash)+" "+commit.Subject)
	}

	return strings.Join(lines, "\n")
}

// changeRows draws each changed file: where it stands, git's two letters, and
// its path — neutralized, because a file name can hold an escape sequence.
func (m Model) changeRows() []string {
	rows := make([]string, 0, len(m.changes.changes))

	for index, change := range m.changes.changes {
		path := sanitize.Line(change.Path)
		if change.OriginalPath != "" {
			path = sanitize.Line(change.OriginalPath) + m.marks.arrow + path
		}

		rows = append(rows, m.marks.marker(index == m.changes.selected)+m.stageGlyph(change)+" "+
			fmt.Sprintf("%-11s", change.Kind())+path)
	}

	return rows
}

// stageGlyph says by shape how much of a change is staged.
func (m Model) stageGlyph(change gitrepo.Change) string {
	switch {
	case change.Conflicted():
		return m.marks.failed
	case change.IsStaged() && change.HasUnstaged():
		return m.marks.inFlight
	case change.IsStaged():
		return m.marks.done
	default:
		return m.marks.notStarted
	}
}

// commitsKeys offers what can be done with the changes as they are.
func (m Model) commitsKeys() []key.Binding {
	if m.outsideRepository() {
		return nil
	}

	var keys []key.Binding

	if _, hasChange := m.changes.current(); hasChange && m.deps.Git.Stage != nil {
		keys = append(keys, m.keys.stage, m.keys.stageAll)
	}

	if m.changes.staged() > 0 && m.deps.Git.Commit != nil {
		keys = append(keys, m.keys.commit)
	}

	if m.deps.Hooks.Run != nil {
		keys = append(keys, m.keys.runHooks)
	}

	return keys
}

// handleCommitsKey answers the Commits pane's own keys.
func (m Model) handleCommitsKey(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.down):
		m.changes.selected = min(m.changes.selected+1, max(0, len(m.changes.changes)-1))
		m = m.followChange()
	case key.Matches(msg, m.keys.up):
		m.changes.selected = max(0, m.changes.selected-1)
		m = m.followChange()
	case key.Matches(msg, m.keys.stage):
		return m.toggleStaged()
	case key.Matches(msg, m.keys.stageAll):
		return m.stageAll()
	case key.Matches(msg, m.keys.commit):
		return m.openCommitComposer()
	case key.Matches(msg, m.keys.runHooks) && m.deps.Hooks.Run != nil:
		return m.runPreCommit()
	case key.Matches(msg, m.keys.refresh):
		return m, tea.Batch(m.loadChanges(), m.loadBranch())
	}

	return m, nil
}

// followChange scrolls the detail so the selected file stays on screen, the way
// the Issues list keeps its selection in view.
func (m Model) followChange() Model {
	m.scroll, _ = window(m.changes.selected, len(m.changes.changes), m.detailRows())

	return m
}

// pickChange selects the file on a clicked line of the detail.
func (m Model) pickChange(line, _ int, inRail bool) (Model, tea.Cmd) {
	index := line + m.scroll
	if inRail || index < 0 || index >= len(m.changes.changes) {
		return m, nil
	}

	m.changes.selected = index

	return m, nil
}

// toggleStaged stages the selected file, or unstages it if it is wholly staged.
func (m Model) toggleStaged() (Model, tea.Cmd) {
	change, ok := m.changes.current()
	if !ok || m.deps.Git.Stage == nil || m.deps.Git.Unstage == nil {
		return m, nil
	}

	unstage := change.IsStaged() && !change.HasUnstaged()

	verb, act := "stage ", m.deps.Git.Stage
	if unstage {
		verb, act = "unstage ", m.deps.Git.Unstage
	}

	if m.dryRun {
		return m.noticed("dry run: would " + verb + sanitize.Line(change.Path)), nil
	}

	return m, func() tea.Msg { return staged{err: act(change)} }
}

// stageAll stages every file with changes not yet staged.
func (m Model) stageAll() (Model, tea.Cmd) {
	var pending []gitrepo.Change

	for _, change := range m.changes.changes {
		if change.HasUnstaged() || change.Conflicted() {
			pending = append(pending, change)
		}
	}

	if len(pending) == 0 || m.deps.Git.Stage == nil {
		return m, nil
	}

	if m.dryRun {
		return m.noticed("dry run: would stage " + plural(len(pending), "file")), nil
	}

	stage := m.deps.Git.Stage

	return m, func() tea.Msg {
		failures := make([]error, 0, len(pending))

		for _, change := range pending {
			failures = append(failures, stage(change))
		}

		return staged{err: errors.Join(failures...)}
	}
}

// staged reports how staging went.
type staged struct {
	err error
}

// apply reads the status again, which is the only way to know what the index
// now holds.
func (msg staged) apply(m Model) (Model, tea.Cmd) {
	if msg.err != nil {
		m = m.noticed(m.failure(msg.err))
	}

	return m, m.loadChanges()
}

// runPreCommit runs the pre-commit hook on what is staged, without committing.
func (m Model) runPreCommit() (Model, tea.Cmd) {
	if m.dryRun {
		return m.noticed("dry run: would run the " + preCommit + " hook"), nil
	}

	run := m.deps.Hooks.Run

	return m.startRun(preCommit, func() (proc.Output, error) { return run(preCommit) }, nil)
}
