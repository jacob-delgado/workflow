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

	"github.com/jacob-delgado/workflow/internal/armed"
	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/tui/latest"
	"github.com/jacob-delgado/workflow/internal/tui/send"
)

// errNoMergeMethod reports a repository that permits no way to merge.
var errNoMergeMethod = errors.New("cannot merge: the repository permits no merge method")

// canMerge reports a pull request that can be merged here: found, mergeable,
// green or with no check reported on its head for armed.NoCIPatience, and
// approved, with a forge that can merge it.
func (s reviewState) canMerge(deps Deps) bool {
	return s.found() && deps.Forge.Merge != nil && s.ciReported() && s.quiet.CanMerge(s.pull(), s.listed())
}

// ciReported reports CI that says what the forge reported: anything but none,
// or none from a read that answered. Before the first read answers, and past
// one that failed, CI reads as none, which CanMerge would take for a
// repository with no CI.
func (s reviewState) ciReported() bool {
	return s.listed().State != forge.CINone || s.ci.Stage() == latest.StageShown
}

// startMerge opens the merge preview at once and reads which merge methods the
// repository permits into it. Reading the methods is not a write, so it runs
// even in a dry run; the merge itself waits for the preview to be confirmed.
func (m Model) startMerge() (Model, tea.Cmd) {
	if !m.review.canMerge(m.deps) || m.deps.Forge.MergeMethods == nil {
		return m, nil
	}

	m, opened := m.nextRound()
	m.overlay = mergePicker{
		vocab: m.vocab, pull: m.review.pull(), noCI: m.review.listed().State == forge.CINone, opened: opened,
	}
	ctx, methods := m.session, m.deps.Forge.MergeMethods

	return m, func() tea.Msg {
		allowed, err := methods(ctx)

		return mergeMethodsLoaded{opened: opened, methods: allowed, err: err}
	}
}

// mergeMethodsLoaded is the merge methods a repository permits, for the
// preview opened in round opened.
type mergeMethodsLoaded struct {
	opened  int
	methods []forge.MergeMethod
	err     error
}

var _ applier = mergeMethodsLoaded{}

// apply fills the preview that asked with the permitted methods, or pins why
// it cannot: a read the token could not make, or a repository that permits
// none. It does nothing once that preview has closed, even when another has
// opened since, whose own read is on its way.
func (msg mergeMethodsLoaded) apply(m Model) (Model, tea.Cmd) {
	picker, open := answerFor[mergePicker](m, msg.opened)
	if !open {
		return m, nil
	}

	err := msg.err
	if err == nil && len(msg.methods) == 0 {
		err = errNoMergeMethod
	}

	// The preview reads its methods once, as it opens, before it could begin
	// a round: the listing is round 0's.
	picker.listing, _ = picker.listing.Answered(0, msg.methods, err)

	return m.withAnswered(picker), nil
}

// mergeRequested is the outcome of merging a pull request, for the preview
// opened in round opened.
type mergeRequested struct {
	opened int
	pull   forge.PullRequest
	err    error
}

var _ applier = mergeRequested{}

