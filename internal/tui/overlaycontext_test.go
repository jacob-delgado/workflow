// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

import (
	"slices"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/jacob-delgado/workflow/internal/tui"
)

// The probed screen is the size most tests draw: an overlay shows all it has
// there, and drawing it after every probed press costs least.
const (
	overlayWidth  = 120
	overlayHeight = 40
)

// functionKeys is how many function keys a terminal names, f1 to f63.
const functionKeys = 63

// movedKeys is every action an overlay may answer, each moved onto a key of
// its own that no binding has by default: a function key, then the same key
// with alt. One session then probes them all, a press of each reaching that
// action alone.
func movedKeys(t *testing.T) map[string]tea.KeyPressMsg {
	t.Helper()

	actions := overlayActions()
	if len(actions) > 2*functionKeys {
		t.Fatalf("%d actions to move, more than the %d function keys to move them onto", len(actions), 2*functionKeys)
	}

	moved := make(map[string]tea.KeyPressMsg, len(actions))

	for index, action := range actions {
		press := tea.KeyPressMsg{Code: tea.KeyF1 + rune(index%functionKeys)}
		if index >= functionKeys {
			press.Mod = tea.ModAlt
		}

		moved[action] = press
	}

	return moved
}

// noKey is a key no action is moved onto, pressed to learn what a press that
// nothing answers leaves on the screen.
func noKey() tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: tea.KeyF1, Mod: tea.ModCtrl}
}

// overlayActions is every action the help lists or an overlay's key context
// names, but jump-to-pane, which no ui.keys entry may move.
func overlayActions() []string {
	var actions []string

	for _, action := range tui.KeyActions("pull request", "Slack", nil) {
		actions = append(actions, action.Action)
	}

	for _, context := range tui.OverlayKeyContexts() {
		actions = append(actions, context.Actions...)
	}

	slices.Sort(actions)

	return slices.DeleteFunc(slices.Compact(actions), func(action string) bool { return action == fixedAction })
}

// keysOf is the ui.keys map that moves each action onto its key.
func keysOf(moved map[string]tea.KeyPressMsg) map[string]string {
	keys := make(map[string]string, len(moved))
	for action, press := range moved {
		keys[action] = press.String()
	}

	return keys
}

// overlayState is an overlay open in a state: a session started with every
// action moved onto its own key, and the steps that open the overlay in it.
// A step that names an action presses the key it was moved onto; any other
// is a key, as typing names it.
type overlayState struct {
	start func(t *testing.T, moved map[string]tea.KeyPressMsg) tui.Model
	steps []string
}

// from is the state a session reaches by steps.
func from(start func(t *testing.T, moved map[string]tea.KeyPressMsg) tui.Model, steps ...string) overlayState {
	return overlayState{start: start, steps: steps}
}

// at is the state a world's session reaches by steps.
func at(made func() *world, steps ...string) overlayState {
	return from(in(made), steps...)
}

// in starts a session in the world a function makes, as live does.
func in(made func() *world) func(t *testing.T, moved map[string]tea.KeyPressMsg) tui.Model {
	return func(t *testing.T, moved map[string]tea.KeyPressMsg) tui.Model {
		t.Helper()

		repo := made()
		repo.cfg.UI.Keys = keysOf(moved)

		return repo.live(t, overlayWidth, overlayHeight)
	}
}

// stepping presses each step in order, finishing whatever each one starts.
func stepping(t *testing.T, model tui.Model, moved map[string]tea.KeyPressMsg, steps ...string) tui.Model {
	t.Helper()

	for _, step := range steps {
		press, isAction := moved[step]
		if !isAction {
			press = keyMsg(step)
		}

		updated, cmd := model.Update(press)
		model = drain(t, concrete(t, updated), cmd)
	}

	return model
}

// overlayProbe is what the probe finds of an open overlay: the key context
// it says it is in, and every action a press of answers, asking for
// something or leaving a screen other than a press nothing answers leaves.
type overlayProbe struct {
	context  tui.KeyContext
	answered []string
}

