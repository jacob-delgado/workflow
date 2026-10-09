// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

import (
	"errors"
	"slices"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/tui"
)

// probeKey is the key an action is moved onto to learn whether a pane answers
// it: no binding has it, so a press of it reaches that action alone.
const probeKey = "f5"

// The probed screen is wide enough that no footer drops a key it offers, and
// tall enough that a notice keeps its own row.
const (
	probeWidth  = 320
	probeHeight = 50
)

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
// to act on with a backend to sync with, pull requests waiting on a review, a
// pull request approved and clean whose CI lists a failed check, and something
// done on the day the Summary opens on.
func everythingAtOnce() *world {
	repo := withSyncedTasks()
	repo.reviews = withReviews().reviews
	repo.ci = checkedCI().ci
	repo.done = summaryWorld().done
	readyToMerge(repo)

	return repo
}

// pagedViews is a list in two views whose first holds more issues than a page.
func pagedViews() *world {
	repo := viewsWorld()
	repo.pageSize = 1

	return repo
}

// foldingWithHooks is a branch with an unpushed commit to amend or fix up, a
// work tree with changes not yet staged, and hooks lefthook does not manage.
func foldingWithHooks() *world {
	repo := unpushedWorld()
	repo.changes = workTree()
	repo.gitHooks = legacyHooks()

	return repo
}

// withSlackDirectory is a Slack the user's token can read the directory of,
// and a store to keep whom each owner is.
func withSlackDirectory() *world {
	repo := newWorld()
	repo.slack = newSlackWorld()

	return repo
}

// paneState is a pane in a state: the digit that jumps to it, the world that
// puts it in that state, and the actions it answers there that the probe
// relies on it to reach, apart by spaces.
type paneState struct {
	digit   string
	world   func() *world
	answers string
}

// paneStates are every pane in the world where it has the most to answer, and
// again in each state that answers an action that world leaves out, so that
// between them they answer every action a pane may.
func paneStates() map[string]paneState {
	return map[string]paneState{
		"Issues, everything at once": {digit: "1", world: everythingAtOnce, answers: "change-status comment " +
			"assign log-work start-work search-issues filter-issues open-link copy-link refresh track-issue"},
		"Issues, in two views with more to load": {digit: "1", world: pagedViews, answers: "switch-view load-more refresh"},
		"Branch, everything at once": {
			digit: "2", world: everythingAtOnce, answers: "new-branch switch-branch link-issue rebase refresh",
		},
		"Branch, unpushed": {digit: "2", world: unpushedWorld, answers: "push refresh"},
		"Commits, everything at once": {
			digit: "3", world: everythingAtOnce, answers: "stage unstage-all discard-change commit run-pre-commit refresh",
		},
		"Commits, unpushed with changes to stage and hooks outside lefthook": {
			digit: "3", world: foldingWithHooks, answers: "stage-all amend fixup set-up-lefthook refresh",
		},
		"Review, everything at once": {
			digit: "4", world: everythingAtOnce, answers: "checks rerun-checks edit open-link copy-link refresh",
		},
		"Review, with no pull request":  {digit: "4", world: withoutPull, answers: "open-pull-request refresh"},
		"Review, ready to merge":        {digit: "4", world: mergeable, answers: "merge refresh"},
		"Review, merged":                {digit: "4", world: mergedBranch, answers: "finish-branch refresh"},
		"messaging, everything at once": {digit: "5", world: everythingAtOnce, answers: "post refresh"},
		"messaging, with the Slack directory": {
			digit: "5", world: withSlackDirectory, answers: "people-and-groups refresh",
		},
		"Reviews, everything at once": {
			digit: "6", world: everythingAtOnce, answers: "sort-reviews filter-reviews open-link copy-link refresh",
		},
		"Tasks, everything at once": {digit: "7", world: everythingAtOnce, answers: "start-stop mark-done add-task " +
			"annotate-task modify-task undo-task sync-tasks search-tasks filter-tasks sort-tasks open-link copy-link " +
			"refresh"},
		"Summary, everything at once": {
			digit: "8", world: everythingAtOnce, answers: "earlier later today calendar copy-summary post-summary refresh",
		},
		"Repositories, everything at once": {
			digit: "9", world: everythingAtOnce, answers: "favorite-directory go-to-directory settings local-data refresh",
		},
	}
}

// probed is what the probe key does on a pane: whether its footer offers the
// key, and whether a press of it answers, asking for something or leaving a
// screen other than a press nothing answers leaves.
type probed struct {
	offered, answered bool
}

// probeAt moves action onto the probe key in the state's world, jumps to its
// pane, and reads its footer and a press of the probe key.
func probeAt(t *testing.T, state paneState, action string) probed {
	t.Helper()

	moved := state.world()
	moved.cfg.UI.Keys = map[string]string{action: probeKey}
	focused := typing(t, moved.live(t, probeWidth, probeHeight), state.digit)

	unanswered, _ := focused.Update(unbound())
	pressed, cmd := focused.Update(probe())

	return probed{
		offered:  strings.Contains(footerLine(focused.View().Content), probeKey+" "),
		answered: cmd != nil || plain(pressed.View().Content) != plain(unanswered.View().Content),
	}
}

