// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

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
		body = lipgloss.JoinHorizontal(lipgloss.Top, m.rail(shape), body)
	}

	rows := []string{m.spine(shape), body}
	if m.showsNotice() {
		rows = append(rows, m.noticeLine(shape.Footer.Width))
	}

	return lipgloss.JoinVertical(lipgloss.Left, append(rows, m.footer(shape.Footer.Width))...)
}

// noticeLine is the one-row report above the hints — the issue filter and
// places while they narrow the list, otherwise what just happened — cut with a mark rather than silently
// where it does not fit.
func (m Model) noticeLine(width int) string {
	if m.showsFilter() {
		return ansi.Truncate(" "+sanitize.Text(m.issues.narrowingLine(m.marks)), width, m.marks.ellipsis)
	}

	if m.filteringTasks() {
		return ansi.Truncate(" "+sanitize.Text("search: "+m.tasks.listing.narrowing.Text), width, m.marks.ellipsis)
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
// second. A borderless detail carries none, so there the rail keeps the focus
// mark through both. The focused pane's title is bold whenever its rules are.
func (m Model) rail(shape layout.Layout) string {
	boxes := shape.Rail
	panes := make([]frame.RailPane, 0, len(boxes))

	for index, box := range boxes {
		current := pane(index)
		focused := current == m.focus && (m.overlay == nil || shape.Borderless())

		title := m.paneTitle(current, current.label(m.cfg.Messaging.Service())+m.viewSuffix(current)+m.tasksSuffix(current))
		if focused {
			title = m.styles.strong.Render(title)
		}

		rows := max(0, box.Height-railRuleRows)
		panes = append(panes, frame.RailPane{
			Title:   title,
			Body:    behaviorOf(current).rail(m, rows),
			Rows:    rows,
			Focused: focused && (shape.Borderless() || !behaviorOf(current).listInDetail),
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
		title, body := m.overlay.view(m.kit(), width, rows)

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

	title = m.paneTitle(m.focus, title+m.viewSuffix(m.focus)+m.tasksSuffix(m.focus))

	// A pane whose list lives in the detail wears the heavy focus border here,
	// where the cursor is, rather than on its rail summary.
	border := m.marks.border
	if behavior.listInDetail {
		border = border.Heavy()
	}

	if shape.Collapsed() && behavior.narrow != nil {
		return title, behavior.narrow(m, rows), border
	}

	return title, scrolled(behavior.detail(m, width), *behavior.scroll(&m), rows), border
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
	return behaviorOf(p).loading(m)
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

// detailLines is how many lines the focused pane's detail runs to, wrapped to
// the detail pane's width.
func (m Model) detailLines() int {
	return strings.Count(behaviorOf(m.focus).detail(m, m.detailWidth()), "\n") + 1
}

// scrolled is the window of text that fits in rows, starting offset lines in.
func scrolled(text string, offset, rows int) string {
	lines := strings.Split(text, "\n")
	first := firstShown(len(lines), offset, rows)
	last := min(len(lines), first+max(0, rows))

	return strings.Join(lines[first:last], "\n")
}

// firstShown is the first of count lines a window of rows shows when it starts
// offset lines in — no further than lets the last line reach the bottom, so an
// offset past the end is drawn, and so is clicked and scrolled from, as the end.
func firstShown(count, offset, rows int) int {
	return min(max(0, offset), max(0, count-rows))
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

const (
	// rankMovement moves the cursor, the focus or between fields; ? lists it.
	rankMovement footerRank = iota
	// rankVerb is what can be done here.
	rankVerb
	// rankAct is enter, which does what the row names.
	rankAct
	// rankWayOut is ? and the way out — esc, q, or ctrl+c while a request is
	// out — which a footer keeps while it can show anything.
	rankWayOut
)

// messagingLabel is the status line's label for the messaging destination: the
// service name, lowercased and padded to the same column width as the labels
// above it.
func messagingLabel(service string) string {
	return fmt.Sprintf("%-7s", strings.ToLower(service))
}

// status is the status line's text: the configuration this session is running
// with and what it still lacks, or, when none loaded, why.
func (m Model) status(width int) string {
	if m.loadErr != nil {
		return m.configErrorStatus(width)
	}

	label := m.styles.label

	lines := []string{
		label.Render("config ") + m.shownFiles(m.cfg.Layers()),
		label.Render("jira   ") + sanitize.Line(config.DisplayURL(m.cfg.Jira.BaseURL)) +
			label.Render(m.marks.separator+m.cfg.Jira.AuthMode().String()),
		label.Render(messagingLabel(m.cfg.Messaging.Service())) + sanitize.Line(m.cfg.Messaging.Target()) +
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
// names both setup steps, in the one wording every surface shares — with the
// key that sets one up here first, where one can be — and gives each problem
// an invalid file has a row of its own, wrapped to width.
func (m Model) configErrorStatus(width int) string {
	if errors.Is(m.loadErr, config.ErrNotFound) {
		return m.styles.strong.Render(config.NoConfigHeadline) + "\n" +
			m.setupStep() + "\n" +
			m.styles.label.Render(config.DoctorStep)
	}

	return m.styles.strong.Render("configuration error") + "\n" +
		m.kit().failureBlock(m.loadErr, width) + "\n" +
		m.styles.label.Render("start over with `workflow config init --force`")
}

// setupStep names how to set up a first file: the key that does it here,
// where one can be, or config init.
func (m Model) setupStep() string {
	if !m.offersSetup() {
		return m.styles.label.Render(config.InitStep)
	}

	return m.styles.strong.Render(m.keys.confirm.Help().Key) + " sets one up here.\n" +
		m.styles.label.Render("Or "+strings.ToLower(config.InitStep[:1])+config.InitStep[1:])
}

// plural counts things, in words.
func plural(count int, noun string) string {
	if count == 1 {
		return "1 " + noun
	}

	return strconv.Itoa(count) + " " + noun + "s"
}

// age says how long ago something happened, as briefly as is still clear.
func age(now, then time.Time) string {
	elapsed := now.Sub(then)

	switch {
	case then.IsZero():
		return "some time ago"
	case elapsed < time.Minute:
		return "just now"
	case elapsed < time.Hour:
		return strconv.Itoa(int(elapsed.Minutes())) + "m ago"
	case elapsed < day:
		return strconv.Itoa(int(elapsed.Hours())) + "h ago"
	case elapsed < month:
		return strconv.Itoa(int(elapsed/day)) + "d ago"
	default:
		return then.Format(time.DateOnly)
	}
}
