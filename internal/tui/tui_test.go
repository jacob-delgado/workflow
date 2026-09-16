// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

import (
	"errors"
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

	view := tui.New(completeConfig(), nil, nil).View()

	wants := []string{"workflow", "jira.example.com", devChannel, "bearer token", "quit"}
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
	cfg.Slack.WebhookURL = "https://hooks.slack.com/services/T0/B0/secret3333"

	view := tui.New(cfg, nil, nil).View()

	for _, secret := range []string{cfg.Jira.Token, cfg.Slack.Token, cfg.Slack.WebhookURL} {
		if strings.Contains(view, secret) {
			t.Errorf("view leaked %q:\n%s", secret, view)
		}
	}
}

func TestViewNamesMissingFields(t *testing.T) {
	t.Parallel()

	cfg := completeConfig()
	cfg.Slack.Channel = ""

	view := tui.New(cfg, nil, nil).View()

	if !strings.Contains(view, "slack.channel") {
		t.Errorf("view does not name the missing field:\n%s", view)
	}
}

func TestViewExplainsAMissingConfiguration(t *testing.T) {
	t.Parallel()

	view := tui.New(config.Config{}, config.ErrNotFound, nil).View()

	if !strings.Contains(view, "config init") {
		t.Errorf("view does not say how to create a config:\n%s", view)
	}
}

func TestViewReportsAnUnreadableConfiguration(t *testing.T) {
	t.Parallel()

	view := tui.New(config.Config{}, errUnreadable, nil).View()

	if !strings.Contains(view, "permission denied") {
		t.Errorf("view does not report the error:\n%s", view)
	}
}

