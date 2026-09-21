// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
)

// overlay is something that takes the keyboard until it is closed: a picker, a
// composer, a preview, a running hook. It draws in the detail pane — Lip Gloss
// v1 cannot layer one view over another — and is drawn with focus while open,
// so there is never a second pane that looks like it has the keys.
//
// Overlays are values. handleKey returns the whole model, so an overlay can
// replace itself, close itself, or change what the panes show as it finishes.
type overlay interface {
	// view is the overlay's title and body, in as many rows as fit.
	view(width, rows int) (string, string)
	// footer is the keys that do something in it right now.
	footer(keys keyMap) []key.Binding
	handleKey(m Model, msg tea.KeyPressMsg) (Model, tea.Cmd)
}

// clickable is an overlay whose rows can be chosen with the mouse. line is the
// row of its body that was clicked, from zero.
type clickable interface {
	click(m Model, line int) (Model, tea.Cmd)
}

// editable is an overlay that has handed its body to $EDITOR and takes the
// result back. One textEdited message serves them all: the open overlay is the
// one that asked, so it updates whichever overlay is editing rather than each
// carrying a message type of its own.
type editable interface {
	overlay
	applyEdit(m Model, text string, err error) (Model, tea.Cmd)
}

// textEdited is a body back from the editor, for whichever overlay is editing.
type textEdited struct {
	text string
	err  error
}

// apply hands the edited text to the open overlay, or drops it if the overlay
// that asked has since closed.
func (msg textEdited) apply(m Model) (Model, tea.Cmd) {
	editing, open := m.overlay.(editable)
	if !open {
		return m, nil
	}

	return editing.applyEdit(m, msg.text, msg.err)
}

// applier is a message that knows what it changes: every load and every result
// the interface waits for. Update hands it the model rather than growing a case
// for each one, and a result for an overlay that has since closed does nothing,
// because the message checks what is open before touching it.
type applier interface {
	apply(m Model) (Model, tea.Cmd)
}

// Every message the interface waits for is an applier.
var (
	_ applier = issuesLoaded{}
	_ applier = detailLoaded{}
	_ applier = detailDue{}
	_ applier = transitionsListed{}
	_ applier = transitionApplied{}
	_ applier = commentEdited{}
	_ applier = commentPosted{}
	_ applier = branchLoaded{}
	_ applier = branchCreated{}
	_ applier = worktreeCreated{}
	_ applier = branchesListed{}
	_ applier = taskSwitched{}
	_ applier = fetched{}
	_ applier = changesLoaded{}
	_ applier = diffLoaded{}
	_ applier = staged{}
	_ applier = runStarted{}
	_ applier = runLine{}
	_ applier = runFinished{}
	_ applier = editorClosed{}
	_ applier = textEdited{}
	_ applier = pullFound{}
	_ applier = ciChecked{}
	_ applier = checkOpened{}
	_ applier = ciPoll{}
	_ applier = pullCreated{}
	_ applier = issueLinked{}
	_ applier = authorFound{}
	_ applier = slackPosted{}
	_ applier = hooksFound{}
	_ applier = hooksWritten{}
)

// noticed sets the footer's report of what just happened.
func (m Model) noticed(text string) Model {
	// A notice is one row, so any newline — a joined staging error, a program's
	// stderr — is folded onto it, or the footer grows past the terminal.
	lines := strings.FieldsFunc(text, func(r rune) bool { return r == '\n' || r == '\r' })
	m.notice = strings.Join(lines, m.marks.separator)

	return m
}

// closeOverlay closes whatever overlay is open.
func (m Model) closeOverlay() Model {
	m.overlay = nil

	return m
}
