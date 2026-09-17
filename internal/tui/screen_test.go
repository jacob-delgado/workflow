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

// detailTop is the detail pane's first row for the world's issue, which only
// shows while the detail is scrolled to the top. The rail lists the same issue,
// but behind a marker rather than the detail's border.
const detailTop = "│ " + issueKey + " " + issueSummary

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

	cases := map[string]struct {
		height int
		keys   []string
		want   []string
	}{
		"the screen": {
			height: 40,
			want:   []string{"# Issue - # Branch", "+= 1 Issues", "> * PROJ-412", "Bug - In Progress", " | tab next pane"},
		},
		"the help": {height: 60, keys: []string{"?"}, want: []string{"up/k", "down/j"}},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Act
			view := typing(t, asciiInterface(t, newWorld(), 120, tt.height), tt.keys...).View()

			// Assert
			requireScreen(t, view, tt.want...)

			for _, character := range view {
				if character > 0x7e {
					t.Fatalf("an ASCII screen drew %q:\n%s", character, view)
				}
			}
		})
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

			// Arrange
			staged := newWorld()
			tt.prepare(staged)

			// Act
			spine, _, _ := strings.Cut(staged.live(t, 120, 40).View(), "\n")

			// Assert
			requireScreen(t, spine, tt.want)
		})
	}
}

func TestTheSpineMarksSlackOnceAnnounced(t *testing.T) {
	t.Parallel()

	// Act
	posted := typing(t, newWorld().live(t, 120, 40), "5", "p", keyEnter)

	// Assert
	spine, _, _ := strings.Cut(posted.View(), "\n")
	requireScreen(t, spine, "● Slack")
}

func TestAShortTerminalCompactsTheSpine(t *testing.T) {
	t.Parallel()

	// Act
	spine, _, _ := strings.Cut(newWorld().live(t, 120, 20).View(), "\n")

	// Assert
	if !strings.HasPrefix(spine, " I● B● C● R● S○") {
		t.Errorf("spine = %q, want each stage labeled with its initial", spine)
	}
}

func TestACompactCollapsedScreenNamesItsStagesAndPane(t *testing.T) {
	t.Parallel()

	// Act
	view := typing(t, newWorld().live(t, 79, 20), "3").View()

	// Assert
	spine, _, _ := strings.Cut(view, "\n")
	requireScreen(t, spine, "I●", "B●", "C●")
	requireScreen(t, view, "3 Commits")
}

func TestADryRunSaysSoOnEveryScreen(t *testing.T) {
	t.Parallel()

	cases := map[string][]string{
		"at start":             nil,
		"on another pane":      {"3"},
		"under the help":       {"?"},
		"under an open picker": {"t"},
	}

	for name, keys := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			faked := newWorld()
			faked.moves = workflowMoves()
			model := sized(t, dryInterface(faked), 120, 40)
			model = drain(t, model, model.Init())

			// Act
			spine, _, _ := strings.Cut(typing(t, model, keys...).View(), "\n")

			// Assert
			if !strings.HasPrefix(spine, " DRY RUN · ") {
				t.Errorf("spine = %q, want it to say this is a dry run", spine)
			}
		})
	}
}

func TestAVeryNarrowTerminalDropsTheBorder(t *testing.T) {
	t.Parallel()

	// Act
	view := newWorld().live(t, 50, 20).View()

	// Assert
	lines := strings.Split(view, "\n")
	if len(lines) < 2 {
		t.Fatalf("drew %d lines, want the spine and the detail:\n%s", len(lines), view)
	}

	if lines[1] != "1 Issues"+strings.Repeat(" ", 42) {
		t.Errorf("the detail's first row = %q, want its numbered title with no border", lines[1])
	}

	refuseScreen(t, view, "┌", "┏", "│")
	requireScreen(t, view, "▸ ◐ PROJ-412")
}

