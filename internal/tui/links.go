// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
)

// linkOffers are opening and copying a URL, each only when its seam is
// present. An empty url offers neither: there is no link to act on.
func (m Model) linkOffers(url string) []offer {
	return []offer{
		{
			binding: m.keys.openLink, can: url != "" && m.deps.OpenURL != nil,
			act: func() (Model, tea.Cmd) { return m.openLink(url) },
		},
		{
			binding: m.keys.copyLink, can: url != "" && m.deps.Copy != nil,
			act: func() (Model, tea.Cmd) { return m.copyLink(url) },
		},
	}
}

// linkKeys is the footer's link keys: the link offers that act right now.
func (m Model) linkKeys(url string) []key.Binding {
	return liveKeys(m.linkOffers(url))
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
