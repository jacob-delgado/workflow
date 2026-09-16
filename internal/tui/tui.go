// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

// Package tui renders workflow's terminal interface.
//
// This is a stub: it reports what configuration was found and quits. It exists
// so the program runs end to end — config loading, rendering, and teardown —
// while the Jira, Slack, and forge integrations are built behind it.
package tui

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/jacob-delgado/workflow/internal/config"
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

// Model satisfies tea.Model with value receivers, so the assertion uses a value
// rather than a pointer.
var _ tea.Model = Model{}

// Model is the stub screen's state.
type Model struct {
	cfg     config.Config
	loadErr error
	styles  styles
	quit    bool
}

// New builds the stub screen for a configuration and the error, if any, from
// loading it.
func New(cfg config.Config, loadErr error) Model {
	return Model{cfg: cfg, loadErr: loadErr, styles: newStyles(), quit: false}
}

// Run starts the interface and blocks until the user quits. The context cancels
// the program, so a caller can shut the interface down.
func Run(ctx context.Context, cfg config.Config, loadErr error, out io.Writer) error {
	program := tea.NewProgram(New(cfg, loadErr), tea.WithOutput(out), tea.WithContext(ctx))

	_, err := program.Run()
	if err != nil {
		return fmt.Errorf("running the interface: %w", err)
	}

	return nil
}

// Init implements tea.Model. The stub has nothing to start.
func (m Model) Init() tea.Cmd {
	return nil
}

// Update implements tea.Model: any of q, esc, or ctrl+c quits.
//
//nolint:ireturn // tea.Model is the return type bubbletea's interface requires
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}

	switch keyMsg.String() {
	case "q", "esc", "ctrl+c":
		m.quit = true

		return m, tea.Quit
	default:
		return m, nil
	}
}

// View implements tea.Model.
func (m Model) View() string {
	sections := []string{
		m.styles.title.Render("workflow"),
		m.status(),
		m.styles.hint.Render("press q to quit"),
	}

	return strings.Join(sections, "\n\n") + "\n"
}

// status describes the configuration this session is running with.
func (m Model) status() string {
	if m.loadErr != nil {
		return m.configErrorStatus()
	}

	label := m.styles.label

	lines := []string{
		label.Render("config ") + m.cfg.Path,
		label.Render("jira   ") + describe(m.cfg.Jira.BaseURL) +
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
