// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

import (
	"slices"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/jacob-delgado/workflow/internal/tui"
)

// freeKey is a key nothing is bound to.
const freeKey = "f12"

// fixedAction is the one placed action ui.keys cannot move: its keys are the
// pane numbers, one per pane, and CheckKeys refuses an override of it.
const fixedAction = "jump-to-pane"

// listedBinding is one binding as the help lists it: the action it answers to,
// the key shown for it and what it says it does. A binding with no help of its
// own rides another's line, and is listed here with none.
type listedBinding struct {
	action, key, help string
}

// helpGroup is one group of the help, by name, with its bindings in order.
type helpGroup struct {
	name     string
	bindings []listedBinding
}

// placedBindings is every binding the interface places, in the group where it
// works and the order ? lists them, under the default keys, a pull request and
// Slack. Nothing exported enumerates them, so this table is the test's own; the
// tests below prove it against the running interface both ways. Each line is an
// action, the key shown for it and what it says it does.
func placedBindings() []helpGroup {
	return []helpGroup{
		placed("Moving around",
			"next-pane         tab        next pane",
			"previous-pane     shift+tab  previous pane",
			"jump-to-pane      1-6        jump to pane",
			"up                ↑/k        up",
			"down              ↓/j        down",
			"scroll-up         pgup/K     scroll up",
			"scroll-down       pgdn/J     scroll down"),
		placed("Issues",
			"change-status     t          change status",
			"comment           c          comment",
			"assign            a          assign",
			"log-work          w          log work",
			"branch-for-issue  b          branch for issue",
			"filter            /          filter",
			"switch-view       v          switch view",
			"load-more         ctrl+n     load more",
			"open-link         o          open",
			"copy-link         y          copy url",
			"refresh           r          refresh"),
		placed("Branch and Commits",
			"new-branch        b          new branch",
			"switch-task       s          switch task",
			"rebase            u          rebase onto base",
			"push              P          push",
			"stage             space      stage/unstage",
			"stage-all         a          stage all",
			"commit            c          commit",
			"amend             A          amend",
			"fixup             f          fix up",
			"run-pre-commit    h          run pre-commit",
			"set-up-lefthook   g          set up lefthook"),
		placed("Review and Slack",
			"open-pull-request n          open pull request",
			"checks            c          checks",
			"rerun-checks      R          re-run checks",
			"merge             M          merge",
			"finish-branch     F          finish branch",
			"post              p          announce to slack"),
		placed("In a composer or preview",
			"edit              e          edit",
			"edit-body         ctrl+o     edit body",
			"next-template     ctrl+t     next template",
			"toggle-draft      ctrl+r     draft",
			"toggle-breaking   ctrl+b     breaking",
			"verbatim          v          keep scripts whole",
			"next-field        tab        next field",
			"previous-field    shift+tab  previous field",
			"cycle-type-left   ←/→        change type",
			"cycle-type-right",
			"toggle-option     space      select",
			"worktree          ctrl+w     worktree",
			"post-when-green   w          announce when CI passes"),
		placed("While a command runs",
			"stop              s          stop",
			"run-again         r          run again",
			"full-output       o          full output"),
		placed("Everywhere",
			"apply             enter      apply",
			"close             esc        close",
			"toggle-mouse      m          toggle mouse",
			"toggle-help       ?          keys",
			"quit              q          quit",
			"interrupt         ctrl+c     quit"),
	}
}

// placed is a group of the table: each line an action, then its key and help,
// or the action alone for a binding with no help of its own.
func placed(name string, lines ...string) helpGroup {
	group := helpGroup{name: name}

	for _, line := range lines {
		fields := append(strings.Fields(line), "", "")
		group.bindings = append(group.bindings, listedBinding{
			action: fields[0], key: fields[1], help: strings.TrimSpace(strings.Join(fields[2:], " ")),
		})
	}

	return group
}

// String is the group as one line, for comparing listings and showing a diff.
func (g helpGroup) String() string {
	lines := make([]string, 0, len(g.bindings))
	for _, binding := range g.bindings {
		lines = append(lines, binding.key+" "+binding.help)
	}

	return g.name + ": " + strings.Join(lines, "; ")
}

// listed is a group as the help draws it: every binding with help of its own,
// by the key shown and what it says, without the action no screen shows.
func (g helpGroup) listed() helpGroup {
	drawn := helpGroup{name: g.name}
	for _, binding := range g.listedActions() {
		drawn.bindings = append(drawn.bindings, listedBinding{key: binding.key, help: binding.help})
	}

	return drawn
}

