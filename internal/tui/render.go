// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/sanitize"
	"github.com/jacob-delgado/workflow/internal/tui/frame"
	"github.com/jacob-delgado/workflow/internal/tui/layout"
)

// detailPadding is the columns a bordered detail pane spends on its border and
// the space inside it.
const detailPadding = 4

// helpTitle titles the detail pane while the keys are shown.
const helpTitle = "Keys"

// View implements tea.Model. v2 takes the screen and its terminal features
// declaratively: the alternate screen keeps the session from scrolling the
// terminal and hands the scrollback back untouched on exit, and the mouse mode
// follows the model's own toggle.
func (m Model) View() tea.View {
	view := tea.NewView(m.screen())
	view.AltScreen = true

	if m.mouse {
		view.MouseMode = tea.MouseModeCellMotion
	}

	return view
}

// screen renders the whole interface to a string: the rail and detail, the
// spine above and the keys below.
func (m Model) screen() string {
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
	if m.showsFilter() {
		return ansi.Truncate(" "+sanitize.Text("filter: "+m.issues.filter), width, m.marks.ellipsis)
	}

	return m.noticeRow(width)
}

// noticeRow is the notice on one row, cut with a mark where it does not fit,
// and in the failure style when it reports one, as red means everywhere else.
func (m Model) noticeRow(width int) string {
	text := ansi.Truncate(sanitize.Text(m.notice.text), max(0, width-1), m.marks.ellipsis)
	if m.notice.failed {
		text = m.styles.failure.Render(text)
	}

	return " " + text
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

		title := m.paneTitle(current, current.label(m.cfg.Messaging.Service())+m.viewSuffix(current))
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

	// Collapsed, the rail that showed the pane numbers is gone, so the detail
	// title carries the number that jumps to the pane.
	title := m.focus.title(m.cfg.Messaging.Service())
	if shape.Collapsed() {
		title = m.focus.label(m.cfg.Messaging.Service())
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
	right := m.helpColumn(helpColumnSplit, len(helpGroups(m.cfg.Messaging.Service())))

	return lipgloss.JoinHorizontal(lipgloss.Top, left, lipgloss.NewStyle().PaddingLeft(helpColumnGap).Render(right))
}

// helpColumn renders the help groups in a range, one key a line under each
// group's name. A binding with no help text of its own is left out; it rides
// another's line.
func (m Model) helpColumn(first, last int) string {
	groups := m.keys.FullHelp()
	names := helpGroups(m.cfg.Messaging.Service())

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
	if m.notice.text != "" && !m.showsNotice() {
		return m.noticeRow(width)
	}

	return ansi.Truncate(" "+m.footerRow(width-1), width, "")
}

// footerRow offers the keys that do something where the user is, then the way
// to the rest — never a verb with nothing to act on — in room columns. Keys
// that do not fit are dropped whole, from the end, and an ellipsis says so. The
// pane's own verbs come first, but ? is reserved: where they would push it off,
// the last of them give way instead, since it lists every key the row cannot.
func (m Model) footerRow(room int) string {
	row := m.keyRow()
	if keys, captured := m.capturedKeys(); captured {
		return fitKeys(row, keys, room)
	}

	verbs, reserved := behaviorOf(m.focus).keys(m), []key.Binding{m.keys.toggleHelp}
	tail := ellipsisOf(row)

	kept := longestFit(row, verbs, reserved, room-lipgloss.Width(tail))
	if kept == len(verbs) {
		return fitKeys(row, slices.Concat(verbs, m.keys.ShortHelp()), room)
	}

	return row.ShortHelpView(slices.Concat(verbs[:kept], reserved)) + tail
}

// capturedKeys is the footer of whatever has the keyboard to itself — an open
// overlay, or the issue filter being typed — and whether anything has. The keys
// that work everywhere else, ? among them, do not work there.
func (m Model) capturedKeys() ([]key.Binding, bool) {
	switch {
	case m.overlay != nil:
		return m.overlay.footer(m.keys), true
	case m.filteringIssues():
		return m.filterKeys(), true
	default:
		return nil, false
	}
}

// fitKeys draws keys in room columns: all of them where they fit, and otherwise
// as many as fit whole, then an ellipsis saying the rest were dropped.
func fitKeys(row help.Model, keys []key.Binding, room int) string {
	if drawn := row.ShortHelpView(keys); lipgloss.Width(drawn) <= room {
		return drawn
	}

	tail := ellipsisOf(row)

	return row.ShortHelpView(keys[:longestFit(row, keys, nil, room-lipgloss.Width(tail))]) + tail
}

// longestFit is how many of keys, from the first, fit in room columns with
// after drawn behind them.
func longestFit(row help.Model, keys, after []key.Binding, room int) int {
	count := len(keys)
	for count > 0 && lipgloss.Width(row.ShortHelpView(slices.Concat(keys[:count], after))) > room {
		count--
	}

	return count
}

// ellipsisOf is the mark a row of keys ends with when some were dropped.
func ellipsisOf(row help.Model) string {
	return " " + row.Styles.Ellipsis.Inline(true).Render(row.Ellipsis)
}

// keyRow is the footer's key renderer, in this session's styles and marks, with
// no width of its own: footerRow decides what fits, since the renderer's own
// cut, finding no room for its ellipsis, lets a key run past the edge.
func (m Model) keyRow() help.Model {
	row := help.New()
	row.Styles.ShortKey = m.styles.strong
	row.Styles.ShortDesc = m.styles.label
	row.Styles.ShortSeparator = m.styles.label
	row.ShortSeparator, row.Ellipsis = m.marks.helpSeparator, m.marks.ellipsis

	return row
}

// relabel is a binding with help that says what it does here.
func relabel(binding key.Binding, help string) key.Binding {
	return key.NewBinding(key.WithKeys(binding.Keys()...), key.WithHelp(binding.Help().Key, help))
}

// status describes the configuration this session is running with.
// messagingLabel is the status line's label for the messaging destination: the
// service name, lowercased and padded to the same column width as the labels
// above it.
func messagingLabel(service string) string {
	return fmt.Sprintf("%-7s", strings.ToLower(service))
}

func (m Model) status(width int) string {
	if m.loadErr != nil {
		return m.configErrorStatus(width)
	}

	label := m.styles.label

	lines := []string{
		label.Render("config ") + m.cfg.Path,
		label.Render("jira   ") + config.DisplayURL(m.cfg.Jira.BaseURL) +
			label.Render(m.marks.separator+m.cfg.Jira.AuthMode().String()),
		label.Render(messagingLabel(m.cfg.Messaging.Service())) + m.cfg.Messaging.Target() +
			label.Render(m.marks.separator+m.cfg.Messaging.Mode().String()),
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
// names both setup steps, in the one wording every surface shares, and gives
// each problem an invalid file has a row of its own, wrapped to width.
func (m Model) configErrorStatus(width int) string {
	if errors.Is(m.loadErr, config.ErrNotFound) {
		return m.styles.strong.Render(config.NoConfigHeadline) + "\n" +
			m.styles.label.Render(config.InitStep) + "\n" +
			m.styles.label.Render(config.DoctorStep)
	}

	return m.styles.strong.Render("configuration error") + "\n" +
		m.failureBlock(m.loadErr, width) + "\n" +
		m.styles.label.Render("start over with `workflow config init --force`")
}

// plural counts things, in words.
func plural(count int, noun string) string {
	if count == 1 {
		return "1 " + noun
	}

	return strconv.Itoa(count) + " " + noun + "s"
}
