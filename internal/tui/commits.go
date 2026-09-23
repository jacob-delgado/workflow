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

	// A reload can return fewer files, leaving the shared scroll offset past the
	// end; re-clamp it so a click still lands on the row it appears to. Only while
	// this pane is focused, since the offset is shared and this reload may arrive
	// from a background stage while another pane is being read.
	if m.focus == paneCommits {
		m.scroll, _ = window(m.changes.selected, len(m.changes.changes), m.detailRows())
	}

	// Staging an untracked file, or an edit behind a refresh, changes a file's
	// diff without changing its path, so drop the loaded one to force a fresh
	// read rather than trust loadDiff's path guard.
	m.diff = diffState{}

	return m, m.loadDiff()
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
		return m.failedGlyph() + " status failed" + m.marks.separator + "see detail"
	}

	counts := strconv.Itoa(m.changes.staged()) + " of " + strconv.Itoa(len(m.changes.changes)) + " staged"

	return counts + "\n" + m.styles.label.Render(plural(len(m.branch.branch.Commits), "commit")+" on this branch")
}

// commitsDetail lists the changed files, then the branch's commits.
func (m Model) commitsDetail(width int) string {
	if m.outsideRepository() {
		return wrap(notInRepository, width)
	}

	if !m.changes.loaded {
		return m.commitsRail(0)
	}

	if m.changes.err != nil {
		return m.failureBlock(m.changes.err, width)
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

	if len(m.hookgen.hooks) > 0 {
		lines = append(lines, "", m.styles.label.Render("A hook is not managed by lefthook. Press g to set up lefthook."))
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

	keys = append(keys, m.foldKeys()...)

	if m.deps.Hooks.Run != nil {
		keys = append(keys, m.keys.runHooks)
	}

	if len(m.hookgen.hooks) > 0 {
		keys = append(keys, m.keys.hookConfig)
	}

	return append(keys, m.keys.refresh)
}

// handleCommitsKey answers the Commits pane's own keys.
func (m Model) handleCommitsKey(msg tea.KeyPressMsg) (Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.up, m.keys.down):
		return m.moveChangeSelection(msg)
	case key.Matches(msg, m.keys.stage):
		return m.toggleStaged()
	case key.Matches(msg, m.keys.stageAll):
		return m.stageAll()
	case key.Matches(msg, m.keys.commit, m.keys.amend, m.keys.fixup):
		return m.handleCommitAction(msg)
	case key.Matches(msg, m.keys.runHooks) && m.deps.Hooks.Run != nil:
		return m.runPreCommit()
	case key.Matches(msg, m.keys.hookConfig) && len(m.hookgen.hooks) > 0:
		return m.openHookgen()
	case key.Matches(msg, m.keys.refresh):
		return m, tea.Batch(m.loadChanges(), m.loadBranch())
	}

	return m, nil
}

// moveChangeSelection moves the selection down or up, keeps it on screen, and
// reads the newly selected file's diff.
func (m Model) moveChangeSelection(msg tea.KeyPressMsg) (Model, tea.Cmd) {
	if key.Matches(msg, m.keys.down) {
		m.changes.selected = min(m.changes.selected+1, max(0, len(m.changes.changes)-1))
	} else {
		m.changes.selected = max(0, m.changes.selected-1)
	}

	m = m.followChange()

	return m, m.loadDiff()
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

	return m, m.loadDiff()
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

// stageAll stages every file with changes not yet staged, by loop's rule, so
// every surface that stages all takes the same files.
func (m Model) stageAll() (Model, tea.Cmd) {
	pending := loop.Stageable(m.changes.changes)
	if len(pending) == 0 || m.deps.Git.Stage == nil {
		return m, nil
	}

	if m.dryRun {
		return m.noticed("dry run: would stage " + plural(len(pending), "file")), nil
	}

	stage := m.deps.Git.Stage

	return m, func() tea.Msg { return staged{err: loop.StageAll(pending, stage)} }
}

// staged reports how staging went.
type staged struct {
	err error
}

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

	return m.startRun(preCommit, func() (proc.Output, error) { return run(preCommit) }, nil)
}

// canFoldStaged reports whether the staged changes can go into an unpushed
// commit — amended into the last, or fixed up into a chosen one — which is safe
// only while those commits are local.
func (m Model) canFoldStaged() bool {
	return m.changes.staged() > 0 && len(m.branch.branch.Unpushed()) > 0
}

// foldKeys offers amending and fixing up, when there are staged changes and an
// unpushed commit to fold them into.
func (m Model) foldKeys() []key.Binding {
	if !m.canFoldStaged() {
		return nil
	}

	var keys []key.Binding

	if m.deps.Git.Amend != nil {
		keys = append(keys, m.keys.amend)
	}

	if m.deps.Git.Fixup != nil {
		keys = append(keys, m.keys.fixup)
	}

	return keys
}

// handleCommitAction routes the keys that turn staged changes into a commit: a
// new one, an amend of the last, or a fixup of a chosen one.
func (m Model) handleCommitAction(msg tea.KeyPressMsg) (Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.commit):
		return m.openCommitComposer()
	case key.Matches(msg, m.keys.amend):
		return m.startAmend()
	default:
		return m.openFixupPicker()
	}
}

// startAmend previews folding the staged changes into the last commit.
func (m Model) startAmend() (Model, tea.Cmd) {
	if m.deps.Git.Amend == nil || !m.canFoldStaged() {
		return m, nil
	}

	unpushed := m.branch.branch.Unpushed()
	m.overlay = amendPreview{subject: unpushed[len(unpushed)-1].Subject}

	return m, nil
}

// applyAmend folds the staged changes into the last commit, hooks and all.
func (m Model) applyAmend(subject string) (Model, tea.Cmd) {
	if m.dryRun {
		return m.closeOverlay().noticed("dry run: would amend " + subject), nil
	}

	amend := m.deps.Git.Amend

	return m.startRun("git commit --amend", amend, func(done Model) (Model, tea.Cmd) {
		done = done.closeOverlay().noticed(done.marks.done + " amended " + subject)

		return done, tea.Batch(done.loadChanges(), done.loadBranch())
	})
}

// applyFixup records a fixup! of the chosen commit, hooks and all.
func (m Model) applyFixup(hash, subject string) (Model, tea.Cmd) {
	if m.dryRun {
		return m.closeOverlay().noticed("dry run: would fix up " + subject), nil
	}

	fixup := m.deps.Git.Fixup

	return m.startRun("git commit --fixup", func() (proc.Output, error) { return fixup(hash) },
		func(done Model) (Model, tea.Cmd) {
			done = done.closeOverlay().noticed(done.marks.done + " recorded a fixup! of " + subject)

			return done, tea.Batch(done.loadChanges(), done.loadBranch())
		})
}

// amendPreview confirms folding the staged changes into the last commit.
type amendPreview struct {
	subject string
}

var _ overlay = amendPreview{}

// view describes the amend as it will happen.
func (p amendPreview) view(width, _ int) (string, string) {
	return "Amend the last commit", wrap("fold the staged changes into "+p.subject, width)
}

// footer offers amending or leaving.
func (p amendPreview) footer(keys keyMap) []key.Binding {
	return []key.Binding{relabel(keys.confirm, "amend"), keys.closeOverlay}
}

// handleKey confirms or discards the amend.
func (p amendPreview) handleKey(m Model, msg tea.KeyPressMsg) (Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.closeOverlay):
		return m.closeOverlay(), nil
	case key.Matches(msg, m.keys.confirm):
		return m.applyAmend(p.subject)
	}

	return m, nil
}