// openHelp opens the help on a terminal tall enough for all of it, with keys
// rebound by overrides — the help's own key among them.
func openHelp(t *testing.T, overrides map[string]string) string {
	t.Helper()

	cfg := completeConfig()
	cfg.UI.Keys = overrides

	helpKey, rebound := overrides["toggle-help"]
	if !rebound {
		helpKey = "?"
	}

	return plain(press(t, sized(t, tui.New(cfg, nil, tui.Deps{}), 160, 70), helpKey).View().Content)
}

// runeColumn is the column, in runes, where text starts on line.
func runeColumn(line, text string) int {
	before, _, _ := strings.Cut(line, text)

	return utf8.RuneCountInString(before)
}

// helpListing reads the help's two columns back into groups: a column's line
// that starts flush is a group's name, and one indented under it is a key and
// what it does. The listing ends at the pane's bottom border.
func helpListing(t *testing.T, view string) []helpGroup {
	t.Helper()

	lines := strings.Split(view, "\n")
	top := slices.IndexFunc(lines, func(line string) bool { return strings.Contains(line, "Moving around") })

	if top < 0 {
		t.Fatalf("the help is not on screen:\n%s", view)
	}

	_, afterFirst, _ := strings.Cut(lines[top], "Moving around")
	left, right := runeColumn(lines[top], "Moving around"), runeColumn(lines[top], strings.TrimLeft(afterFirst, " "))

	var columns [2][]helpGroup

	for _, line := range lines[top:] {
		runes := []rune(line)
		if runes[left-2] != '│' {
			break
		}

		columns[0] = readHelpCell(columns[0], string(runes[left:right]))
		columns[1] = readHelpCell(columns[1], strings.TrimRight(string(runes[right:]), " │"))
	}

	return append(columns[0], columns[1]...)
}

// readHelpCell adds one column's cell of one line to the groups read so far.
func readHelpCell(groups []helpGroup, cell string) []helpGroup {
	switch {
	case strings.TrimSpace(cell) == "":
		return groups
	case !strings.HasPrefix(cell, " "):
		return append(groups, helpGroup{name: strings.TrimSpace(cell)})
	default:
		key, help, _ := strings.Cut(strings.TrimSpace(cell), " ")
		last := &groups[len(groups)-1]
		last.bindings = append(last.bindings, listedBinding{key: key, help: strings.TrimSpace(help)})

		return groups
	}
}

func TestHelpListsEveryPlacedBinding(t *testing.T) {
	t.Parallel()

	// Arrange
	groups := placedBindings()

	want := make([]string, 0, len(groups))
	for _, group := range groups {
		want = append(want, group.listed().String())
	}

	// Act
	listing := helpListing(t, openHelp(t, nil))

	// Assert
	got := make([]string, 0, len(listing))
	for _, group := range listing {
		got = append(got, group.String())
	}

	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("? lists\n%s\nwant every placed binding, grouped where it works:\n%s",
			strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

// helpPageCount is how many screens helpPages reads: enough, a half page apart,
// to reach the end of the help on the shortest terminal a test opens it on.
const helpPageCount = 9

// helpPages opens the help and pages down through it, one screen after
// another, so a terminal too short for the whole list still shows every line.
func helpPages(t *testing.T, model tui.Model) string {
	t.Helper()

	model = typing(t, model, "?")
	pages := make([]string, 0, helpPageCount)

	for range helpPageCount {
		pages = append(pages, plain(model.View().Content))
		model = typing(t, model, "pgdown")
	}

	return strings.Join(pages, "\n")
}

// cutHelpLines is the help's lines that end in an ellipsis at the pane's edge,
// read from the column where the help starts so the rail beside it is not.
func cutHelpLines(t *testing.T, view string) []string {
	t.Helper()

	lines := strings.Split(plain(view), "\n")
	top := slices.IndexFunc(lines, func(line string) bool { return strings.Contains(line, "Moving around") })

	if top < 0 {
		t.Fatalf("the help is not on screen:\n%s", view)
	}

	left := runeColumn(lines[top], "Moving around")

	var cut []string

	for _, line := range lines[top : len(lines)-1] {
		runes := []rune(line)
		if len(runes) > left && strings.HasSuffix(strings.TrimRight(string(runes[left:]), " │"), "…") {
			cut = append(cut, line)
		}
	}

	return cut
}

func TestOnAnEightyColumnTerminalTheHelpNamesEveryKeyWhole(t *testing.T) {
	t.Parallel()

	// Act
	pages := helpPages(t, newWorld().live(t, 80, 24))

	// Assert
	requireScreen(t, pages, "In a composer or preview", "ctrl+w    worktree", "w         announce when CI passes")
}

func TestTheHelpCutsNoLineShortAtThePanesEdge(t *testing.T) {
	t.Parallel()

	widths := map[string]int{"beside a narrow rail": 80, "beside a wider rail": 100, "in two columns": 160}

	for name, width := range widths {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Act
			view := typing(t, newWorld().live(t, width, 40), "?").View().Content

			// Assert
			if cut := cutHelpLines(t, view); len(cut) > 0 {
				t.Errorf("the help cuts %d lines short at the edge:\n%s", len(cut), strings.Join(cut, "\n"))
			}
		})
	}
}

