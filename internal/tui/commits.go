// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"

	"github.com/jacob-delgado/workflow/internal/gitrepo"
	"github.com/jacob-delgado/workflow/internal/loop"
	"github.com/jacob-delgado/workflow/internal/proc"
	"github.com/jacob-delgado/workflow/internal/sanitize"
)

// preCommit is the hook h runs.
const preCommit = "pre-commit"

// changeList is the work tree's changed files, as far as they have loaded, and
// how far the Commits pane's detail is scrolled.
type changeList struct {
	changes  []gitrepo.Change
	loaded   bool
	err      error
	selected int
	scroll   int
	// loading is a refresh begun and not yet answered.
	loading bool
}

// changesLoaded carries the work tree's status.
type changesLoaded struct {
	changes []gitrepo.Change
	err     error
}

var _ applier = changesLoaded{}

// apply records the changes, keeping the selection on the same file.
func (msg changesLoaded) apply(m Model) (Model, tea.Cmd) {
	previous, _ := m.changes.current()
	m.changes = changeList{changes: msg.changes, loaded: true, err: msg.err, selected: 0}

	index := slices.IndexFunc(msg.changes, func(change gitrepo.Change) bool { return change.Path == previous.Path })
	m.changes.selected = max(0, min(index, len(msg.changes)-1))
	m.changes = m.changes.following(m.detailRows())

	// Staging an untracked file, or an edit behind a refresh, changes a file's
	// diff without changing its path, so drop the loaded one to force a fresh
	// read rather than trust loadDiff's path guard.
	m.diff = diffState{}

	return m, m.loadDiff()
}

// refreshCommits reads the work tree again, and the branch and the hooks with
// it.
func (m Model) refreshCommits() (Model, tea.Cmd) {
	read := m.loadChanges()
	m.changes.loading = read != nil

	return m, tea.Batch(read, m.loadBranch(), m.findHooks())
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

// following is the list scrolled so its selection shows in rows lines.
func (l changeList) following(rows int) changeList {
	l.scroll, _ = window(l.selected, len(l.changes), rows)

	return l
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
		return m.marks.reading()
	case m.changes.err != nil:
		return m.kit().unreadRow(m.changes.err, "status failed")
	}

	counts := strconv.Itoa(m.changes.staged()) + " of " + strconv.Itoa(len(m.changes.changes)) + " staged"

	return counts + "\n" + m.styles.label.Render(plural(len(m.branch.branch.Commits), "commit")+" on this branch")
}

