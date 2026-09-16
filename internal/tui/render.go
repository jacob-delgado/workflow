// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"net/url"
	"strings"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/tui/frame"
	"github.com/jacob-delgado/workflow/internal/tui/layout"
)

// notStarted is a stage of the loop nothing has reached yet. State is carried by
// the fill of the glyph rather than its color, so it reads in monochrome.
const notStarted = "○"

// View implements tea.Model. It only composes: every region renders itself, so
// no single function has to know the whole screen.
func (m Model) View() string {
	shape := layout.Compute(m.width, m.height, paneCount)
	body := m.detail(shape.Detail)

	if !shape.Collapsed() {
		body = lipgloss.JoinHorizontal(lipgloss.Top, m.rail(shape.Rail), body)
	}

	return lipgloss.JoinVertical(lipgloss.Left,
		m.spine(shape.Spine.Width), body, m.footer(shape.Footer.Width))
}

// rail draws the stacked panes down the left.
func (m Model) rail(boxes []layout.Box) string {
	rendered := make([]string, 0, len(boxes))

	for index, box := range boxes {
		current := pane(index)
		rendered = append(rendered,
			frame.Render(current.label(), m.summary(current), box.Width, box.Height, m.border(current)))
	}

	return lipgloss.JoinVertical(lipgloss.Left, rendered...)
}

// border draws the focused pane heavier than the rest.
func (m Model) border(candidate pane) frame.Style {
	if candidate == m.focus {
		return frame.Heavy
	}

	return frame.Light
}

// summary is the one line a rail pane shows about itself.
func (m Model) summary(candidate pane) string {
	switch candidate {
	case paneIssues:
		return jiraHost(m.cfg.Jira.BaseURL)
	case paneSlack:
		return m.cfg.Slack.Target()
	case paneBranch, paneCommits, paneReview:
	}

	return ""
}

// jiraHost is the part of the Jira URL worth a rail's width. The full URL is in
// the detail pane; the rail has room for a name, not an address. url.URL.Host
// never carries userinfo, so nothing here can surface a password.
func jiraHost(raw string) string {
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Host == "" {
		return describe(config.RedactURL(raw))
	}

	return parsed.Host
}

// detail draws the main pane, titled with whichever rail pane has focus. On a
// narrow terminal it is the only pane, so the title is what says where you are.
func (m Model) detail(box layout.Box) string {
	body := m.status()
	if m.helpOpen {
		body = help.New().FullHelpView(m.keys.FullHelp())
	}

	return frame.Render(m.focus.title(), body, box.Width, box.Height, frame.Light)
}

// spine draws the loop's stages across the top.
func (m Model) spine(width int) string {
	stages := []string{"Issue", "Branch", "Commits", "Review", "Slack"}
	parts := make([]string, 0, len(stages))

	for _, stage := range stages {
		parts = append(parts, notStarted+" "+stage)
	}

	return ansi.Truncate(" "+strings.Join(parts, " ─ "), width, "")
}

// footer draws the keys that matter right now.
func (m Model) footer(width int) string {
	return ansi.Truncate(" "+help.New().ShortHelpView(m.keys.ShortHelp()), width, "")
}
