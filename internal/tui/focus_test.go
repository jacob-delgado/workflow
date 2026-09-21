// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

import (
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/jacob-delgado/workflow/internal/tui"
)

// issuesPane is the first rail pane, which starts with focus.
const issuesPane = "1 Issues"

// focused is a pane's title on a heavy rule, which is how focus is drawn — in
// the shared rail its title rule goes heavy, and the list-in-detail panes carry
// the heavy border on the detail. Either way the label follows a heavy dash.
func focused(label string) string {
	return "━ " + label + " "
}

// heavyRuleLines counts the rows carrying a heavy rule. A focused pane has
// exactly two — the rule above it and the one below — so this is two when
// exactly one pane has focus, whether that is a rail pane or the detail.
func heavyRuleLines(view string) int {
	count := 0

	for line := range strings.SplitSeq(view, "\n") {
		if strings.Contains(line, "━") {
			count++
		}
	}

	return count
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
		// shift+tab from the first pane lands on the last rather than stopping. The
		// last pane's list lives in the detail, so its focus shows as the detail
		// title rather than the rail's.
		"shift+tab from the first wraps to the last": {keys: []string{keyShiftTab}, want: "Reviews"},
		"six tabs is a full lap": {
			keys: []string{keyTab, keyTab, keyTab, keyTab, keyTab, keyTab}, want: issuesPane,
		},
		"a number jumps straight to its pane": {keys: []string{"4"}, want: "4 Review"},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Act
			view := press(t, fresh(t), tt.keys...).View().Content

			// Assert
			// Focus moves rather than being added: exactly one pane is heavy, so
			// exactly two rows carry a heavy rule.
			requireScreen(t, view, focused(tt.want))

			if heavy := heavyRuleLines(view); heavy != 2 {
				t.Errorf("found %d heavy rule rows, want two around one focused pane:\n%s", heavy, view)
			}
		})
	}
}

func TestALeftClickOnTheRailFocusesThatPane(t *testing.T) {
	t.Parallel()

	// At 120x40, with Issues focused and so taking the spare height, the Commits
	// rail pane's content sits on row 27.
	cases := map[string]struct {
		msg  tea.MouseMsg
		want string
	}{
		// The Commits pane's heavy border is on its detail, where the cursor is,
		// so focus shows as the detail title rather than the rail's.
		"a left click on Commits": {
			msg: tea.MouseClickMsg{X: 5, Y: 27, Button: tea.MouseLeft}, want: "Commits",
		},
		"a release": {
			msg: tea.MouseReleaseMsg{X: 5, Y: 28, Button: tea.MouseLeft}, want: issuesPane,
		},
		"a right click": {
			msg: tea.MouseClickMsg{X: 5, Y: 28, Button: tea.MouseRight}, want: issuesPane,
		},
		"a click on the detail": {
			msg: tea.MouseClickMsg{X: 80, Y: 28, Button: tea.MouseLeft}, want: issuesPane,
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Act
			after, _ := fresh(t).Update(tt.msg)

			// Assert
			requireScreen(t, after.View().Content, focused(tt.want))
		})
	}
}

func TestHelpShowsEveryKey(t *testing.T) {
	t.Parallel()

	// Act
	view := press(t, fresh(t), "?").View().Content

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
			requireScreen(t, open.View().Content, "toggle mouse")

			// Act
			closed := press(t, open, key).View().Content

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
	updated, _ := model.Update(keyMsg("m"))
	released := concrete(t, updated)

	// Assert: the mouse is released — v2 shows it in the next View's mouse mode.
	if got := released.View().MouseMode; got != tea.MouseModeNone {
		t.Errorf("m left mouse mode %v, want it released", got)
	}

	// Act: m again
	updated, _ = released.Update(keyMsg("m"))

	// Assert: it is captured again
	if got := concrete(t, updated).View().MouseMode; got != tea.MouseModeCellMotion {
		t.Errorf("a second m left mouse mode %v, want it captured again", got)
	}
}
