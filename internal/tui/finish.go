// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"strconv"
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"

	"github.com/jacob-delgado/workflow/internal/forge"
)

// mergedDetail describes a merged pull request: that it merged, and how to
// finish its branch.
func (m Model) mergedDetail(pull forge.PullRequest) string {
	lines := []string{
		m.styles.strong.Render(m.vocab.sigil+strconv.Itoa(pull.Number)) + " " + pull.Title,
		pull.URL,
		"",
		m.marks.done + " merged",
	}

	if m.canFinish() {
		lines = append(lines, "", "F finishes the branch: switch to "+m.branch.branch.BaseName()+", pull, delete it.")
	}

	return strings.Join(lines, "\n")
}

// canFinish reports a merged branch that can be finished: its pull request
// merged, we are on the feature branch with a base to return to, the repository
// can finish it, and no unpushed commits would be lost to the force delete.
func (m Model) canFinish() bool {
	return m.review.found && m.review.pull.State == forge.StateMerged &&
		m.deps.Git.Finish != nil && m.branch.onFeatureBranch() &&
		m.branch.branch.Base != "" && !m.branch.branch.HasUnpushedWork()
}

// startFinish opens the finish preview: the three git commands that switch to
// base, catch it up, and delete the merged branch.
func (m Model) startFinish() (Model, tea.Cmd) {
	if !m.canFinish() {
		return m, nil
	}

	m.overlay = finishPreview{
		marks: m.marks, vocab: m.vocab, pull: m.review.pull,
		branch: m.branch.branch.Name, base: m.branch.branch.BaseName(),
	}

	return m, nil
}

// finishPreview previews finishing a merged branch: the three git commands it
// runs, sent only once confirmed.
type finishPreview struct {
	marks     glyphs
	vocab     reviewVocab
	pull      forge.PullRequest
	branch    string
	base      string
	finishing bool
}

var _ overlay = finishPreview{}

// commands are the three git commands a finish runs, as they are shown and run.
func (p finishPreview) commands() []string {
	return []string{"git switch " + p.base, "git pull --ff-only", "git branch -D " + p.branch}
}

// view draws the merged pull request and the commands that finish its branch.
func (p finishPreview) view(_, _ int) (string, string) {
	lines := []string{p.vocab.sigil + strconv.Itoa(p.pull.Number) + " merged; finish " + p.branch + " by running:", ""}

	for _, command := range p.commands() {
		lines = append(lines, "  "+command)
	}

	if p.finishing {
		lines = append(lines, "", "finishing…")
	}

	return "Finish the branch", strings.Join(lines, "\n")
}

// footer offers finishing and leaving.
func (p finishPreview) footer(keys keyMap) []key.Binding {
	if p.finishing {
		return []key.Binding{keys.interrupt}
	}

	return []key.Binding{relabel(keys.confirm, "finish"), relabel(keys.closeOverlay, "cancel")}
}

// handleKey answers a key while the finish is being previewed.
func (p finishPreview) handleKey(m Model, msg tea.KeyPressMsg) (Model, tea.Cmd) {
	switch {
	case p.finishing:
		return m, nil
	case key.Matches(msg, m.keys.closeOverlay):
		return m.closeOverlay(), nil
	case key.Matches(msg, m.keys.confirm):
		return p.confirm(m)
	}

	m.overlay = p

	return m, nil
}

// confirm runs the finish, or, in a dry run, says what it would do.
func (p finishPreview) confirm(m Model) (Model, tea.Cmd) {
	if m.dryRun {
		return m.closeOverlay().noticed("dry run: would finish " + p.branch +
			" (switch to " + p.base + ", pull, delete it)"), nil
	}

	p.finishing = true
	m.overlay = p
	branch, base, finish := p.branch, p.base, m.deps.Git.Finish

	return m, func() tea.Msg {
		return finished{branch: branch, err: finish(branch, base)}
	}
}

// finished is the outcome of finishing a merged branch.
type finished struct {
	branch string
	err    error
}

// apply reports the finish and reloads onto the base branch, or closes the
// preview with why it failed.
func (msg finished) apply(m Model) (Model, tea.Cmd) {
	if msg.err != nil {
		return m.closeOverlay().noticed("could not finish " + msg.branch + ": " + oneLine(msg.err)), nil
	}

	done := m.closeOverlay().noticed(m.marks.done + " finished " + msg.branch)

	return done, done.loadBranch()
}

// oneLine collapses an error's message onto a single line, so a git failure's
// multi-line output does not break the one-line notice that shows it.
func oneLine(err error) string {
	return strings.Join(strings.Fields(err.Error()), " ")
}
