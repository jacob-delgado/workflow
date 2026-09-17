// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

import (
	"fmt"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

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

	fake := &fakeJira{moves: workflowMoves()}
	screen := press(t, jiraScreen(t, fake.deps(twoIssues())), "j")

	loading, cmd := pressed(t, screen, "t")

	if view := loading.View(); !strings.Contains(view, pickerTitle) || !strings.Contains(view, "loading") {
		t.Errorf("t did not open a loading picker:\n%s", view)
	}

	listed, _ := finish(t, loading, cmd)

	if got := fake.listed.Load(); got != "OPS-2" {
		t.Errorf("listed transitions for %v, want the selected OPS-2", got)
	}

	view := listed.View()
	for _, want := range []string{"OPS-2 Rotate keys", "▸ ◐ Start Review → In Review", "  ● Done"} {
		if !strings.Contains(view, want) {
			t.Errorf("the picker does not show %q:\n%s", want, view)
		}
	}

	// A status that matches its transition's name is not repeated.
	if strings.Contains(view, "Done → Done") {
		t.Errorf("the picker repeated a status that names itself:\n%s", view)
	}

	// The picker has the keyboard, so the rail pane no longer draws focus.
	if strings.Contains(view, focused("1 Issues")) {
		t.Errorf("two panes are drawn with focus:\n%s", view)
	}
}

func TestThePickerTakesTheListKeysAndEscapeClosesIt(t *testing.T) {
	t.Parallel()

	fake := &fakeJira{moves: workflowMoves()}
	screen := openPicker(t, jiraScreen(t, fake.deps(twoIssues())))

	if view := press(t, screen, "j").View(); !strings.Contains(view, "▸ ● Done") {
		t.Errorf("j did not move the picker's selection:\n%s", view)
	}

	if view := press(t, screen, "j", "k").View(); !strings.Contains(view, "▸ ◐ Start Review") {
		t.Errorf("k did not move the picker's selection back:\n%s", view)
	}

	closed := press(t, screen, "j", "esc").View()

	if strings.Contains(closed, pickerTitle) {
		t.Errorf("esc did not close the picker:\n%s", closed)
	}

	// The j went to the picker, not to the list underneath it.
	if !strings.Contains(closed, "▸ ◐ OPS-1") || !strings.Contains(closed, focused("1 Issues")) {
		t.Errorf("the list changed underneath the picker:\n%s", closed)
	}
}

func TestThePickerKeepsTheKeyboardWhileOpen(t *testing.T) {
	t.Parallel()

	fake := &fakeJira{moves: workflowMoves()}
	screen := openPicker(t, jiraScreen(t, fake.deps(twoIssues())))

	for _, key := range []string{keyTab, "3", "?", "m"} {
		if view := press(t, screen, key).View(); !strings.Contains(view, pickerTitle) {
			t.Errorf("%q escaped the open picker:\n%s", key, view)
		}
	}

	// Row 28 is inside the Commits pane. Focus is only drawn once the picker
	// closes, so look there.
	clicked, _ := screen.Update(tea.MouseMsg{X: 2, Y: 28, Action: tea.MouseActionPress, Button: tea.MouseButtonLeft})
	if view := press(t, concrete(t, clicked), "esc").View(); !strings.Contains(view, focused("1 Issues")) {
		t.Errorf("a click moved focus out from under the open picker:\n%s", view)
	}

	if _, cmd := pressed(t, screen, "ctrl+c"); cmd == nil {
		t.Error("ctrl+c did not quit while the picker was open")
	}
}

func TestAPickerWithNothingToApplyIgnoresEnter(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		fake *fakeJira
		want string
	}{
		"the listing failed": {
			fake: &fakeJira{listErr: jira.ErrUnauthorized},
			want: "✗ the credential was not accepted",
		},
		"the workflow offers nothing": {
			fake: &fakeJira{},
			want: "Jira offers no status change for OPS-1",
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			screen := openPicker(t, jiraScreen(t, tt.fake.deps(twoIssues())))

			if view := screen.View(); !strings.Contains(view, tt.want) {
				t.Errorf("the picker does not say %q:\n%s", tt.want, view)
			}

			if _, cmd := pressed(t, press(t, screen, "j"), keyEnter); cmd != nil {
				t.Error("enter sent something with nothing to apply")
			}

			if view := press(t, screen, "esc").View(); strings.Contains(view, pickerTitle) {
				t.Errorf("esc did not close the picker:\n%s", view)
			}
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

			next, cmd := pressed(t, screen, "t")
			if cmd != nil || strings.Contains(next.View(), pickerTitle) {
				t.Errorf("t opened a picker with no issue to move:\n%s", next.View())
			}
		})
	}
}

