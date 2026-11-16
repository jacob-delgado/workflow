// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"

	"github.com/jacob-delgado/workflow/internal/convention"
	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/jira"
	"github.com/jacob-delgado/workflow/internal/loop"
	"github.com/jacob-delgado/workflow/internal/seams"
	"github.com/jacob-delgado/workflow/internal/tui/send"
)

// errNotAnIssue is text typed into the link form that names no issue.
var errNotAnIssue = errors.New("name an issue: a Jira key like PROJ-7, or a forge number like #42")

// errNotLinkedNorDescribed is a refused link, with the description shown as
// about to change left as it was.
var errNotLinkedNorDescribed = errors.New("the branch was not linked, and the description was left as it was")

// errLinkedNotDescribed is the forge refusing the description's edit once the
// branch was linked.
var errLinkedNotDescribed = errors.New("the branch is linked, but the description was not updated")

// branchLinker links the checked-out branch to an issue by hand, for work
// begun outside workflow on a branch whose name names none. With a pull
// request open from the branch, it shows the description with the issue's
// line added before anything is sent, then links the branch and edits the
// description, in that order. A branch already linked opens the unlinker
// (branchUnlinker) instead.
type branchLinker struct {
	vocab   reviewVocab
	branch  string
	pull    forge.PullRequest
	hasPull bool
	input   textinput.Model
	stage   linkStage
	// body is the pull request's description with the issue's line added,
	// shown while the stage is linkPreviewing; empty while typing.
	body   string
	chosen jira.Key
	opened int
	// send is the link and the edit after it, sent as one write: a refused
	// edit is a refusal carrying errLinkedNotDescribed, and enter sends both
	// again, the link being one git keeps as often as it is asked.
	send send.State
	// problem is why the text typed names no issue, found before anything
	// is sent.
	problem error
}

// linkStage is where the link form stands: typing the issue, or showing the
// description the issue's line will be added to, which enter confirms. A
// write is out or refused in either, as send says.
type linkStage int

const (
	// linkTyping takes the issue's key, which enter reads.
	linkTyping linkStage = iota
	// linkPreviewing shows the pull request's description with the issue's
	// line added, which enter links and sends.
	linkPreviewing
)

var (
	_ sender[branchLinker] = branchLinker{}
	_ pasteable            = branchLinker{}
)

// canLinkIssue reports a checked-out branch and a repository that can keep a
// link for it.
func (s branchState) canLinkIssue(deps Deps) bool {
	branch := s.checkedOut()

	return deps.Git.LinkIssue != nil && !branch.Detached && branch.Name != ""
}

// openBranchLink opens the link form on the checked-out branch: the
// unlinker when it is linked by hand already, or the linker, holding the
// issue selected in the Issues list to start with.
func (m Model) openBranchLink() (Model, tea.Cmd) {
	checkedOut := m.branch.checkedOut()
	if checkedOut.IssueLink != "" {
		return m.openBranchUnlinker(checkedOut.Name, jira.Key(checkedOut.IssueLink)), nil
	}

	start := ""
	if selected, ok := m.issues.current(); ok {
		start = shownKey(selected.Key)
	}

	// A merged or closed pull request stays out of the form, so linking the
	// branch neither edits its description nor offers it on the issue.
	var pull forge.PullRequest

	hasPull := m.review.found() && m.review.pull().IsOpen()
	if hasPull {
		pull = m.review.pull()
	}

	m, round := m.nextRound()
	m.overlay = branchLinker{
		vocab: m.vocab, branch: checkedOut.Name, pull: pull, hasPull: hasPull, input: newInput(start),
		stage: linkTyping, opened: round, send: send.State{},
	}

	return m, nil
}

// view draws the issue being chosen, or the description it will add itself
// to, with how sending is going or why the form cannot send pinned under the
// title, so a long refusal is wrapped and seen rather than clipped.
func (l branchLinker) view(kit renderKit, width, _ int) (string, string) {
	lines := append(kit.pinnedSend(l.send, "linking", width),
		kit.pinnedProblem(l.problem, width)...)

	l.input.SetWidth(max(1, width-len(l.input.Prompt)-1))

	lines = append(lines, "Link "+l.branch+" to an issue: a Jira key, or a forge number.", "", l.input.View())
	if l.stage == linkPreviewing {
		lines = append(lines, "", "Its "+l.vocab.noun+"'s description becomes:", "", l.body)
	}

	return "Link " + l.branch, strings.Join(lines, "\n")
}

