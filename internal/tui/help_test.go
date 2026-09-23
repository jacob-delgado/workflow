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
			"post              p          post to slack"),
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
			"post-when-green   w          post when CI passes"),
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

func TestCheckKeysKnowsEveryPlacedAction(t *testing.T) {
	t.Parallel()

	for _, group := range placedBindings() {
		for _, binding := range group.bindings {
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
		for _, binding := range group.listedActions() {
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

// groupNamed is the listing's group called name, or an empty one.
func groupNamed(listing []helpGroup, name string) helpGroup {
	for _, group := range listing {
		if group.name == name {
			return group
		}
	}

	return helpGroup{name: name}
}
