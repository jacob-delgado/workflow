// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"fmt"
	"strconv"
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"

	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/tui/send"
)

// prLabelWidth is the columns a pull request field's marker, label, prompt and
// the cursor after the text take.
const prLabelWidth = 12

// prEditor edits an open pull request's title and description. Unlike the
// composer, it changes an existing pull request rather than opening one, so it
// carries neither the branch fields nor the push a new pull request needs.
type prEditor struct {
	title textinput.Model
	body  string
	// pull is the pull request being edited, kept for its number and to leave
	// everything the edit does not touch as it is.
	pull  forge.PullRequest
	vocab reviewVocab
	send  send.State
	// problem is what the editor finds wrong before saving, an empty title,
	// or why the editor would not take the description.
	problem error
	// opened is the round the editor opened in, which an edit and the save
	// carry back, so they land in no editor opened after.
	opened int
}

var (
	_ editable         = prEditor{}
	_ sender[prEditor] = prEditor{}
	_ pasteable        = prEditor{}
)

// openPullRequestEditor opens the editor on the branch's pull request, seeded
// with its current title and description.
func (m Model) openPullRequestEditor() (Model, tea.Cmd) {
	pull := m.review.pull()
	m, opened := m.nextRound()

	editor := prEditor{
		title: newInput(pull.Title), body: pull.Body, pull: pull, vocab: m.vocab, opened: opened,
	}
	editor.title.Focus()

	m.overlay = editor

	return m, nil
}

// view shows the title and description as they will be saved.
func (p prEditor) view(kit renderKit, width, _ int) (string, string) {
	p.title.SetWidth(max(1, width-prLabelWidth))

	lines := append(kit.pinnedSend(p.send, "saving", width), kit.pinnedProblem(p.problem, width)...)
	lines = append(lines,
		kit.marks.marker(true)+fmt.Sprintf("%-9s ", "title")+p.title.View(),
		fmt.Sprintf("  %-9s %s%d", "on", p.vocab.sigil, p.pull.Number),
		"",
	)

	body := strings.Split(wrap(strings.TrimRight(p.body, "\n"), width), "\n")
	lines = append(lines, body[:min(len(body), prBodyPreviewLines)]...)

	return "Edit " + p.vocab.noun, strings.Join(lines, "\n")
}

// footer offers editing the body, saving, and leaving.
func (p prEditor) footer(keys keyMap) []key.Binding {
	if p.send.Stage() == send.StageInFlight {
		return []key.Binding{keys.interrupt}
	}

	return []key.Binding{keys.editBody, relabel(keys.confirm, "save"), relabel(keys.closeOverlay, escDiscard)}
}

// handleKey answers a key while the pull request is edited.
func (p prEditor) handleKey(m Model, msg tea.KeyPressMsg) (Model, tea.Cmd) {
	switch {
	case p.send.Stage() == send.StageInFlight:
		return m, nil
	case key.Matches(msg, m.keys.closeOverlay):
		return m.closeOverlay(), nil
	case key.Matches(msg, m.keys.confirm):
		return p.save(m)
	case key.Matches(msg, m.keys.editBody):
		m.overlay = p

		return m, p.editBody(m)
	default:
		p.title, _ = p.title.Update(msg)
		p.problem, p.send = nil, p.send.Cleared()
	}

	m.overlay = p

	return m, nil
}

// which names the pull request editor.
func (prEditor) which() overlayKind { return overlayPREditor }

// openedAs is the round the editor opened in.
func (p prEditor) openedAs() int { return p.opened }

// sent is the edit's save.
func (p prEditor) sent() send.State { return p.send }

// withSend is the editor with its save as state.
func (p prEditor) withSend(state send.State) prEditor {
	p.send = state

	return p
}

// pasted types a paste into the title, as typing it would.
func (p prEditor) pasted(m Model, paste tea.PasteMsg) (Model, tea.Cmd) {
	if p.send.Stage() == send.StageInFlight {
		return m, nil
	}

	p.title, _ = p.title.Update(paste)
	p.problem, p.send = nil, p.send.Cleared()
	m.overlay = p

	return m, nil
}

// editBody opens the editor on the description.
func (p prEditor) editBody(m Model) tea.Cmd {
	if m.deps.Editor.Edit == nil {
		return nil
	}

	return m.deps.Editor.Edit(p.body, prBodyHelp(p.vocab), editedFor(p.opened))
}

// applyEdit puts the edited description back, or keeps why the editor failed
// as its problem, in place of a refusal an earlier save had.
func (p prEditor) applyEdit(m Model, text string, err error) (Model, tea.Cmd) {
	if err != nil {
		p.problem, p.send = err, p.send.Cleared()
	} else {
		p.body = text
	}

	return m.withAnswered(p), nil
}

// save asks the forge to update the pull request, or, in a dry run, says what it
// would do. A title is required, as it is to open one. handleKey takes no key
// while a save is out, so Sent always sends.
func (p prEditor) save(m Model) (Model, tea.Cmd) {
	title := strings.TrimSpace(p.title.Value())
	if title == "" {
		p.problem, p.send = errNoTitle, p.send.Cleared()
		m.overlay = p

		return m, nil
	}

	if m.dryRun {
		return m.closeOverlay().noticed("dry run: would save " + p.vocab.sigil + strconv.Itoa(p.pull.Number) +
			" as \"" + title + "\""), nil
	}

	p.send, _ = p.send.Sent()
	p.problem = nil
	m.overlay = p
	ctx, edit, pull, opened := m.session, m.deps.Forge.EditPullRequest, p.pull, p.opened
	request := forge.PullRequestEdit{Title: title, Body: p.body}

	return m, func() tea.Msg {
		updated, err := edit(ctx, pull, request)

		return pullEdited{opened: opened, number: pull.Number, pull: updated, err: err}
	}
}

// pullEdited reports how saving the edit of the pull request numbered number
// went, for the editor opened as opened.
type pullEdited struct {
	opened int
	number int
	pull   forge.PullRequest
	err    error
}

var _ applier = pullEdited{}

// apply shows the updated pull request, or keeps the editor open with why the
// forge turned the change down. The Review pane may show another pull request
// by now, which keeps its own title and description.
func (msg pullEdited) apply(m Model) (Model, tea.Cmd) {
	m, _, done := answered[prEditor](m, msg.opened, msg.err)
	if !done {
		return m, nil
	}

	if m.review.found() && m.review.pull().Number == msg.number {
		m.review = m.review.edited(msg.pull)
	}

	return m.closeOverlay().noticed(m.marks.done + " saved " + m.vocab.sigil + strconv.Itoa(msg.number)), nil
}
