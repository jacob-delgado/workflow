// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package wiring_test

// The Repositories pane and the web's picker read where the session works,
// look at another directory before switching to it, browse the directories
// in one, and keep the favorites you mark.

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/wiring"
	"github.com/jacob-delgado/workflow/internal/workdirs"
)

// originRemote is the origin every repository a case makes names.
const originRemote = "git@github.com:acme/api.git"

// homeOfItsOwn gives the test a home with no configuration and an empty store.
func homeOfItsOwn(t *testing.T) string {
	t.Helper()

	home, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	t.Setenv("HOME", home)
	t.Setenv("XDG_STATE_HOME", "")

	return home
}

// repositoryWithOrigin is a new repository whose origin is originRemote, and
// a directory within it.
func repositoryWithOrigin(t *testing.T) (string, string) {
	t.Helper()

	root := repository(t)
	git(t, root, "remote", "add", "origin", originRemote)

	within := filepath.Join(root, "cmd")

	err := os.Mkdir(within, 0o750)
	if err != nil {
		t.Fatal(err)
	}

	return root, within
}

func TestHereIsWhereTheSessionWorksAndTheRepositoryItIsIn(t *testing.T) {
	// Arrange
	isolateGit(t)
	homeOfItsOwn(t)
	root, within := repositoryWithOrigin(t)
	where := wiring.Locate(t.Context(), within)

	// Act
	here := wired(t, config.Default(), where, nil).Repositories.Here

	// Assert
	want := forge.Repo{Kind: forge.KindGitHub, Host: "github.com", Path: "acme/api"}
	if here.Dir != within || here.Root != root || here.Remote != want {
		t.Errorf("Here = %+v; want %s, in %s, on %+v", here, within, root, want)
	}
}

func TestLookReadsAnotherDirectoryAsAPlace(t *testing.T) {
	// Arrange
	isolateGit(t)
	homeOfItsOwn(t)
	root, within := repositoryWithOrigin(t)
	write(t, filepath.Join(root, ".workflow.json"), "{}\n", 0o600)
	repositories := wired(t, config.Default(), wiring.Workspace{Root: t.TempDir()}, nil).Repositories

	// Act
	place, err := repositories.Look(within)

	// Assert
	if err != nil || place.Dir != within || place.Root != root || place.Remote.Path != "acme/api" ||
		place.Config.Repo != filepath.Join(root, ".workflow.json") {
		t.Errorf("Look = %+v, %v; want %s in %s, with its configuration", place, err, within, root)
	}
}

func TestADirectoryOutsideARepositoryHasNoRoot(t *testing.T) {
	// Arrange
	isolateGit(t)
	homeOfItsOwn(t)

	plain, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	repositories := wired(t, config.Default(), wiring.Workspace{Root: t.TempDir()}, nil).Repositories

	// Act
	place, err := repositories.Look(plain)

	// Assert
	if err != nil || place.Dir != plain || place.Root != "" || place.Remote != (forge.Repo{}) {
		t.Errorf("Look = %+v, %v; want %s with no repository", place, err, plain)
	}
}

func TestLookRefusesWhatIsNotADirectory(t *testing.T) {
	// Arrange
	homeOfItsOwn(t)
	repositories := wired(t, config.Default(), wiring.Workspace{Root: t.TempDir()}, nil).Repositories

	// Act
	_, err := repositories.Look(filepath.Join(t.TempDir(), "gone"))

	// Assert
	if !errors.Is(err, workdirs.ErrNotFound) {
		t.Errorf("Look = %v, want workdirs.ErrNotFound", err)
	}
}

func TestTheDirectoriesInOneAreListed(t *testing.T) {
	// Arrange
	home := homeOfItsOwn(t)
	write(t, filepath.Join(home, "notes.md"), "", 0o600)

	err := os.Mkdir(filepath.Join(home, "src"), 0o750)
	if err != nil {
		t.Fatal(err)
	}

	repositories := wired(t, config.Default(), wiring.Workspace{Root: t.TempDir()}, nil).Repositories

	// Act
	listing, err := repositories.Subdirectories(home, "")

	// Assert
	if err != nil || len(listing.Entries) != 1 || listing.Entries[0].Name != "src" || repositories.Home != home {
		t.Errorf("Subdirectories = %+v, %v, home %q; want src alone under %s", listing, err, repositories.Home, home)
	}
}