func TestLongDetailScrolls(t *testing.T) {
	t.Parallel()

	// Arrange
	wordy := newWorld()
	wordy.detail.Description = strings.Repeat("line of the description\n", 60) + "THE END"

	// Act: open on a description longer than the pane
	screen := wordy.live(t, 120, 30)

	// Assert: its end is out of sight
	requireScreen(t, screen.View(), detailTop)
	refuseScreen(t, screen.View(), "THE END")

	// Act: scroll down
	scrolled := typing(t, screen, "pgdown", "pgdown", "pgdown", "pgdown", "pgdown", "J")

	// Assert: the end is in sight, and the top is not
	requireScreen(t, scrolled.View(), "THE END")
	refuseScreen(t, scrolled.View(), detailTop)

	// Act: scroll back up
	back := typing(t, scrolled, "pgup", "pgup", "pgup", "pgup", "pgup", "pgup", "K", "K")

	// Assert: the top is in sight again
	requireScreen(t, back.View(), detailTop)
	refuseScreen(t, back.View(), "THE END")
}

func TestMovingToAnotherPaneStartsTheDetailAtTheTop(t *testing.T) {
	t.Parallel()

	// Arrange
	wordy := newWorld()
	wordy.detail.Description = strings.Repeat("line of the description\n", 60) + "THE END"
	scrolled := typing(t, wordy.live(t, 120, 30), "pgdown", "pgdown", "pgdown", "pgdown", "pgdown", "J")

	// Act
	view := typing(t, scrolled, keyTab, keyShiftTab).View()

	// Assert
	requireScreen(t, view, detailTop)
	refuseScreen(t, view, "THE END")
}

func TestTheHelpListsEveryGroupAndScrolls(t *testing.T) {
	t.Parallel()

	// Arrange
	model := newWorld().live(t, 120, 20)

	// Act: open the help on a short terminal
	short := typing(t, model, "?")

	// Assert: it starts at the first group, says there is more, and the bottom
	// of the taller column is out of sight
	requireScreen(t, short.View(), "┌─ Keys", "Moving around", "… more below")
	refuseScreen(t, short.View(), "Review and Slack")
	requireScreen(t, footerLine(short.View()), "esc close", "q quit")

	// Act: page down
	paged := typing(t, short, "pgdown", "pgdown", "pgdown")

	// Assert: the group that was out of sight is in sight
	requireScreen(t, paged.View(), "Review and Slack")
}

func TestQQuitsFromTheHelp(t *testing.T) {
	t.Parallel()

	// Arrange
	help := typing(t, newWorld().live(t, 120, 20), "?")

	// Act
	_, cmd := pressed(t, help, "q")

	// Assert
	if got, want := messageType(cmd), messageType(tea.Quit); got != want {
		t.Errorf("q from the help returned %s, want %s", got, want)
	}
}

func TestATransitionWithNoIssueURLAnnouncesWithoutALink(t *testing.T) {
	t.Parallel()

	// Arrange
	deps := newWorld().deps()
	deps.Jira.BrowseURL = nil
	model := sized(t, tui.New(completeConfig(), nil, deps), 120, 40)
	model = drain(t, model, model.Init())

	// Act
	view := typing(t, model, "5").View()

	// Assert
	requireScreen(t, view, "PROJ-412 "+issueSummary)
	refuseScreen(t, view, "browse/PROJ-412")
}

func TestPaneKeysNeedWhatTheyActOn(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		pane string
		key  string
	}{
		"t on Issues":      {pane: "1", key: "t"},
		"c on Issues":      {pane: "1", key: "c"},
		"b on Issues":      {pane: "1", key: "b"},
		"b on Branch":      {pane: "2", key: "b"},
		"P on Branch":      {pane: "2", key: "P"},
		"r on Branch":      {pane: "2", key: "r"},
		"space on Commits": {pane: "3", key: keySpace},
		"a on Commits":     {pane: "3", key: "a"},
		"c on Commits":     {pane: "3", key: "c"},
		"h on Commits":     {pane: "3", key: "h"},
		"n on Review":      {pane: "4", key: "n"},
		"p on Slack":       {pane: "5", key: "p"},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			// With nothing but a search wired, no key reaches anything outside.
			bare := sized(t, tui.New(completeConfig(), nil, tui.Deps{Jira: tui.JiraDeps{
				Search: func() (jira.SearchResult, error) {
					return jira.SearchResult{Issues: []jira.Issue{{Key: issueKey, Summary: issueSummary}}, Total: 1}, nil
				},
			}}), 120, 40)
			pane := press(t, drain(t, bare, bare.Init()), tt.pane)

			// Act
			after, cmd := pressed(t, pane, tt.key)

			// Assert
			if cmd != nil {
				t.Errorf("%s produced a command with nothing to act on", name)
			}

			if after.View() != pane.View() {
				t.Errorf("%s changed the screen with nothing to act on:\n%s", name, after.View())
			}
		})
	}
}

