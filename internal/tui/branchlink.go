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
// description, in that order. On a branch already linked it names the issue
// instead, and unlinks it at once: the description is left as it is, so
// nothing leaves the machine.
type branchLinker struct {
	vocab  reviewVocab
	branch string
	// linked is the issue the branch is linked to by hand, or empty when it is
	// linked to none and the form asks for one.
	linked  jira.Key
	pull    forge.PullRequest
	hasPull bool
	input   textinput.Model
	// body is the pull request's description with the issue's line added,
	// once the issue is chosen and the description needs it; empty before.
	body    string
	chosen  jira.Key
	send    sendState
	problem error
}

var (
	_ failable[branchLinker] = branchLinker{}
	_ pasteable              = branchLinker{}
)

// canLinkIssue reports a checked-out branch and a repository that can keep a
// link for it.
func (s branchState) canLinkIssue(deps Deps) bool {
	return deps.Git.LinkIssue != nil && s.loaded && !s.branch.Detached && s.branch.Name != ""
}

// openBranchLink opens the link form on the checked-out branch, holding the
// issue selected in the Issues list to start with.
func (m Model) openBranchLink() (Model, tea.Cmd) {
	start := ""
	if selected, ok := m.issues.current(); ok {
		start = shownKey(selected.Key)
	}

	// A merged or closed pull request stays out of the form, so linking the
	// branch neither edits its description nor offers it on the issue.
	var pull forge.PullRequest

	hasPull := m.review.found && m.review.pull.IsOpen()
	if hasPull {
		pull = m.review.pull
	}

	m.overlay = branchLinker{
		vocab: m.vocab, branch: m.branch.branch.Name,
		linked: jira.Key(m.branch.branch.IssueLink), pull: pull, hasPull: hasPull, input: newInput(start),
	}

	return m, nil
}

// view draws the issue being chosen, or the description it will add itself
// to, with how sending is going or why the form cannot send pinned under the
// title, so a long refusal is wrapped and seen rather than clipped.
func (l branchLinker) view(kit renderKit, width, _ int) (string, string) {
	lines := append(kit.pinnedOutcome(l.send, l.doing(), width),
		kit.pinnedProblem(l.problem, width)...)

	if l.linked != "" {
		lines = append(lines, l.branch+" is linked to "+shownKey(l.linked)+".")

		return "Link " + l.branch, strings.Join(lines, "\n")
	}

	l.input.SetWidth(max(1, width-len(l.input.Prompt)-1))

	lines = append(lines, "Link "+l.branch+" to an issue: a Jira key, or a forge number.", "", l.input.View())
	if l.body != "" {
		lines = append(lines, "", "Its "+l.vocab.noun+"'s description becomes:", "", l.body)
	}

	return "Link " + l.branch, strings.Join(lines, "\n")
}

// doing names the request in flight: unlinking on a linked branch, linking
// otherwise.
func (l branchLinker) doing() string {
	if l.linked != "" {
		return "unlinking"
	}

	return "linking"
}

// footer offers choosing the issue, then linking it, or canceling.
func (l branchLinker) footer(keys keyMap) []key.Binding {
	if l.send.sending {
		return []key.Binding{keys.interrupt}
	}

	if l.linked != "" {
		return []key.Binding{keys.unlinkIssue, relabel(keys.closeOverlay, escClose)}
	}

	confirm := relabel(keys.confirm, "link")
	if l.body != "" {
		confirm = relabel(keys.confirm, "link and update the description")
	}

	return []key.Binding{confirm, relabel(keys.closeOverlay, escCancel)}
}

// handleKey answers a key while the form has the keyboard.
func (l branchLinker) handleKey(m Model, msg tea.KeyPressMsg) (Model, tea.Cmd) {
	switch {
	case l.send.sending:
		return m, nil
	case key.Matches(msg, m.keys.closeOverlay):
		return m.closeOverlay(), nil
	case l.linked != "":
		return l.unlinkOn(m, msg)
	case key.Matches(msg, m.keys.confirm) && l.body != "":
		return l.link(m)
	case key.Matches(msg, m.keys.confirm):
		return l.choose(m)
	default:
		return l.retyped(m, msg), nil
	}
}

// retyped is the form once the issue typed changes: what was wrong with the
// last one, the description it needed and the refusal it met no longer hold.
func (l branchLinker) retyped(m Model, typed tea.Msg) Model {
	l.input, _ = l.input.Update(typed)
	l.send, l.problem, l.body = sendState{}, nil, ""
	m.overlay = l

	return m
}

// which names the link form.
func (branchLinker) which() overlayKind { return overlayBranchLinker }

