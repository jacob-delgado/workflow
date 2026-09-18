// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

// Package tui renders workflow's terminal interface: a rail of panes down the
// left, a detail pane beside it, the loop's stages across the top and the keys
// along the bottom.
package tui

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"

	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/tui/layout"
)

// defaultWidth and defaultHeight stand in until the terminal reports its real
// size, which Bubble Tea sends straight after start.
const (
	defaultWidth  = 100
	defaultHeight = 30
)

// Model satisfies tea.Model with value receivers, so the assertion uses a value
// rather than a pointer — and every method on it stays a value receiver, which
// recvcheck enforces.
var _ tea.Model = Model{}

// Model is the interface's state.
type Model struct {
	cfg     config.Config
	loadErr error
	deps    Deps
	keys    keyMap
	styles  styles
	marks   glyphs

	width, height int
	focus         pane
	mouse         bool
	dryRun        bool
	// scroll is how far the detail pane is scrolled; moving to another issue or
	// pane starts it at the top again.
	scroll int
	// runs counts the programs started, so the output of one run is never
	// shown in another.
	runs int
	// draft is the commit message last composed and not yet committed.
	draft commitDraft
	// prDraft is the pull request last composed and not yet opened, kept per
	// branch so a failed push or an esc does not lose it.
	prDraft prDraft
	// vocab is what a change is called on this forge, pull request or merge
	// request, fixed for the session by the forge the remote points at.
	vocab reviewVocab

	// overlay takes the keyboard while it is open; nil when none is.
	overlay overlay
	// notice is the footer's one-line report of something that just happened.
	// The next key press clears it.
	notice string

	// views are the named issue lists the Issues pane moves between, and
	// viewIndex is the one it shows now.
	views     []issueView
	viewIndex int

	issues  issueList
	detail  issueDetail
	branch  branchState
	changes changeList
	review  reviewState
	slack   slackState
	hookgen hookgenState
}

// New builds the interface for a configuration, the error if any from loading
// it, and what it may ask of the world outside.
func New(cfg config.Config, loadErr error, deps Deps) Model {
	marks := unicodeGlyphs()
	if cfg.UI.ASCII {
		marks = asciiGlyphs()
	}

	vocab := forgeVocab(deps.Forge.Kind)

	return Model{
		cfg: cfg, loadErr: loadErr, deps: deps,
		keys: newKeyMap(marks, vocab.noun), styles: newStyles(true), marks: marks, vocab: vocab,
		width: defaultWidth, height: defaultHeight,
		focus: paneIssues, mouse: cfg.UI.Mouse,
		views: issueViews(cfg.Jira.Views), viewIndex: 0,
	}
}

// WithDryRun is the interface holding back every write — to Jira, the forge,
// Slack, git and files — and saying instead what it would have done. Reads stay
// live, so what it says is about the real state of things.
func (m Model) WithDryRun() Model {
	m.deps = heldBack(m.deps)
	m.dryRun = true

	return m
}

// WithoutColor draws no hue while keeping the bold, faint and reverse that carry
// meaning without it.
func (m Model) WithoutColor() Model {
	m.styles = newStyles(false)

	return m
}

// Run starts the interface and blocks until the user quits. The context cancels
// the program, so a caller can shut the interface down.
func Run(ctx context.Context, model Model, out io.Writer) error {
	// NO_COLOR and ui.color "never" drop the hues, but not the bold, faint and
	// reverse-video cursor that carry meaning without them: force a profile that
	// keeps those, then strip the hues at the style level.
	if !model.cfg.UI.DrawColor(os.Getenv("NO_COLOR")) {
		lipgloss.SetColorProfile(termenv.ANSI)

		model = model.WithoutColor()
	}

	options := []tea.ProgramOption{
		tea.WithOutput(out),
		tea.WithContext(ctx),
		// The alternate screen keeps the session from scrolling the terminal,
		// and gives the scrollback back untouched on exit.
		tea.WithAltScreen(),
	}

	if model.mouse {
		options = append(options, tea.WithMouseCellMotion())
	}

	_, err := tea.NewProgram(model, options...).Run()
	if err != nil {
		return fmt.Errorf("running the interface: %w", err)
	}

	return nil
}

// Init implements tea.Model: it starts every load the panes need. Each runs
// inside a command, which Bubble Tea executes off the update loop, so a slow
// service never freezes the screen — and each pane fills in, or fails, on its
// own.
func (m Model) Init() tea.Cmd {
	return tea.Batch(m.searchIssues(), m.loadBranch(), m.loadChanges(), m.findHooks())
}

// Update implements tea.Model. Every load and result is an applier, which knows
// what it changes, so this only routes.
//
//nolint:ireturn // tea.Model is the return type bubbletea's interface requires
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height

		return m, nil
	case tea.KeyMsg:
		return m.handleKey(msg)
	case tea.MouseMsg:
		return m.handleMouse(msg)
	case applier:
		return msg.apply(m)
	default:
		return m, nil
	}
}

