// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/gitrepo"
	"github.com/jacob-delgado/workflow/internal/jira"
	"github.com/jacob-delgado/workflow/internal/tui"
)

// asciiInterface is the world's interface with ui.ascii on.
func asciiInterface(t *testing.T, faked *world, width, height int) tui.Model {
	t.Helper()

	cfg := completeConfig()
	cfg.UI.ASCII = true

	model := sized(t, tui.New(cfg, nil, faked.deps()), width, height)

	return drain(t, model, model.Init())
}

func TestASCIIModeDrawsInASCII(t *testing.T) {
	t.Parallel()

	view := asciiInterface(t, newWorld(), 120, 40).View()
	requireScreen(t, view, "# Issue - # Branch", "#= 1 Issues", "> * PROJ-412", "Bug - In Progress",
		" | tab next pane")

	help := typing(t, asciiInterface(t, newWorld(), 120, 60), "?").View()
	requireScreen(t, help, "up/k", "down/j")

	for _, screen := range []string{view, help} {
		for _, character := range screen {
			if character > 0x7e {
				t.Fatalf("an ASCII screen drew %q:\n%s", character, screen)
			}
		}
	}
}

func TestTheSpineShowsHowFarTheWorkHasGot(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		prepare func(*world)
		want    string
	}{
		"just started": {
			prepare: func(w *world) {
				w.branch = gitrepo.Branch{Name: baseName, Base: baseRef}
				w.changes, w.pullFound = nil, false
			},
			want: "◐ Issue ─ ○ Branch ─ ○ Commits ─ ○ Review ─ ○ Slack",
		},
		"changes on a branch": {
			prepare: func(w *world) { w.branch.Commits, w.pullFound = nil, false },
			want:    "● Issue ─ ● Branch ─ ◐ Commits ─ ○ Review ─ ○ Slack",
		},
		"in review": {
			prepare: func(w *world) { w.ci = []forge.CI{{State: forge.CIRunning}} },
			want:    "● Issue ─ ● Branch ─ ● Commits ─ ◐ Review ─ ○ Slack",
		},
		"CI failed": {
			prepare: func(w *world) { w.ci = []forge.CI{{State: forge.CIFailed}} },
			want:    "● Issue ─ ● Branch ─ ● Commits ─ ✗ Review ─ ○ Slack",
		},
		"nothing selected": {
			prepare: func(w *world) {
				w.issues, w.branch, w.changes, w.pullFound = nil, gitrepo.Branch{}, nil, false
			},
			want: "○ Issue ─ ○ Branch ─ ○ Commits ─ ○ Review ─ ○ Slack",
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			staged := newWorld()
			tt.prepare(staged)

			if spine, _, _ := strings.Cut(staged.live(t, 120, 40).View(), "\n"); !strings.Contains(spine, tt.want) {
				t.Errorf("spine = %q, want %q", spine, tt.want)
			}
		})
	}

	posted := typing(t, newWorld().live(t, 120, 40), "5", "p", keyEnter)
	requireScreen(t, strings.Split(posted.View(), "\n")[0], "● Slack")
}

func TestAShortTerminalCompactsTheSpine(t *testing.T) {
	t.Parallel()

	spine, _, _ := strings.Cut(newWorld().live(t, 120, 20).View(), "\n")
	if !strings.HasPrefix(spine, " [●●●●○]") {
		t.Errorf("spine = %q, want the glyphs alone", spine)
	}
}

func TestADryRunSaysSoOnEveryScreen(t *testing.T) {
	t.Parallel()

	model := sized(t, dryInterface(newWorld()), 120, 40)

	if spine, _, _ := strings.Cut(model.View(), "\n"); !strings.HasPrefix(spine, " DRY RUN · ") {
		t.Errorf("spine = %q, want it to say this is a dry run", spine)
	}
}

func TestAVeryNarrowTerminalDropsTheBorder(t *testing.T) {
	t.Parallel()

	view := newWorld().live(t, 50, 20).View()
	lines := strings.Split(view, "\n")

	if lines[1] != "Issues"+strings.Repeat(" ", 44) {
		t.Errorf("the detail's first row = %q, want its title with no border", lines[1])
	}

	refuseScreen(t, view, "┌", "┏", "│")
	requireScreen(t, view, "▸ ◐ PROJ-412")
}