func TestFavoritesAreKeptThroughTheStoreSeams(t *testing.T) {
	// Arrange
	homeOfItsOwn(t)
	kept := wired(t, config.Default(), wiring.Workspace{Root: t.TempDir()}, nil).Store
	favorite := t.TempDir()

	err := kept.Favor(favorite)
	if err != nil {
		t.Fatalf("Favor: %v", err)
	}

	// Act
	favorites, err := kept.Favorites()

	// Assert
	if err != nil || !slices.Equal(favorites, []string{favorite}) {
		t.Errorf("Favorites = %v, %v; want %s", favorites, err, favorite)
	}
}

func TestADryRunReadsFavoritesAndKeepsNone(t *testing.T) {
	// Arrange
	homeOfItsOwn(t)

	cfg := config.Default()

	// Act
	kept := wiring.ReadOnlyStore(t.Context(), cfg, wiring.Workspace{Root: t.TempDir()})

	// Assert
	if kept.Favorites == nil || kept.Favor != nil || kept.Unfavor != nil {
		t.Errorf("a dry run's store: Favorites %v, Favor %v, Unfavor %v; want the read alone",
			kept.Favorites != nil, kept.Favor != nil, kept.Unfavor != nil)
	}
}

func TestAStoreTurnedOffOffersNoFavorites(t *testing.T) {
	// Arrange
	homeOfItsOwn(t)

	cfg := config.Default()
	cfg.Store.Disabled = true

	// Act
	kept := wired(t, cfg, wiring.Workspace{Root: t.TempDir()}, nil).Store

	// Assert
	if kept.Favorites != nil || kept.Favor != nil || kept.Unfavor != nil {
		t.Errorf("a store turned off offers favorites")
	}
}

func TestADirectoryReachedThroughALinkIsReadWhereItIs(t *testing.T) {
	// Arrange
	// git names the repository's root with every link resolved, so the
	// directory is too, or the path within the root would climb out of it.
	isolateGit(t)
	homeOfItsOwn(t)
	root, within := repositoryWithOrigin(t)
	link := filepath.Join(t.TempDir(), "linked")

	err := os.Symlink(root, link)
	if err != nil {
		t.Fatal(err)
	}

	// Act
	where := wiring.Locate(t.Context(), filepath.Join(link, "cmd"))

	// Assert
	if where.Dir != within || where.Root != root {
		t.Errorf("Locate = %+v, want %s in %s", where, within, root)
	}
}

func TestTheWorktreesOfTheRepositoryHereAreListed(t *testing.T) {
	// Arrange
	isolateGit(t)

	root := repository(t)
	linked := filepath.Join(t.TempDir(), "hotfix")
	git(t, root, "worktree", "add", "--quiet", "-b", "hotfix", linked)

	repositories := wired(t, config.Default(), wiring.Locate(t.Context(), root), nil).Repositories

	// Act
	worktrees, err := repositories.Worktrees()

	// Assert
	if err != nil || len(worktrees) != 2 || worktrees[0].Dir != root || worktrees[1].Branch != "hotfix" {
		t.Errorf("Worktrees = %+v, %v; want %s on main, then hotfix", worktrees, err, root)
	}
}

func TestOutsideARepositoryThereAreNoWorktreesToList(t *testing.T) {
	// Arrange
	where := wiring.Locate(t.Context(), t.TempDir())

	// Act
	repositories := wired(t, config.Default(), where, nil).Repositories

	// Assert
	if repositories.Worktrees != nil {
		t.Error("Worktrees outside a repository is set, want nil")
	}
}

func TestALockedWorktreeWhoseDirectoryIsGoneReadsAsGone(t *testing.T) {
	// Arrange
	// git never marks a locked worktree prunable, as on a drive since
	// unmounted, so its directory is looked for.
	isolateGit(t)

	root := repository(t)
	linked := filepath.Join(t.TempDir(), "usb")
	git(t, root, "worktree", "add", "--quiet", "-b", "usb", linked)
	git(t, root, "worktree", "lock", linked)

	err := os.RemoveAll(linked)
	if err != nil {
		t.Fatal(err)
	}

	repositories := wired(t, config.Default(), wiring.Locate(t.Context(), root), nil).Repositories

	// Act
	worktrees, err := repositories.Worktrees()

	// Assert
	if err != nil || len(worktrees) != 2 || !worktrees[1].Missing || !worktrees[1].Locked {
		t.Errorf("Worktrees = %+v, %v; want the locked one read as gone", worktrees, err)
	}
}