// handleKey answers a key press. ctrl+c always quits. Otherwise an open overlay
// has the keyboard — including q, which a text field needs to type — then the
// help, then the keys that work everywhere, then the focused pane's own.
func (m Model) handleKey(msg tea.KeyMsg) (Model, tea.Cmd) {
	// A notice is cleared when the next action starts, not by moving around, so
	// looking about after a result does not erase the record of it.
	if !m.navigates(msg) {
		m.notice = ""
	}

	switch {
	case key.Matches(msg, m.keys.interrupt):
		return m, tea.Quit
	case m.overlay != nil:
		return m.overlay.handleKey(m, msg)
	case m.filteringIssues():
		return m.handleIssueFilterKey(msg)
	default:
		return m.handleGlobalKey(msg)
	}
}

// filteringIssues reports that the Issues pane is capturing keystrokes into its
// filter, which takes every key — q and the digits included — until it closes.
func (m Model) filteringIssues() bool {
	return m.focus == paneIssues && m.issues.filtering
}

// quitOrGuard quits, unless a post is waiting for CI, in which case it asks
// first: quitting would lose the post without a word.
func (m Model) quitOrGuard() (Model, tea.Cmd) {
	if m.slack.pending.waiting() {
		m.overlay = quitGuard{}

		return m, nil
	}

	return m, tea.Quit
}

// handleGlobalKey answers the keys that work in every pane, and hands the rest
// to the focused one.
func (m Model) handleGlobalKey(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.quit):
		return m.quitOrGuard()
	case key.Matches(msg, m.keys.toggleHelp):
		return m.openHelp()
	case key.Matches(msg, m.keys.next):
		return m.focusOn((m.focus + 1) % paneCount), nil
	case key.Matches(msg, m.keys.previous):
		return m.focusOn((m.focus + paneCount - 1) % paneCount), nil
	case key.Matches(msg, m.keys.jump):
		// The binding only matches the digits 1 through 5, so the digit is
		// always a valid pane.
		return m.focusOn(pane(msg.String()[0] - '1')), nil
	case key.Matches(msg, m.keys.toggleMouse):
		return m.toggleMouse()
	case key.Matches(msg, m.keys.scrollDown):
		return m.scrollDetail(m.halfPage()), nil
	case key.Matches(msg, m.keys.scrollUp):
		return m.scrollDetail(-m.halfPage()), nil
	default:
		return behaviorOf(m.focus).handle(m, msg)
	}
}

// halfPage is how far a scroll key moves the detail.
func (m Model) halfPage() int {
	return max(1, m.detailRows()/2) //nolint:mnd // half, as in half a page
}

// scrollDetail moves the detail by delta lines and clamps the result to the
// content. Clamping where the offset is written — not only where it is drawn —
// is what stops an over-scroll from stranding the view past the end, so one
// scroll-up moves it rather than undoing offsets the content never had.
func (m Model) scrollDetail(delta int) Model {
	body := behaviorOf(m.focus).detail(m, m.detailWidth())
	maxOffset := max(0, strings.Count(body, "\n")+1-m.detailRows())
	m.scroll = min(max(0, m.scroll+delta), maxOffset)

	return m
}

// focusOn moves focus to a pane, with its detail scrolled to the top. Leaving
// the Issues pane cancels any filter, so it never narrows a list you cannot see.
func (m Model) focusOn(target pane) Model {
	m.focus, m.scroll = target, 0
	m.issues = m.issues.clearFilter()

	return m
}

// toggleMouse gives the terminal its own click-drag selection back, or takes it
// again. Capture is on by default and breaks copying text out of the screen,
// which is why this exists.
func (m Model) toggleMouse() (Model, tea.Cmd) {
	m.mouse = !m.mouse
	if m.mouse {
		return m.noticed("mouse on: clicks focus panes and pick rows"), tea.EnableMouseCellMotion
	}

	return m.noticed("mouse off: your terminal selects text again"), tea.DisableMouse
}

// navigates reports a key that only moves the view, which keeps a notice rather
// than clearing it.
func (m Model) navigates(msg tea.KeyMsg) bool {
	return key.Matches(msg, m.keys.up, m.keys.down, m.keys.scrollUp, m.keys.scrollDown,
		m.keys.next, m.keys.previous, m.keys.jump, m.keys.toggleHelp)
}

// minNoticeHeight is the shortest terminal that gives a notice its own row: a
// spine, a body row, the notice and the footer.
const minNoticeHeight = 4

// showsNotice reports a notice — or the issue filter — that has room for its own
// row above the hints.
func (m Model) showsNotice() bool {
	return (m.notice != "" || m.showsFilter()) && m.height >= minNoticeHeight
}

// showsFilter reports that the Issues pane's filter should be shown on its own
// row, which is whenever it is being typed or is still narrowing the list.
func (m Model) showsFilter() bool {
	return m.focus == paneIssues && (m.issues.filtering || m.issues.filter != "")
}

// shape is the layout for the terminal as it is now, a row shorter when a notice
// takes one above the footer.
func (m Model) shape() layout.Layout {
	if m.showsNotice() {
		result, _ := layout.ComputeWithNotice(m.width, m.height, paneCount, int(m.focus))

		return result
	}

	return layout.Compute(m.width, m.height, paneCount, int(m.focus))
}
