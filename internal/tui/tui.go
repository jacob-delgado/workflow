// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

// Package tui renders workflow's terminal interface: a rail of panes down the
// left, a detail pane beside it, the loop's stages across the top and the keys
// along the bottom.
package tui

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/jira"
	"github.com/jacob-delgado/workflow/internal/tui/layout"
)

// styles is the stub screen's rendering. Lip Gloss degrades to plain text when
// the terminal cannot do color, so there is no capability check here.
type styles struct {
	title  lipgloss.Style
	label  lipgloss.Style
	status lipgloss.Style
	hint   lipgloss.Style
}

// newStyles builds the screen's styles.
func newStyles() styles {
	return styles{
		title:  lipgloss.NewStyle().Bold(true),
		label:  lipgloss.NewStyle().Faint(true),
		status: lipgloss.NewStyle().Bold(true),
		hint:   lipgloss.NewStyle().Faint(true).Italic(true),
	}
}

// pane is one panel in the rail.
type pane int

const (
	paneIssues pane = iota
	paneBranch
	paneCommits
	paneReview
	paneSlack
)

// paneCount is untyped on purpose: typed as pane, the exhaustive linter would
// count it as a member and demand a case for it in every switch.
const paneCount = 5

// defaultWidth and defaultHeight stand in until the terminal reports its real
// size, which Bubble Tea sends straight after start.
const (
	defaultWidth  = 100
	defaultHeight = 30
)

// title names a pane. A lookup rather than a switch, because a switch over every
// pane leaves a final arm that can never be false.
func (p pane) title() string {
	titles := [paneCount]string{"Issues", "Branch", "Commits", "Review", "Slack"}

	return titles[p]
}

// label is the title with the number that jumps to it.
func (p pane) label() string {
	return strconv.Itoa(int(p)+1) + " " + p.title()
}

// IssueSearch finds the issues assigned to the user. It is a function rather than
// a client so the model never holds a context or a credential, and so a test can
// hand it canned answers without a network.
type IssueSearch func() (jira.SearchResult, error)

// Model satisfies tea.Model with value receivers, so the assertion uses a value
// rather than a pointer — and every method on it stays a value receiver, which
// recvcheck enforces.
var _ tea.Model = Model{}

// Model is the interface's state.
type Model struct {
	cfg      config.Config
	loadErr  error
	keys     keyMap
	styles   styles
	width    int
	height   int
	focus    pane
	helpOpen bool
	mouse    bool
	search   IssueSearch
	issues   issueList
}

// New builds the interface for a configuration, the error if any from loading
// it, and the search that fills the Issues pane.
func New(cfg config.Config, loadErr error, search IssueSearch) Model {
	return Model{
		cfg:      cfg,
		loadErr:  loadErr,
		keys:     newKeyMap(),
		styles:   newStyles(),
		width:    defaultWidth,
		height:   defaultHeight,
		focus:    paneIssues,
		helpOpen: false,
		mouse:    true,
		search:   search,
		issues:   issueList{found: jira.SearchResult{Issues: nil, Total: 0}, err: nil, settled: false, selected: 0},
	}
}

// Run starts the interface and blocks until the user quits. The context cancels
// the program, so a caller can shut the interface down.
func Run(ctx context.Context, model Model, out io.Writer) error {
	program := tea.NewProgram(model,
		tea.WithOutput(out),
		tea.WithContext(ctx),
		// The alternate screen keeps the session from scrolling the terminal,
		// and gives the scrollback back untouched on exit.
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)

	_, err := program.Run()
	if err != nil {
		return fmt.Errorf("running the interface: %w", err)
	}

	return nil
}

// Init implements tea.Model: it starts the issue search. The search runs inside
// the returned command, which Bubble Tea executes off the update loop, so a slow
// Jira never freezes the screen.
func (m Model) Init() tea.Cmd {
	if m.search == nil {
		return nil
	}

	search := m.search

	return func() tea.Msg {
		found, err := search()

		return issuesLoaded{found: found, err: err}
	}
}

// Update implements tea.Model.
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
		return m.handleMouse(msg), nil
	case issuesLoaded:
		m.issues = m.issues.settle(msg)

		return m, nil
	default:
		return m, nil
	}
}

// handleKey answers a key press.
func (m Model) handleKey(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.quit):
		return m, tea.Quit
	case key.Matches(msg, m.keys.toggleHelp):
		m.helpOpen = !m.helpOpen
	case key.Matches(msg, m.keys.closeOverlay):
		m.helpOpen = false
	case key.Matches(msg, m.keys.next):
		m.focus = (m.focus + 1) % paneCount
	case key.Matches(msg, m.keys.previous):
		m.focus = (m.focus + paneCount - 1) % paneCount
	case key.Matches(msg, m.keys.jump):
		// The binding only matches the digits 1 through 5, so the digit is
		// always a valid pane.
		m.focus = pane(msg.String()[0] - '1')
	case key.Matches(msg, m.keys.toggleMouse):
		return m.toggleMouse()
	default:
		return m.handlePaneKey(msg), nil
	}

	return m, nil
}

// handlePaneKey gives the focused pane the keys the rail did not claim. Only the
// Issues pane has any yet.
func (m Model) handlePaneKey(msg tea.KeyMsg) Model {
	if m.focus != paneIssues {
		return m
	}

	switch {
	case key.Matches(msg, m.keys.down):
		m.issues = m.issues.move(1)
	case key.Matches(msg, m.keys.up):
		m.issues = m.issues.move(-1)
	}

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

// handleMouse focuses the rail pane under a left click.
func (m Model) handleMouse(msg tea.MouseMsg) Model {
	if msg.Action != tea.MouseActionPress || msg.Button != tea.MouseButtonLeft {
		return m
	}

	index, ok := layout.Compute(m.width, m.height, paneCount).RailAt(msg.X, msg.Y)
	if ok {
		m.focus = pane(index)
	}

	return m
}

// status describes the configuration this session is running with.
func (m Model) status() string {
	if m.loadErr != nil {
		return m.configErrorStatus()
	}

	label := m.styles.label

	lines := []string{
		label.Render("config ") + m.cfg.Path,
		label.Render("jira   ") + describe(config.RedactURL(m.cfg.Jira.BaseURL)) +
			label.Render(" · "+m.cfg.Jira.AuthMode().String()),
		label.Render("slack  ") + m.cfg.Slack.Target() +
			label.Render(" · "+m.cfg.Slack.Mode().String()),
	}

	missing := m.cfg.Missing()
	if len(missing) > 0 {
		lines = append(lines,
			"",
			m.styles.status.Render("incomplete: ")+strings.Join(missing, ", "),
			label.Render("run `workflow doctor` for detail"),
		)
	}

	return strings.Join(lines, "\n")
}

// configErrorStatus renders the screen shown when no configuration loaded.
func (m Model) configErrorStatus() string {
	if errors.Is(m.loadErr, config.ErrNotFound) {
		return m.styles.status.Render("no "+config.FileName+" found") + "\n" +
			m.styles.label.Render("create one with `workflow config init`")
	}

	return m.styles.status.Render("configuration error") + "\n" +
		m.styles.label.Render(m.loadErr.Error())
}

// describe renders an unset value as something a reader can act on.
func describe(value string) string {
	if value == "" {
		return "(not set)"
	}

	return value
}
