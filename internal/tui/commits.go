// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"context"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"

	"github.com/jacob-delgado/workflow/internal/gitrepo"
	"github.com/jacob-delgado/workflow/internal/loop"
	"github.com/jacob-delgado/workflow/internal/sanitize"
	"github.com/jacob-delgado/workflow/internal/tui/latest"
)

// changeList is the work tree's changed files, as the latest read of its
// status found them, and how far the Commits pane's detail is scrolled.
type changeList struct {
	read     latest.Read[[]gitrepo.Change]
	selected int
	scroll   int
}

// changesLoaded carries the work tree's status, as the read begun in round
// found it.
type changesLoaded struct {
	changes []gitrepo.Change
	err     error
	round   latest.Round
}

var _ applier = changesLoaded{}

// apply records the changes, keeping the selection on the same file. The
// answer of a read a later one superseded is dropped: it would show the work
// tree as it stood before. A failed refresh keeps the files listed, beside
// its failure.
func (msg changesLoaded) apply(m Model) (Model, tea.Cmd) {
	read, taken := m.changes.read.Answered(msg.round, msg.changes, msg.err)
	if !taken {
		return m, nil
	}

	previous, _ := m.changes.current()
	m.changes.read = read
	files := m.changes.files()

	index := slices.IndexFunc(files, func(change gitrepo.Change) bool { return change.Path == previous.Path })
	m.changes.selected = max(0, min(index, len(files)-1))
	m.changes = m.changes.following(m.detailRows())

	// Staging an untracked file, or an edit behind a refresh, changes a file's
	// diff without changing its path, so drop the loaded one to force a fresh
	// read rather than trust loadDiff's path guard.
	m.diff = diffState{}

	return m.loadDiff()
}

// readingChanges begins a read of the work tree's status, superseding any
// still out, and is the command that reads it; with no git to ask, it begins
// nothing.
func (m Model) readingChanges() (Model, tea.Cmd) {
	if m.deps.Git.Changes == nil {
		return m, nil
	}

	read, round := m.changes.read.Begin()
	m.changes.read = read

	return m, loadChanges(m.session, m.deps, round)
}

// loadChanges is the command that reads the work tree's status, in round.
func loadChanges(ctx context.Context, deps Deps, round latest.Round) tea.Cmd {
	status := deps.Git.Changes
	if status == nil {
		return nil
	}

	return func() tea.Msg {
		changes, err := status(ctx)

		return changesLoaded{changes: changes, err: err, round: round}
	}
}

// files is the changed files held: the latest successful read's, or none.
func (l changeList) files() []gitrepo.Change {
	files, _ := l.read.Value()

	return files
}

// writable reports files listed as the latest read found them, which a write
// can act on: none while a first read is out, and none after a read failed,
// since the files still listed may have changed since.
func (l changeList) writable() bool {
	return l.read.Stage() == latest.StageShown
}

// following is the list scrolled so its selection shows in rows lines.
func (l changeList) following(rows int) changeList {
	l.scroll, _ = window(l.selected, len(l.files()), rows)

	return l
}

// current is the selected change, if there is one.
func (l changeList) current() (gitrepo.Change, bool) {
	files := l.files()
	if len(files) == 0 {
		return gitrepo.Change{}, false
	}

	return files[l.selected], true
}

// staged counts the changes in the index.
func (l changeList) staged() int {
	count := 0

	for _, change := range l.files() {
		if change.IsStaged() {
			count++
		}
	}

	return count
}

// rail counts what is staged and changed, and the branch's commits; after a
// failed read, the failure, then the counts of the files still held.
func (l changeList) rail(kit renderKit, commits int) string {
	failed := func() string { return kit.unreadRow(l.read.Err(), "status failed") }

	return drawRead(kit, l.read, drawnRead{
		shown: func() string {
			return l.counts() + "\n" + kit.styles.label.Render(plural(commits, "commit")+" on this branch")
		},
		failed:     failed,
		failedOver: func() string { return failed() + "\n" + l.counts() },
	})
}

// counts is how many of the files held are staged.
func (l changeList) counts() string {
	return strconv.Itoa(l.staged()) + " of " + strconv.Itoa(len(l.files())) + " staged"
}

