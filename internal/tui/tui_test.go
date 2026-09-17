// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

import (
	"errors"
	"strconv"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/tui"
)

// devChannel is the Slack channel the fixtures post to.
const devChannel = "#dev"

// errUnreadable stands in for a config file that exists but cannot be read.
var errUnreadable = errors.New("permission denied")

// completeConfig is a configuration with nothing missing.
func completeConfig() config.Config {
	return config.Config{
		Jira:  config.Jira{BaseURL: "https://jira.example.com", Token: "t", User: ""},
		Slack: config.Slack{Token: "xoxb-t", WebhookURL: "", Channel: devChannel},
		Path:  "/home/example/.workflow.json",
	}
}

func TestViewShowsTheLoadedConfiguration(t *testing.T) {
	t.Parallel()

	// Act
	view := tui.New(completeConfig(), nil, tui.Deps{}).View()

	// Assert
	requireScreen(t, view, "workflow", "jira.example.com", devChannel, "bearer token", "quit")
}

func TestViewNeverShowsACredential(t *testing.T) {
	t.Parallel()

	tokens := completeConfig()
	tokens.Jira.Token = "jira-secret-1111"
	tokens.Slack.Token = "xoxb-secret-2222"
	tokens.Slack.WebhookURL = "https://hooks.slack.com/services/T0/B0/secret3333"

	// jira.base_url can carry userinfo. doctor masks it; this screen printed it
	// verbatim in the detail pane, which is the same leak in a second place.
	password := completeConfig()
	password.Jira.BaseURL = "https://alice:hunter2@jira.example.com"

	cases := map[string]struct {
		cfg    config.Config
		hidden []string
	}{
		"tokens and a webhook":        {cfg: tokens, hidden: []string{"jira-secret-1111", "xoxb-secret-2222", "secret3333"}},
		"a password in jira.base_url": {cfg: password, hidden: []string{"hunter2"}},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Act
			view := tui.New(tt.cfg, nil, tui.Deps{}).View()

			// Assert
			// The host stays: masking a credential must not lose where it goes.
			refuseScreen(t, view, tt.hidden...)
			requireScreen(t, view, "jira.example.com")
		})
	}
}

func TestViewSaysWhatTheConfigurationStillNeeds(t *testing.T) {
	t.Parallel()

	noChannel := completeConfig()
	noChannel.Slack.Channel = ""

	noJiraURL := completeConfig()
	noJiraURL.Jira.BaseURL = ""

	cases := map[string]struct {
		cfg     config.Config
		loadErr error
		want    string
	}{
		"a missing field is named":       {cfg: noChannel, want: "slack.channel"},
		"no file says how to create one": {cfg: config.Config{}, loadErr: config.ErrNotFound, want: "config init"},
		"an unreadable file says why":    {cfg: config.Config{}, loadErr: errUnreadable, want: "permission denied"},
		// An empty value must read as a thing to do, not as a blank the eye skips.
		"an unset Jira URL is marked": {cfg: noJiraURL, want: "(not set)"},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Act
			view := tui.New(tt.cfg, tt.loadErr, tui.Deps{}).View()

			// Assert
			requireScreen(t, view, tt.want)
		})
	}
}

func TestQuitKeysQuit(t *testing.T) {
	t.Parallel()

	for _, key := range []string{"q", "ctrl+c"} {
		t.Run(key, func(t *testing.T) {
			t.Parallel()

			// Arrange
			model := tui.New(completeConfig(), nil, tui.Deps{})

			// Act
			_, cmd := model.Update(keyMsg(key))

			// Assert
			if cmd == nil {
				t.Fatalf("%q did not quit", key)
			}

			if msg, isQuit := cmd().(tea.QuitMsg); !isQuit {
				t.Errorf("%q produced %T, want tea.QuitMsg", key, msg)
			}
		})
	}
}

func TestMessagesThatNeedNoWorkProduceNoCommand(t *testing.T) {
	t.Parallel()

	cases := map[string]tea.Msg{
		"a key that means nothing here": keyMsg("x"),
		// A resize is recorded, but there is nothing to go and do about it.
		"a window resize": tea.WindowSizeMsg{Width: 80, Height: 24},
	}

	for name, msg := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			model := tui.New(completeConfig(), nil, tui.Deps{})

			// Act
			_, cmd := model.Update(msg)

			// Assert
			if cmd != nil {
				t.Errorf("%s produced a command, want none", name)
			}
		})
	}
}

