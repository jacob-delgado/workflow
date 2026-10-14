// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

import (
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/jacob-delgado/workflow/internal/tui"
)

// dirPickerTitle is the directory picker's title.
const dirPickerTitle = "Go to a directory"

// The names the picker's world lists under srcDir and where you work.
const (
	apiDir      = "api"
	webDir      = "web"
	notesDir    = "workday-notes"
	internalDir = "internal"
)

// pickingWorld is reposWorld with directories to browse: api, web, a notes
// directory and cli under srcDir, api and web repositories, and one directory
// where you work.
func pickingWorld() *world {
	working := reposWorld()
	working.dirs.listings = map[string][]string{
		srcDir:  {apiDir, webDir, notesDir, "cli"},
		anaHome: {"src", "old"},
		apiCmd:  {internalDir},
		webRoot: {"src"},
	}
	working.dirs.repositories = []string{apiDir, webDir}

	return working
}

// picking is the Repositories pane with the picker open and path typed in it.
func picking(t *testing.T, working *world, path string, more ...string) tui.Model {
	t.Helper()

	keys := slices.Concat([]string{reposKey, "g"}, letters(path), more)

	return typing(t, working.live(t, 120, 40), keys...)
}

// pickerLine is the picker's row that shows name, plain.
func pickerLine(view, name string) string {
	shown := regexp.MustCompile(`(^|[ ▸])` + regexp.QuoteMeta(name) + `( |$)`)

	for line := range strings.SplitSeq(plain(view), "\n") {
		if shown.MatchString(line) {
			return line
		}
	}

	return ""
}

func TestTypingInThePickerNarrowsItAndMarksRepositories(t *testing.T) {
	t.Parallel()

	// Act
	view := picking(t, pickingWorld(), "~/src/w").View().Content

	// Assert
	requireScreen(t, view, dirPickerTitle, "› ~/src/w", "in ~/src")

	if !strings.Contains(pickerLine(view, webDir), "repository") {
		t.Errorf("web is not marked a repository:\n%s", plain(view))
	}

	if pickerLine(view, notesDir) == "" || pickerLine(view, "cli") != "" {
		t.Errorf("typing w should list web and %s, not cli:\n%s", notesDir, plain(view))
	}
}

func TestRightDescendsIntoTheHighlightedDirectory(t *testing.T) {
	t.Parallel()

	// Act
	view := picking(t, pickingWorld(), "~/src/w", "right").View().Content

	// Assert
	requireScreen(t, view, "› ~/src/web/", "in ~/src/web")
}

func TestBackspaceOnAnEmptyNameGoesUp(t *testing.T) {
	t.Parallel()

	// Act
	view := picking(t, pickingWorld(), "~/src/", keyBackspace).View().Content

	// Assert
	requireScreen(t, view, "› ~/", "in ~")
}

func TestJTypesInTheFieldButMovesInTheListAfterTab(t *testing.T) {
	t.Parallel()

	// Arrange
	browsing := picking(t, pickingWorld(), "~/src/")

	// Act: j in the field
	typed := typing(t, browsing, "j")

	// Assert: it is typed
	requireScreen(t, typed.View().Content, "› ~/src/j")

	// Act: tab, then j in the list
	moved := typing(t, browsing, keyTab, "j")

	// Assert: it moves the highlight to the first directory
	requireScreen(t, moved.View().Content, "› ~/src/ ", "▸ api")
}

func TestLeftInTheListGoesUp(t *testing.T) {
	t.Parallel()

	// Act
	view := picking(t, pickingWorld(), "~/src/", keyTab, "left").View().Content

	// Assert
	requireScreen(t, view, "› ~/", "in ~")
}

func TestSwitchHereOpensALastLookAndEscGoesBackToThePicker(t *testing.T) {
	t.Parallel()

	// Arrange
	browsing := picking(t, pickingWorld(), "~/src/web/")

	// Act: enter on switch here
	looking := typing(t, browsing, keyEnter)

	// Assert: the last look asks, over the picker it goes back to
	requireScreen(t, looking.View().Content, "Switch directory", "Switch to ~/src/web?", dirPickerTitle)

	// Act: esc
	back := typing(t, looking, keyEsc)

	// Assert: the picker is where it was
	requireScreen(t, back.View().Content, dirPickerTitle, "› ~/src/web/")
	refuseScreen(t, back.View().Content, "Switch to ~/src/web?")
}

