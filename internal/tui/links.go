// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
)

// linkKeys offers opening and copying a URL, each only when its seam is present.
// An empty url offers nothing: there is no link to act on.
func (m Model) linkKeys(url string) []key.Binding {
	if url == "" {
		return nil
	}

	var keys []key.Binding

	if m.deps.OpenURL != nil {
		keys = append(keys, m.keys.openLink)
	}

	if m.deps.Copy != nil {
		keys = append(keys, m.keys.copyLink)
	}

	return keys
}

// openLink opens url in the browser, leaving the reason on screen if the opener
// fails.
func (m Model) openLink(url string) (Model, tea.Cmd) {
	open := m.deps.OpenURL
	if open == nil || url == "" {
		return m, nil
	}

	return m, func() tea.Msg { return linkOpened{err: open(url)} }
}

// copyLink copies url to the clipboard and says so. The clipboard write reaches
// the terminal as a command with no result to wait on, so the notice is shown at
// once rather than on a reply.
func (m Model) copyLink(url string) (Model, tea.Cmd) {
	copyText := m.deps.Copy
	if copyText == nil || url == "" {
		return m, nil
	}

	return m.noticed(m.marks.done + " copied " + url), copyText(url)
}

// linkOpened reports how opening a link went.
type linkOpened struct {
	err error
}

var _ applier = linkOpened{}

// apply keeps the reason on screen when opening failed, and says nothing when it
// worked — the browser is now in front of the user.
func (msg linkOpened) apply(m Model) (Model, tea.Cmd) {
	if msg.err != nil {
		return m.noticedFailure(msg.err), nil
	}

	return m, nil
}