// commitsDetail lists the changed files, then the branch's commits.
func (m Model) commitsDetail(width int) string {
	if m.outsideRepository() {
		return m.kit().failureBlock(m.branch.err, width)
	}

	if !m.changes.loaded {
		return m.commitsRail(0)
	}

	if m.changes.err != nil {
		return m.kit().failureBlock(m.changes.err, width)
	}

	lines := m.changeRows()
	if len(lines) == 0 {
		lines = []string{"Nothing has changed."}
	}

	commits := m.branch.branch.Commits
	if len(commits) > 0 {
		lines = append(lines, "", m.styles.strong.Render("On this branch"))
	}

	for _, commit := range commits {
		lines = append(lines, m.styles.label.Render(commit.Hash)+" "+commit.Subject)
	}

	if len(m.hookgen.hooks) > 0 {
		lines = append(lines, "", m.styles.label.Render("A hook is not managed by lefthook. Press "+
			m.keys.hookConfig.Help().Key+" to set up lefthook."))
	}

	lines = append(lines, m.diffSection(width)...)

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

// commitsOffers are the Commits pane's keys, as far as the changes and the
// seams allow: moving changes into and out of the index and dropping one,
// turning the staged ones into a commit, an amend or a fixup, running the
// hooks, offering lefthook for the repository's own, and reading it all again.
// Outside a repository there is nothing to act on, and none is offered.
func (m Model) commitsOffers() []offer {
	if m.outsideRepository() {
		return nil
	}

	return slices.Concat(m.stagingOffers(), m.committingOffers(), []offer{
		{binding: m.keys.runHooks, can: m.deps.Hooks.Run != nil, act: m.runPreCommit},
		{binding: m.keys.hookConfig, can: len(m.hookgen.hooks) > 0, act: m.openHookgen},
		{binding: m.keys.refresh, can: true, act: func() (Model, tea.Cmd) { return m.refreshPane(paneCommits) }},
	})
}

// commitsKeys is the Commits pane's footer: the offers that act right now.
func (m Model) commitsKeys() []key.Binding {
	return liveKeys(m.commitsOffers())
}

// stagingOffers are moving the changes into and out of the index, and
// dropping the selected one.
func (m Model) stagingOffers() []offer {
	_, hasChange := m.changes.current()
	stage, unstage := m.deps.Git.Stage, m.deps.Git.Unstage

	return []offer{
		{binding: m.keys.stage, can: hasChange && stage != nil && unstage != nil, act: m.toggleStaged},
		{binding: m.keys.stageAll, can: len(loop.Stageable(m.changes.changes)) > 0 && stage != nil, act: m.stageAll},
		{binding: m.keys.unstageAll, can: m.changes.staged() > 0 && unstage != nil, act: m.unstageAll},
		{binding: m.keys.discard, can: hasChange && m.deps.Git.Discard != nil, act: m.previewDiscard},
	}
}

// committingOffers are turning the staged changes into a new commit, which
// says what it needs when nothing is staged, or folding them into an unpushed
// one: amended into the last, or fixed up into a chosen one.
func (m Model) committingOffers() []offer {
	commit := offer{
		binding: m.keys.commit, can: m.changes.staged() > 0 && m.deps.Git.Commit != nil, act: m.openCommitComposer,
	}
	if m.deps.Git.Commit != nil {
		commit.refusal = loop.RefuseNothingStaged(m.changes.changes)
	}

	foldable := m.canFoldStaged()

	return []offer{
		commit,
		{binding: m.keys.amend, can: foldable && m.deps.Git.Amend != nil, act: m.startAmend},
		{binding: m.keys.fixup, can: foldable && m.deps.Git.Fixup != nil, act: m.openFixupPicker},
	}
}

// handleCommitsKey answers the Commits pane's own keys: moving through the
// changes, and what its footer offers.
func (m Model) handleCommitsKey(msg tea.KeyPressMsg) (Model, tea.Cmd) {
	if key.Matches(msg, m.keys.up, m.keys.down) {
		return m.moveChangeBy(m.keys.stepOf(msg))
	}

	return m.answer(m.commitsOffers(), msg)
}

// moveChangeBy moves the selection delta files down, or up for a negative
// delta, stopping at either end, keeps it on screen, and reads the newly
// selected file's diff.
func (m Model) moveChangeBy(delta int) (Model, tea.Cmd) {
	m.changes.selected = max(0, min(m.changes.selected+delta, len(m.changes.changes)-1))

	m.changes = m.changes.following(m.detailRows())

	return m, m.loadDiff()
}

// pickChange selects the file on a clicked line of the detail.
func (m Model) pickChange(line, _ int, inRail bool) (Model, tea.Cmd) {
	index, drawn := m.detailLineAt(line)
	if inRail || !drawn || index >= len(m.changes.changes) {
		return m, nil
	}

	m.changes.selected = index

	return m, m.loadDiff()
}

// toggleStaged stages the selected file, or unstages it if it is wholly staged.
func (m Model) toggleStaged() (Model, tea.Cmd) {
	change, _ := m.changes.current()
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

// stageAll stages every file with changes not yet staged, by loop's rule, so
// every surface that stages all takes the same files.
func (m Model) stageAll() (Model, tea.Cmd) {
	pending := loop.Stageable(m.changes.changes)
	if m.dryRun {
		return m.noticed("dry run: would stage " + plural(len(pending), "file")), nil
	}

	stage := m.deps.Git.Stage

	return m, func() tea.Msg { return staged{err: loop.StageAll(pending, stage)} }
}

// unstageAll takes every staged change out of the index, by loop's rule, at
// once: it only undoes staging, which space or a can redo.
func (m Model) unstageAll() (Model, tea.Cmd) {
	count := m.changes.staged()
	if m.dryRun {
		return m.noticed("dry run: would unstage " + plural(count, "file")), nil
	}

	changes, unstage := m.changes.changes, m.deps.Git.Unstage

	return m, func() tea.Msg { return staged{err: loop.UnstageAll(changes, unstage)} }
}

// staged reports how staging went.
type staged struct {
	err error
}

var _ applier = staged{}

// apply reads the status again, which is the only way to know what the index
// now holds.
func (msg staged) apply(m Model) (Model, tea.Cmd) {
	if msg.err != nil {
		m = m.noticedFailure(msg.err)
	}

	return m, m.loadChanges()
}

// runPreCommit runs the pre-commit hook on what is staged, without committing.
func (m Model) runPreCommit() (Model, tea.Cmd) {
	if m.dryRun {
		return m.noticed("dry run: would run the " + preCommit + " hook"), nil
	}

	run := m.deps.Hooks.Run

	return m.startRun(preCommitRun(), func() (proc.Output, error) { return run(preCommit) }, nil)
}

// preCommitRun is a run of the pre-commit hook alone, which a check can fail.
func preCommitRun() runKind {
	return runKind{title: preCommit, refusal: "the " + preCommit + " hook failed"}
}

// commitsBehavior is the Commits pane's behavior.
func commitsBehavior() behavior {
	return behavior{
		rail: Model.commitsRail, detail: Model.commitsDetail, narrow: nil,
		keys: Model.commitsKeys, handle: Model.handleCommitsKey, pick: Model.pickChange, move: Model.moveChangeBy,
		refresh: Model.refreshCommits, loading: func(m Model) bool { return m.changes.loading },
		scroll: func(m *Model) *int { return &m.changes.scroll }, listInDetail: true, readsBranch: true,
		answers: []string{
			"stage", "stage-all", "unstage-all", "discard-change", "commit", "amend", "fixup", "run-pre-commit",
			"set-up-lefthook", actionRefresh,
		},
	}
}