// footer offers choosing the issue, then linking it, or canceling.
func (l branchLinker) footer(keys keyMap) []key.Binding {
	if l.send.InFlight() {
		return []key.Binding{keys.interrupt}
	}

	confirm := relabel(keys.confirm, "link")
	if l.stage == linkPreviewing {
		confirm = relabel(keys.confirm, "link and update the description")
	}

	return []key.Binding{confirm, relabel(keys.closeOverlay, escCancel)}
}

// handleKey answers a key while the form has the keyboard.
func (l branchLinker) handleKey(m Model, msg tea.KeyPressMsg) (Model, tea.Cmd) {
	switch {
	case l.send.InFlight():
		return m, nil
	case key.Matches(msg, m.keys.closeOverlay):
		return m.closeOverlay(), nil
	case key.Matches(msg, m.keys.confirm):
		return l.confirmed(m)
	default:
		return l.retyped(m, msg), nil
	}
}

// confirmed answers enter: linking once the description is shown, and in
// any other stage reading the issue typed.
func (l branchLinker) confirmed(m Model) (Model, tea.Cmd) {
	if l.stage == linkPreviewing {
		return l.link(m)
	}

	return l.choose(m)
}

// retyped is the form once the issue typed changes: what was wrong with the
// last one, the description it needed and the refusal it met no longer hold.
func (l branchLinker) retyped(m Model, typed tea.Msg) Model {
	l.input, _ = l.input.Update(typed)
	l.stage, l.body, l.send, l.problem = linkTyping, "", l.send.Cleared(), nil
	m.overlay = l

	return m
}

// which names the link form.
func (branchLinker) which() overlayKind { return overlayBranchLinker }

// openedAs is the round the form opened in, which its link carries back.
func (l branchLinker) openedAs() int { return l.opened }

// sent is the link's write.
func (l branchLinker) sent() send.State { return l.send }

// withSend is the form with its link's write as state.
func (l branchLinker) withSend(state send.State) branchLinker {
	l.send = state

	return l
}

// pasted types a paste into the issue key, as typing it would.
func (l branchLinker) pasted(m Model, paste tea.PasteMsg) (Model, tea.Cmd) {
	if l.send.InFlight() {
		return m, nil
	}

	return l.retyped(m, paste), nil
}

// choose reads the issue typed, then shows the description it needs adding
// to, or links at once when there is none to change.
func (l branchLinker) choose(m Model) (Model, tea.Cmd) {
	ref, known := convention.RefOf(strings.TrimSpace(l.input.Value()))
	if !known {
		l.send, l.problem = l.send.Cleared(), errNotAnIssue
		m.overlay = l

		return m, nil
	}

	l.chosen = jira.Key(ref.Key)

	// Only a description the forge can edit is shown as about to change.
	if l.hasPull && m.deps.Forge.RewriteDescription != nil {
		if body, changed := convention.WithIssueLine(l.pull.Body, ref.Key, issueBrowseURL(m.deps, l.chosen)); changed {
			l.stage, l.body = linkPreviewing, body
			m.overlay = l

			return m, nil
		}
	}

	return l.link(m)
}

// link keeps the link, then adds the issue's line to the pull request's
// description when it was shown, holding both back in a dry run. handleKey
// takes no key while a link is out, so Sent always sends.
func (l branchLinker) link(m Model) (Model, tea.Cmd) {
	if m.dryRun {
		return m.closeOverlay().noticed("dry run: would link " + l.branch + " to " + shownKey(l.chosen)), nil
	}

	l.send, _ = l.send.Sent()
	m.overlay = l
	ctx, git, forgeDeps := m.session, m.deps.Git, m.deps.Forge
	issueURL := issueBrowseURL(m.deps, l.chosen)

	return m, func() tea.Msg {
		return l.sendLink(ctx, git, forgeDeps, issueURL)
	}
}