func TestLongDetailScrolls(t *testing.T) {
	t.Parallel()

	wordy := newWorld()
	wordy.detail.Description = strings.Repeat("line of the description\n", 60) + "THE END"

	screen := wordy.live(t, 120, 30)
	refuseScreen(t, screen.View(), "THE END")

	scrolled := typing(t, screen, "pgdown", "pgdown", "pgdown", "pgdown", "pgdown", "J")
	requireScreen(t, scrolled.View(), "THE END")

	back := typing(t, scrolled, "pgup", "pgup", "pgup", "pgup", "pgup", "pgup", "K", "K")
	requireScreen(t, back.View(), "PROJ-412 "+issueSummary)

	// Moving to another pane starts at the top.
	requireScreen(t, typing(t, scrolled, keyTab, keyShiftTab).View(), "PROJ-412 "+issueSummary)
}

func TestTheWheelScrollsTheDetail(t *testing.T) {
	t.Parallel()

	wordy := newWorld()
	wordy.detail.Description = strings.Repeat("line\n", 60) + "THE END"

	screen := wordy.live(t, 120, 30)

	for range 25 {
		screen = wheel(t, screen, 80, 10, tea.MouseButtonWheelDown)
	}

	requireScreen(t, screen.View(), "THE END")

	for range 30 {
		screen = wheel(t, screen, 80, 10, tea.MouseButtonWheelUp)
	}

	requireScreen(t, screen.View(), "PROJ-412 "+issueSummary)

	// The wheel over the rail moves nothing.
	requireScreen(t, wheel(t, screen, 5, 5, tea.MouseButtonWheelDown).View(), "PROJ-412 "+issueSummary)
}

// wheel is one notch of the mouse wheel over a cell.
func wheel(t *testing.T, model tui.Model, column, row int, button tea.MouseButton) tui.Model {
	t.Helper()

	updated, cmd := model.Update(tea.MouseMsg{X: column, Y: row, Action: tea.MouseActionPress, Button: button})

	return drain(t, concrete(t, updated), cmd)
}

func TestTheWheelMovesAnOverlaysList(t *testing.T) {
	t.Parallel()

	choosing := newWorld()
	choosing.moves = workflowMoves()

	picker := typing(t, choosing.live(t, 120, 40), "t")
	requireScreen(t, wheel(t, picker, 80, 10, tea.MouseButtonWheelDown).View(), "▸ ● Done")
	requireScreen(t, wheel(t, picker, 80, 10, tea.MouseButtonWheelUp).View(), "▸ ◐ Start Review")
}

func TestClickingPicksRows(t *testing.T) {
	t.Parallel()

	choosing := newWorld()
	choosing.moves = workflowMoves()

	// The issue list in the focused rail pane: its second row is row 3.
	requireScreen(t, click(t, choosing.live(t, 120, 40), 5, 3).View(), "▸ ○ PROJ-388")

	// The picker's second transition is row 6: the border, the issue, its
	// status and a blank line come first.
	picker := typing(t, choosing.live(t, 120, 40), "t")
	requireScreen(t, click(t, picker, 60, 6).View(), "▸ ● Done")
	requireScreen(t, click(t, picker, 60, 2).View(), "▸ ◐ Start Review")

	// A click outside the overlay does nothing while it is open.
	requireScreen(t, click(t, picker, 5, 30).View(), "┏━ Change status")

	// On a narrow terminal the list is the detail.
	narrow := choosing.live(t, 80, 30)
	requireScreen(t, click(t, narrow, 10, 3).View(), "▸ ○ PROJ-388")
}

func TestClickingARunsFailuresPicksOne(t *testing.T) {
	t.Parallel()

	failing := newWorld()
	failing.commitErr = errHookFailed
	failing.commitLines = []string{"a.go:1:1: first", "b.go:2:1: second"}

	failed := typing(t, failing.live(t, 120, 40), commitKeys("x")...)

	// The border, the outcome and a blank line come before the list.
	picked := click(t, failed, 60, 5)
	requireScreen(t, picked.View(), "▸ b.go:2 second")
	requireScreen(t, click(t, picked, 60, 1).View(), "▸ b.go:2 second")
}

