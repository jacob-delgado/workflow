// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
)

// keyMap satisfies help.KeyMap, which is what renders it in the footer and the
// help overlay from one definition.
var _ help.KeyMap = keyMap{}

// keyMap is every key the interface answers to. It is built by a function and
// carried on the model rather than held in a package variable, which
// gochecknoglobals forbids.
type keyMap struct {
	next, previous, jump    key.Binding
	up, down                key.Binding
	changeStatus, confirm   key.Binding
	toggleMouse, toggleHelp key.Binding
	closeOverlay, quit      key.Binding
}

// newKeyMap builds the key bindings.
func newKeyMap() keyMap {
	return keyMap{
		next:         key.NewBinding(key.WithKeys("tab"), key.WithHelp("tab", "next pane")),
		previous:     key.NewBinding(key.WithKeys("shift+tab"), key.WithHelp("shift+tab", "previous pane")),
		jump:         key.NewBinding(key.WithKeys("1", "2", "3", "4", "5"), key.WithHelp("1-5", "jump to pane")),
		up:           key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("↑/k", "up the list")),
		down:         key.NewBinding(key.WithKeys("down", "j"), key.WithHelp("↓/j", "down the list")),
		changeStatus: key.NewBinding(key.WithKeys("t"), key.WithHelp("t", "change status")),
		confirm:      key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "apply")),
		toggleMouse:  key.NewBinding(key.WithKeys("m"), key.WithHelp("m", "toggle mouse")),
		toggleHelp:   key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "keys")),
		closeOverlay: key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "close")),
		quit:         key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "quit")),
	}
}

// ShortHelp is the footer: enough to move around and to find the rest.
func (k keyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.next, k.jump, k.toggleHelp, k.quit}
}

// FullHelp is the overlay: every key, in columns.
func (k keyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.next, k.previous, k.jump},
		{k.up, k.down, k.changeStatus, k.confirm},
		{k.toggleMouse, k.toggleHelp, k.closeOverlay, k.quit},
	}
}

// pickerHelp is the footer while a picker has the keyboard: only what works in it.
func (k keyMap) pickerHelp() []key.Binding {
	return []key.Binding{k.up, k.down, k.confirm, k.closeOverlay}
}