func TestThePickerOpensOnWorktreesAndFavoritesBeforeWhatIsHere(t *testing.T) {
	t.Parallel()

	// Act
	view := plain(picking(t, pickingWorld(), "").View().Content)

	// Assert
	favorite, here := strings.Index(view, "★ ~/src/web"), strings.Index(view, internalDir)
	if favorite < 0 || here < 0 || favorite > here {
		t.Errorf("want the favorite ~/src/web listed before %s, where you work:\n%s", internalDir, view)
	}
}

func TestATruncatedListingIsAskedAgainWithWhatWasTyped(t *testing.T) {
	t.Parallel()

	// Arrange
	working := pickingWorld()
	working.dirs.truncated = map[string]bool{srcDir: true}

	// Act
	view := picking(t, working, "~/src/wo").View().Content

	// Assert
	if asked := working.asked("subdirectories " + srcDir + " wo"); len(asked) == 0 {
		t.Errorf("never asked for %s narrowed to wo: %q", srcDir, working.asked("subdirectories"))
	}

	if pickerLine(view, notesDir) == "" {
		t.Errorf("%s, past the first two, is not listed:\n%s", notesDir, plain(view))
	}
}

func TestAPathThatIsNotADirectoryIsRefusedInPlace(t *testing.T) {
	t.Parallel()

	// Act
	view := picking(t, pickingWorld(), "~/nowhere/").View().Content

	// Assert
	requireScreen(t, view, dirPickerTitle, "✗", "there is no such directory")
}

func TestThePickerFitsAnEightyByTwentyFourTerminal(t *testing.T) {
	t.Parallel()

	// Act
	view := typing(t, pickingWorld().live(t, 80, 24), reposKey, "g").View().Content

	// Assert
	if row, column, found := cornerOf(view, dirPickerTitle); !found || row != 2 || column != 8 {
		t.Errorf("the picker starts at row %d, column %d, want a 64 by 18 dialog at 2, 8:\n%s", row, column, plain(view))
	}
}

func TestBelowSixtyColumnsThePickerFillsTheBody(t *testing.T) {
	t.Parallel()

	// Act
	view := typing(t, pickingWorld().live(t, 50, 24), reposKey, "g").View().Content

	// Assert
	lines := strings.Split(plain(view), "\n")
	if !strings.HasPrefix(lines[1], dirPickerTitle) {
		t.Errorf("the body's first row = %q, want the picker's title as a sheet", lines[1])
	}
}

func TestAWriteOnItsWayRefusesASwitchFromThePicker(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := withTasks()
	repo.dirs = pickingWorld().dirs
	writing := writingATask(t, repo)

	// Act
	view := typing(t, writing, slices.Concat([]string{reposKey, "g"}, letters("~/src/web/"), []string{keyEnter})...).
		View().Content

	// Assert
	requireScreen(t, view, "wait for the change to a task to finish before switching", dirPickerTitle)
	refuseScreen(t, view, "Switch directory")
}

func TestThePickerDrawsTwoHundredRowsAndCountsTheRest(t *testing.T) {
	t.Parallel()

	// Arrange
	working := pickingWorld()

	many := make([]string, 0, 205)
	for index := range 205 {
		many = append(many, "dir"+strconv.Itoa(1000+index))
	}

	working.dirs.listings[srcDir] = many

	// Act
	view := picking(t, working, "~/src/dir").View().Content

	// Assert
	requireScreen(t, view, "5 more match; keep typing to narrow")
}

func TestThePickerSaysWhatItCannotList(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		typed string
		want  string
	}{
		"an empty directory": {typed: "~/old/", want: "no directories in ~/old"},
		"nothing matching":   {typed: "~/src/zz", want: "nothing in ~/src matches zz"},
		"a listing cut short": {
			typed: "~/src/", want: "first 1000 shown; keep typing to narrow",
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			working := pickingWorld()
			working.dirs.listings[oldDir] = []string{}
			working.dirs.truncated = map[string]bool{srcDir: true}

			// Act
			view := picking(t, working, tt.typed).View().Content

			// Assert
			requireScreen(t, view, tt.want)
		})
	}
}
