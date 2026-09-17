// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

import (
	"fmt"
	"slices"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/jacob-delgado/workflow/internal/jira"
	"github.com/jacob-delgado/workflow/internal/tui"
)

// pickerTitle is the detail pane's title, drawn heavy, while the picker has the
// keyboard.
const pickerTitle = "┏━ Change status"

// transition builds a fixture transition.
func transition(id, name, status, category string) jira.Transition {
	return jira.Transition{ID: id, Name: name, ToStatus: status, ToStatusCategory: category}
}

// workflowMoves is what a typical workflow offers an issue in progress: a verb
// that differs from its status, and one that does not.
func workflowMoves() []jira.Transition {
	return []jira.Transition{
		transition("21", "Start Review", "In Review", "indeterminate"),
		transition("31", "Done", "Done", "done"),
	}
}

// fakeJira stands in for Jira: it offers the same transitions for every issue,
// answers applying one with applyErr, and records what it was asked.
type fakeJira struct {
	moves    []jira.Transition
	listErr  error
	applyErr error
	listed   atomic.Value
	applied  atomic.Value
	values   atomic.Value
	applies  atomic.Int32
}

// deps wires the fake behind a search.
func (f *fakeJira) deps(search func() (jira.SearchResult, error)) tui.Deps {
	return tui.Deps{Jira: tui.JiraDeps{
		Search: search,
		Transitions: func(issueKey string) ([]jira.Transition, error) {
			f.listed.Store(issueKey)

			if f.listErr != nil {
				return nil, f.listErr
			}

			return f.moves, nil
		},
		Transition: func(issueKey string, to jira.Transition, values []jira.FieldValue) error {
			f.applies.Add(1)
			f.applied.Store(issueKey + " " + to.ID)
			f.values.Store(values)

			return f.applyErr
		},
	}}
}

// jiraScreen is a started model on these seams, at a size that shows the rail.
func jiraScreen(t *testing.T, deps tui.Deps) tui.Model {
	t.Helper()

	return started(t, sized(t, tui.New(completeConfig(), nil, deps), 120, 40))
}

// pressed sends one key and returns the command it produced, which a test runs
// when it wants the work to finish.
func pressed(t *testing.T, model tui.Model, key string) (tui.Model, tea.Cmd) {
	t.Helper()

	next, cmd := model.Update(keyMsg(key))

	return concrete(t, next), cmd
}

// finish runs a command and delivers its message, as Bubble Tea does once the
// work behind it returns.
func finish(t *testing.T, model tui.Model, cmd tea.Cmd) (tui.Model, tea.Cmd) {
	t.Helper()

	if cmd == nil {
		t.Fatal("no command to run, want one")
	}

	next, followUp := model.Update(cmd())

	return concrete(t, next), followUp
}

// twoIssues is a search that finds OPS-1 and OPS-2.
func twoIssues() func() (jira.SearchResult, error) {
	return assigned(issue("OPS-1", "Fix login", "indeterminate"), issue("OPS-2", "Rotate keys", "new"))
}

// openPicker presses t and delivers the listing.
func openPicker(t *testing.T, model tui.Model) tui.Model {
	t.Helper()

	model, cmd := pressed(t, model, "t")
	model, _ = finish(t, model, cmd)

	return model
}

func TestTOpensTheStatusPickerForTheSelectedIssue(t *testing.T) {
	t.Parallel()

	// Arrange
	fake := &fakeJira{moves: workflowMoves()}
	screen := press(t, jiraScreen(t, fake.deps(twoIssues())), "j")

	// Act: press t
	loading, cmd := pressed(t, screen, "t")

	// Assert: the picker opens, loading
	requireScreen(t, loading.View(), pickerTitle, "loading statuses…")

	// Act: the listing arrives
	listed, _ := finish(t, loading, cmd)

	// Assert: the selected issue's transitions are listed
	if got := fake.listed.Load(); got != "OPS-2" {
		t.Errorf("listed transitions for %v, want the selected OPS-2", got)
	}

	view := listed.View()
	requireScreen(t, view, "OPS-2 Rotate keys", "▸ ◐ Start Review → In Review", "  ● Done")

	// A status that matches its transition's name is not repeated, and with the
	// keyboard in the picker, the rail pane no longer draws focus.
	refuseScreen(t, view, "Done → Done", focused("1 Issues"))
}

