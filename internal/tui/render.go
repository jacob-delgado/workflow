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

	rows := []string{m.spine(shape), body}
	if m.showsNotice() {
		rows = append(rows, m.noticeLine(shape.Footer.Width))
	}

	return lipgloss.JoinVertical(lipgloss.Left, append(rows, m.footer(shape.Footer.Width))...)
}

// noticeLine is the one-row report above the hints — the issue filter while it
// is active, otherwise what just happened — cut with a mark rather than silently
// where it does not fit.
func (m Model) noticeLine(width int) string {
	text := m.notice
	if m.showsFilter() {
		text = "filter: " + m.issues.filter
	}

	return ansi.Truncate(" "+sanitize.Text(text), width, m.marks.ellipsis)
}

// railRuleRows is the one shared-rule row each rail pane's box holds above its
// content, so a box's content is one row shorter than the box.
const railRuleRows = 1

// rail draws the stacked panes as one shared box. The focused pane's rules and
// sides are drawn heavy — unless an overlay has the keyboard, or its list lives
// in the detail, which then carries the heavy border, so there is never a
// second. The focused pane's title is bold either way.
func (m Model) rail(boxes []layout.Box) string {
	panes := make([]frame.RailPane, 0, len(boxes))

	for index, box := range boxes {
		current := pane(index)
		focused := current == m.focus && m.overlay == nil

		title := m.paneTitle(current, current.label()+m.viewSuffix(current))
		if focused {
			title = m.styles.strong.Render(title)
		}

		rows := max(0, box.Height-railRuleRows)
		panes = append(panes, frame.RailPane{
			Title:   title,
			Body:    behaviorOf(current).rail(m, rows),
			Rows:    rows,
			Focused: focused && !behaviorOf(current).listInDetail,
		})
	}

	width := 0
	if len(boxes) > 0 {
		width = boxes[0].Width
	}

	return frame.Rail(panes, width, m.marks.border)
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

	if m.overlay != nil {
		title, body := m.overlay.view(width, rows)

		// Most overlays act, and wear the heavy focus border; one that only
		// reports, like the key list, marks itself for the light one.
		border := m.marks.border.Heavy()
		if _, reports := m.overlay.(lightBordered); reports {
			border = m.marks.border
		}

		return title, body, border
	}

	behavior := behaviorOf(m.focus)

	// Collapsed, the rail that showed 1-5 is gone, so the detail title carries
	// the number that jumps to the pane.
	title := m.focus.title()
	if shape.Collapsed() {
		title = m.focus.label()
	}

	title = m.paneTitle(m.focus, title+m.viewSuffix(m.focus))

	// A pane whose list lives in the detail wears the heavy focus border here,
	// where the cursor is, rather than on its rail summary.
	border := m.marks.border
	if behavior.listInDetail {
		border = border.Heavy()
	}

	if shape.Collapsed() && behavior.narrow != nil {
		return title, behavior.narrow(m, rows), border
	}

	return title, scrolled(behavior.detail(m, width), m.scroll, rows), border
}

// paneTitle adds the in-flight glyph to a pane's title while it is loading, so a
// refresh shows in the title until the answer arrives.
func (m Model) paneTitle(p pane, title string) string {
	if m.loading(p) {
		return title + " " + m.marks.inFlight
	}

	return title
}

// loading reports a pane waiting on a load it started.
func (m Model) loading(p pane) bool {
	if p == paneIssues {
		return m.issues.loading
	}

	return false
}

// helpColumnSplit is the group after which the help wraps into a second column,
// so the two columns come out close to the same height.
const helpColumnSplit = 4

// helpColumnGap is the space between the help's two columns.
const helpColumnGap = 4

// helpView lists every key, grouped by where it works, in two columns so the
// whole set fits a short pane with less scrolling.
func (m Model) helpView() string {
	left := m.helpColumn(0, helpColumnSplit)
	right := m.helpColumn(helpColumnSplit, len(helpGroups()))

	return lipgloss.JoinHorizontal(lipgloss.Top, left, lipgloss.NewStyle().PaddingLeft(helpColumnGap).Render(right))
}