func TestTheMouseSettingDecidesWhetherItIsCaptured(t *testing.T) {
	t.Parallel()

	cfg := completeConfig()
	cfg.UI.Mouse = false

	// With capture off, m turns it on.
	_, cmd := tui.New(cfg, nil, tui.Deps{}).Update(keyMsg("m"))
	if cmd == nil {
		t.Fatal("m did nothing")
	}

	if _, ok := cmd().(tea.MouseMsg); ok {
		t.Error("m with the mouse off reported a mouse event instead of enabling it")
	}
}

func TestTheHelpListsEveryGroupAndScrolls(t *testing.T) {
	t.Parallel()

	short := typing(t, newWorld().live(t, 120, 20), "?")
	requireScreen(t, short.View(), "┌─ Keys", "Moving around")
	requireScreen(t, footerLine(short.View()), "esc close", "q quit")

	requireScreen(t, typing(t, short, "pgdown", "pgdown", "pgdown").View(), "Everywhere")

	if _, cmd := pressed(t, short, "q"); cmd == nil {
		t.Error("q did not quit from the help")
	}
}

func TestATransitionWithNoIssueURLAnnouncesWithoutALink(t *testing.T) {
	t.Parallel()

	unlinked := newWorld()
	deps := unlinked.deps()
	deps.Jira.BrowseURL = nil

	model := sized(t, tui.New(completeConfig(), nil, deps), 120, 40)
	model = drain(t, model, model.Init())

	requireScreen(t, typing(t, model, "5").View(), "PROJ-412 "+issueSummary)
	refuseScreen(t, typing(t, model, "5").View(), "browse/PROJ-412")
}

func TestPaneKeysNeedWhatTheyActOn(t *testing.T) {
	t.Parallel()

	// With nothing wired, no key reaches anything outside.
	bare := sized(t, tui.New(completeConfig(), nil, tui.Deps{Jira: tui.JiraDeps{
		Search: func() (jira.SearchResult, error) {
			return jira.SearchResult{Issues: []jira.Issue{{Key: issueKey, Summary: issueSummary}}, Total: 1}, nil
		},
	}}), 120, 40)
	bare = drain(t, bare, bare.Init())

	for _, key := range []string{"t", "c", "b", "r", "2", "b", "P", "3", "space", "a", "c", "h", "4", "n", "5", "p"} {
		updated, _ := bare.Update(keyMsg(key))
		bare = concrete(t, updated)
	}

	refuseScreen(t, bare.View(), "Change status", "Comment on", "New branch", "git push", "┏━ Commit",
		"pre-commit", "Open pull request", "Post to Slack")
}

func TestANarrowFooterDropsWholeKeysAndKeepsTheWayToTheRest(t *testing.T) {
	t.Parallel()

	footer := strings.TrimRight(footerLine(newWorld().live(t, 70, 30).View()), " ")
	requireScreen(t, footer, "t change status", "? keys", "…")

	if strings.HasSuffix(footer, "•") || strings.Contains(footer, "tab next pane •") {
		t.Errorf("the footer cuts a key in half:\n%q", footer)
	}

	ascii := strings.TrimRight(footerLine(asciiInterface(t, newWorld(), 70, 30).View()), " ")
	if !strings.HasSuffix(ascii, "...") {
		t.Errorf("an ASCII footer ends %q, want its own ellipsis", ascii)
	}
}

func TestAWordWiderThanThePaneIsCutRatherThanLost(t *testing.T) {
	t.Parallel()

	linked := newWorld()
	address := "https://example.com/" + strings.Repeat("a", 100) + "/END"
	linked.detail.Description = "see " + address + " for more"

	view := linked.live(t, 120, 40).View()
	requireScreen(t, view, "see", "/END for more")

	// Nothing of the address is dropped: every piece of it is on screen.
	for _, piece := range []string{"https://example.com/aaaa", strings.Repeat("a", 30), "/END"} {
		requireScreen(t, view, piece)
	}
}
