// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/jira"
	"github.com/jacob-delgado/workflow/internal/tui"
)

// wheel is one notch of the mouse wheel over a cell.
func wheel(t *testing.T, model tui.Model, column, row int, button tea.MouseButton) tui.Model {
	t.Helper()

	updated, cmd := model.Update(tea.MouseWheelMsg{X: column, Y: row, Button: button})

	return drain(t, concrete(t, updated), cmd)
}

// notches turns the wheel the same way count times over a cell.
func notches(t *testing.T, model tui.Model, count int, button tea.MouseButton) tui.Model {
	t.Helper()

	for range count {
		model = wheel(t, model, 80, 10, button)
	}

	return model
}

func TestTheWheelScrollsTheDetail(t *testing.T) {
	t.Parallel()

	// Arrange
	wordy := newWorld()
	wordy.detail.Description = strings.Repeat("line\n", 60) + "THE END"
	screen := wordy.live(t, 120, 30)

	// Act: wheel down over the detail
	down := notches(t, screen, 25, tea.MouseWheelDown)

	// Assert: the end is in sight
	requireScreen(t, down.View().Content, "THE END")
	refuseScreen(t, down.View().Content, detailTop)

	// Act: wheel back up
	raised := notches(t, down, 30, tea.MouseWheelUp)

	// Assert: the top is in sight
	requireScreen(t, raised.View().Content, detailTop)
	refuseScreen(t, raised.View().Content, "THE END")

	// Act: wheel over the rail
	overRail := wheel(t, raised, 5, 5, tea.MouseWheelDown)

	// Assert: nothing moves
	if overRail.View().Content != raised.View().Content {
		t.Errorf("the wheel over the rail changed the screen:\n%s", overRail.View().Content)
	}
}

// The actions a turn of the wheel stands in for, as ui.keys names them.
const (
	downAction = "down"
	upAction   = "up"
)

// choosingStatus is a world whose issue can move two ways.
func choosingStatus() *world {
	choosing := newWorld()
	choosing.moves = workflowMoves()

	return choosing
}

// resolvingIssue is a world whose one move asks for a resolution, chosen from
// two, and then a root cause, typed in.
func resolvingIssue() *world {
	resolving := newWorld()
	resolving.moves = []jira.Transition{resolveIssue()}

	return resolving
}

// switchingTwoWays is a clean tree with two other task branches to switch to.
func switchingTwoWays() *world {
	switching := cleanSwitcher()
	switching.branches = append(switching.branches, "feat/PROJ-500-add-metrics")

	return switching
}

// mergeableTwoWays is a pull request ready to merge by either of two methods.
func mergeableTwoWays() *world {
	reviewing := mergeable()
	reviewing.mergeMethods = []forge.MergeMethod{forge.MergeCommit, forge.MergeSquash}

	return reviewing
}

// wheeledOverlay is an overlay that moves on up and down: the world it opens
// in, the keys that open it, and what it shows once the wheel has turned a
// notch down and then a notch back up.
type wheeledOverlay struct {
	world    func() *world
	height   int
	open     []string
	down, up string
}

// overlaysTheWheelMoves is every overlay that moves on up and down, each with
// something to move through.
func overlaysTheWheelMoves() map[string]wheeledOverlay {
	return map[string]wheeledOverlay{
		"the status picker": {
			world: choosingStatus, height: 40, open: []string{"t"}, down: "▸ ● Done", up: "▸ ◐ Start Review",
		},
		"a transition's options": {
			world: resolvingIssue, height: 40, open: []string{"t", keyEnter}, down: "▸ Won't Fix", up: "▸ Fixed",
		},
		"the task switcher": {
			world: switchingTwoWays, height: 40, open: []string{"2", "s"}, down: "▸ PROJ-500", up: "▸ PROJ-388",
		},
		"the fixup picker": {
			world: twoUnpushedWorld, height: 40, open: []string{"3", "f"}, down: "▸ aaa1111", up: "▸ bbb2222",
		},
		"the merge preview": {
			world: mergeableTwoWays, height: 40, open: []string{"4", "M"},
			down: "▸ squash and merge", up: "▸ merge commit",
		},
		"the checks": {
			world: checkedCI, height: 40, open: []string{"4", "c"}, down: "▸ ● build", up: "▸ ✗ lint",
		},
		"a failed run's places": {
			world: failingLint, height: 40, open: commitKeys("x"), down: "▸ b.go:2 second", up: "▸ a.go:1 first",
		},
		"the help": {
			world: newWorld, height: 20, open: []string{"?"}, down: "Branch and Commits", up: "Moving around",
		},
	}
}