func TestANarrowFooterDropsWholeKeysAndKeepsTheWayToTheRest(t *testing.T) {
	t.Parallel()

	// Act
	footer := strings.TrimRight(footerLine(newWorld().live(t, 80, 30).View()), " ")

	// Assert
	requireScreen(t, footer, "t change status", "? keys", "…")

	if strings.HasSuffix(footer, "•") || strings.Contains(footer, "tab next pane •") {
		t.Errorf("the footer cuts a key in half:\n%q", footer)
	}
}

func TestANarrowASCIIFooterUsesItsOwnEllipsis(t *testing.T) {
	t.Parallel()

	// Act
	footer := strings.TrimRight(footerLine(asciiInterface(t, newWorld(), 70, 30).View()), " ")

	// Assert
	if !strings.HasSuffix(footer, "...") {
		t.Errorf("an ASCII footer ends %q, want its own ellipsis", footer)
	}
}

func TestAWordWiderThanThePaneIsCutRatherThanLost(t *testing.T) {
	t.Parallel()

	// Arrange
	linked := newWorld()
	address := "https://example.com/" + strings.Repeat("a", 100) + "/END"
	linked.detail.Description = "see " + address + " for more"

	// Act
	view := linked.live(t, 120, 40).View()

	// Assert
	// Nothing of the address is dropped: every piece of it is on screen.
	requireScreen(t, view, "see", "/END for more", "https://example.com/aaaa", strings.Repeat("a", 30))
}

func TestASCIIStaysASCIIInEveryOverlay(t *testing.T) {
	t.Parallel()

	blocked := resolveIssue()
	blocked.Fields = append(blocked.Fields, jira.Field{ID: "assignee", Name: "Assignee", Kind: jira.FieldUnsupported})

	cases := map[string]struct {
		prepare func(*world)
		keys    []string
	}{
		"the status picker with a field only Jira can fill": {
			prepare: func(w *world) { w.moves = []jira.Transition{blocked} },
			keys:    []string{"t", keyEnter},
		},
		"the new-branch overlay":    {keys: []string{"2", "b"}},
		"the commit composer":       {keys: []string{"3", "c"}},
		"the comment preview":       {prepare: func(w *world) { w.edited = "Looks good" }, keys: []string{"c"}},
		"the pull-request composer": {prepare: func(w *world) { w.pullFound = false }, keys: []string{"4", "n"}},
		"the slack preview":         {keys: []string{"5", "p"}},
		// commandRun is excluded on purpose: its body is a tool's own output, not
		// interface-authored text, so it is outside the ASCII-glyph guarantee.
		"the lefthook offer": {prepare: func(w *world) { w.gitHooks = legacyHooks() }},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			faked := newWorld()
			if tt.prepare != nil {
				tt.prepare(faked)
			}

			// Act
			view := typing(t, asciiInterface(t, faked, 120, 50), tt.keys...).View()

			// Assert
			for _, character := range view {
				if character > 0x7e {
					t.Fatalf("%s drew %q above ASCII:\n%s", name, character, view)
				}
			}
		})
	}
}

func TestReloadIsOfferedInEveryPaneThatReloads(t *testing.T) {
	t.Parallel()

	cases := map[string]struct{ pane string }{
		"the Issues pane":  {pane: "1"},
		"the Branch pane":  {pane: "2"},
		"the Commits pane": {pane: "3"},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			model := newWorld().live(t, 160, 40)

			// Act
			view := typing(t, model, tt.pane).View()

			// Assert
			requireScreen(t, footerLine(view), "r refresh")
		})
	}
}

func TestANoticeSurvivesMovingAround(t *testing.T) {
	t.Parallel()

	// Arrange
	dry := newWorld()
	dry.branch.Ahead = 1
	model := sized(t, dryInterface(dry), 120, 40)
	model = drain(t, model, model.Init())
	pushed := typing(t, model, "2", "P", keyEnter)

	// Act
	after := typing(t, pushed, "j", "k", keyTab)
	view := after.View()

	// Assert
	requireScreen(t, view, "dry run: would push "+featureName)
	requireScreen(t, footerLine(view), "keys")
}
