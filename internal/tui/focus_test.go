// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

import (
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/jacob-delgado/workflow/internal/tui"
)

// issuesPane is the first rail pane, which starts with focus.
const issuesPane = "1 Issues"

// focused is the heavy top border of a rail pane, which is how focus is drawn.
func focused(label string) string {
	return "┏━ " + label + " "
}

// fresh is an interface with nothing loaded, sized for a roomy terminal.
func fresh(t *testing.T) tui.Model {
	t.Helper()

	return sized(t, tui.New(completeConfig(), nil, tui.Deps{}), 120, 40)
}

func TestKeysMoveFocusAlongTheRail(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		keys []string
		want string
	}{
		"nothing pressed leaves the first pane focused": {keys: nil, want: issuesPane},
		"tab moves down": {keys: []string{keyTab}, want: "2 Branch"},
		// shift+tab from the first pane lands on the last rather than stopping.
		"shift+tab from the first wraps to the last": {keys: []string{keyShiftTab}, want: "5 Slack"},
		"five tabs is a full lap": {
			keys: []string{keyTab, keyTab, keyTab, keyTab, keyTab}, want: issuesPane,
		},
		"a number jumps straight to its pane": {keys: []string{"4"}, want: "4 Review"},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Act
			view := press(t, fresh(t), tt.keys...).View()

			// Assert
			// Focus moves rather than being added: exactly one pane is heavy.
			requireScreen(t, view, focused(tt.want))

			if heavy := strings.Count(view, "┏"); heavy != 1 {
				t.Errorf("found %d focused panes, want exactly one:\n%s", heavy, view)
			}
		})
	}
}

func TestALeftClickOnTheRailFocusesThatPane(t *testing.T) {
	t.Parallel()

	// At 120x40, with Issues focused and so taking the spare height, the third
	// rail pane spans rows 27 through 30.
	cases := map[string]struct {
		msg  tea.MouseMsg
		want string
	}{
		// The Commits pane's heavy border is on its detail, where the cursor is,
		// so focus shows as the detail title rather than the rail's.
		"a left click on Commits": {
			msg: tea.MouseMsg{X: 5, Y: 28, Action: tea.MouseActionPress, Button: tea.MouseButtonLeft}, want: "Commits",
		},
		"a release": {
			msg: tea.MouseMsg{X: 5, Y: 28, Action: tea.MouseActionRelease, Button: tea.MouseButtonLeft}, want: issuesPane,
		},
		"a right click": {
			msg: tea.MouseMsg{X: 5, Y: 28, Action: tea.MouseActionPress, Button: tea.MouseButtonRight}, want: issuesPane,
		},
		"a click on the detail": {
			msg: tea.MouseMsg{X: 80, Y: 28, Action: tea.MouseActionPress, Button: tea.MouseButtonLeft}, want: issuesPane,
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Act
			after, _ := fresh(t).Update(tt.msg)

			// Assert
			requireScreen(t, after.View(), focused(tt.want))
		})
	}
}

func TestHelpShowsEveryKey(t *testing.T) {
	t.Parallel()

	// Act
	view := press(t, fresh(t), "?").View()

	// Assert
	requireScreen(t, view, keyShiftTab, "toggle mouse", "jump to pane")
}

func TestHelpClosesOnEscapeOrASecondQuestionMark(t *testing.T) {
	t.Parallel()

	for _, key := range []string{keyEsc, "?"} {
		t.Run(key, func(t *testing.T) {
			t.Parallel()

			// Arrange
			open := press(t, fresh(t), "?")
			requireScreen(t, open.View(), "toggle mouse")

			// Act
			closed := press(t, open, key).View()

			// Assert
			refuseScreen(t, closed, "toggle mouse")
		})
	}
}

func TestEscapeDoesNotQuit(t *testing.T) {
	t.Parallel()

	// Act
	// In a pane interface esc backs out of an overlay. Quitting on it throws
	// away a session to a key pressed out of habit.
	_, cmd := tui.New(completeConfig(), nil, tui.Deps{}).Update(keyMsg(keyEsc))

	// Assert
	if cmd != nil {
		if _, isQuit := cmd().(tea.QuitMsg); isQuit {
			t.Error("esc quit the interface")
		}
	}
}

// messageType is the type of the message a command produces, which is all a test
// can compare for Bubble Tea's own unexported mouse messages.
func messageType(cmd tea.Cmd) string {
	if cmd == nil {
		return "no command"
	}

	return fmt.Sprintf("%T", cmd())
}

func TestMouseKeyTogglesCapture(t *testing.T) {
	t.Parallel()

	// Arrange
	// Capture breaks the terminal's own click-drag text selection, which is why
	// there is a key to give it back.
	cfg := completeConfig()
	cfg.UI.Mouse = true
	model := tui.New(cfg, nil, tui.Deps{})

	// Act: m while the mouse is captured
	released, off := model.Update(keyMsg("m"))

	// Assert: the mouse is released
	if got, want := messageType(off), messageType(tea.DisableMouse); got != want {
		t.Errorf("m returned %s, want %s releasing the mouse", got, want)
	}

	// Act: m again
	_, on := released.Update(keyMsg("m"))

	// Assert: it is captured again
	if got, want := messageType(on), messageType(tea.EnableMouseCellMotion); got != want {
		t.Errorf("a second m returned %s, want %s capturing the mouse again", got, want)
	}
}