func TestTheWheelMovesAnOverlaysList(t *testing.T) {
	t.Parallel()

	keymaps := map[string]map[string]string{
		"on the default keys": nil,
		// The wheel moves an overlay as its up and down do, wherever ui.keys has
		// moved them, and not by pressing the arrow keys they are bound to by default.
		"with up and down rebound": {downAction: "ctrl+j", upAction: "ctrl+k"},
	}

	for overlayName, overlay := range overlaysTheWheelMoves() {
		for keymapName, keymap := range keymaps {
			t.Run(overlayName+" "+keymapName, func(t *testing.T) {
				t.Parallel()

				// Arrange
				moving := overlay.world()
				moving.cfg.UI.Keys = keymap
				opened := typing(t, moving.live(t, 120, overlay.height), overlay.open...)

				// Act: wheel down
				down := wheel(t, opened, 80, 10, tea.MouseWheelDown)

				// Assert: the overlay moves down
				requireScreen(t, down.View().Content, overlay.down)

				// Act: wheel up
				up := wheel(t, down, 80, 10, tea.MouseWheelUp)

				// Assert: and back up
				requireScreen(t, up.View().Content, overlay.up)
			})
		}
	}
}

func TestTheWheelCyclesAComposersSuggestions(t *testing.T) {
	t.Parallel()

	// Arrange
	// A composer has no list of its own to step, so the wheel turns as the arrow
	// keys do there: through the scopes offered for what is typed. There are
	// three, so turning back up cannot land where a second turn down would.
	world := newWorld()
	world.recentSubjects = []string{"feat(ideas): a", "fix(infra): b", "docs(inputs): c"}
	opened := typing(t, world.live(t, 120, 40), append([]string{"3", "c", keyShiftTab}, letters("i")...)...)

	// Act: wheel down
	down := wheel(t, opened, 80, 10, tea.MouseWheelDown)

	// Assert: the next scope is offered
	requireScreen(t, down.View().Content, "scope   > infra")
	refuseScreen(t, down.View().Content, "ideas")

	// Act: wheel up
	up := wheel(t, down, 80, 10, tea.MouseWheelUp)

	// Assert: and the first again
	requireScreen(t, up.View().Content, "scope   > ideas")
}

func TestTheWheelMovesNothingWhileAnOverlaySends(t *testing.T) {
	t.Parallel()

	// Each overlay is opened and enter pressed, its request left unanswered.
	cases := map[string]struct {
		world func() *world
		open  []string
	}{
		"a status change": {world: choosingStatus, open: []string{"t"}},
		"a switch":        {world: switchingTwoWays, open: []string{"2", "s"}},
		"a merge":         {world: mergeableTwoWays, open: []string{"4", "M"}},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			opened := typing(t, tt.world().live(t, 120, 40), tt.open...)
			sending, _ := pressed(t, opened, keyEnter)

			// Act
			after := wheel(t, sending, 80, 10, tea.MouseWheelDown)

			// Assert
			if after.View().Content != sending.View().Content {
				t.Errorf("the wheel changed the screen while %s is sent:\n%s", name, plain(after.View().Content))
			}
		})
	}
}

func TestTheWheelWhileTheMethodsAreReadLeavesTheFirstChosen(t *testing.T) {
	t.Parallel()

	// Arrange
	// The wheel turns while the methods are still out, and they land after it.
	reviewing := mergeableTwoWays()
	picker, read := pressed(t, typing(t, reviewing.live(t, 120, 40), "4"), "M")
	turned := wheel(t, picker, 80, 10, tea.MouseWheelDown)
	loaded, _ := finish(t, turned, read)

	// Act
	typing(t, loaded, keyEnter)

	// Assert
	if calls := reviewing.asked("merge 42 merge"); len(calls) != 1 {
		t.Errorf("merge calls = %q, want the first method", reviewing.asked("merge 42"))
	}
}