// firstHelpLine is the help's top line as the Keys box shows it, read from the
// box's left edge so the rail beside it is not.
func firstHelpLine(t *testing.T, view string) string {
	t.Helper()

	lines := strings.Split(plain(view), "\n")
	top := slices.IndexFunc(lines, func(line string) bool { return strings.Contains(line, "┌─ Keys") })

	if top < 0 || top+1 == len(lines) {
		t.Fatalf("the help is not on screen:\n%s", view)
	}

	return string([]rune(lines[top+1])[runeColumn(lines[top], "┌─ Keys"):])
}

func TestTheHelpPagesBackAtOnceAfterPagingPastTheEnd(t *testing.T) {
	t.Parallel()

	// Arrange
	help := typing(t, newWorld().live(t, 120, 20), "?")
	pastTheEnd := typing(t, help, pagesDown(helpPageCount)...)
	before := firstHelpLine(t, pastTheEnd.View().Content)

	// Act
	back := typing(t, pastTheEnd, "pgup")

	// Assert
	if after := firstHelpLine(t, back.View().Content); after == before {
		t.Errorf("one pgup after paging past the end left the help's top line at %q, want it moved up", after)
	}
}

func TestTheWholeHelpFitsATallTerminal(t *testing.T) {
	t.Parallel()

	// Act
	view := typing(t, newWorld().live(t, 120, 40), "?").View().Content

	// Assert
	requireScreen(t, view, "Everywhere")
	refuseScreen(t, view, "more below")
}

func TestAHelpThatFitsThePaneOffersNoScrollKeys(t *testing.T) {
	t.Parallel()

	// Act
	view := typing(t, newWorld().live(t, 160, 70), "?").View().Content

	// Assert
	refuseScreen(t, view, "more below")
	requireScreen(t, footerLine(view), "esc close")
	refuseScreen(t, footerLine(view), "scroll")
}

func TestANarrowHelpKeepsTheKeyThatClosesIt(t *testing.T) {
	t.Parallel()

	// Act
	// At 50 columns the help is one column, taller than the pane, and the
	// footer has room for only some of its keys.
	view := typing(t, newWorld().live(t, 50, 30), "?").View().Content

	// Assert
	requireScreen(t, view, "more below")
	requireScreen(t, footerLine(view), "esc close")
}

func TestCheckKeysKnowsEveryPlacedAction(t *testing.T) {
	t.Parallel()

	for _, group := range placedBindings() {
		for _, binding := range movable(group.bindings) {
			t.Run(binding.action, func(t *testing.T) {
				t.Parallel()

				// Arrange
				// The free key is bound to nothing, so a real action moved onto it
				// collides with nothing; an action that does not exist is refused.
				rebound := map[string]string{binding.action: freeKey}

				// Act
				err := tui.CheckKeys(rebound)
				// Assert
				if err != nil {
					t.Errorf("CheckKeys(%v) = %v, want the action known and free to move", rebound, err)
				}
			})
		}
	}
}

func TestTheHelpListsEachPlacedActionWhereItWorks(t *testing.T) {
	t.Parallel()

	for _, group := range placedBindings() {
		for _, binding := range movable(group.listedActions()) {
			t.Run(binding.action, func(t *testing.T) {
				t.Parallel()

				// Arrange
				// Moved to a key nothing else shows, the action's own line is the
				// one line that names it.
				rebound := map[string]string{binding.action: freeKey}
				want := listedBinding{key: freeKey, help: binding.help}

				// Act
				listing := helpListing(t, openHelp(t, rebound))

				// Assert
				if !slices.Contains(groupNamed(listing, group.name).bindings, want) {
					t.Errorf("? does not list %s under %s:\n%v", want, group.name, listing)
				}
			})
		}
	}
}

// listedActions is the group's bindings that have help of their own, with their
// actions.
func (g helpGroup) listedActions() []listedBinding {
	return slices.DeleteFunc(slices.Clone(g.bindings), func(binding listedBinding) bool { return binding.help == "" })
}

// movable is bindings without the one ui.keys cannot move.
func movable(bindings []listedBinding) []listedBinding {
	return slices.DeleteFunc(slices.Clone(bindings), func(binding listedBinding) bool {
		return binding.action == fixedAction
	})
}

// groupNamed is the listing's group called name, or an empty one.
func groupNamed(listing []helpGroup, name string) helpGroup {
	for _, group := range listing {
		if group.name == name {
			return group
		}
	}

	return helpGroup{name: name}
}