func TestALateListingIsNotShownForTheWrongIssue(t *testing.T) {
	t.Parallel()

	fake := &fakeJira{moves: workflowMoves()}
	screen := jiraScreen(t, fake.deps(twoIssues()))

	// Open on OPS-1 and walk away before the answer; then open on OPS-2.
	screen, forFirst := pressed(t, screen, "t")
	screen = press(t, screen, "esc", "j")
	screen, forSecond := pressed(t, screen, "t")

	screen, _ = finish(t, screen, forFirst)

	if view := screen.View(); !strings.Contains(view, "loading") {
		t.Errorf("OPS-1's transitions were shown for OPS-2:\n%s", view)
	}

	screen, _ = finish(t, screen, forSecond)

	if view := screen.View(); !strings.Contains(view, "▸ ◐ Start Review") {
		t.Errorf("OPS-2's own transitions were not shown:\n%s", view)
	}
}

func TestASecondListingDoesNotReplaceTheOneOnScreen(t *testing.T) {
	t.Parallel()

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
	screen = press(t, screen, "esc")
	screen, second := pressed(t, screen, "t")

	screen, _ = finish(t, screen, first)
	screen = press(t, screen, "j")
	screen, _ = finish(t, screen, second)

	if view := screen.View(); !strings.Contains(view, "▸ ● Done") {
		t.Errorf("the second listing replaced the list being chosen from:\n%s", view)
	}
}

func TestALongPickerScrollsToKeepTheSelectionVisible(t *testing.T) {
	t.Parallel()

	moves := make([]jira.Transition, 0, 40)
	for index := 1; index <= 40; index++ {
		moves = append(moves, transition(strconv.Itoa(index), fmt.Sprintf("Step %d", index), "Doing", "indeterminate"))
	}

	fake := &fakeJira{moves: moves}
	screen := openPicker(t, jiraScreen(t, fake.deps(twoIssues())))

	keys := make([]string, 39)
	for index := range keys {
		keys[index] = "j"
	}

	if view := press(t, screen, keys...).View(); !strings.Contains(view, "▸ ◐ Step 40") {
		t.Errorf("the last transition scrolled out of view when selected:\n%s", view)
	}
}

func TestTheFooterOffersTheKeysThatDoSomethingHere(t *testing.T) {
	t.Parallel()

	fake := &fakeJira{moves: workflowMoves()}
	screen := jiraScreen(t, fake.deps(twoIssues()))

	footer := func(model tui.Model) string {
		lines := strings.Split(model.View(), "\n")

		return lines[len(lines)-1]
	}

	if got := footer(screen); !strings.Contains(got, "t change status") {
		t.Errorf("the Issues pane's footer does not offer t: %q", got)
	}

	if got := footer(press(t, screen, "2")); strings.Contains(got, "change status") {
		t.Errorf("the Branch pane's footer offers a key that does nothing there: %q", got)
	}

	if got := footer(jiraScreen(t, fake.deps(assigned()))); strings.Contains(got, "change status") {
		t.Errorf("an empty list's footer offers a key that does nothing there: %q", got)
	}

	if got := footer(openPicker(t, screen)); !strings.Contains(got, "enter apply") || !strings.Contains(got, "esc") {
		t.Errorf("the picker's footer does not say how to apply or leave: %q", got)
	}
}

func TestThePickerFitsANarrowTerminal(t *testing.T) {
	t.Parallel()

	fake := &fakeJira{moves: workflowMoves()}
	screen := openPicker(t, started(t, sized(t, tui.New(completeConfig(), nil, fake.deps(twoIssues())), 80, 30)))

	view := screen.View()
	if !strings.Contains(view, pickerTitle) || !strings.Contains(view, "▸ ◐ Start Review → In Review") {
		t.Errorf("the narrow picker does not show its transitions:\n%s", view)
	}

	for index, line := range strings.Split(view, "\n") {
		if width := len([]rune(line)); width > 80 {
			t.Errorf("line %d is %d cells, wider than the terminal: %q", index, width, line)
		}
	}
}
