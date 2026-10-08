// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package report_test

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/jacob-delgado/workflow/internal/api"
	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/gitrepo"
	"github.com/jacob-delgado/workflow/internal/report"
	"github.com/jacob-delgado/workflow/internal/seams"
)

// errUnreadable is a read that could not be made.
var errUnreadable = errors.New("unreadable")

// Where the views here work: a repository under home, worked in one level
// down.
const (
	home    = "/home/dev"
	repo    = "/home/dev/src/workflow"
	workdir = "/home/dev/src/workflow/internal"
)

// workingIn is a view of the repository under home, its origin on GitHub and
// its configuration in both files, its favorites read by favorites.
func workingIn(favorites func() ([]string, error)) report.RepositoriesView {
	here := seams.Place{
		Dir: workdir, Root: repo, Remote: forge.Repo{Host: "github.com", Path: "owner/workflow", Kind: forge.KindGitHub},
		Config: config.Files{Home: filepath.Join(home, config.FileName), Repo: filepath.Join(repo, config.FileName)},
	}

	return report.RepositoriesView{
		Repositories: seams.Repositories{Here: here, Home: home, Look: nil, Subdirectories: nil, Worktrees: nil},
		Favorites:    favorites, FavoritesKept: true,
	}
}

func TestTheRepositoriesReportSaysWhereItWorksWrittenFromHome(t *testing.T) {
	t.Parallel()

	// Act
	answer, err := workingIn(nil).Report(worded)
	// Assert
	if err != nil {
		t.Fatalf("Report = %v, want where it works", err)
	}

	here := answer.Here
	if here.Dir != workdir || here.Shown != "~/src/workflow/internal" || here.Root != repo ||
		here.RootShown != "~/src/workflow" || here.Within != "internal" || here.Origin != "github.com/owner/workflow" {
		t.Errorf("Here = %+v, want the directory, its root and origin, each written from home", here)
	}

	want := []string{"~/src/workflow/" + config.FileName, "~/" + config.FileName}
	if !reflect.DeepEqual(here.Config, want) {
		t.Errorf("Here.Config = %v, want the repository's file before home's: %v", here.Config, want)
	}
}

func TestADirectoryOutsideARepositoryHasNoRootOrOrigin(t *testing.T) {
	t.Parallel()

	// Arrange
	view := report.RepositoriesView{Repositories: seams.Repositories{Here: seams.Place{Dir: home}, Home: home}}

	// Act
	answer, _ := view.Report(worded)

	// Assert
	if here := answer.Here; here.Shown != "~" || here.Root != "" || here.Within != "" || here.Origin != "" {
		t.Errorf("Here = %+v, want home itself with no repository", here)
	}
}

func TestTheRepositoriesReportSaysWhichWorktreeIsWhich(t *testing.T) {
	t.Parallel()

	// Arrange
	view := workingIn(nil)
	view.Repositories.Worktrees = func() ([]gitrepo.Worktree, error) {
		return []gitrepo.Worktree{
			{Dir: repo, Branch: "main", Head: "0123456789abcdef"},
			{Dir: repo + "-feat", Branch: "feat/x", Head: "fedcba9876543210", Locked: true},
			{Dir: repo + "-gone", Branch: "fix/y", Head: "abc", Missing: true},
		}, nil
	}

	// Act
	answer, _ := view.Report(worded)

	// Assert
	states := make([]api.WorktreeState, 0, len(answer.Worktrees))
	for _, worktree := range answer.Worktrees {
		states = append(states, worktree.State)
	}

	want := []api.WorktreeState{api.WorktreeStateHere, api.WorktreeStateWorktree, api.WorktreeStateMissing}
	if !reflect.DeepEqual(states, want) {
		t.Fatalf("the worktrees' states = %v, want %v", states, want)
	}

	if feat := answer.Worktrees[1]; feat.Head != "fedcba9" || !feat.Locked || feat.Shown != "~/src/workflow-feat" {
		t.Errorf("feat/x's worktree = %+v, want its short head, locked, written from home", feat)
	}
}

func TestWorktreesThatCannotBeReadAreNoneAndSayWhy(t *testing.T) {
	t.Parallel()

	// Arrange
	view := workingIn(nil)
	view.Repositories.Worktrees = func() ([]gitrepo.Worktree, error) { return nil, errUnreadable }

	// Act
	answer, _ := view.Report(worded)

	// Assert
	if len(answer.Worktrees) != 0 || answer.WorktreesError != "worded: unreadable" {
		t.Errorf("worktrees = %v, %q; want none, and why in describe's words", answer.Worktrees, answer.WorktreesError)
	}
}

