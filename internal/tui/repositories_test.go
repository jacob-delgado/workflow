// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

import (
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