func TestThePickerTakesTheListKeysAndEscapeClosesIt(t *testing.T) {
	t.Parallel()

	// Arrange
	fake := &fakeJira{moves: workflowMoves()}
	screen := openPicker(t, jiraScreen(t, fake.deps(twoIssues())))

	// Act: j
	down := press(t, screen, "j")

	// Assert: the picker's selection moves down
	requireScreen(t, down.View(), "▸ ● Done")

	// Act: k
	back := press(t, down, "k")

	// Assert: and back up
	requireScreen(t, back.View(), "▸ ◐ Start Review")

	// Act: esc
	closed := press(t, back, keyEsc).View()

	// Assert: the picker closes, and the j went to it, not to the list underneath
	refuseScreen(t, closed, pickerTitle)
	requireScreen(t, closed, "▸ ◐ OPS-1", focused("1 Issues"))
}

func TestThePickerKeepsTheKeyboardWhileOpen(t *testing.T) {
	t.Parallel()

	for _, key := range []string{keyTab, "3", "?", "m"} {
		t.Run(key, func(t *testing.T) {
			t.Parallel()

			// Arrange
			fake := &fakeJira{moves: workflowMoves()}
			screen := openPicker(t, jiraScreen(t, fake.deps(twoIssues())))

			// Act
			view := press(t, screen, key).View()

			// Assert
			requireScreen(t, view, pickerTitle)
		})
	}
}

func TestAClickDoesNotMoveFocusFromUnderAnOpenPicker(t *testing.T) {
	t.Parallel()

	// Arrange
	fake := &fakeJira{moves: workflowMoves()}
	screen := openPicker(t, jiraScreen(t, fake.deps(twoIssues())))

	// Act
	// Row 28 is inside the Commits pane. Focus is only drawn once the picker
	// closes, so close it to look.
	clicked, _ := screen.Update(tea.MouseMsg{X: 2, Y: 28, Action: tea.MouseActionPress, Button: tea.MouseButtonLeft})
	view := press(t, concrete(t, clicked), keyEsc).View()

	// Assert
	requireScreen(t, view, focused("1 Issues"))
	refuseScreen(t, view, focused("3 Commits"))
}

func TestCtrlCQuitsWhileThePickerIsOpen(t *testing.T) {
	t.Parallel()

	// Arrange
	fake := &fakeJira{moves: workflowMoves()}
	screen := openPicker(t, jiraScreen(t, fake.deps(twoIssues())))

	// Act
	_, cmd := pressed(t, screen, "ctrl+c")

	// Assert
	if cmd == nil {
		t.Fatal("ctrl+c did not quit while the picker was open")
	}

	if msg, isQuit := cmd().(tea.QuitMsg); !isQuit {
		t.Errorf("ctrl+c produced %T, want tea.QuitMsg", msg)
	}
}

func TestAPickerWithNothingToApplyIgnoresEnter(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		moves   []jira.Transition
		listErr error
		want    string
	}{
		"the listing failed":          {listErr: jira.ErrUnauthorized, want: "✗ the credential was not accepted"},
		"the workflow offers nothing": {want: "Jira offers no status change for OPS-1"},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			fake := &fakeJira{moves: tt.moves, listErr: tt.listErr}
			model := jiraScreen(t, fake.deps(twoIssues()))

			// Act: open the picker
			screen := openPicker(t, model)

			// Assert: it says why there is nothing to choose
			requireScreen(t, screen.View(), tt.want)

			// Act: try to apply
			tried, cmd := pressed(t, press(t, screen, "j"), keyEnter)

			// Assert: nothing is sent
			if cmd != nil {
				t.Error("enter sent something with nothing to apply")
			}

			// Act: esc
			closed := press(t, tried, keyEsc).View()

			// Assert: the picker closes
			refuseScreen(t, closed, pickerTitle)
		})
	}
}

func TestTDoesNothingWithoutAnIssueToMove(t *testing.T) {
	t.Parallel()

	fake := &fakeJira{moves: workflowMoves()}

	cases := map[string]tui.Model{
		"nothing assigned": jiraScreen(t, fake.deps(assigned())),
		"search failed":    jiraScreen(t, fake.deps(failing(jira.ErrUnauthorized))),
		"still loading":    sized(t, tui.New(completeConfig(), nil, fake.deps(twoIssues())), 120, 40),
		"another pane":     press(t, jiraScreen(t, fake.deps(twoIssues())), "2"),
	}

	for name, screen := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Act
			next, cmd := pressed(t, screen, "t")

			// Assert
			if cmd != nil || strings.Contains(next.View(), pickerTitle) {
				t.Errorf("t opened a picker with no issue to move:\n%s", next.View())
			}
		})
	}
}

