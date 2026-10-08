// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

import (
	"errors"
	"slices"
	"strconv"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/jacob-delgado/workflow/internal/tui"
)

// probeKey is the key an action is moved onto to learn whether a pane answers
// it: no binding has it, so a press of it reaches that action alone.
const probeKey = "f5"

// panesToProbe is every pane, by the digit that jumps to it.
const panesToProbe = 9

// probe is the probe key pressed.
func probe() tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: tea.KeyF5}
}

// unbound is a key no binding has, pressed to learn what a press that nothing
// answers leaves on the screen.
func unbound() tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: tea.KeyF6}
}

// everythingAtOnce is a world where every pane has the most to answer: tasks
// to act on with a backend to sync with, pull requests waiting on a review,
// and a pull request approved and clean whose CI lists a failed check.
func everythingAtOnce() *world {
	repo := withSyncedTasks()
	repo.reviews = withReviews().reviews
	repo.ci = checkedCI().ci
	readyToMerge(repo)

	return repo
}

// answers reports whether the pane jumped to by digit answers action: with
// action moved onto the probe key, a press of it asks for something, or leaves
// a screen other than a press nothing answers leaves.
func answers(t *testing.T, digit, action string) bool {
	t.Helper()

	moved := everythingAtOnce()
	moved.cfg.UI.Keys = map[string]string{action: probeKey}
	focused := typing(t, moved.live(t, 120, 40), digit)

	unanswered, _ := focused.Update(unbound())
	pressed, cmd := focused.Update(probe())

	return cmd != nil || plain(pressed.View().Content) != plain(unanswered.View().Content)
}

// rebindable is every action a ui.keys entry may move.
func rebindable() []string {
	var actions []string

	for _, action := range tui.KeyActions("pull request", "Slack", nil) {
		if action.Action != fixedAction {
			actions = append(actions, action.Action)
		}
	}

	return actions
}

// A pane that answers two actions makes them live at once, so CheckKeys must
// refuse both bound to one key. Were a handler to answer an action its key
// context leaves out, a ui.keys map binding that action onto another of the
// pane's keys would pass the check and leave a press there ambiguous.
func TestEveryActionAPaneAnswersIsInItsKeyContext(t *testing.T) {
	t.Parallel()

	for pane := 1; pane <= panesToProbe; pane++ {
		digit := strconv.Itoa(pane)

		t.Run("pane "+digit, func(t *testing.T) {
			t.Parallel()

			// Arrange
			var answered []string

			for _, action := range rebindable() {
				if answers(t, digit, action) {
					answered = append(answered, action)
				}
			}

			// Act & Assert
			for index, first := range answered {
				for _, second := range answered[index+1:] {
					both := map[string]string{first: probeKey, second: probeKey}

					err := tui.CheckKeys(both)
					if !errors.Is(err, tui.ErrKeyConflict) {
						t.Errorf("pane %s answers %s and %s, yet CheckKeys(%v) = %v, want ErrKeyConflict",
							digit, first, second, both, err)
					}
				}
			}
		})
	}
}

// The probe is only as good as its sense of an answer: a press nothing
// answers must read as unanswered, and a pane's own verb as answered.
func TestTheKeyContextProbeTellsAnAnswerFromNone(t *testing.T) {
	t.Parallel()

	// Act
	commentAnswered, commitAnswered := answers(t, "1", "comment"), answers(t, "1", "commit")

	// Assert
	if !commentAnswered || commitAnswered {
		t.Errorf("on the Issues pane comment answered = %v and commit answered = %v, want true and false",
			commentAnswered, commitAnswered)
	}

	if slices.Contains(rebindable(), fixedAction) {
		t.Errorf("the probed actions include %s, which no ui.keys entry may move", fixedAction)
	}
}
