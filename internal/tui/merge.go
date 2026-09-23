// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"cmp"
	"errors"
	"strconv"
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"

	"github.com/jacob-delgado/workflow/internal/forge"
)

// canMerge reports a pull request that can be merged here: found, mergeable,
// green and approved, with a forge that can merge it.
func (m Model) canMerge() bool {
	pull := m.review.pull

	return m.review.found &&
		pull.State == forge.StateOpen &&
		m.deps.Forge.Merge != nil &&
		!pull.Draft &&
		pull.Mergeable == forge.MergeClean &&
		pull.Approvals > 0 &&
		!pull.ChangesRequested &&
		m.review.ci.State == forge.CIPassed
}

// startMerge reads which merge methods the repository permits, then opens the
// merge preview on them. Reading the methods is not a write, so it runs even in
// a dry run; the merge itself waits for the preview to be confirmed.
func (m Model) startMerge() (Model, tea.Cmd) {
	if !m.canMerge() || m.deps.Forge.MergeMethods == nil {
		return m, nil
	}

	methods := m.deps.Forge.MergeMethods

	return m, func() tea.Msg {
		allowed, err := methods()

		return mergeMethodsLoaded{methods: allowed, err: err}
	}
}

// mergeMethodsLoaded is the merge methods a repository permits, for the preview.
type mergeMethodsLoaded struct {
	methods []forge.MergeMethod
	err     error
}

// apply opens the merge preview on the permitted methods, or says why it
// cannot: a read the token could not make, or a repository that permits none.
func (msg mergeMethodsLoaded) apply(m Model) (Model, tea.Cmd) {
	switch {
	case msg.err != nil:
		return m.noticed("cannot merge: " + forgeReason(msg.err)), nil
	case len(msg.methods) == 0:
		return m.noticed("cannot merge: the repository permits no merge method"), nil
	}

	m.overlay = mergePicker{marks: m.marks, vocab: m.vocab, pull: m.review.pull, methods: msg.methods}

	return m, nil
}

// mergeRequested is the outcome of merging a pull request.
type mergeRequested struct {
	pull forge.PullRequest
	err  error
}

// apply reports a merge and refreshes the pane, or closes the preview with why
// the merge was refused.
func (msg mergeRequested) apply(m Model) (Model, tea.Cmd) {
	if msg.err != nil {
		return m.closeOverlay().noticed("could not merge: " + mergeReason(msg.err)), nil
	}

	merged := m.closeOverlay().noticed(m.marks.done + " merged " + m.vocab.sigil + strconv.Itoa(msg.pull.Number))

	return merged, merged.findPullRequest()
}

// mergeReason names why a merge was refused, spelling out the write scope a
// read-only token lacks so the fix is plain.
func mergeReason(err error) string {
	if errors.Is(err, forge.ErrRefused) || errors.Is(err, forge.ErrUnauthorized) {
		return "the token needs a write scope the read path does not"
	}

	return forgeReason(err)
}

// mergeMethodLabel names a merge method for the preview.
func mergeMethodLabel(method forge.MergeMethod) string {
	labels := map[forge.MergeMethod]string{
		forge.MergeCommit: "merge commit",
		forge.MergeSquash: "squash and merge",
		forge.MergeRebase: "rebase and merge",
	}

	return cmp.Or(labels[method], string(method))
}

// mergePicker previews merging a pull request: which of the permitted methods
// to use, sent only once it is confirmed.
type mergePicker struct {
	marks    glyphs
	vocab    reviewVocab
	pull     forge.PullRequest
	methods  []forge.MergeMethod
	selected int
	merging  bool
}

var _ overlay = mergePicker{}

// view draws the pull request and the methods it may be merged by.
func (p mergePicker) view(_, _ int) (string, string) {
	lines := []string{p.vocab.sigil + strconv.Itoa(p.pull.Number) + " " + p.pull.Title, "", "Merge by:"}

	for index, method := range p.methods {
		lines = append(lines, p.marks.marker(index == p.selected)+mergeMethodLabel(method))
	}

	if p.merging {
		lines = append(lines, "", "merging…")
	}

	return "Merge " + p.vocab.noun, strings.Join(lines, "\n")
}

// footer offers moving between the methods, merging, and leaving.
func (p mergePicker) footer(keys keyMap) []key.Binding {
	if p.merging {
		return []key.Binding{keys.interrupt}
	}

	return []key.Binding{keys.up, keys.down, relabel(keys.confirm, "merge"), relabel(keys.closeOverlay, "cancel")}
}

// handleKey answers a key while the merge is being previewed.
func (p mergePicker) handleKey(m Model, msg tea.KeyPressMsg) (Model, tea.Cmd) {
	switch {
	case p.merging:
		return m, nil
	case key.Matches(msg, m.keys.closeOverlay):
		return m.closeOverlay(), nil
	case key.Matches(msg, m.keys.confirm):
		return p.confirm(m)
	case key.Matches(msg, m.keys.down):
		p.selected = min(p.selected+1, len(p.methods)-1)
	case key.Matches(msg, m.keys.up):
		p.selected = max(0, p.selected-1)
	}

	m.overlay = p

	return m, nil
}

// confirm merges by the chosen method, or, in a dry run, says what it would do.
func (p mergePicker) confirm(m Model) (Model, tea.Cmd) {
	method := p.methods[p.selected]

	if m.dryRun {
		return m.closeOverlay().noticed("dry run: would merge " + p.vocab.sigil +
			strconv.Itoa(p.pull.Number) + " by " + mergeMethodLabel(method)), nil
	}

	p.merging = true
	m.overlay = p
	pull, merge := p.pull, m.deps.Forge.Merge

	return m, func() tea.Msg {
		return mergeRequested{pull: pull, err: merge(pull, method)}
	}
}
