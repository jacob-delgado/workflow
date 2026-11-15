// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"

	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/gitrepo"
	"github.com/jacob-delgado/workflow/internal/loop"
	"github.com/jacob-delgado/workflow/internal/seams"
	"github.com/jacob-delgado/workflow/internal/tui/send"
)

// errNoLongerFinishable refuses a confirmed finish when the branch, read
// again, is no longer one that can be: the preview showed an older read.
var errNoLongerFinishable = errors.New("the branch cannot be finished now")

// errOtherBranchOut is the branch read again being another than the one the
// preview named, switched to from another terminal.
var errOtherBranchOut = errors.New("another branch is checked out")

// errBaseMoved is the branch read again finishing onto another base or from
// another remote than the preview listed, its upstream reset from another
// terminal.
var errBaseMoved = errors.New("its base changed since the preview opened")

// mergedDetail describes a merged pull request: that it merged, how to finish
// its branch or why it cannot be, and that a new one can still be opened from
// it.
func (m Model) mergedDetail(pull forge.PullRequest) string {
	lines := []string{
		m.styles.strong.Render(m.vocab.sigil+strconv.Itoa(pull.Number)) + " " + pull.Title,
		pull.URL,
		"",
		m.marks.done + " merged",
	}

	lines = append(lines, m.finishOffer()...)

	if m.review.canOpenPull(m.deps, m.branch) {
		lines = append(lines, "", m.keys.newPullRequest.Help().Key+" opens a new "+m.vocab.noun+
			" from this branch's commits.")
	}

	return strings.Join(lines, "\n")
}

// doneWithDetail describes a pull request whose review is done with, merged or
// merging, and reports whether it is one; any other state gets nothing, as an
// open one has its review to show.
func (m Model) doneWithDetail(pull forge.PullRequest) (string, bool) {
	if pull.State == forge.StateMerged {
		return m.mergedDetail(pull), true
	}

	if pull.State == forge.StateMerging {
		return m.mergingDetail(pull), true
	}

	return "", false
}

// mergingDetail describes a pull request the forge is merging: that it is,
// and, since it is no longer open and not yet merged, that nothing waits on
// it here until it has.
func (m Model) mergingDetail(pull forge.PullRequest) string {
	return strings.Join([]string{
		m.styles.strong.Render(m.vocab.sigil+strconv.Itoa(pull.Number)) + " " + pull.Title,
		pull.URL,
		"",
		m.marks.inFlight + " merging",
		"",
		"The forge is merging it; once it has, the branch can be finished.",
	}, "\n")
}

// finishOffer is what the merged detail says about finishing the branch: F
// and what it runs when it can be finished, plainly why not when something is
// in the way, and nothing where the repository can finish no branch at all.
func (m Model) finishOffer() []string {
	if m.deps.Git.Finish == nil {
		return nil
	}

	refusal := m.review.finishRefusal(m.branch)
	if refusal != nil {
		return []string{"", "The branch cannot be finished yet: " + refusal.Error() + "."}
	}

	finishing := m.branch.checkedOut().Finishing()

	return []string{"", m.keys.finish.Help().Key + " finishes the branch: switch to " +
		finishing.Base + ", pull it from " + finishing.Remote + ", delete it."}
}

// canFinish reports a merged branch that can be finished: the repository can
// finish it, and nothing is in its way.
func (s reviewState) canFinish(deps Deps, branch branchState) bool {
	return s.found() && deps.Git.Finish != nil && s.finishRefusal(branch) == nil
}

// finishRefusal is what loop.RefuseFinish finds in the way of finishing the
// branch, then what loop.RefuseMadeAfter found when the pull request was
// read, or nil when nothing is.
func (s reviewState) finishRefusal(branch branchState) error {
	refused := loop.RefuseFinish(s.pull(), s.found(), branch.checkedOut())
	if refused != nil {
		return refused
	}

	return s.madeAfter()
}

// startFinish opens the finish preview, in a round of its own: the three git
// commands that switch to base, catch it up, and delete the merged branch.
func (m Model) startFinish() (Model, tea.Cmd) {
	if !m.review.canFinish(m.deps, m.branch) {
		return m, nil
	}

	m, opened := m.nextRound()
	m.overlay = finishPreview{
		vocab: m.vocab, pull: m.review.pull(), finishing: m.branch.checkedOut().Finishing(), opened: opened,
	}

	return m, nil
}

// finishPreview previews finishing a merged branch: the three git commands it
// runs, sent only once confirmed.
type finishPreview struct {
	vocab     reviewVocab
	pull      forge.PullRequest
	finishing gitrepo.Finishing
	// opened is the round the preview opened in, which its finish carries
	// back.
	opened int
	send   send.State
}

var _ sender[finishPreview] = finishPreview{}

// view draws the merged pull request and the commands that finish its branch,
// the finish's outcome pinned under the title.
func (p finishPreview) view(kit renderKit, width, _ int) (string, string) {
	lines := kit.pinnedSend(p.send, "finishing", width)
	lines = append(lines,
		p.vocab.sigil+strconv.Itoa(p.pull.Number)+" merged; finish "+p.finishing.Branch+" by running:", "")

	for _, command := range p.finishing.Commands() {
		lines = append(lines, "  "+command)
	}

	return "Finish the branch", strings.Join(lines, "\n")
}