// commitsDetail lists the changed files, then the branch's commits; a failed
// read of the status is said beneath the files it kept.
func (m Model) commitsDetail(width int) string {
	if m.branch.outsideRepository() {
		return m.kit().failureBlock(m.branch.err, width)
	}

	failure := func() string { return m.kit().failureBlock(m.changes.read.Err(), width) }

	return drawRead(m.kit(), m.changes.read, drawnRead{
		shown:      func() string { return m.commitsListing(width, nil) },
		failed:     failure,
		failedOver: func() string { return m.commitsListing(width, []string{failure()}) },
	})
}

// commitsListing is the changed files held, failure under them when a read
// of them failed since, then the branch's commits, the lefthook offer and the
// selected file's diff.
func (m Model) commitsListing(width int, failure []string) string {
	lines := m.changes.rows(m.marks)
	if len(lines) == 0 {
		lines = []string{"Nothing has changed."}
	}

	lines = append(lines, failure...)

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

	if change, selected := m.changes.current(); selected && m.deps.Git.Diff != nil {
		lines = append(lines, m.diff.section(m.kit(), change, width)...)
	}

	return strings.Join(lines, "\n")
}

// rows draws each changed file: where it stands, git's two letters, and its
// path — neutralized, because a file name can hold an escape sequence.
func (l changeList) rows(marks glyphs) []string {
	files := l.files()
	rows := make([]string, 0, len(files))

	for index, change := range files {
		path := sanitize.Line(change.Path)
		if change.OriginalPath != "" {
			path = sanitize.Line(change.OriginalPath) + marks.arrow + path
		}

		rows = append(rows, marks.marker(index == l.selected)+stageGlyph(marks, change)+" "+
			fmt.Sprintf("%-11s", change.Kind())+path)
	}

	return rows
}

// stageGlyph says by shape how much of a change is staged.
func stageGlyph(marks glyphs, change gitrepo.Change) string {
	switch {
	case change.Conflicted():
		return marks.failed
	case change.IsStaged() && change.HasUnstaged():
		return marks.inFlight
	case change.IsStaged():
		return marks.done
	default:
		return marks.notStarted
	}
}

// commitsOffers are the Commits pane's keys, as far as the changes and the
// seams allow: moving changes into and out of the index and dropping one,
// turning the staged ones into a commit, an amend or a fixup, running one of
// lefthook's hooks, offering lefthook for the repository's own, and reading it
// all again.
// Outside a repository there is nothing to act on, and none is offered.
func (m Model) commitsOffers() []offer {
	if m.branch.outsideRepository() {
		return nil
	}

	return slices.Concat(m.stagingOffers(), m.committingOffers(), []offer{
		{binding: m.keys.runHook, can: m.deps.Hooks.Run != nil && m.deps.Hooks.Configured != nil, act: m.openHookPicker},
		{binding: m.keys.hookConfig, can: len(m.hookgen.hooks) > 0, act: m.openHookgen},
		{binding: m.keys.refresh, can: true, act: func() (Model, tea.Cmd) { return m.refreshPane(paneCommits) }},
	})
}

// stagingOffers are moving the changes into and out of the index, and
// dropping the selected one.
func (m Model) stagingOffers() []offer {
	_, hasChange := m.changes.current()
	hasChange = hasChange && m.changes.writable()
	stage, unstage := m.deps.Git.Stage, m.deps.Git.Unstage
	stageable := m.changes.writable() && len(loop.Stageable(m.changes.files())) > 0
	unstageable := m.changes.writable() && m.changes.staged() > 0

	return []offer{
		{binding: m.keys.stage, can: hasChange && stage != nil && unstage != nil, act: m.toggleStaged},
		{binding: m.keys.stageAll, can: stageable && stage != nil, act: m.stageAll},
		{binding: m.keys.unstageAll, can: unstageable && unstage != nil, act: m.unstageAll},
		{binding: m.keys.discard, can: hasChange && m.deps.Git.Discard != nil, act: m.previewDiscard},
	}
}