func TestQuitKeysQuit(t *testing.T) {
	t.Parallel()

	for _, key := range []string{"q", "ctrl+c"} {
		t.Run(key, func(t *testing.T) {
			t.Parallel()

			model := tui.New(completeConfig(), nil, nil)

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

	model := tui.New(completeConfig(), nil, nil)

	_, cmd := model.Update(keyMsg("x"))
	if cmd != nil {
		t.Errorf("x produced a command, want none")
	}
}

func TestNonKeyMessagesAreIgnored(t *testing.T) {
	t.Parallel()

	model := tui.New(completeConfig(), nil, nil)

	_, cmd := model.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	if cmd != nil {
		t.Errorf("a window resize produced a command, want none")
	}
}

func TestInitDoesNothing(t *testing.T) {
	t.Parallel()

	if cmd := tui.New(completeConfig(), nil, nil).Init(); cmd != nil {
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
	case "tab":
		return tea.KeyMsg{Type: tea.KeyTab}
	case "shift+tab":
		return tea.KeyMsg{Type: tea.KeyShiftTab}
	case "down":
		return tea.KeyMsg{Type: tea.KeyDown}
	case "up":
		return tea.KeyMsg{Type: tea.KeyUp}
	default:
		return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)}
	}
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

// focused is the heavy top border of a rail pane, which is how focus is drawn.
func focused(label string) string {
	return "┏━ " + label + " "
}

func TestViewAcceptsAWebhookWithoutAChannel(t *testing.T) {
	t.Parallel()

	cfg := completeConfig()
	cfg.Slack.Token = ""
	cfg.Slack.Channel = ""
	cfg.Slack.WebhookURL = "https://hooks.slack.com/services/T0/B0/secretpayload"

	view := tui.New(cfg, nil, nil).View()

	// A webhook carries its own channel, so this configuration is complete and
	// the screen must not call it incomplete.
	if strings.Contains(view, "incomplete") {
		t.Errorf("view called a webhook-only configuration incomplete:\n%s", view)
	}

	if !strings.Contains(view, "incoming webhook") {
		t.Errorf("view does not name the Slack transport:\n%s", view)
	}

	if strings.Contains(view, "hooks.slack.com") || strings.Contains(view, "secretpayload") {
		t.Errorf("view leaked the webhook URL:\n%s", view)
	}
}

func TestViewMarksAnUnsetJiraURL(t *testing.T) {
	t.Parallel()

	cfg := completeConfig()
	cfg.Jira.BaseURL = ""

	view := tui.New(cfg, nil, nil).View()

	// An empty value must read as a thing to do, not as a blank the eye skips.
	if !strings.Contains(view, "(not set)") {
		t.Errorf("view does not mark the unset Jira URL:\n%s", view)
	}
}

func TestTheFirstPaneStartsWithFocus(t *testing.T) {
	t.Parallel()

	view := sized(t, tui.New(completeConfig(), nil, nil), 120, 40).View()

	if !strings.Contains(view, focused("1 Issues")) {
		t.Errorf("the Issues pane does not start focused:\n%s", view)
	}
}

func TestTabMovesFocusDownTheRail(t *testing.T) {
	t.Parallel()

	view := press(t, sized(t, tui.New(completeConfig(), nil, nil), 120, 40), "tab").View()

	if !strings.Contains(view, focused("2 Branch")) {
		t.Errorf("tab did not move focus to Branch:\n%s", view)
	}

	// Focus moved rather than being added: exactly one pane is heavy.
	if strings.Count(view, "┏") != 1 {
		t.Errorf("found %d focused panes, want exactly one:\n%s", strings.Count(view, "┏"), view)
	}
}

func TestFocusWrapsAtBothEndsOfTheRail(t *testing.T) {
	t.Parallel()

	start := sized(t, tui.New(completeConfig(), nil, nil), 120, 40)

	// shift+tab from the first pane lands on the last rather than stopping.
	if view := press(t, start, "shift+tab").View(); !strings.Contains(view, focused("5 Slack")) {
		t.Errorf("shift+tab from Issues did not wrap to Slack:\n%s", view)
	}

	// Five tabs is a full lap.
	if view := press(t, start, "tab", "tab", "tab", "tab", "tab").View(); !strings.Contains(view, focused("1 Issues")) {
		t.Errorf("five tabs did not come back to Issues:\n%s", view)
	}
}

func TestNumberKeysJumpStraightToAPane(t *testing.T) {
	t.Parallel()

	view := press(t, sized(t, tui.New(completeConfig(), nil, nil), 120, 40), "4").View()

	if !strings.Contains(view, focused("4 Review")) {
		t.Errorf("4 did not jump to Review:\n%s", view)
	}
}

func TestClickingARailPaneFocusesIt(t *testing.T) {
	t.Parallel()

	model := sized(t, tui.New(completeConfig(), nil, nil), 120, 40)

	// At 120x40 the third rail pane spans rows 17 through 24.
	clicked, _ := model.Update(tea.MouseMsg{
		X:      5,
		Y:      20,
		Action: tea.MouseActionPress,
		Button: tea.MouseButtonLeft,
	})

	if view := concrete(t, clicked).View(); !strings.Contains(view, focused("3 Commits")) {
		t.Errorf("clicking the Commits pane did not focus it:\n%s", view)
	}
}

func TestOnlyALeftClickOnTheRailMovesFocus(t *testing.T) {
	t.Parallel()

	model := sized(t, tui.New(completeConfig(), nil, nil), 120, 40)

	for name, msg := range map[string]tea.MouseMsg{
		"a release":         {X: 5, Y: 20, Action: tea.MouseActionRelease, Button: tea.MouseButtonLeft},
		"a right click":     {X: 5, Y: 20, Action: tea.MouseActionPress, Button: tea.MouseButtonRight},
		"a click on detail": {X: 80, Y: 20, Action: tea.MouseActionPress, Button: tea.MouseButtonLeft},
	} {
		after, _ := model.Update(msg)

		if view := after.View(); !strings.Contains(view, focused("1 Issues")) {
			t.Errorf("%s moved focus off Issues:\n%s", name, view)
		}
	}
}

func TestHelpShowsEveryKeyAndEscapeClosesIt(t *testing.T) {
	t.Parallel()

	start := sized(t, tui.New(completeConfig(), nil, nil), 120, 40)

	open := press(t, start, "?").View()
	for _, want := range []string{"shift+tab", "toggle mouse", "jump to pane"} {
		if !strings.Contains(open, want) {
			t.Errorf("help does not mention %q:\n%s", want, open)
		}
	}

	closed := press(t, start, "?", "esc").View()
	if strings.Contains(closed, "toggle mouse") {
		t.Errorf("esc did not close help:\n%s", closed)
	}

	// ? is a toggle as well.
	if toggled := press(t, start, "?", "?").View(); strings.Contains(toggled, "toggle mouse") {
		t.Errorf("a second ? did not close help:\n%s", toggled)
	}
}

func TestEscapeDoesNotQuit(t *testing.T) {
	t.Parallel()

	// In a pane interface esc backs out of an overlay. Quitting on it throws
	// away a session to a key pressed out of habit.
	_, cmd := tui.New(completeConfig(), nil, nil).Update(keyMsg("esc"))
	if cmd != nil {
		if _, isQuit := cmd().(tea.QuitMsg); isQuit {
			t.Error("esc quit the interface")
		}
	}
}

func TestMouseKeyTogglesCapture(t *testing.T) {
	t.Parallel()

	// Capture breaks the terminal's own click-drag text selection, which is why
	// there is a key to give it back.
	model, off := tui.New(completeConfig(), nil, nil).Update(keyMsg("m"))
	if off == nil {
		t.Fatal("m returned no command, want one releasing the mouse")
	}

	_, on := model.Update(keyMsg("m"))
	if on == nil {
		t.Error("a second m returned no command, want one capturing the mouse again")
	}
}

func TestANarrowTerminalCollapsesTheRail(t *testing.T) {
	t.Parallel()

	view := sized(t, tui.New(completeConfig(), nil, nil), 80, 30).View()

	if strings.Contains(view, "1 Issues") {
		t.Errorf("the rail survived at 80 columns:\n%s", view)
	}

	// Focus still means something: the detail is titled with the focused pane.
	if !strings.Contains(view, "Issues") {
		t.Errorf("the collapsed view does not say which pane it is showing:\n%s", view)
	}
}

func TestViewFitsTheTerminal(t *testing.T) {
	t.Parallel()

	for _, size := range [][2]int{{120, 40}, {90, 24}, {80, 30}, {200, 60}} {
		view := sized(t, tui.New(completeConfig(), nil, nil), size[0], size[1]).View()
		rows := strings.Split(view, "\n")

		if len(rows) > size[1] {
			t.Errorf("%dx%d: rendered %d rows", size[0], size[1], len(rows))
		}

		for index, row := range rows {
			if width := lipgloss.Width(row); width > size[0] {
				t.Errorf("%dx%d: row %d is %d cells wide", size[0], size[1], index, width)
			}
		}
	}
}

// unrelated is a message no part of the interface answers to.
type unrelated struct{}

func TestAMessageNothingHandlesChangesNothing(t *testing.T) {
	t.Parallel()

	model := sized(t, tui.New(completeConfig(), nil, nil), 120, 40)
	before := model.View()

	after, cmd := model.Update(unrelated{})
	if cmd != nil {
		t.Error("an unrelated message produced a command")
	}

	if concrete(t, after).View() != before {
		t.Error("an unrelated message changed the screen")
	}
}

func TestViewNeverShowsAPasswordFromTheJiraURL(t *testing.T) {
	t.Parallel()

	// jira.base_url can carry userinfo. doctor masks it; this screen printed it
	// verbatim in the detail pane, which is the same leak in a second place.
	cfg := completeConfig()
	cfg.Jira.BaseURL = "https://alice:hunter2@jira.example.com"

	view := tui.New(cfg, nil, nil).View()

	if strings.Contains(view, "hunter2") {
		t.Errorf("the view printed the password from jira.base_url:\n%s", view)
	}

	if !strings.Contains(view, "jira.example.com") {
		t.Errorf("the view lost the host while masking the password:\n%s", view)
	}
}
