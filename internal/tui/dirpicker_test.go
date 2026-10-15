// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

import (
	"encoding/json"
	"os"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/jacob-delgado/workflow/internal/tui"
	"github.com/jacob-delgado/workflow/internal/workdirs"
)

// dirPickerTitle is the directory picker's title.
const dirPickerTitle = "Go to a directory"

// The names the picker's world lists under srcDir and where you work;
// srcName, which it lists in your home and in web; oldName, beside it in
// your home; and configName, a hidden directory a world may put there.
const (
	apiDir      = "api"
	webDir      = "web"
	notesDir    = "workday-notes"
	internalDir = "internal"
	srcName     = "src"
	oldName     = "old"
	configName  = ".config"
)

// srcTyped and webTyped are srcDir and webRoot as typed in the picker, from
// your home.
const (
	srcTyped = "~/src/"
	webTyped = "~/src/web/"
)

// pickingWorld is reposWorld with directories to browse: api, web, a notes
// directory and cli under srcDir, api and web repositories, and one directory
// where you work.
func pickingWorld() *world {
	working := reposWorld()
	working.dirs.listings = map[string][]string{
		srcDir:  {apiDir, webDir, notesDir, "cli"},
		anaHome: {srcName, oldName},
		apiCmd:  {internalDir},
		webRoot: {srcName},
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

// pickerText is the picker's dialog alone, plain, cut from the screen so
// the panes behind it are left out.
func pickerText(t *testing.T, view string) string {
	t.Helper()

	top, left, found := cornerOf(view, dirPickerTitle)
	if !found {
		t.Fatalf("no picker on the screen:\n%s", plain(view))
	}

	lines := strings.Split(plain(view), "\n")[top:]
	width := slices.Index([]rune(lines[0])[left:], '┓') + 1
	inside := make([]string, 0, len(lines))

	for _, line := range lines {
		row := []rune(line)[left:]
		inside = append(inside, string(row[:min(len(row), width)]))

		if row[0] == '┗' {
			break
		}
	}

	return strings.Join(inside, "\n")
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

func TestAListingAnsweredUnderTheLastLookIsThereOnTheWayBack(t *testing.T) {
	t.Parallel()

	// Arrange
	// ~/src/web is listed, and while that read is out it is looked at and
	// the last look opens over the picker before the listing answers.
	typed, listing := pressed(t, picking(t, pickingWorld(), "~/src/web"), "/")
	asked, look := pressed(t, typed, keyEnter)
	looking, _ := finish(t, asked, look)
	answered, _ := finish(t, looking, listing)

	// Act
	view := typing(t, answered, keyEsc).View().Content

	// Assert
	refuseScreen(t, view, "reading ~/src/web")

	if pickerLine(view, srcName) == "" {
		t.Errorf("the listing of ~/src/web, answered under the last look, is not drawn:\n%s", plain(view))
	}
}

func TestAListingAnsweredAfterThePickerClosedIsAskedForAgain(t *testing.T) {
	t.Parallel()

	// An answer for a picker closed since is dropped, whether nothing is
	// open when it arrives or a picker opened since is, so the next picker
	// to need that listing asks for it again.
	cases := map[string]struct {
		before, after []string
	}{
		"with nothing open":           {before: []string{keyEsc}, after: append([]string{"g"}, letters(webTyped)...)},
		"under a picker opened since": {before: []string{keyEsc, "g"}, after: letters(webTyped)},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			working := pickingWorld()
			typed, listing := pressed(t, picking(t, working, "~/src/web"), "/")
			closed := typing(t, typed, tt.before...)
			answered, _ := finish(t, closed, listing)

			// Act
			typing(t, answered, tt.after...)

			// Assert
			if asked := working.asked("subdirectories " + webRoot + " "); len(asked) != 2 {
				t.Errorf("%s was listed %d times, want twice: once for each picker", webRoot, len(asked))
			}
		})
	}
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

func TestThePickerLeavesOutWorktreesAndFavoritesThatAreGone(t *testing.T) {
	t.Parallel()

	// Arrange
	// The favorite ~/old is not there, and git lists ~/src/api-gone with its
	// directory gone.
	working := pickingWorld()
	working.dirs.worktrees = worktreesWorld().dirs.worktrees

	// Act
	shown := pickerText(t, picking(t, working, "").View().Content)

	// Assert
	if !strings.Contains(shown, "~/src/api-feat-x") || !strings.Contains(shown, "★ ~/src/web") ||
		strings.Contains(shown, "~/old") || strings.Contains(shown, "~/src/api-gone") {
		t.Errorf("want the worktree and favorite there offered, not those gone:\n%s", shown)
	}
}

func TestATruncatedListingIsAskedAgainWithWhatWasTyped(t *testing.T) {
	t.Parallel()

	// Arrange
	working := truncatedWorld()

	// Act
	view := picking(t, working, "~/src/wo").View().Content

	// Assert
	if asked := working.asked("subdirectories " + srcDir + " wo"); len(asked) == 0 {
		t.Errorf("never asked for %s narrowed to wo: %q", srcDir, working.asked("subdirectories"))
	}

	if pickerLine(view, notesDir) == "" {
		t.Errorf("%s, past the first thousand, is not listed:\n%s", notesDir, plain(view))
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
	view := typing(t, writing, slices.Concat([]string{reposKey, "g"}, letters(webTyped), []string{keyEnter})...).
		View().Content

	// Assert
	requireScreen(t, view, "wait for the change to a task to finish before switching", dirPickerTitle)
	refuseScreen(t, view, "Switch directory")
}

// manyDirs is count directories, named dir1000 on.
func manyDirs(count int) []string {
	many := make([]string, 0, count)
	for index := range count {
		many = append(many, "dir"+strconv.Itoa(1000+index))
	}

	return many
}

// rowsFile holds the most rows the picker draws, which the web's directory
// dialog answers to as well.
const rowsFile = "../../testdata/twins/directory_rows.json"

// mostRows is the most rows the picker draws, as rowsFile holds it, refusing
// a field this side would ignore so the file cannot pin one surface's cap and
// pass the other's unread, and a count below one, which no test could draw
// past.
func mostRows(t *testing.T) int {
	t.Helper()

	file, err := os.Open(rowsFile)
	if err != nil {
		t.Fatal(err)
	}

	defer func() { _ = file.Close() }()

	decoder := json.NewDecoder(file)
	decoder.DisallowUnknownFields()

	var read struct {
		About string `json:"about"`
		Most  int    `json:"most"`
	}

	err = decoder.Decode(&read)
	if err != nil {
		t.Fatalf("%s: %v", rowsFile, err)
	}

	if read.Most < 1 {
		t.Fatalf("%s: most is %d, want a count of rows of 1 or more", rowsFile, read.Most)
	}

	return read.Most
}

// manyDirsWorld is pickingWorld with count directories in srcDir.
func manyDirsWorld(count int) *world {
	working := pickingWorld()
	working.dirs.listings[srcDir] = manyDirs(count)

	return working
}

// truncatedWorld is pickingWorld with srcDir holding more than a listing
// keeps: workdirs.ListLimit directories and two hundred more, then its own.
func truncatedWorld() *world {
	working := pickingWorld()
	working.dirs.listings[srcDir] = append(manyDirs(workdirs.ListLimit+200), working.dirs.listings[srcDir]...)

	return working
}

func TestThePickerDrawsTheMostRowsTheSharedFileHoldsAndCountsTheRest(t *testing.T) {
	t.Parallel()

	// Arrange
	most := mostRows(t)
	presses := slices.Repeat([]string{keyDown}, most-1)

	// Act
	view := picking(t, manyDirsWorld(most+5), "~/src/dir", presses...).View().Content

	// Assert
	// Down to the last row drawn highlights the last directory under the
	// most, and the five past it are counted.
	requireScreen(t, view, "▸ "+manyDirs(most)[most-1], "5 more match; keep typing to narrow")
}

func TestTheHighlightStopsAtTheLastRowThePickerDraws(t *testing.T) {
	t.Parallel()

	// Arrange
	most := mostRows(t)
	presses := slices.Repeat([]string{keyDown}, most+10)

	// Act
	view := picking(t, manyDirsWorld(most+5), "~/src/dir", presses...).View().Content

	// Assert
	// Past the last match drawn, down reaches switching here, after them,
	// and goes no further into directories that are counted but not drawn.
	lines := strings.Split(pickerText(t, view), "\n")
	here := slices.IndexFunc(lines, func(line string) bool { return strings.Contains(line, "▸ ↳ switch here") })

	if last := manyDirs(most)[most-1]; here < 1 || !strings.Contains(lines[here-1], last) {
		t.Errorf("the highlight is not on switching here, just after %s, the last directory drawn:\n%s",
			last, plain(view))
	}
}

func TestThePickerSaysWhatItCannotList(t *testing.T) {
	t.Parallel()

	// ~/src holds more than a listing keeps, and more than the picker draws
	// match all but a name it has no directory for.
	counted := strconv.Itoa(workdirs.ListLimit-mostRows(t)) + " more match; keep typing to narrow"
	cases := map[string]struct {
		typed string
		keys  []string
		want  []string
	}{
		"an empty directory": {typed: "~/old/", want: []string{"no directories in ~/old"}},
		"nothing matching":   {typed: "~/src/zz", want: []string{"nothing in ~/src matches zz"}},
		"a listing cut short": {
			typed: srcTyped,
			want:  []string{counted, "first 1000 shown; keep typing to narrow"},
		},
		"a refusal below the rows drawn": {
			typed: srcTyped, keys: []string{keyTab, "j", "s"},
			want: []string{counted, "there is no such directory"},
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			working := truncatedWorld()
			working.dirs.listings[oldDir] = []string{}

			// Act
			view := picking(t, working, tt.typed, tt.keys...).View().Content

			// Assert
			requireScreen(t, view, tt.want...)
		})
	}
}
