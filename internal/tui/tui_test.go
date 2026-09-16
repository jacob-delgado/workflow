// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

import (
	"errors"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/tui"
)

// errUnreadable stands in for a config file that exists but cannot be read.
var errUnreadable = errors.New("permission denied")

// completeConfig is a configuration with nothing missing.
func completeConfig() config.Config {
	return config.Config{
		Jira:  config.Jira{BaseURL: "https://jira.example.com", Token: "t", User: ""},
		Slack: config.Slack{Token: "xoxb-t", Channel: "#dev"},
		Path:  "/home/example/.workflow.json",
	}
}

func TestViewShowsTheLoadedConfiguration(t *testing.T) {
	t.Parallel()

	view := tui.New(completeConfig(), nil).View()

	wants := []string{"workflow", "jira.example.com", "#dev", "bearer token", "press q to quit"}
	for _, want := range wants {
		if !strings.Contains(view, want) {
			t.Errorf("view does not mention %q:\n%s", want, view)
		}
	}
}

func TestViewNeverShowsAToken(t *testing.T) {
	t.Parallel()

	cfg := completeConfig()
	cfg.Jira.Token = "jira-secret-1111"
	cfg.Slack.Token = "xoxb-secret-2222"

	view := tui.New(cfg, nil).View()

	for _, secret := range []string{cfg.Jira.Token, cfg.Slack.Token} {
		if strings.Contains(view, secret) {
			t.Errorf("view leaked %q:\n%s", secret, view)
		}
	}
}

func TestViewNamesMissingFields(t *testing.T) {
	t.Parallel()

	cfg := completeConfig()
	cfg.Slack.Channel = ""

	view := tui.New(cfg, nil).View()

	if !strings.Contains(view, "slack.channel") {
		t.Errorf("view does not name the missing field:\n%s", view)
	}
}

func TestViewExplainsAMissingConfiguration(t *testing.T) {
	t.Parallel()

	view := tui.New(config.Config{}, config.ErrNotFound).View()

	if !strings.Contains(view, "config init") {
		t.Errorf("view does not say how to create a config:\n%s", view)
	}
}

func TestViewReportsAnUnreadableConfiguration(t *testing.T) {
	t.Parallel()

	view := tui.New(config.Config{}, errUnreadable).View()

	if !strings.Contains(view, "permission denied") {
		t.Errorf("view does not report the error:\n%s", view)
	}
}

func TestQuitKeysQuit(t *testing.T) {
	t.Parallel()

	for _, key := range []string{"q", "esc", "ctrl+c"} {
		t.Run(key, func(t *testing.T) {
			t.Parallel()

			model := tui.New(completeConfig(), nil)

			_, cmd := model.Update(keyMsg(key))
			if cmd == nil {
				t.Fatalf("%q did not quit", key)
			}

			if _, isQuit := cmd().(tea.QuitMsg); !isQuit {
				t.Errorf("%q produced %T, want tea.QuitMsg", key, cmd())
			}
		})
	}
}

func TestOtherKeysDoNotQuit(t *testing.T) {
	t.Parallel()

	model := tui.New(completeConfig(), nil)

	_, cmd := model.Update(keyMsg("x"))
	if cmd != nil {
		t.Errorf("x produced a command, want none")
	}
}

func TestNonKeyMessagesAreIgnored(t *testing.T) {
	t.Parallel()

	model := tui.New(completeConfig(), nil)

	_, cmd := model.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	if cmd != nil {
		t.Errorf("a window resize produced a command, want none")
	}
}

func TestInitDoesNothing(t *testing.T) {
	t.Parallel()

	if cmd := tui.New(completeConfig(), nil).Init(); cmd != nil {
		t.Errorf("Init returned a command, want none")
	}
}

// keyMsg builds the key message bubbletea delivers for a key name.
func keyMsg(key string) tea.KeyMsg {
	switch key {
	case "esc":
		return tea.KeyMsg{Type: tea.KeyEsc}
	case "ctrl+c":
		return tea.KeyMsg{Type: tea.KeyCtrlC}
	default:
		return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)}
	}
}