// footer offers finishing and leaving.
func (p finishPreview) footer(keys keyMap) []key.Binding {
	if p.send.Stage() == send.StageInFlight {
		return []key.Binding{keys.interrupt}
	}

	return []key.Binding{relabel(keys.confirm, "finish"), relabel(keys.closeOverlay, escCancel)}
}

// handleKey answers a key while the finish is being previewed.
func (p finishPreview) handleKey(m Model, msg tea.KeyPressMsg) (Model, tea.Cmd) {
	switch {
	case p.send.Stage() == send.StageInFlight:
		return m, nil
	case key.Matches(msg, m.keys.closeOverlay):
		return m.closeOverlay(), nil
	case key.Matches(msg, m.keys.confirm):
		return p.confirm(m)
	}

	m.overlay = p

	return m, nil
}

// which names the finish preview.
func (finishPreview) which() overlayKind { return overlayFinishPreview }

// openedAs is the round the preview opened in, which its finish carries back.
func (p finishPreview) openedAs() int { return p.opened }

// sent is the finish.
func (p finishPreview) sent() send.State { return p.send }

// withSend is the preview with its finish as state.
func (p finishPreview) withSend(state send.State) finishPreview {
	p.send = state

	return p
}

// confirm runs the finish, or, in a dry run, says what it would do. handleKey
// takes no key while a finish is out, so Sent always sends.
func (p finishPreview) confirm(m Model) (Model, tea.Cmd) {
	if m.dryRun {
		return m.closeOverlay().noticed("dry run: would finish " + p.finishing.Branch +
			" (switch to " + p.finishing.Base + ", pull it from " + p.finishing.Remote + ", delete it)"), nil
	}

	p.send, _ = p.send.Sent()
	m.overlay = p
	ctx, git := m.session, m.deps.Git

	return m, func() tea.Msg {
		return p.finishAfresh(ctx, git)
	}
}

// finishAfresh reads the branch, and the commits made on it since it merged,
// again and finishes it only while it is still the one previewed, onto the
// base and from the remote the preview listed, and neither loop.RefuseFinish
// nor loop.RefuseMadeAfter finds anything in the way: a commit made from
// another terminal since the preview opened, pushed or not, would otherwise
// go with the force delete.
// The merged pull request is not read again, since a merge is final and the
// commit it merged never changes.
func (p finishPreview) finishAfresh(ctx context.Context, git seams.Git) finished {
	branch, err := git.Branch(ctx)
	if err != nil {
		return p.ended(err)
	}

	refused := cmp.Or(loop.RefuseFinish(p.pull, true, branch), p.moved(branch.Finishing()))
	if refused == nil {
		refused = loop.RefuseMadeAfter(ctx, git.MadeAfter, p.pull)
	}

	if refused != nil {
		return p.ended(fmt.Errorf("%w: %w", errNoLongerFinishable, refused))
	}

	err = git.Finish(ctx, p.finishing)
	if err != nil {
		return p.failedAfter(ctx, git, err)
	}

	return p.ended(nil)
}

// ended is the preview's finish, ended with err: nil when it went through.
func (p finishPreview) ended(err error) finished {
	return finished{opened: p.opened, branch: p.finishing.Branch, base: p.finishing.Base, err: err}
}

// failedAfter is the preview's finish failed with err, the branch read again
// to tell whether the finish left it, switched to the base, before a later
// step failed: a pull that could not catch the base up, or the delete. A
// branch that cannot be read again leaves that unknown, so the finish is
// taken as one that never left it, and the preview stays, its retry reading
// the branch again first.
func (p finishPreview) failedAfter(ctx context.Context, git seams.Git, err error) finished {
	ended := p.ended(err)
	again, readErr := git.Branch(ctx)
	ended.left = readErr == nil && again.Name != p.finishing.Branch

	return ended
}

// moved is why the finish read again is not the one the preview listed, or
// nil when it is the same: the same branch, onto the same base, from the same
// remote.
func (p finishPreview) moved(again gitrepo.Finishing) error {
	switch {
	case again.Branch != p.finishing.Branch:
		return errOtherBranchOut
	case again != p.finishing:
		return errBaseMoved
	default:
		return nil
	}
}

// finished is the outcome of finishing a merged branch, for the preview
// opened as opened.
type finished struct {
	opened int
	branch string
	// base is the branch the finish switches to, which a finish that left
	// the branch and then failed stopped on.
	base string
	err  error
	// left is a finish that failed after it switched to the base, so the
	// branch the preview offered to finish is checked out no more.
	left bool
}

var _ applier = finished{}

// apply reports the finish and reloads onto the base branch, or keeps the
// preview open with why it failed — git's own words, every line of them —
// pinned under its title until esc. A finish that failed once it had left the
// branch closes the preview, which would offer to finish a branch no longer
// checked out, says in the footer where it stopped and why, and reads the
// branch again, so the panes show the base it is on.
func (msg finished) apply(m Model) (Model, tea.Cmd) {
	m, _, done := answered[finishPreview](m, msg.opened, msg.refusal())
	if !done {
		return m, nil
	}

	m = m.closeOverlay()
	if msg.err != nil {
		return m.noticedFailureLedBy("finish stopped on "+msg.base+", branch not deleted: ", msg.err).readingBranch()
	}

	return m.noticed(m.marks.done + " finished " + msg.branch).readingBranch()
}

// refusal is what the preview takes as its finish's answer: the failure, or
// none once the finish has left the branch, since the preview is then done
// with it as a finish that went through would be.
func (msg finished) refusal() error {
	if msg.left {
		return nil
	}

	return msg.err
}