func TestInitDoesNothingWithNothingToLoad(t *testing.T) {
	t.Parallel()

	// Act & Assert
	if cmd := tui.New(completeConfig(), nil, tui.Deps{}).Init(); cmd != nil {
		t.Errorf("Init returned a command, want none")
	}
}

// keyMsg builds the key message bubbletea delivers for a key name.
func keyMsg(key string) tea.KeyMsg {
	named := map[string]tea.KeyType{
		keyEsc: tea.KeyEsc, "ctrl+c": tea.KeyCtrlC, "tab": tea.KeyTab, "shift+tab": tea.KeyShiftTab,
		"down": tea.KeyDown, "up": tea.KeyUp, "left": tea.KeyLeft, keyRight: tea.KeyRight,
		"enter": tea.KeyEnter, keySpace: tea.KeySpace, "backspace": tea.KeyBackspace,
		"pgdown": tea.KeyPgDown, "pgup": tea.KeyPgUp,
		"ctrl+e": tea.KeyCtrlE, "ctrl+t": tea.KeyCtrlT, "ctrl+d": tea.KeyCtrlD,
	}

	if kind, ok := named[key]; ok {
		return tea.KeyMsg{Type: kind}
	}

	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)}
}

// concrete recovers the model Update returned. Update's signature is fixed by
// tea.Model, but the tests work with the concrete type so that a helper never
// hands back an interface.
func concrete(t *testing.T, model tea.Model) tui.Model {
	t.Helper()

	typed, ok := model.(tui.Model)
	if !ok {
		t.Fatalf("Update returned %T, want tui.Model", model)
	}

	return typed
}

// press sends keys in order and returns the resulting model.
func press(t *testing.T, model tui.Model, keys ...string) tui.Model {
	t.Helper()

	for _, key := range keys {
		next, _ := model.Update(keyMsg(key))
		model = concrete(t, next)
	}

	return model
}

// sized delivers the window size a real terminal sends at startup.
func sized(t *testing.T, model tui.Model, width, height int) tui.Model {
	t.Helper()

	next, _ := model.Update(tea.WindowSizeMsg{Width: width, Height: height})

	return concrete(t, next)
}

func TestViewAcceptsAWebhookWithoutAChannel(t *testing.T) {
	t.Parallel()

	// Arrange
	cfg := completeConfig()
	cfg.Slack.Token = ""
	cfg.Slack.Channel = ""
	cfg.Slack.WebhookURL = "https://hooks.slack.com/services/T0/B0/secretpayload"

	// Act
	view := tui.New(cfg, nil, tui.Deps{}).View()

	// Assert
	// A webhook carries its own channel, so this configuration is complete and
	// the screen must not call it incomplete.
	refuseScreen(t, view, "incomplete", "hooks.slack.com", "secretpayload")
	requireScreen(t, view, "incoming webhook")
}

func TestANarrowTerminalCollapsesTheRail(t *testing.T) {
	t.Parallel()

	// Act
	view := sized(t, tui.New(completeConfig(), nil, tui.Deps{}), 80, 30).View()

	// Assert
	// Focus still means something: the detail is titled with the focused pane.
	refuseScreen(t, view, "1 Issues")
	requireScreen(t, view, "Issues")
}

func TestViewFitsTheTerminal(t *testing.T) {
	t.Parallel()

	for _, size := range [][2]int{{120, 40}, {90, 24}, {80, 30}, {200, 60}} {
		t.Run(strconv.Itoa(size[0])+"x"+strconv.Itoa(size[1]), func(t *testing.T) {
			t.Parallel()

			// Act
			rows := strings.Split(sized(t, tui.New(completeConfig(), nil, tui.Deps{}), size[0], size[1]).View(), "\n")

			// Assert
			if len(rows) > size[1] {
				t.Errorf("rendered %d rows", len(rows))
			}

			for index, row := range rows {
				if width := lipgloss.Width(row); width > size[0] {
					t.Errorf("row %d is %d cells wide", index, width)
				}
			}
		})
	}
}

// unrelated is a message no part of the interface answers to.
type unrelated struct{}

func TestAMessageNothingHandlesChangesNothing(t *testing.T) {
	t.Parallel()

	// Arrange
	model := sized(t, tui.New(completeConfig(), nil, tui.Deps{}), 120, 40)
	before := model.View()

	// Act
	after, cmd := model.Update(unrelated{})

	// Assert
	if cmd != nil {
		t.Error("an unrelated message produced a command")
	}

	if concrete(t, after).View() != before {
		t.Error("an unrelated message changed the screen")
	}
}