func TestALateListingIsNotShownForTheWrongIssue(t *testing.T) {
	t.Parallel()

	// Arrange
	fake := &fakeJira{moves: workflowMoves()}
	screen := jiraScreen(t, fake.deps(twoIssues()))

	// Open on OPS-1 and walk away before the answer; then open on OPS-2.
	screen, forFirst := pressed(t, screen, "t")
	screen = press(t, screen, keyEsc, "j")
	screen, forSecond := pressed(t, screen, "t")

	// Act: OPS-1's listing arrives
	late, _ := finish(t, screen, forFirst)

	// Assert: OPS-2's picker is still waiting for its own
	requireScreen(t, late.View(), "loading statuses…")
	refuseScreen(t, late.View(), "▸ ◐ Start Review")

	// Act: OPS-2's listing arrives
	listed, _ := finish(t, late, forSecond)

	// Assert: it is shown
	requireScreen(t, listed.View(), "▸ ◐ Start Review")
}

func TestASecondListingDoesNotReplaceTheOneOnScreen(t *testing.T) {
	t.Parallel()

	// Arrange
	var calls atomic.Int32

	deps := (&fakeJira{}).deps(twoIssues())
	deps.Jira.Transitions = func(string) ([]jira.Transition, error) {
		if calls.Add(1) == 1 {
			return workflowMoves(), nil
		}

		// Shorter than the first: were it to replace a list the user is
		// already choosing from, the selection would point past its end.
		return workflowMoves()[:1], nil
	}

	// Open, close and reopen on the same issue: two listings are on their way.
	screen, first := pressed(t, jiraScreen(t, deps), "t")
	screen = press(t, screen, keyEsc)
	screen, second := pressed(t, screen, "t")

	// Act
	screen, _ = finish(t, screen, first)
	screen = press(t, screen, "j")
	screen, _ = finish(t, screen, second)

	// Assert
	requireScreen(t, screen.View(), "▸ ● Done")
}

func TestALongPickerScrollsToKeepTheSelectionVisible(t *testing.T) {
	t.Parallel()

	// Arrange
	moves := make([]jira.Transition, 0, 40)
	for index := 1; index <= 40; index++ {
		moves = append(moves, transition(strconv.Itoa(index), fmt.Sprintf("Step %d", index), "Doing", "indeterminate"))
	}

	fake := &fakeJira{moves: moves}
	screen := openPicker(t, jiraScreen(t, fake.deps(twoIssues())))

	// Act
	view := press(t, screen, slices.Repeat([]string{"j"}, 39)...).View()

	// Assert
	requireScreen(t, view, "▸ ◐ Step 40")
	refuseScreen(t, view, "◐ Step 1 ")
}

func TestTheIssuesPaneFooterOffersChangingStatus(t *testing.T) {
	t.Parallel()

	// Arrange
	fake := &fakeJira{moves: workflowMoves()}

	// Act
	footer := footerLine(jiraScreen(t, fake.deps(twoIssues())).View())

	// Assert
	requireScreen(t, footer, "t change status")
}

func TestTheFooterDoesNotOfferChangingStatusWhereItDoesNothing(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		issues []jira.Issue
		keys   []string
	}{
		"on the Branch pane": {issues: []jira.Issue{issue("OPS-1", "Fix login", "new")}, keys: []string{"2"}},
		"with an empty list": {issues: nil, keys: nil},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			fake := &fakeJira{moves: workflowMoves()}
			screen := jiraScreen(t, fake.deps(assigned(tt.issues...)))

			// Act
			footer := footerLine(press(t, screen, tt.keys...).View())

			// Assert
			refuseScreen(t, footer, "change status")
			requireScreen(t, footer, "quit")
		})
	}
}

func TestThePickerFooterSaysHowToApplyOrLeave(t *testing.T) {
	t.Parallel()

	// Arrange
	fake := &fakeJira{moves: workflowMoves()}
	screen := jiraScreen(t, fake.deps(twoIssues()))

	// Act
	footer := footerLine(openPicker(t, screen).View())

	// Assert
	requireScreen(t, footer, "enter apply", keyEsc)
}

func TestThePickerFitsANarrowTerminal(t *testing.T) {
	t.Parallel()

	// Arrange
	fake := &fakeJira{moves: workflowMoves()}
	model := started(t, sized(t, tui.New(completeConfig(), nil, fake.deps(twoIssues())), 79, 30))

	// Act
	view := openPicker(t, model).View()

	// Assert
	requireScreen(t, view, pickerTitle, "▸ ◐ Start Review → In Review")

	for index, line := range strings.Split(view, "\n") {
		if width := lipgloss.Width(line); width > 80 {
			t.Errorf("line %d is %d cells, wider than the terminal: %q", index, width, line)
		}
	}
}