// probeOverlay opens the state's overlay and presses every moved key on it.
// The screens are compared with their escapes, which is where a text field
// draws its cursor.
func probeOverlay(t *testing.T, state overlayState) overlayProbe {
	t.Helper()

	moved := movedKeys(t)
	opened := stepping(t, state.start(t, moved), moved, state.steps...)

	context, open := opened.OverlayKeyContext()
	if !open {
		t.Fatalf("no overlay is open after %v:\n%s", state.steps, plain(opened.View().Content))
	}

	// A press nothing answers first clears any notice the opening left, which
	// a moving key would keep and any other key clear.
	cleared, _ := opened.Update(noKey())
	model := concrete(t, cleared)
	unanswered, _ := model.Update(noKey())
	found := overlayProbe{context: context, answered: nil}

	for action, press := range moved {
		pressed, cmd := model.Update(press)
		if cmd != nil || pressed.View().Content != unanswered.View().Content {
			found.answered = append(found.answered, action)
		}
	}

	return found
}

// probeStates probes an overlay in each of its states.
func probeStates(t *testing.T, states []overlayState) []overlayProbe {
	t.Helper()

	probes := make([]overlayProbe, 0, len(states))
	for _, state := range states {
		probes = append(probes, probeOverlay(t, state))
	}

	return probes
}

// answeredIn is every action a press of answers in any of the probed states.
func answeredIn(probes []overlayProbe) []string {
	var answered []string
	for _, probe := range probes {
		answered = append(answered, probe.answered...)
	}

	slices.Sort(answered)

	return slices.Compact(answered)
}

// An overlay's key context is what CheckKeys refuses two actions on one key
// in. An action the overlay answers that its context leaves out would let a
// map bind it onto another of the overlay's keys, leaving a press there
// ambiguous; an action its context lists that no state of the overlay
// answers refuses a map for a press the overlay never reads.
func TestAnOverlayAnswersExactlyTheActionsItsKeyContextLists(t *testing.T) {
	t.Parallel()

	for name, states := range overlayStates() {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Act
			probes := probeStates(t, states)

			// Assert
			for index, probe := range probes {
				if probe.context.Name != name {
					t.Errorf("state %d opens the overlay in %q, want %q", index, probe.context.Name, name)
				}

				for _, answered := range probe.answered {
					if !slices.Contains(probe.context.Actions, answered) {
						t.Errorf("state %d of %s answers %s, which its key context leaves out", index, name, answered)
					}
				}
			}

			answered := answeredIn(probes)
			for _, listed := range probes[0].context.Actions {
				if !slices.Contains(answered, listed) {
					t.Errorf("%s lists %s, yet no state of it answers a press of it", name, listed)
				}
			}
		})
	}
}

// The probe is only as good as its table: an overlay whose key context no
// state opens would go unchecked.
func TestEveryOverlayKeyContextIsProbed(t *testing.T) {
	t.Parallel()

	// Arrange
	states := overlayStates()

	// Act
	contexts := tui.OverlayKeyContexts()

	// Assert
	for _, context := range contexts {
		if len(states[context.Name]) == 0 {
			t.Errorf("no state opens an overlay in %s", context.Name)
		}
	}
}

// An open overlay names the key context CheckKeys reads for it, so a test
// can hold what the overlay answers against what the check refuses.
func TestAnOpenOverlaysKeyContextIsOneCheckKeysReads(t *testing.T) {
	t.Parallel()

	// Arrange
	help := typing(t, newWorld().live(t, 120, 40), "?")

	// Act
	context, open := help.OverlayKeyContext()

	// Assert
	read := slices.ContainsFunc(tui.OverlayKeyContexts(), func(listed tui.KeyContext) bool {
		return listed.Name == context.Name && slices.Equal(listed.Actions, context.Actions)
	})
	if !open || !read || !slices.Contains(context.Actions, "close") {
		t.Errorf("the help's key context is %+v (open %v), want one of %+v, with close among its actions",
			context, open, tui.OverlayKeyContexts())
	}
}

func TestNoOverlayKeyContextIsLiveWhileNoOverlayIsOpen(t *testing.T) {
	t.Parallel()

	// Arrange
	panes := newWorld().live(t, 120, 40)

	// Act
	context, open := panes.OverlayKeyContext()

	// Assert
	if open {
		t.Errorf("with no overlay open, OverlayKeyContext = %+v, want none", context)
	}
}
