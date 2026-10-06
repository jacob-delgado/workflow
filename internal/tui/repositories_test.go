// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/jacob-delgado/workflow/internal/seams"
)

// reposKey jumps to the Repositories pane, whose list is in its detail, so
// its focus shows on the detail's title, reposTitle.
const (
	reposKey   = "9"
	reposTitle = "Repositories"
)

func TestTheRepositoriesPaneSaysWhereYouWork(t *testing.T) {
	t.Parallel()

	// Act
	view := typing(t, reposWorld().live(t, 120, 40), reposKey).View().Content

	// Assert
	// The directory, the repository it is in and the path within it, origin,
	// and the configuration files that apply, each written from your home.
	requireScreen(t, view, "9 Repositories", "Working in", "~/src/api/cmd", "~/src/api", "cmd",
		"github.com/acme/api", "~/src/api/.workflow.json over ~/.workflow.json")
}

func TestADirectoryOutsideARepositorySaysSo(t *testing.T) {
	t.Parallel()

	// Arrange
	working := reposWorld()
	working.dirs.here = seams.Place{Dir: oldDir}

	// Act
	view := typing(t, working.live(t, 120, 40), reposKey).View().Content

	// Assert
	requireScreen(t, view, "~/old", "not in a repository", "the defaults")
}

func TestFavoritesAreListedWithWhatIsThereNow(t *testing.T) {
	t.Parallel()

	// Act
	view := typing(t, reposWorld().live(t, 120, 40), reposKey).View().Content

	// Assert
	requireScreen(t, view, "Favorites", "~/src/web", "github.com/acme/web", "~/old", "not there")
}

func TestWithNoFavoritesTheHeadingSaysHowToMakeOne(t *testing.T) {
	t.Parallel()

	// Arrange
	working := reposWorld()
	working.dirs.favorites = nil

	// Act
	view := typing(t, working.live(t, 120, 40), reposKey).View().Content

	// Assert
	requireScreen(t, view, "Favorites", "No favorites yet; f marks the directory under the cursor.")
}

func TestFavoritesAreNotReadUntilThePaneIsLookedAt(t *testing.T) {
	t.Parallel()

	// Arrange
	working := reposWorld()

	// Act
	typing(t, working.live(t, 120, 40), "2")

	// Assert
	if read := working.asked("favorites"); len(read) != 0 {
		t.Errorf("favorites read %d times before the pane was opened, want none", len(read))
	}
}

func TestFMarksWhereYouAreAFavorite(t *testing.T) {
	t.Parallel()

	// Arrange
	working := reposWorld()
	opened := typing(t, working.live(t, 120, 40), reposKey)

	// Act
	view := typing(t, opened, "f").View().Content

	// Assert
	if marked := working.asked("favor "); len(marked) != 1 || marked[0] != "favor "+apiCmd {
		t.Errorf("marked %q, want %s", marked, apiCmd)
	}

	requireScreen(t, view, "added ~/src/api/cmd to favorites")
}

func TestFOnAFavoriteForgetsIt(t *testing.T) {
	t.Parallel()

	// Arrange
	working := reposWorld()
	opened := typing(t, working.live(t, 120, 40), reposKey, "j")

	// Act
	view := typing(t, opened, "f").View().Content

	// Assert
	if forgot := working.asked("unfavor "); len(forgot) != 1 || forgot[0] != "unfavor "+webRoot {
		t.Errorf("forgot %q, want %s", forgot, webRoot)
	}

	requireScreen(t, view, "removed ~/src/web from favorites")
}

func TestADryRunMarksNoFavorite(t *testing.T) {
	t.Parallel()

	// Arrange
	working := reposWorld()
	opened := typing(t, working.live(t, 120, 40).WithDryRun(), reposKey)

	// Act
	view := typing(t, opened, "f").View().Content

	// Assert
	requireScreen(t, view, "dry run: would add ~/src/api/cmd to favorites")

	if marked := working.asked("favor "); len(marked) != 0 {
		t.Errorf("marked %q in a dry run, want nothing", marked)
	}
}

func TestAStoreTurnedOffSaysFavoritesAreNotKept(t *testing.T) {
	t.Parallel()

	// Arrange
	working := reposWorld()
	working.dirs.keepsNothing = true
	opened := typing(t, working.live(t, 120, 40), reposKey)

	// Act
	view := typing(t, opened, "f").View().Content

	// Assert
	requireScreen(t, view, "favorites are not kept")
}