// sendLink links the branch first and edits the description only once it is
// linked, so a refused edit leaves a linked branch, which the form says,
// never a description naming an issue the branch is not linked to. The line
// is added to the description as the forge holds it, not to the one shown,
// whose controls and marks are neutralized, so nothing else of it changes.
func (l branchLinker) sendLink(
	ctx context.Context, git seams.Git, forgeDeps seams.Forge, issueURL string,
) branchLinked {
	sent := branchLinked{opened: l.opened, branch: l.branch, issueKey: l.chosen, pull: l.pull}

	err := git.LinkIssue(ctx, l.branch, string(l.chosen))
	if err != nil {
		sent.err = l.notLinked(err)

		return sent
	}

	sent.linked = true
	if l.stage != linkPreviewing {
		return sent
	}

	sent.described, err = forgeDeps.RewriteDescription(ctx, l.pull, func(body string) (string, bool) {
		return convention.WithIssueLine(body, string(l.chosen), issueURL)
	})
	if err != nil {
		sent.err = fmt.Errorf("%w: %w", errLinkedNotDescribed, err)
	}

	return sent
}

// notLinked is a refused link, saying the description shown was left as it
// was when there was one to change.
func (l branchLinker) notLinked(err error) error {
	if l.stage != linkPreviewing {
		return err
	}

	return fmt.Errorf("%w: %w", errNotLinkedNorDescribed, err)
}

// issueBrowseURL is the issue's page, for the line naming it, or empty when
// the tracker gives none.
func issueBrowseURL(deps Deps, issueKey jira.Key) string {
	if deps.Jira.BrowseURL == nil {
		return ""
	}

	return deps.Jira.BrowseURL(issueKey)
}

// branchLinked reports how linking the branch went, for the form opened as
// opened.
type branchLinked struct {
	opened    int
	branch    string
	issueKey  jira.Key
	pull      forge.PullRequest
	linked    bool
	described bool
	err       error
}

var _ applier = branchLinked{}

// apply keeps the form open with the reason the link failed, or closes it,
// reads the branch again, and — for a Jira issue with a pull request open —
// offers to link the pull request on the issue, as opening one would have.
// A late answer, its form no longer the one open, changes nothing in the
// form; none arrives, since the form takes no key but ctrl+c, which quits,
// while its link is out.
func (msg branchLinked) apply(m Model) (Model, tea.Cmd) {
	m, _, done := answered[branchLinker](m, msg.opened, msg.err)
	if !done {
		return msg.notDone(m)
	}

	m = m.closeOverlay().noticed(m.marks.done + " " + msg.said(m.vocab.sigil))

	ref, _ := convention.RefOf(string(msg.issueKey))
	if ref.Tracker == convention.TrackerJira && msg.pull.Number != 0 && m.deps.Jira.LinkPullRequest != nil {
		m = m.openIssueLinker(msg.issueKey, msg.pull)
	}

	return m.readingBranch()
}

// notDone follows a link refused, its reason pinned in the form so enter
// tries again: the branch is read again when it was linked all the same, so
// the panes name its issue.
func (msg branchLinked) notDone(m Model) (Model, tea.Cmd) {
	if !msg.linked {
		return m, nil
	}

	return m.readingBranch()
}

// said is what the link did, in the words its key offered: the branch linked,
// and the pull request's description updated when it was.
func (msg branchLinked) said(sigil string) string {
	linked := "linked " + msg.branch + " to " + shownKey(msg.issueKey)
	if !msg.described {
		return linked
	}

	return linked + " and updated " + sigil + strconv.Itoa(msg.pull.Number)
}

// reviewIssue is the line naming the issue the branch and its pull request are
// for — by the link the branch was given, its name, or the pull request's own
// words — with its summary when the Issues list holds it; empty when none is
// named.
func (m Model) reviewIssue(pull forge.PullRequest) string {
	ref, _, found := loop.BranchIssue(loop.IssueSource{
		Branch: m.branch.checkedOut().Name, Link: m.branch.checkedOut().IssueLink, Pull: &pull, Project: m.cfg.Jira.Project,
	})
	if !found {
		return ""
	}

	issueKey := jira.Key(ref.Key)
	issue, _ := m.issues.find(issueKey)

	return strings.TrimSpace(m.styles.label.Render("issue  ") + shownKey(issueKey) + " " + issue.Summary)
}