func TestClickingPicksAnIssue(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		width, height, column int
		keys                  []string
	}{
		// The issue list in the focused rail pane: its second row is row 3.
		"in the rail": {width: 120, height: 40, column: 5},
		// Reading an issue only takes the list's place below 80 columns.
		"in the rail while an issue is read": {width: 120, height: 40, column: 5, keys: []string{keyEnter}},
		// Below 80 columns the rail is dropped and the list fills the detail.
		"on a narrow terminal": {width: 79, height: 30, column: 10},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			screen := typing(t, newWorld().live(t, tt.width, tt.height), tt.keys...)

			// Act
			view := click(t, screen, tt.column, 3).View().Content

			// Assert
			requireScreen(t, view, "▸ ○ PROJ-388")
		})
	}
}

func TestClickingTheIssueBeingReadBelow80ColumnsPicksNothing(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := newWorld()
	reading := typing(t, repo.live(t, 79, 24), keyEnter)

	// Act
	// Row 3 holds the second issue while the list is drawn; now it is the
	// first issue's text.
	view := click(t, reading, 10, 3).View().Content

	// Assert
	// Every issue in the world shares one description, so the header and the
	// reads tell which issue is shown.
	requireScreen(t, view, issueKey+" "+issueSummary)
	refuseScreen(t, view, "Add retries")

	if reads := repo.asked("issue " + secondIssue); len(reads) != 0 {
		t.Errorf("a click on the issue being read asked for %q", reads)
	}
}

func TestClickingPicksATransition(t *testing.T) {
	t.Parallel()

	// Arrange
	choosing := newWorld()
	choosing.moves = workflowMoves()
	picker := typing(t, choosing.live(t, 120, 40), "t")

	// Act: click the second transition
	// Row 6: the border, the issue, its status and a blank line come first.
	second := click(t, picker, 60, 6)

	// Assert: it is selected
	requireScreen(t, second.View().Content, "▸ ● Done")

	// Act: click the first
	first := click(t, second, 60, 5)

	// Assert: it is selected
	requireScreen(t, first.View().Content, "▸ ◐ Start Review")

	// Act: click the issue above the transitions
	header := click(t, first, 60, 2)

	// Assert: nothing changes
	if header.View().Content != first.View().Content {
		t.Errorf("a click on the picker's header changed the screen:\n%s", header.View().Content)
	}
}

func TestAClickOutsideAnOpenOverlayDoesNothing(t *testing.T) {
	t.Parallel()

	// Arrange
	choosing := newWorld()
	choosing.moves = workflowMoves()
	picker := typing(t, choosing.live(t, 120, 40), "t")

	// Act
	clicked := click(t, picker, 5, 30)

	// Assert
	if clicked.View().Content != picker.View().Content {
		t.Errorf("a click outside the picker changed the screen:\n%s", clicked.View().Content)
	}
}

func TestClickingARunsFailuresPicksOne(t *testing.T) {
	t.Parallel()

	// Arrange
	failing := newWorld()
	failing.commitErr = errHookFailed
	failing.commitLines = []string{"a.go:1:1: first", "b.go:2:1: second"}
	failed := typing(t, failing.live(t, 120, 40), commitKeys("x")...)

	// Act: click the second failure
	// The border, the outcome and a blank line come before the list.
	picked := click(t, failed, 60, 5)

	// Assert: it is selected
	requireScreen(t, picked.View().Content, "▸ b.go:2 second")

	// Act: click above the list
	above := click(t, picked, 60, 1)

	// Assert: nothing changes
	if above.View().Content != picked.View().Content {
		t.Errorf("a click above the failures changed the screen:\n%s", above.View().Content)
	}
}

func TestTheMouseSettingDecidesWhetherItIsCaptured(t *testing.T) {
	t.Parallel()

	// m flips capture from wherever the setting started it, and says what it did.
	cases := map[string]struct {
		mouse  bool
		want   tea.MouseMode
		notice string
	}{
		"captured":     {mouse: true, want: tea.MouseModeNone, notice: "mouse off"},
		"not captured": {mouse: false, want: tea.MouseModeCellMotion, notice: "mouse on"},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			cfg := completeConfig()
			cfg.UI.Mouse = tt.mouse
			model := sized(t, tui.New(cfg, nil, tui.Deps{}), 120, 40)

			// Act
			updated, _ := model.Update(keyMsg("m"))
			after := concrete(t, updated)

			// Assert
			// v2 sets the mouse mode declaratively in View, so the toggle shows in
			// the next View's MouseMode rather than in a returned command.
			if got := after.View().MouseMode; got != tt.want {
				t.Errorf("m left mouse mode %v, want %v", got, tt.want)
			}

			requireScreen(t, after.View().Content, tt.notice)
		})
	}
}