func TestADirectoryNameCannotDriveTheTerminal(t *testing.T) {
	t.Parallel()

	// Arrange
	// A favorite is read back from a file on disk, and a directory may be
	// named anything.
	working := reposWorld()
	working.dirs.favorites = []string{"/home/ana/\x1b]0;owned\x07evil"}

	// Act
	view := typing(t, working.live(t, 120, 40), reposKey).View().Content

	// Assert
	refuseScreen(t, view, "\x1b]0;owned")
}

func TestTheTopRowNamesWhereYouWork(t *testing.T) {
	t.Parallel()

	// Act
	view := reposWorld().live(t, 120, 40).View().Content

	// Assert
	// The repository and the path within it, on the row that says how far
	// along the loop the work is, whatever pane has focus.
	requireScreen(t, spineLine(view), "api/cmd", "Issue", "Branch")
}

func TestTheTopRowGivesUpThePlaceBeforeAnyStage(t *testing.T) {
	t.Parallel()

	// Act
	view := reposWorld().live(t, 56, 40).View().Content

	// Assert
	spine := spineLine(view)
	requireScreen(t, spine, "Issue", "Slack")
	refuseScreen(t, spine, "api/cmd")
}

func TestOutsideARepositoryTheTopRowNamesTheDirectory(t *testing.T) {
	t.Parallel()

	// Arrange
	working := reposWorld()
	working.dirs.here = seams.Place{Dir: oldDir}

	// Act
	view := working.live(t, 120, 40).View().Content

	// Assert
	requireScreen(t, spineLine(view), "old ")
}

func TestFWhereYouWorkForgetsTheFavoriteAsItWasKept(t *testing.T) {
	t.Parallel()

	// Arrange
	// A favorite kept through a link is where you work under another name;
	// forgetting it forgets the name it was kept by.
	target := t.TempDir()
	link := filepath.Join(t.TempDir(), "linked")

	err := os.Symlink(target, link)
	if err != nil {
		t.Fatal(err)
	}

	working := reposWorld()
	working.dirs.here = seams.Place{Dir: target}
	working.dirs.places[link] = seams.Place{Dir: target}
	working.dirs.favorites = []string{link}
	opened := typing(t, working.live(t, 120, 40), reposKey)

	// Act
	typing(t, opened, "f")

	// Assert
	if forgot := working.asked("unfavor "); len(forgot) != 1 || forgot[0] != "unfavor "+link {
		t.Errorf("forgot %q, want the favorite as it was kept, %s", forgot, link)
	}
}

func TestEachFavoriteSaysWhatKindOfDirectoryItIs(t *testing.T) {
	t.Parallel()

	// Arrange
	working := reposWorld()
	working.dirs.places["/home/ana/notes"] = seams.Place{Dir: "/home/ana/notes"}
	scratch := anaHome + "/scratch"
	working.dirs.places[scratch] = seams.Place{Dir: scratch, Root: scratch}
	working.dirs.favorites = []string{"/home/ana/notes", scratch}

	// Act
	view := typing(t, working.live(t, 120, 40), reposKey).View().Content

	// Assert
	requireScreen(t, view, "~/notes · not a repository", "~/scratch · repository")
}

func TestWorkingAtTheRootSaysSo(t *testing.T) {
	t.Parallel()

	// Arrange
	working := reposWorld()
	working.dirs.here = webPlace()

	// Act
	view := typing(t, working.live(t, 120, 40), reposKey).View().Content

	// Assert
	requireScreen(t, view, "the repository's root")
}

func TestKMovesBackUpAndRReadsTheFavoritesAgain(t *testing.T) {
	t.Parallel()

	// Arrange
	working := reposWorld()
	opened := typing(t, working.live(t, 120, 40), reposKey, "j", "k")

	// Act
	typing(t, opened, "r", "f")

	// Assert
	if reads := working.asked("favorites"); len(reads) < 2 {
		t.Errorf("favorites read %d times, want again on r", len(reads))
	}

	if marked := working.asked("favor "); len(marked) != 1 || marked[0] != "favor "+apiCmd {
		t.Errorf("marked %q, want where you work, back under the cursor", marked)
	}
}

func TestFavoritesThatCannotBeReadOrChangedSayWhy(t *testing.T) {
	t.Parallel()

	// Arrange
	working := reposWorld()
	working.dirs.failing = errJiraDown
	opened := typing(t, working.live(t, 120, 40), reposKey)

	// Act
	view := typing(t, opened, "f").View().Content

	// Assert
	requireScreen(t, view, "jira is down")

	if marked := working.asked("favor "); len(marked) != 1 {
		t.Errorf("marked %q, want the one refused", marked)
	}
}