// committingOffers are turning the staged changes into a new commit, which
// says what it needs when the files listed are current and none is staged,
// or folding them into an unpushed one: amended into the last, or fixed up
// into a chosen one.
func (m Model) committingOffers() []offer {
	writable := m.changes.writable()

	commit := offer{
		binding: m.keys.commit, can: writable && m.changes.staged() > 0 && m.deps.Git.Commit != nil,
		act: m.openCommitComposer,
	}
	if writable && m.deps.Git.Commit != nil {
		commit.refusal = loop.RefuseNothingStaged(m.changes.files())
	}

	foldable := writable && m.changes.canFold(m.branch.branch)

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
	m.changes.selected = max(0, min(m.changes.selected+delta, len(m.changes.files())-1))

	m.changes = m.changes.following(m.detailRows())

	return m.loadDiff()
}

// pickChange selects the file on a clicked line of the detail.
func (m Model) pickChange(line, _ int, inRail bool) (Model, tea.Cmd) {
	index, drawn := m.detailLineAt(line)
	if inRail || !drawn || index >= len(m.changes.files()) {
		return m, nil
	}

	m.changes.selected = index

	return m.loadDiff()
}

// toggleStaged stages the selected file, or unstages it if it is wholly staged.
func (m Model) toggleStaged() (Model, tea.Cmd) {
	change, _ := m.changes.current()
	unstage := change.IsStaged() && !change.HasUnstaged()

	ctx, verb, act := m.session, "stage ", m.deps.Git.Stage
	if unstage {
		verb, act = "unstage ", m.deps.Git.Unstage
	}

	if m.dryRun {
		return m.noticed("dry run: would " + verb + sanitize.Line(change.Path)), nil
	}

	m.paneWrites.staging++

	return m, func() tea.Msg { return staged{err: act(ctx, change)} }
}

// stageAll stages every file with changes not yet staged, by loop's rule, so
// every surface that stages all takes the same files.
func (m Model) stageAll() (Model, tea.Cmd) {
	pending := loop.Stageable(m.changes.files())
	if m.dryRun {
		return m.noticed("dry run: would stage " + plural(len(pending), "file")), nil
	}

	ctx, stage := m.session, m.deps.Git.Stage
	m.paneWrites.staging++

	return m, func() tea.Msg { return staged{err: loop.StageAll(ctx, pending, stage)} }
}

// unstageAll takes every staged change out of the index, by loop's rule, at
// once: it only undoes staging, which space or a can redo.
func (m Model) unstageAll() (Model, tea.Cmd) {
	count := m.changes.staged()
	if m.dryRun {
		return m.noticed("dry run: would unstage " + plural(count, "file")), nil
	}

	ctx, changes, unstage := m.session, m.changes.files(), m.deps.Git.Unstage
	m.paneWrites.staging++

	return m, func() tea.Msg { return staged{err: loop.UnstageAll(ctx, changes, unstage)} }
}

// staged reports how staging went.
type staged struct {
	err error
}

var _ applier = staged{}

// apply reads the status again, which is the only way to know what the index
// now holds.
func (msg staged) apply(m Model) (Model, tea.Cmd) {
	m.paneWrites.staging--

	if msg.err != nil {
		m = m.noticedFailure(msg.err)
	}

	return m.readingChanges()
}

// commitsBehavior is the Commits pane's behavior.
func commitsBehavior() behavior {
	return behavior{
		rail:   func(m Model, _ int) string { return m.changes.rail(m.kit(), len(m.branch.branch.Commits)) },
		detail: Model.commitsDetail, narrow: nil,
		keys:   func(m Model) []key.Binding { return liveKeys(m.commitsOffers()) },
		handle: Model.handleCommitsKey, pick: Model.pickChange, move: Model.moveChangeBy,
		// refresh reads the work tree again, and the branch and the hooks with
		// it.
		refresh: func(m Model) (Model, tea.Cmd) {
			m, read := m.readingChanges()

			return m, tea.Batch(read, loadBranch(m.session, m.deps), m.hookgen.find(m.session, m.deps))
		},
		loading: func(m Model) bool { return m.changes.read.Refreshing() },
		scroll:  func(m *Model) *int { return &m.changes.scroll }, listInDetail: true, readsBranch: true,
		answers: []string{
			"stage", "stage-all", "unstage-all", "discard-change", "commit", "amend", "fixup", "run-hook",
			"set-up-lefthook", actionRefresh,
		},
	}
}
