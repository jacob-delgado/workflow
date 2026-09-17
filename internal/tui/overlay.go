// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
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
	handleKey(m Model, msg tea.KeyMsg) (Model, tea.Cmd)
}

// clickable is an overlay whose rows can be chosen with the mouse. line is the
// row of its body that was clicked, from zero.
type clickable interface {
	click(m Model, line int) (Model, tea.Cmd)
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
	_ applier = changesLoaded{}
	_ applier = staged{}
	_ applier = runStarted{}
	_ applier = runLine{}
	_ applier = runFinished{}
	_ applier = editorClosed{}
	_ applier = commitBodyEdited{}
	_ applier = pullFound{}
	_ applier = ciChecked{}
	_ applier = ciPoll{}
	_ applier = pullCreated{}
	_ applier = prBodyEdited{}
	_ applier = authorFound{}
	_ applier = slackTextEdited{}
	_ applier = slackPosted{}
	_ applier = hooksFound{}
	_ applier = hooksWritten{}
)

// noticed sets the footer's report of what just happened.
func (m Model) noticed(text string) Model {
	m.notice = text

	return m
}

// closeOverlay closes whatever overlay is open.
func (m Model) closeOverlay() Model {
	m.overlay = nil

	return m
}