// sweep is every probed action a pane in a state offers in its footer, and
// every one a press of it answers.
type sweep struct {
	offered, answered []string
}

// sweepAt probes the state with each action a pane may answer.
func sweepAt(t *testing.T, state paneState) sweep {
	t.Helper()

	var found sweep

	for _, action := range probedActions() {
		result := probeAt(t, state, action)
		if result.offered {
			found.offered = append(found.offered, action)
		}

		if result.answered {
			found.answered = append(found.answered, action)
		}
	}

	return found
}

// keysEveryPaneTakes are the help groups of the keys that work on every pane,
// which every pane's key context holds whatever its handler answers, apart by
// a bar.
func keysEveryPaneTakes() []string {
	return strings.Split("Moving around|Everywhere", "|")
}

// paneGroups are the help groups of the keys a pane answers, each of which
// some pane state must answer for the probe to cover it.
func paneGroups() []string {
	return strings.Split("Issues|Branch|Commits|Review|Slack|Reviews|Tasks|Summary|Repositories", "|")
}

// probedActions is every action a pane's handler may answer: all but the keys
// every pane takes, jump-to-pane, which no ui.keys entry may move, among them.
func probedActions() []string {
	var actions []string

	for _, action := range tui.KeyActions("pull request", "Slack", nil) {
		if !slices.Contains(keysEveryPaneTakes(), action.Group) {
			actions = append(actions, action.Action)
		}
	}

	return actions
}

// The probe covers a pane's key context only in a state that answers: between
// them the states must answer every action a pane's help group holds, or a
// new one would go unprobed.
func TestThePaneStatesAnswerEveryPaneAction(t *testing.T) {
	t.Parallel()

	// Arrange
	states := paneStates()

	answers := make([]string, 0, len(states))
	for _, state := range states {
		answers = append(answers, state.answers)
	}

	covered := strings.Fields(strings.Join(answers, " "))

	// Act
	listed := tui.KeyActions("pull request", "Slack", nil)

	// Assert
	for _, action := range listed {
		if slices.Contains(paneGroups(), action.Group) && !slices.Contains(covered, action.Action) {
			t.Errorf("no pane state answers %s, of %s", action.Action, action.Group)
		}
	}
}

// A pane that answers two actions makes them live at once, so CheckKeys must
// refuse both bound to one key. Were a handler to answer an action its key
// context leaves out, a ui.keys map binding that action onto another of the
// pane's keys would pass the check and leave a press there ambiguous.
func TestEveryActionAPaneAnswersIsInItsKeyContext(t *testing.T) {
	t.Parallel()

	for name, state := range paneStates() {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			answered := sweepAt(t, state).answered
			for want := range strings.FieldsSeq(state.answers) {
				if !slices.Contains(answered, want) {
					t.Fatalf("the pane answers %v, want %s among them", answered, want)
				}
			}

			// Act & Assert
			for index, first := range answered {
				for _, second := range answered[index+1:] {
					both := map[string]string{first: probeKey, second: probeKey}

					err := tui.CheckKeys(both)
					if !errors.Is(err, config.ErrKeyConflict) {
						t.Errorf("the pane answers %s and %s, yet CheckKeys(%v) = %v, want ErrKeyConflict",
							first, second, both, err)
					}
				}
			}
		})
	}
}

// A key the footer offers is one a press of answers: with each action in turn
// moved onto the probe key, wherever the footer offers the probe key a press
// of it must do something.
func TestEveryKeyAPaneOffersAnswersAPress(t *testing.T) {
	t.Parallel()

	for name, state := range paneStates() {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Act
			found := sweepAt(t, state)

			// Assert
			for _, offered := range found.offered {
				if !slices.Contains(found.answered, offered) {
					t.Errorf("the footer offers %s, yet a press of it does nothing", offered)
				}
			}
		})
	}
}

// The probe is only as good as its sense of an answer and an offer: a press
// nothing answers must read as unanswered, and a pane's own verb as answered
// and offered.
func TestTheKeyContextProbeTellsAnAnswerFromNone(t *testing.T) {
	t.Parallel()

	// Arrange
	issues := paneStates()["Issues, everything at once"]

	// Act
	comment, commit := probeAt(t, issues, commentAction), probeAt(t, issues, "commit")

	// Assert
	if comment != (probed{offered: true, answered: true}) || commit != (probed{offered: false, answered: false}) {
		t.Errorf("on the Issues pane comment is %+v and commit is %+v, want both offered and answered, then neither",
			comment, commit)
	}
}

// The probe moves each action a pane may answer onto its key, which no
// ui.keys entry may do to jump-to-pane, so the help must list jump-to-pane
// among the keys every pane takes, which the probe leaves alone.
func TestTheHelpListsJumpToPaneAmongTheKeysEveryPaneTakes(t *testing.T) {
	t.Parallel()

	// Act
	listed := tui.KeyActions("pull request", "Slack", nil)

	// Assert
	group := ""

	for _, action := range listed {
		if action.Action == fixedAction {
			group = action.Group
		}
	}

	if !slices.Contains(keysEveryPaneTakes(), group) {
		t.Errorf("the help lists %s in %q, want it among %v, which the probe leaves alone",
			fixedAction, group, keysEveryPaneTakes())
	}
}
