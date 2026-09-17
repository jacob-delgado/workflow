// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/jira"
	"github.com/jacob-delgado/workflow/internal/sanitize"
	"github.com/jacob-delgado/workflow/internal/slack"
	"github.com/jacob-delgado/workflow/internal/tui/frame"
	"github.com/jacob-delgado/workflow/internal/tui/layout"
)

// detailPadding is the columns a bordered detail pane spends on its border and
// the space inside it.
const detailPadding = 4

// helpTitle titles the detail pane while the keys are shown.
const helpTitle = "Keys"

// View implements tea.Model. It only composes: every region renders itself, so
// no single function has to know the whole screen.
func (m Model) View() string {
	shape := m.shape()
	body := m.detailView(shape)

	if !shape.Collapsed() {
		body = lipgloss.JoinHorizontal(lipgloss.Top, m.rail(shape.Rail), body)
	}

	return lipgloss.JoinVertical(lipgloss.Left, m.spine(shape), body, m.footer(shape.Footer.Width))
}

// rail draws the stacked panes down the left. The focused one is drawn heavy —
// unless an overlay has the keyboard, which is then the one drawn heavy, so
// there is never a second.
func (m Model) rail(boxes []layout.Box) string {
	rendered := make([]string, 0, len(boxes))

	for index, box := range boxes {
		current := pane(index)

		style := m.marks.border
		if current == m.focus && m.overlay == nil {
			style = style.Heavy()
		}

		rendered = append(rendered, frame.Render(current.label(),
			behaviorOf(current).rail(m, frame.BodyRows(box.Height)), box.Width, box.Height, style))
	}

	return lipgloss.JoinVertical(lipgloss.Left, rendered...)
}

// detailView draws the detail pane, without its border where the terminal is
// too narrow for one.
func (m Model) detailView(shape layout.Layout) string {
	box := shape.Detail
	title, body, style := m.detailContent(shape)

	if shape.Borderless() {
		return frame.Plain(title, body, box.Width, box.Height, style)
	}

	return frame.Render(title, body, box.Width, box.Height, style)
}

// detailContent picks what the detail pane shows: an open overlay, then the
// keys, then the focused pane — which, with the rail gone, shows its own list
// where it has one.
func (m Model) detailContent(shape layout.Layout) (string, string, frame.Style) {
	rows, width := m.detailRows(), m.detailWidth()

	switch {
	case m.overlay != nil:
		title, body := m.overlay.view(width, rows)

		return title, body, m.marks.border.Heavy()
	case m.helpOpen:
		return helpTitle, scrolled(m.helpView(), m.scroll, rows), m.marks.border
	}

	behavior := behaviorOf(m.focus)
	if shape.Collapsed() && behavior.narrow != nil {
		return m.focus.title(), behavior.narrow(m, rows), m.marks.border
	}

	return m.focus.title(), scrolled(behavior.detail(m, width), m.scroll, rows), m.marks.border
}

// helpView lists every key, a group at a time, one key a line: the detail pane
// is too narrow for the groups side by side.
func (m Model) helpView() string {
	groups := m.keys.FullHelp()
	lines := make([]string, 0, len(groups))

	for index, name := range helpGroups() {
		if index > 0 {
			lines = append(lines, "")
		}

		lines = append(lines, m.styles.strong.Render(name))

		for _, binding := range groups[index] {
			lines = append(lines, "  "+fmt.Sprintf("%-10s", binding.Help().Key)+binding.Help().Desc)
		}
	}

	return strings.Join(lines, "\n")
}

// detailRows is how many rows of content the detail pane holds.
func (m Model) detailRows() int {
	shape := m.shape()
	if shape.Borderless() {
		return max(0, shape.Detail.Height-1)
	}

	return frame.BodyRows(shape.Detail.Height)
}

// detailWidth is how many columns of content the detail pane holds, which is
// what text is wrapped to.
func (m Model) detailWidth() int {
	shape := m.shape()
	if shape.Borderless() {
		return shape.Detail.Width
	}

	return max(1, shape.Detail.Width-detailPadding)
}

// scrolled is the window of text that fits in rows, starting offset lines in —
// and no further than lets the last line reach the bottom.
func scrolled(text string, offset, rows int) string {
	lines := strings.Split(text, "\n")
	first := min(max(0, offset), max(0, len(lines)-rows))
	last := min(len(lines), first+max(0, rows))

	return strings.Join(lines[first:last], "\n")
}

// wrap breaks text into lines no wider than width. It breaks only at spaces —
// never inside "--verbose" or an issue key such as PROJ-412, where a hyphen
// break reads as two things — and cuts a word only when it is wider than a line.
func wrap(text string, width int) string {
	width = max(1, width)
	lines := []string{}

	for paragraph := range strings.SplitSeq(text, "\n") {
		lines = append(lines, wrapLine(paragraph, width)...)
	}

	return strings.Join(lines, "\n")
}

