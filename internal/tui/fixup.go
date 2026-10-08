// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"slices"
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"

	"github.com/jacob-delgado/workflow/internal/gitrepo"
	"github.com/jacob-delgado/workflow/internal/loop"
	"github.com/jacob-delgado/workflow/internal/proc"
)

// canFoldStaged reports whether the staged changes can go into an unpushed
// commit — amended into the last, or fixed up into a chosen one — which is safe
// only while those commits are local.
func (m Model) canFoldStaged() bool {
	return len(loop.Foldable(m.changes.changes, m.branch.branch)) > 0
}

// startAmend previews folding the staged changes into the last commit.
func (m Model) startAmend() (Model, tea.Cmd) {
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

	return m.startRun(amendRun(), amend, func(done Model) (Model, tea.Cmd) {
		done = done.closeOverlay().noticed(done.marks.done + " amended " + subject)

		return done, tea.Batch(done.loadChanges(), done.loadBranch())
	})
}

// amendRun is an amend of the last commit, which the hooks can refuse.
func amendRun() runKind {
	return runKind{title: "git commit --amend", refusal: "the amend was refused"}
}

// applyFixup records a fixup! of the chosen commit, hooks and all.
func (m Model) applyFixup(hash, subject string) (Model, tea.Cmd) {
	if m.dryRun {
		return m.closeOverlay().noticed("dry run: would fix up " + subject), nil
	}

	fixup := m.deps.Git.Fixup

	return m.startRun(fixupRun(), func() (proc.Output, error) { return fixup(hash) },
		func(done Model) (Model, tea.Cmd) {
			done = done.closeOverlay().noticed(done.marks.done + " recorded a fixup! of " + subject)

			return done, tea.Batch(done.loadChanges(), done.loadBranch())
		})
}

// fixupRun is a fixup! of a chosen commit, which the hooks can refuse.
func fixupRun() runKind {
	return runKind{title: "git commit --fixup", refusal: "the fixup was refused"}
}

// amendPreview confirms folding the staged changes into the last commit.
type amendPreview struct {
	subject string
}

var _ overlay = amendPreview{}

// view describes the amend as it will happen.
func (p amendPreview) view(_ renderKit, width, _ int) (string, string) {
	return "Amend the last commit", wrap("Fold the staged changes into "+p.subject+"?", width)
}

// footer offers amending or leaving.
func (p amendPreview) footer(keys keyMap) []key.Binding {
	return []key.Binding{relabel(keys.confirm, "amend"), relabel(keys.closeOverlay, escCancel)}
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

// fixupTitle titles the pane while the fixup picker is open.
const fixupTitle = "Fix up a commit"

// fixupPicker chooses which unpushed commit to record a fixup! of.
type fixupPicker struct {
	commits pickList[gitrepo.Commit]
}

var (
	_ overlay   = fixupPicker{}
	_ steppable = fixupPicker{}
)

// openFixupPicker offers the branch's unpushed commits, the most recent first so
// the likeliest target is the default selection.
func (m Model) openFixupPicker() (Model, tea.Cmd) {
	newestFirst := slices.Clone(m.branch.branch.Unpushed())
	slices.Reverse(newestFirst)
	m.overlay = fixupPicker{commits: pickList[gitrepo.Commit]{items: newestFirst}}

	return m, nil
}

// header is the rows above the fixup picker's list: a prompt and a blank.
func (p fixupPicker) header() []string {
	return []string{"Fold the staged changes into which commit?", ""}
}

// view draws the commits to choose from, in as many rows as fit.
func (p fixupPicker) view(kit renderKit, _, rows int) (string, string) {
	lines := p.header()
	lines = append(lines, p.commits.rows(kit.marks, rows-len(lines), func(commit gitrepo.Commit) string {
		return commitRow(kit.styles, commit)
	})...)

	return fixupTitle, strings.Join(lines, "\n")
}

// commitRow names a commit by its hash and subject.
func commitRow(sty styles, commit gitrepo.Commit) string {
	return sty.label.Render(commit.Hash) + " " + commit.Subject
}

// footer offers moving, choosing and leaving.
func (p fixupPicker) footer(keys keyMap) []key.Binding {
	return keys.listKeys()
}

// handleKey moves the selection, chooses a commit to fix up, or leaves.
func (p fixupPicker) handleKey(m Model, msg tea.KeyPressMsg) (Model, tea.Cmd) {
	if listed, answered := m.listKey(p, msg); answered {
		return listed, nil
	}

	if !key.Matches(msg, m.keys.confirm) {
		return m, nil
	}

	// The picker opens only over unpushed commits, so one is always chosen.
	chosen, _ := p.commits.chosen()

	return m.applyFixup(chosen.Hash, chosen.Subject)
}

// step moves the choice of commit by delta.
func (p fixupPicker) step(m Model, delta int) Model {
	p.commits = p.commits.moved(delta)
	m.overlay = p

	return m
}