// helpColumn renders the help groups in a range, one key a line under each
// group's name. A binding with no help text of its own is left out; it rides
// another's line.
func (m Model) helpColumn(first, last int) string {
	groups := m.keys.FullHelp()
	names := helpGroups()

	var lines []string

	for index := first; index < last; index++ {
		if index > first {
			lines = append(lines, "")
		}

		lines = append(lines, m.styles.strong.Render(names[index]))

		for _, binding := range groups[index] {
			if binding.Help().Key == "" {
				continue
			}

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

// footer draws the keys that matter right now. A notice normally has its own
// row above this one; only on a terminal too short for that row does the footer
// stand in and report it, so a result is never lost.
func (m Model) footer(width int) string {
	if m.notice != "" && !m.showsNotice() {
		return ansi.Truncate(" "+sanitize.Text(m.notice), width, m.marks.ellipsis)
	}

	keys := help.New()
	keys.Styles.ShortKey = m.styles.strong
	keys.Styles.ShortDesc = m.styles.label
	keys.Styles.ShortSeparator = m.styles.label
	keys.ShortSeparator, keys.Ellipsis = m.marks.helpSeparator, m.marks.ellipsis
	// Keys that do not fit are dropped whole, and an ellipsis says so.
	keys.Width = width - 1

	return ansi.Truncate(" "+keys.ShortHelpView(m.footerKeys()), width, "")
}

// footerKeys offers the keys that do something where the user is: never a verb
// with nothing to act on.
func (m Model) footerKeys() []key.Binding {
	if m.overlay != nil {
		return m.overlay.footer(m.keys)
	}

	return append(behaviorOf(m.focus).keys(m), m.keys.ShortHelp()...)
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

// configErrorStatus renders the screen shown when no configuration loaded. It
// names both setup steps, in the one wording every surface shares.
func (m Model) configErrorStatus() string {
	if errors.Is(m.loadErr, config.ErrNotFound) {
		return m.styles.strong.Render(config.NoConfigHeadline) + "\n" +
			m.styles.label.Render(config.InitStep) + "\n" +
			m.styles.label.Render(config.DoctorStep)
	}

	return m.styles.strong.Render("configuration error") + "\n" +
		m.styles.label.Render(m.loadErr.Error()) + "\n" +
		m.styles.label.Render("start over with `workflow config init --force`")
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

// failedGlyph is the failure mark in red, so red always means something broke —
// the free form lets a seam that carries only styles and glyphs redden it too.
func failedGlyph(sty styles, marks glyphs) string {
	return sty.failure.Render(marks.failed)
}

// failedGlyph is failedGlyph for a Model.
func (m Model) failedGlyph() string {
	return failedGlyph(m.styles, m.marks)
}

// failureBlock is an error wrapped to a width and styled per row, so every row
// opens and closes its own color and none runs on into the border beside it.
func failureBlock(sty styles, marks glyphs, err error, width int) string {
	return sty.failure.Render(wrap(marks.failed+" "+sanitize.Text(err.Error()), width))
}

// pinnedOutcome is an overlay's outcome — the in-flight word or the refusal —
// drawn under the title rather than at the bottom, so a long reason is wrapped
// and seen instead of clipped below the fold. Empty when nothing has happened.
func pinnedOutcome(sty styles, marks glyphs, send sendState, doing string, width int) []string {
	switch {
	case send.sending:
		return []string{doing + marks.ellipsis, ""}
	case send.err != nil:
		return []string{failureBlock(sty, marks, send.err, width), ""}
	default:
		return nil
	}
}

// failureWithin is failure for a pane: recognized as a sentence like failure,
// but wrapped to its width first and styled after.
func (m Model) failureWithin(err error, width int) string {
	sentence, known := errorSentence(err)
	if !known {
		return failureBlock(m.styles, m.marks, err, width)
	}

	return m.styles.failure.Render(wrap(m.marks.failed+" "+sentence, width)) + "\n" +
		m.styles.label.Render(wrap(sanitize.Text(err.Error()), width))
}

// plural counts things, in words.
func plural(count int, noun string) string {
	if count == 1 {
		return "1 " + noun
	}

	return strconv.Itoa(count) + " " + noun + "s"
}