// unlinkOn answers a key on a branch already linked, where unlink is the one
// act offered.
func (l branchLinker) unlinkOn(m Model, msg tea.KeyPressMsg) (Model, tea.Cmd) {
	if !key.Matches(msg, m.keys.unlinkIssue) {
		return m, nil
	}

	if m.dryRun {
		return m.closeOverlay().noticed("dry run: would unlink " + l.branch + " from " + shownKey(l.linked)), nil
	}

	l.send, l.problem = starting(), nil
	m.overlay = l
	ctx, unlink, branch, issueKey := m.session, m.deps.Git.UnlinkIssue, l.branch, l.linked

	return m, func() tea.Msg {
		return branchUnlinked{branch: branch, issueKey: issueKey, err: unlink(ctx, branch)}
	}
}

// branchUnlinked reports how forgetting the branch's link went.
type branchUnlinked struct {
	branch   string
	issueKey jira.Key
	err      error
}

var _ applier = branchUnlinked{}

// apply keeps the form open with the reason the link was kept, or closes it
// and reads the branch again.
func (msg branchUnlinked) apply(m Model) (Model, tea.Cmd) {
	if msg.err != nil {
		return keepOpenWith[branchLinker](m, msg.err), nil
	}

	m = m.closeOverlay().noticed(m.marks.done + " unlinked " + msg.branch + " from " + shownKey(msg.issueKey))

	return m, loadBranch(m.session, m.deps)
}

// pasted types a paste into the issue key, as typing it would.
func (l branchLinker) pasted(m Model, paste tea.PasteMsg) (Model, tea.Cmd) {
	if l.send.sending || l.linked != "" {
		return m, nil
	}

	return l.retyped(m, paste), nil
}

// choose reads the issue typed, then shows the description it needs adding
// to, or links at once when there is none to change.
func (l branchLinker) choose(m Model) (Model, tea.Cmd) {
	ref, known := convention.RefOf(strings.TrimSpace(l.input.Value()))
	if !known {
		l.send, l.problem = sendState{}, errNotAnIssue
		m.overlay = l

		return m, nil
	}

	l.chosen = jira.Key(ref.Key)

	// Only a description the forge can edit is shown as about to change.
	if l.hasPull && m.deps.Forge.RewriteDescription != nil {
		if body, changed := convention.WithIssueLine(l.pull.Body, ref.Key, issueBrowseURL(m.deps, l.chosen)); changed {
			l.body = body
			m.overlay = l

			return m, nil
		}
	}

	return l.link(m)
}

// link keeps the link, then adds the issue's line to the pull request's
// description when it was shown, holding both back in a dry run.
func (l branchLinker) link(m Model) (Model, tea.Cmd) {
	if m.dryRun {
		return m.closeOverlay().noticed("dry run: would link " + l.branch + " to " + shownKey(l.chosen)), nil
	}

	l.send = starting()
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
	sent := branchLinked{branch: l.branch, issueKey: l.chosen, pull: l.pull}

	err := git.LinkIssue(ctx, l.branch, string(l.chosen))
	if err != nil {
		sent.err = l.notLinked(err)

		return sent
	}

	sent.linked = true
	if l.body == "" {
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
	if l.body == "" {
		return err
	}

	return fmt.Errorf("%w: %w", errNotLinkedNorDescribed, err)
}

// failed pins a refusal in the form, so it is read before the form closes.
func (l branchLinker) failed(err error) branchLinker {
	l.send = l.send.failed(err)

	return l
}

// issueBrowseURL is the issue's page, for the line naming it, or empty when
// the tracker gives none.
func issueBrowseURL(deps Deps, issueKey jira.Key) string {
	if deps.Jira.BrowseURL == nil {
		return ""
	}

	return deps.Jira.BrowseURL(issueKey)
}

// branchLinked reports how linking the branch went.
type branchLinked struct {
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
func (msg branchLinked) apply(m Model) (Model, tea.Cmd) {
	if msg.err != nil {
		return msg.keptOpen(m)
	}

	m = m.closeOverlay().noticed(m.marks.done + " " + msg.said(m.vocab.sigil))

	ref, _ := convention.RefOf(string(msg.issueKey))
	if ref.Tracker == convention.TrackerJira && msg.pull.Number != 0 && m.deps.Jira.LinkPullRequest != nil {
		m.overlay = issueLinker{vocab: m.vocab, issueKey: msg.issueKey, pull: msg.pull}
	}

	return m, loadBranch(m.session, m.deps)
}

// keptOpen keeps the form open with the reason the link or the edit failed,
// so enter tries again, and reads the branch again when it was linked all the
// same, so the panes name its issue.
func (msg branchLinked) keptOpen(m Model) (Model, tea.Cmd) {
	m = keepOpenWith[branchLinker](m, msg.err)
	if !msg.linked {
		return m, nil
	}

	return m, loadBranch(m.session, m.deps)
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
		Branch: m.branch.branch.Name, Link: m.branch.branch.IssueLink, Pull: &pull, Project: m.cfg.Jira.Project,
	})
	if !found {
		return ""
	}

	issueKey := jira.Key(ref.Key)
	issue, _ := m.issues.find(issueKey)

	return strings.TrimSpace(m.styles.label.Render("issue  ") + shownKey(issueKey) + " " + issue.Summary)
}
