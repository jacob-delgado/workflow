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

// errNoMergeMethod reports a repository that permits no way to merge.
var errNoMergeMethod = errors.New("cannot merge: the repository permits no merge method")

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

// startMerge opens the merge preview at once and reads which merge methods the
// repository permits into it. Reading the methods is not a write, so it runs
// even in a dry run; the merge itself waits for the preview to be confirmed.
func (m Model) startMerge() (Model, tea.Cmd) {
	if !m.canMerge() || m.deps.Forge.MergeMethods == nil {
		return m, nil
	}

	m.overlay = mergePicker{marks: m.marks, styles: m.styles, vocab: m.vocab, pull: m.review.pull}
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

// apply fills the open preview with the permitted methods, or pins why it
// cannot: a read the token could not make, or a repository that permits none.
// It does nothing once the preview has closed, or once it has been filled — a
// preview closed and reopened has two reads out — so a choice being made there
// is kept.
func (msg mergeMethodsLoaded) apply(m Model) (Model, tea.Cmd) {
	picker, open := m.overlay.(mergePicker)
	if !open || picker.settled {
		return m, nil
	}

	switch {
	case msg.err != nil:
		picker.listErr = msg.err
	case len(msg.methods) == 0:
		picker.listErr = errNoMergeMethod
	default:
		picker.methods = msg.methods
	}

	picker.settled = true
	m.overlay = picker

	return m, nil
}

// mergeRequested is the outcome of merging a pull request.
type mergeRequested struct {
	pull forge.PullRequest
	err  error
}

// apply reports a merge and refreshes the pane, or keeps the preview open with
// why the merge was refused, pinned under its title until esc.
func (msg mergeRequested) apply(m Model) (Model, tea.Cmd) {
	if msg.err != nil {
		return keepOpenWith[mergePicker](m, writeRefusal(msg.err)), nil
	}

	merged := m.closeOverlay().noticed(m.marks.done + " merged " + m.vocab.sigil + strconv.Itoa(msg.pull.Number))

	return merged, merged.findPullRequest()
}

// failed is the preview kept open with the reason the merge was refused.
func (p mergePicker) failed(err error) mergePicker {
	p.send = p.send.failed(err)

	return p
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
	styles   styles
	vocab    reviewVocab
	pull     forge.PullRequest
	methods  []forge.MergeMethod
	listErr  error
	settled  bool
	selected int
	send     sendState
}

var _ failable[mergePicker] = mergePicker{}

// view draws the pull request and the methods it may be merged by, the merge's
// outcome pinned under the title.
func (p mergePicker) view(width, _ int) (string, string) {
	lines := pinnedOutcome(p.styles, p.marks, p.send, "merging", width)
	lines = append(lines, p.vocab.sigil+strconv.Itoa(p.pull.Number)+" "+p.pull.Title, "")

	switch {
	case !p.settled:
		lines = append(lines, "loading merge methods"+p.marks.ellipsis)
	case p.listErr != nil:
		lines = append(lines, failureLine(p.styles, p.marks, p.listErr))
	default:
		lines = append(lines, "Merge by:")
		for index, method := range p.methods {
			lines = append(lines, p.marks.marker(index == p.selected)+mergeMethodLabel(method))
		}
	}

	return "Merge " + p.vocab.noun, strings.Join(lines, "\n")
}

// footer offers moving between the methods, merging, and leaving; only leaving
// while there is no method to choose.
func (p mergePicker) footer(keys keyMap) []key.Binding {
	cancel := relabel(keys.closeOverlay, "cancel")

	switch {
	case p.send.sending:
		return []key.Binding{keys.interrupt}
	case len(p.methods) == 0:
		return []key.Binding{cancel}
	default:
		return []key.Binding{keys.up, keys.down, relabel(keys.confirm, "merge"), cancel}
	}
}

// handleKey answers a key while the merge is being previewed. With no method to
// choose — still being read, or none permitted — only esc does anything.
func (p mergePicker) handleKey(m Model, msg tea.KeyPressMsg) (Model, tea.Cmd) {
	switch {
	case p.send.sending:
		return m, nil
	case key.Matches(msg, m.keys.closeOverlay):
		return m.closeOverlay(), nil
	case len(p.methods) == 0:
		return m, nil
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

	p.send = starting()
	m.overlay = p
	pull, merge := p.pull, m.deps.Forge.Merge

	return m, func() tea.Msg {
		return mergeRequested{pull: pull, err: merge(pull, method)}
	}
}