func TestEachFavoriteSaysWhatIsThereNow(t *testing.T) {
	t.Parallel()

	// Arrange
	const (
		plain = "/home/dev/notes"
		other = "/home/dev/src/other"
		gone  = "/home/dev/src/gone"
	)

	view := workingIn(func() ([]string, error) { return []string{workdir, plain, other, gone}, nil })
	view.Repositories.Look = func(dir string) (seams.Place, error) {
		switch dir {
		case plain:
			return seams.Place{Dir: plain}, nil
		case other:
			return seams.Place{Dir: other, Root: other, Remote: forge.Repo{Host: "gitlab.com", Path: "group/other"}}, nil
		default:
			return seams.Place{}, errUnreadable
		}
	}

	// Act
	answer, _ := view.Report(worded)

	// Assert
	want := []api.Favorite{
		{Dir: workdir, Shown: "~/src/workflow/internal", State: api.FavoriteStateHere},
		{Dir: plain, Shown: "~/notes", State: api.FavoriteStateDirectory},
		{Dir: other, Shown: "~/src/other", State: api.FavoriteStateRepository, Origin: "gitlab.com/group/other"},
		{Dir: gone, Shown: "~/src/gone", State: api.FavoriteStateMissing},
	}
	if !reflect.DeepEqual(answer.Favorites, want) {
		t.Errorf("Favorites = %+v, want %+v", answer.Favorites, want)
	}
}

func TestAFavoriteWithNoWayToLookAtItIsMissing(t *testing.T) {
	t.Parallel()

	// Arrange
	view := workingIn(func() ([]string, error) { return []string{"/home/dev/notes"}, nil })

	// Act
	answer, _ := view.Report(worded)

	// Assert
	if len(answer.Favorites) != 1 || answer.Favorites[0].State != api.FavoriteStateMissing {
		t.Errorf("Favorites = %+v, want the favorite missing", answer.Favorites)
	}
}

func TestFavoritesThatCannotBeReadAreNoneAndTheErrorIsReturned(t *testing.T) {
	t.Parallel()

	// Arrange
	view := workingIn(func() ([]string, error) { return nil, errUnreadable })

	// Act
	answer, err := view.Report(worded)

	// Assert
	if !errors.Is(err, errUnreadable) || len(answer.Favorites) != 0 {
		t.Errorf("Report = %v, %v; want no favorites and why", answer.Favorites, err)
	}
}

func TestAStoreThatKeepsNoFavoritesListsNone(t *testing.T) {
	t.Parallel()

	// Arrange
	view := workingIn(nil)
	view.FavoritesKept = false

	// Act
	answer, err := view.Report(worded)

	// Assert
	if err != nil || answer.Favorites == nil || len(answer.Favorites) != 0 || answer.FavoritesKept {
		t.Errorf("Report = %+v, %v; want an empty list, none kept", answer.Favorites, err)
	}
}

func TestAFavoriteThatLinksToWhereItWorksIsHere(t *testing.T) {
	t.Parallel()

	// Arrange
	dir := t.TempDir()
	link := filepath.Join(t.TempDir(), "link")

	err := os.Symlink(dir, link)
	if err != nil {
		t.Fatalf("linking to the directory: %v", err)
	}

	view := report.RepositoriesView{
		Repositories: seams.Repositories{Here: seams.Place{Dir: dir}},
		Favorites:    func() ([]string, error) { return []string{link}, nil },
	}

	// Act
	answer, _ := view.Report(worded)

	// Assert
	if len(answer.Favorites) != 1 || answer.Favorites[0].State != api.FavoriteStateHere {
		t.Errorf("Favorites = %+v, want the link to where it works said to be here", answer.Favorites)
	}
}

func TestWorkingAtTheRootOfARepositoryIsWithinNothing(t *testing.T) {
	t.Parallel()

	// Arrange
	view := report.RepositoriesView{Repositories: seams.Repositories{Here: seams.Place{Dir: repo, Root: repo}}}

	// Act
	answer, _ := view.Report(worded)

	// Assert
	if answer.Here.Root != repo || answer.Here.Within != "" {
		t.Errorf("Here = %+v, want the root, within nothing", answer.Here)
	}
}

func TestADirectoryThatCannotBeWrittenFromItsRootIsWithinNothing(t *testing.T) {
	t.Parallel()

	// Arrange
	// A relative directory has no path from an absolute root.
	view := report.RepositoriesView{Repositories: seams.Repositories{Here: seams.Place{Dir: "internal", Root: repo}}}

	// Act
	answer, _ := view.Report(worded)

	// Assert
	if answer.Here.Within != "" {
		t.Errorf("Here.Within = %q, want nothing where no path leads from the root", answer.Here.Within)
	}
}