// apply reports a merge and refreshes the pane, or keeps the preview open with
// why the merge was refused, pinned under its title until esc or another
// merge sent. A merge for the branch's issue offers to complete its task.
// A late answer, its preview no longer the one open, changes nothing; none
// arrives, since the preview takes no key but ctrl+c, which quits, while its
// merge is out.
func (msg mergeRequested) apply(m Model) (Model, tea.Cmd) {
	m, _, done := answered[mergePicker](m, msg.opened, msg.err)
	if !done {
		return m, nil
	}

	merged := m.closeOverlay().noticed(m.marks.done + " merged " + m.vocab.sigil + strconv.Itoa(msg.pull.Number))
	if issueKey, named := merged.branch.jiraIssue(merged.cfg.Jira.Project); named {
		merged.followUp = merged.offerMarkDone(issueKey)
	}

	return merged.findingPull()
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

// patienceWords is armed.NoCIPatience as the merge preview says it: "10
// minutes".
func patienceWords() string {
	return strconv.Itoa(int(armed.NoCIPatience.Minutes())) + " minutes"
}

// mergePicker previews merging a pull request: which of the permitted methods
// to use, sent only once it is confirmed, and whether no CI was reported on
// it in armed.NoCIPatience.
type mergePicker struct {
	vocab reviewVocab
	pull  forge.PullRequest
	noCI  bool
	// listing is the merge methods the repository permits, read once as the
	// preview opens, a repository that permits none read as failed;
	// selected is which of them is chosen.
	listing  latest.Read[[]forge.MergeMethod]
	selected int
	// send is the merge, whose refusal stays pinned across a move to
	// another method, since a forge refuses the pull request more often
	// than the method, until another merge is sent.
	send send.State
	// opened is the round the preview opened in, which its read and its
	// merge carry back, so they land in no preview opened after.
	opened int
}

var (
	_ sender[mergePicker] = mergePicker{}
	_ steppable           = mergePicker{}
)

// view draws the pull request, that no CI was reported on it where none was,
// and the methods it may be merged by: reading them, why they could not be
// read, or them; the merge's outcome pinned under the title.
func (p mergePicker) view(kit renderKit, width, _ int) (string, string) {
	lines := kit.pinnedSend(p.send, "merging", width)
	name := p.vocab.sigil + strconv.Itoa(p.pull.Number)
	lines = append(lines, name+" "+p.pull.Title, "")

	if p.noCI {
		lines = append(lines, "no CI was reported on "+name+" in "+patienceWords()+
			"; a forge that requires checks refuses the merge", "")
	}

	failed := func() []string { return []string{kit.failureLine(p.listing.Err())} }
	lines = append(lines, drawRead(p.listing, drawnRead[[]string]{
		reading:    func() []string { return []string{"reading the merge methods" + kit.marks.ellipsis} },
		shown:      func() []string { return p.listed(kit) },
		failed:     failed,
		failedOver: failed,
	})...)

	return "Merge " + p.vocab.noun, strings.Join(lines, "\n")
}

// listed draws the methods read, the chosen one marked.
func (p mergePicker) listed(kit renderKit) []string {
	methods := p.methods()
	lines := append(make([]string, 0, 1+len(methods)), "Merge by:")

	for index, method := range methods {
		lines = append(lines, kit.marks.marker(index == p.selected)+mergeMethodLabel(method))
	}

	return lines
}

// methods is the merge methods read, none until they are.
func (p mergePicker) methods() []forge.MergeMethod {
	found, _ := p.listing.Value()

	return found
}

// footer offers moving between the methods, merging, and leaving; only leaving
// while there is no method to choose.
func (p mergePicker) footer(keys keyMap) []key.Binding {
	cancel := relabel(keys.closeOverlay, escCancel)

	switch {
	case p.send.InFlight():
		return []key.Binding{keys.interrupt}
	case len(p.methods()) == 0:
		return []key.Binding{cancel}
	default:
		return []key.Binding{keys.up, keys.down, relabel(keys.confirm, "merge"), cancel}
	}
}

// handleKey answers a key while the merge is being previewed. With no method to
// choose — still being read, or none permitted — only esc does anything.
func (p mergePicker) handleKey(m Model, msg tea.KeyPressMsg) (Model, tea.Cmd) {
	switch {
	case p.send.InFlight():
		return m, nil
	case key.Matches(msg, m.keys.closeOverlay):
		return m.closeOverlay(), nil
	case len(p.methods()) == 0:
		return m, nil
	case key.Matches(msg, m.keys.confirm):
		return p.confirm(m)
	case key.Matches(msg, m.keys.cursorKeys()...):
		return p.step(m, m.keys.stepOf(msg)), nil
	}

	m.overlay = p

	return m, nil
}

// which names the merge picker.
func (mergePicker) which() overlayKind { return overlayMergePicker }

// openedAs is the round the preview opened in.
func (p mergePicker) openedAs() int { return p.opened }

// sent is the merge's write.
func (p mergePicker) sent() send.State { return p.send }

// withSend is the preview with its merge's write as state.
func (p mergePicker) withSend(state send.State) mergePicker {
	p.send = state

	return p
}

// step moves the choice of method by delta, held within the methods, so it
// stays on the first while they are read; nothing moves while a merge is sent.
func (p mergePicker) step(m Model, delta int) Model {
	if p.send.InFlight() {
		return m
	}

	p.selected = max(0, min(p.selected+delta, len(p.methods())-1))
	m.overlay = p

	return m
}

// confirm merges by the chosen method, or, in a dry run, says what it would do.
func (p mergePicker) confirm(m Model) (Model, tea.Cmd) {
	method := p.methods()[p.selected]

	if m.dryRun {
		return m.closeOverlay().noticed("dry run: would merge " + p.vocab.sigil +
			strconv.Itoa(p.pull.Number) + " by " + mergeMethodLabel(method)), nil
	}

	p.send, _ = p.send.Sent()
	m.overlay = p
	ctx, pull, merge, opened := m.session, p.pull, m.deps.Forge.Merge, p.opened

	return m, func() tea.Msg {
		return mergeRequested{opened: opened, pull: pull, err: merge(ctx, pull, method)}
	}
}
