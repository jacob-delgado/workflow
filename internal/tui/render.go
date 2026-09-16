// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"strings"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/jacob-delgado/workflow/internal/tui/frame"
	"github.com/jacob-delgado/workflow/internal/tui/layout"
)

// notStarted is a stage of the loop nothing has reached yet. State is carried by
// the fill of the glyph rather than its color, so it reads in monochrome.
const notStarted = "○"

// View implements tea.Model. It only composes: every region renders itself, so
// no single function has to know the whole screen.
func (m Model) View() string {
	shape := layout.Compute(m.width, m.height, paneCount, int(m.focus))
	body := m.detail(shape.Detail, shape.Collapsed())

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
			frame.Render(current.label(), m.paneBody(current, frame.BodyRows(box.Height)),
				box.Width, box.Height, m.border(current)))
	}

	return lipgloss.JoinVertical(lipgloss.Left, rendered...)
}

// border draws the focused pane heavier than the rest. While the picker has the
// keyboard it is the one drawn heavy, so there is never a second.
func (m Model) border(candidate pane) frame.Style {
	if candidate == m.focus && !m.picker.open {
		return frame.Heavy
	}

	return frame.Light
}

// paneBody is what a rail pane shows inside its border, in as many rows as fit.
func (m Model) paneBody(candidate pane, rows int) string {
	switch candidate {
	case paneIssues:
		return m.issues.render(rows)
	case paneSlack:
		return m.cfg.Slack.Target()
	case paneBranch, paneCommits, paneReview:
	}

	return ""
}

// detail draws the main pane, titled with whichever rail pane has focus. On a
// narrow terminal it is the only pane, so the title is what says where you are.
func (m Model) detail(box layout.Box, collapsed bool) string {
	body := m.detailBody(frame.BodyRows(box.Height), collapsed)

	if m.picker.open {
		return frame.Render(pickerTitle, body, box.Width, box.Height, frame.Heavy)
	}

	return frame.Render(m.focus.title(), body, box.Width, box.Height, frame.Light)
}

// detailBody picks what the main pane shows. With the rail collapsed there is
// nowhere else for a pane's own content to go, so the focused pane's list takes
// the whole body instead of a description of one row of it.
func (m Model) detailBody(rows int, collapsed bool) string {
	switch {
	case m.picker.open:
		return m.picker.render(rows)
	case m.helpOpen:
		return help.New().FullHelpView(m.keys.FullHelp())
	case m.focus != paneIssues:
		return m.status()
	case collapsed:
		return m.issues.render(rows)
	default:
		return m.issues.detail(m.status())
	}
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

// footer draws the keys that matter right now, or reports what just happened.
func (m Model) footer(width int) string {
	text := m.notice
	if text == "" {
		text = help.New().ShortHelpView(m.footerKeys())
	}

	return ansi.Truncate(" "+text, width, "")
}

// footerKeys offers the keys that do something where the user is: never a verb
// with nothing to act on.
func (m Model) footerKeys() []key.Binding {
	_, selectable := m.issues.current()

	switch {
	case m.picker.sending:
		return []key.Binding{m.keys.quit}
	case m.picker.open:
		return m.keys.pickerHelp()
	case m.focus == paneIssues && selectable:
		return append([]key.Binding{m.keys.changeStatus}, m.keys.ShortHelp()...)
	default:
		return m.keys.ShortHelp()
	}
}
