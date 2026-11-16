// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"

	"github.com/jacob-delgado/workflow/internal/jira"
	"github.com/jacob-delgado/workflow/internal/tui/send"
)

// branchUnlinker is the link form on a branch already linked by hand: it
// names the issue, and unlinks it at once. The description is left as it
// is, so nothing leaves the machine.
type branchUnlinker struct {
	branch string
	issue  jira.Key
	opened int
	send   send.State
}

var _ sender[branchUnlinker] = branchUnlinker{}

// openBranchUnlinker opens the form on branch, linked by hand to issue, in a
// round of its own.
func (m Model) openBranchUnlinker(branch string, issue jira.Key) Model {
	m, round := m.nextRound()
	m.overlay = branchUnlinker{branch: branch, issue: issue, opened: round, send: send.State{}}

	return m
}

// view names the issue the branch is linked to, with how unlinking is going
// pinned under the title.
func (u branchUnlinker) view(kit renderKit, width, _ int) (string, string) {
	lines := append(kit.pinnedSend(u.send, "unlinking", width), u.branch+" is linked to "+shownKey(u.issue)+".")

	return "Link " + u.branch, strings.Join(lines, "\n")
}

// footer offers unlinking or closing, and only quitting while an unlink is
// out.
func (u branchUnlinker) footer(keys keyMap) []key.Binding {
	if u.send.Stage() == send.StageInFlight {
		return []key.Binding{keys.interrupt}
	}

	return []key.Binding{keys.unlinkIssue, relabel(keys.closeOverlay, escClose)}
}

// handleKey unlinks the branch or closes the form, and takes no other key.
func (u branchUnlinker) handleKey(m Model, msg tea.KeyPressMsg) (Model, tea.Cmd) {
	switch {
	case u.send.Stage() == send.StageInFlight:
		return m, nil
	case key.Matches(msg, m.keys.closeOverlay):
		return m.closeOverlay(), nil
	case key.Matches(msg, m.keys.unlinkIssue):
		return u.unlink(m)
	default:
		return m, nil
	}
}

// which names the link form on a linked branch.
func (branchUnlinker) which() overlayKind { return overlayBranchUnlinker }

// openedAs is the round the form opened in, which its unlink carries back.
func (u branchUnlinker) openedAs() int { return u.opened }

// sent is the unlink's write.
func (u branchUnlinker) sent() send.State { return u.send }

// withSend is the form with its unlink's write as state.
func (u branchUnlinker) withSend(state send.State) branchUnlinker {
	u.send = state

	return u
}

// unlink forgets the branch's link, holding it back in a dry run. handleKey
// takes no key while an unlink is out, so Sent always sends.
func (u branchUnlinker) unlink(m Model) (Model, tea.Cmd) {
	if m.dryRun {
		return m.closeOverlay().noticed("dry run: would unlink " + u.branch + " from " + shownKey(u.issue)), nil
	}

	u.send, _ = u.send.Sent()
	m.overlay = u
	ctx, forget := m.session, m.deps.Git.UnlinkIssue
	branch, issue, opened := u.branch, u.issue, u.opened

	return m, func() tea.Msg {
		return branchUnlinked{opened: opened, branch: branch, issueKey: issue, err: forget(ctx, branch)}
	}
}

// branchUnlinked reports how forgetting the branch's link went, for the
// form opened as opened.
type branchUnlinked struct {
	opened   int
	branch   string
	issueKey jira.Key
	err      error
}

var _ applier = branchUnlinked{}

// apply keeps the form open with the reason the link was kept, or closes it
// and reads the branch again.
// A late answer, its form no longer the one open, changes nothing; none
// arrives, since the form takes no key but ctrl+c, which quits, while its
// unlink is out.
func (msg branchUnlinked) apply(m Model) (Model, tea.Cmd) {
	m, _, done := answered[branchUnlinker](m, msg.opened, msg.err)
	if !done {
		return m, nil
	}

	m = m.closeOverlay().noticed(m.marks.done + " unlinked " + msg.branch + " from " + shownKey(msg.issueKey))

	return m.readingBranch()
}
