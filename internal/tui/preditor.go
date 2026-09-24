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
)

// prEditor edits an open pull request's title and description. Unlike the
// composer, it changes an existing pull request rather than opening one, so it
// carries neither the branch fields nor the push a new pull request needs.
type prEditor struct {
	marks  glyphs
	styles styles
	title  textinput.Model
	body   string
	// pull is the pull request being edited, kept for its number and to leave
	// everything the edit does not touch as it is.
	pull  forge.PullRequest
	vocab reviewVocab
	send  sendState
}

var (
	_ editable           = prEditor{}
	_ failable[prEditor] = prEditor{}
)

// openPullRequestEditor opens the editor on the branch's pull request, seeded
// with its current title and description.
func (m Model) openPullRequestEditor() (Model, tea.Cmd) {
	pull := m.review.pull

	editor := prEditor{
		marks: m.marks, styles: m.styles,
		title: newInput(pull.Title), body: pull.Body, pull: pull, vocab: m.vocab,
	}
	editor.title.Focus()

	m.overlay = editor

	return m, nil
}

// view shows the title and description as they will be saved.
func (p prEditor) view(width, _ int) (string, string) {
	p.title.SetWidth(max(1, width-prLabelWidth))

	lines := pinnedOutcome(p.styles, p.marks, p.send, "saving", width)
	lines = append(lines,
		p.marks.marker(true)+fmt.Sprintf("%-9s ", "title")+p.title.View(),
		fmt.Sprintf("  %-9s %s%d", "on", p.vocab.sigil, p.pull.Number),
		"",
	)

	body := strings.Split(wrap(strings.TrimRight(p.body, "\n"), width), "\n")
	lines = append(lines, body[:min(len(body), prBodyPreviewLines)]...)

	return "Edit " + p.vocab.noun, strings.Join(lines, "\n")
}

// footer offers editing the body, saving, and leaving.
func (p prEditor) footer(keys keyMap) []key.Binding {
	if p.send.sending {
		return []key.Binding{keys.interrupt}
	}

	return []key.Binding{keys.editBody, relabel(keys.confirm, "save"), relabel(keys.closeOverlay, "discard")}
}

// handleKey answers a key while the pull request is edited.
func (p prEditor) handleKey(m Model, msg tea.KeyPressMsg) (Model, tea.Cmd) {
	switch {
	case p.send.sending:
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
		p.send.err = nil
	}

	m.overlay = p

	return m, nil
}

// editBody opens the editor on the description.
func (p prEditor) editBody(m Model) tea.Cmd {
	if m.deps.Editor.Edit == nil {
		return nil
	}

	return m.deps.Editor.Edit(p.body, prBodyHelp, func(text string, err error) tea.Msg {
		return textEdited{text: text, err: err}
	})
}

// applyEdit puts the edited description back, or records why the editor failed.
func (p prEditor) applyEdit(m Model, text string, err error) (Model, tea.Cmd) {
	if err != nil {
		p.send = p.send.failed(err)
	} else {
		p.body = text
	}

	m.overlay = p

	return m, nil
}

// save asks the forge to update the pull request, or, in a dry run, says what it
// would do. A title is required, as it is to open one.
func (p prEditor) save(m Model) (Model, tea.Cmd) {
	title := strings.TrimSpace(p.title.Value())
	if title == "" {
		p.send.err = errNoTitle
		m.overlay = p

		return m, nil
	}

	if m.dryRun {
		return m.closeOverlay().noticed("dry run: would save " + p.vocab.sigil + strconv.Itoa(p.pull.Number) +
			" as \"" + title + "\""), nil
	}

	p.send = starting()
	m.overlay = p
	edit, pull := m.deps.Forge.EditPullRequest, p.pull
	request := forge.PullRequestEdit{Title: title, Body: p.body}

	return m, func() tea.Msg {
		updated, err := edit(pull, request)

		return pullEdited{pull: updated, err: err}
	}
}

// pullEdited reports how saving the edit went.
type pullEdited struct {
	pull forge.PullRequest
	err  error
}

// apply shows the updated pull request, or keeps the editor open with why the
// forge turned the change down.
func (msg pullEdited) apply(m Model) (Model, tea.Cmd) {
	if msg.err != nil {
		return keepOpenWith[prEditor](m, writeRefusal(msg.err)), nil
	}

	m.review.pull.Title, m.review.pull.Body = msg.pull.Title, msg.pull.Body

	return m.closeOverlay().noticed(m.marks.done + " updated " + m.vocab.sigil + strconv.Itoa(msg.pull.Number)), nil
}

// failed is the editor kept open with the reason the change was turned down.
func (p prEditor) failed(err error) prEditor {
	p.send = p.send.failed(err)

	return p
}
