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
	helpOpen      bool
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
	}
}

// WithDryRun is the interface holding back every write — to Jira, the forge,
// Slack, git and files — and saying instead what it would have done. Reads stay
// live, so what it says is about the real state of things.
func (m Model) WithDryRun() Model {
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
	m.notice = ""

	switch {
	case key.Matches(msg, m.keys.interrupt):
		return m, tea.Quit
	case m.overlay != nil:
		return m.overlay.handleKey(m, msg)
	case m.helpOpen:
		return m.handleHelpKey(msg)
	default:
		return m.handleGlobalKey(msg)
	}
}

// handleHelpKey answers a key while the help is open, which scrolls on a
// terminal too short to show every key at once.
func (m Model) handleHelpKey(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.quit):
		return m, tea.Quit
	case key.Matches(msg, m.keys.toggleHelp, m.keys.closeOverlay):
		m.helpOpen, m.scroll = false, 0
	case key.Matches(msg, m.keys.scrollDown, m.keys.down):
		m.scroll += m.halfPage()
	case key.Matches(msg, m.keys.scrollUp, m.keys.up):
		m.scroll = max(0, m.scroll-m.halfPage())
	}

	return m, nil
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
		m.helpOpen, m.scroll = true, 0
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
		m.scroll += m.halfPage()
	case key.Matches(msg, m.keys.scrollUp):
		m.scroll = max(0, m.scroll-m.halfPage())
	default:
		return behaviorOf(m.focus).handle(m, msg)
	}

	return m, nil
}

// halfPage is how far a scroll key moves the detail.
func (m Model) halfPage() int {
	return max(1, m.detailRows()/2) //nolint:mnd // half, as in half a page
}

// focusOn moves focus to a pane, with its detail scrolled to the top.
func (m Model) focusOn(target pane) Model {
	m.focus, m.scroll = target, 0

	return m
}

// toggleMouse gives the terminal its own click-drag selection back, or takes it
// again. Capture is on by default and breaks copying text out of the screen,
// which is why this exists.
func (m Model) toggleMouse() (Model, tea.Cmd) {
	m.mouse = !m.mouse
	if m.mouse {
		return m, tea.EnableMouseCellMotion
	}

	return m, tea.DisableMouse
}

// shape is the layout for the terminal as it is now.
func (m Model) shape() layout.Layout {
	return layout.Compute(m.width, m.height, paneCount, int(m.focus))
}