// wrapLine wraps one line of text.
func wrapLine(line string, width int) []string {
	if ansi.StringWidth(line) <= width {
		return []string{line}
	}

	var lines []string

	current := ""

	for word := range strings.SplitSeq(line, " ") {
		for ansi.StringWidth(word) > width {
			if current != "" {
				lines, current = append(lines, current), ""
			}

			head := ansi.Truncate(word, width, "")
			lines, word = append(lines, head), ansi.TruncateLeft(word, ansi.StringWidth(head), "")
		}

		switch {
		case current == "":
			current = word
		case ansi.StringWidth(current)+1+ansi.StringWidth(word) <= width:
			current += " " + word
		default:
			lines, current = append(lines, current), word
		}
	}

	return append(lines, current)
}

// footer draws the keys that matter right now, or reports what just happened.
func (m Model) footer(width int) string {
	text := m.notice
	if text == "" {
		keys := help.New()
		keys.ShortSeparator, keys.Ellipsis = m.marks.helpSeparator, m.marks.ellipsis
		// Keys that do not fit are dropped whole, and an ellipsis says so.
		// Truncating stays as the backstop for a notice, and for a width too
		// narrow even for the ellipsis, where help adds the key anyway.
		keys.Width = width - 1
		text = keys.ShortHelpView(m.footerKeys())
	}

	return ansi.Truncate(" "+text, width, "")
}

// footerKeys offers the keys that do something where the user is: never a verb
// with nothing to act on.
func (m Model) footerKeys() []key.Binding {
	switch {
	case m.overlay != nil:
		return m.overlay.footer(m.keys)
	case m.helpOpen:
		return []key.Binding{m.keys.closeOverlay, m.keys.quit}
	default:
		return append(behaviorOf(m.focus).keys(m), m.keys.ShortHelp()...)
	}
}

// relabel is a binding with help that says what it does here.
func relabel(binding key.Binding, help string) key.Binding {
	return key.NewBinding(key.WithKeys(binding.Keys()...), key.WithHelp(binding.Help().Key, help))
}

// status describes the configuration this session is running with.
func (m Model) status() string {
	if m.loadErr != nil {
		return m.configErrorStatus()
	}

	label := m.styles.label

	lines := []string{
		label.Render("config ") + m.cfg.Path,
		label.Render("jira   ") + config.DisplayURL(m.cfg.Jira.BaseURL) +
			label.Render(m.marks.separator+m.cfg.Jira.AuthMode().String()),
		label.Render("slack  ") + m.cfg.Slack.Target() +
			label.Render(m.marks.separator+m.cfg.Slack.Mode().String()),
	}

	missing := m.cfg.Missing()
	if len(missing) > 0 {
		lines = append(lines,
			"",
			m.styles.strong.Render("incomplete: ")+strings.Join(missing, ", "),
			label.Render("run `workflow doctor` for detail"),
		)
	}

	return strings.Join(lines, "\n")
}

// configErrorStatus renders the screen shown when no configuration loaded.
func (m Model) configErrorStatus() string {
	if errors.Is(m.loadErr, config.ErrNotFound) {
		return m.styles.strong.Render("no "+config.FileName+" found") + "\n" +
			m.styles.label.Render("create one with `workflow config init`")
	}

	return m.styles.strong.Render("configuration error") + "\n" +
		m.styles.label.Render(m.loadErr.Error())
}

// errorSentence rewrites a recognized error as a sentence in the interface's
// voice, reporting false for one it does not recognize. It is built here rather
// than as a package map because gochecknoglobals forbids the latter.
func errorSentence(err error) (string, bool) {
	for _, known := range []struct {
		sentinel error
		sentence string
	}{
		{jira.ErrNoCredential, "Jira is not set up. Add `jira.token` to `.workflow.json`."},
		{jira.ErrUnreachable, "Jira did not answer within 10 seconds. Check the VPN, then press `r`."},
		{forge.ErrNoToken, "No forge token found. Run `gh auth login`, or set `$GITHUB_TOKEN`."},
		{forge.ErrUnreachable, "The forge did not answer within 10 seconds. Check the network, then press `r`."},
		{slack.ErrNoCredential, "Slack is not set up. Add `slack.token` or `slack.webhook_url` to `.workflow.json`."},
		{slack.ErrRejected, "Slack refused the post: check the bot is in the channel."},
	} {
		if errors.Is(err, known.sentinel) {
			return known.sentence, true
		}
	}

	return "", false
}

// failure draws an error the one way the interface says something broke. A
// recognized error reads as a sentence in the interface's voice, with the raw
// chain beneath it; an unrecognized one keeps its raw text. Either way the text
// is made safe here as well as where it was written.
func (m Model) failure(err error) string {
	glyph := m.marks.failed + " "

	sentence, known := errorSentence(err)
	if !known {
		return m.styles.failure.Render(glyph + sanitize.Text(err.Error()))
	}

	return m.styles.failure.Render(glyph+sentence) + "\n" +
		m.styles.label.Render(sanitize.Text(err.Error()))
}

// failureWithin is failure for a pane: wrapped to its width first and styled
// after, so every row opens and closes its own color and none runs on into the
// border beside it.
func (m Model) failureWithin(err error, width int) string {
	return m.styles.failure.Render(wrap(m.marks.failed+" "+sanitize.Text(err.Error()), width))
}

// plural counts things, in words.
func plural(count int, noun string) string {
	if count == 1 {
		return "1 " + noun
	}

	return strconv.Itoa(count) + " " + noun + "s"
}
