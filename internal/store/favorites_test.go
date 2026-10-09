// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package store_test

// The directories you mark as favorites are kept, so the Repositories pane and
// the web's picker offer them again in the next session.

import (
	"errors"
	"slices"
	"testing"

	"github.com/jacob-delgado/workflow/internal/store"
)

// apiDir and webDir are the directories the cases mark.
const (
	apiDir = "/home/ana/src/api"
	webDir = "/home/ana/src/web"
)

// favor marks each dir a favorite, failing the test if it cannot.
func favor(t *testing.T, kept store.Store, dirs ...string) {
	t.Helper()

	for _, dir := range dirs {
		err := kept.Favor(t.Context(), dir, theTime())
		if err != nil {
			t.Fatalf("Favor(%q): %v", dir, err)
		}
	}
}

// favoriteDirs are the directories the store reads back, in its order.
func favoriteDirs(t *testing.T, kept store.Store) []string {
	t.Helper()

	favorites, err := kept.Favorites(t.Context())
	if err != nil {
		t.Fatalf("Favorites: %v", err)
	}

	dirs := make([]string, 0, len(favorites))
	for _, favorite := range favorites {
		dirs = append(dirs, favorite.Dir)
	}

	return dirs
}

func TestFavoritesRoundTripByDirectory(t *testing.T) {
	t.Parallel()

	// Arrange
	kept := store.New(t.TempDir(), false)
	favor(t, kept, webDir, apiDir)

	// Act
	favorites, err := kept.Favorites(t.Context())

	// Assert
	want := []store.Favorite{{Dir: apiDir, Added: theTime()}, {Dir: webDir, Added: theTime()}}
	if err != nil || !slices.Equal(favorites, want) {
		t.Errorf("Favorites = %+v, %v; want %+v", favorites, err, want)
	}
}

func TestMarkingAFavoriteAgainKeepsOneEntry(t *testing.T) {
	t.Parallel()

	// Arrange
	kept := store.New(t.TempDir(), false)
	favor(t, kept, apiDir)

	// Act
	favor(t, kept, apiDir)

	// Assert
	if got := favoriteDirs(t, kept); !slices.Equal(got, []string{apiDir}) {
		t.Errorf("Favorites = %v, want %s once", got, apiDir)
	}
}

func TestUnfavoringRemovesOnlyThatDirectory(t *testing.T) {
	t.Parallel()

	// Arrange
	kept := store.New(t.TempDir(), false)
	favor(t, kept, apiDir, webDir)

	// Act
	err := kept.Unfavor(t.Context(), apiDir)

	// Assert
	if got := favoriteDirs(t, kept); err != nil || !slices.Equal(got, []string{webDir}) {
		t.Errorf("Favorites = %v, %v; want %s alone", got, err, webDir)
	}
}

func TestOnlyAnAbsoluteCleanDirectoryIsAFavorite(t *testing.T) {
	t.Parallel()

	for _, dir := range []string{"src/api", "/home/ana/src/../api", "/home/ana/src/api/", "", "/home/\x00ana"} {
		t.Run(dir, func(t *testing.T) {
			t.Parallel()

			// Arrange
			kept := store.New(t.TempDir(), false)

			// Act
			err := kept.Favor(t.Context(), dir, theTime())

			// Assert
			if !errors.Is(err, store.ErrNotADirectoryPath) {
				t.Errorf("Favor(%q) = %v, want ErrNotADirectoryPath", dir, err)
			}
		})
	}
}

func TestAFavoriteRowOfTheWrongShapeIsNotReadBack(t *testing.T) {
	t.Parallel()

	// Arrange
	// The file is outside this process, so a row is not trusted to be one
	// Favor wrote.
	dir := t.TempDir()
	kept := store.New(dir, false)
	favor(t, kept, apiDir)
	execKept(t, dir, `INSERT INTO favorite_dir (dir, added_at) VALUES ('relative/dir', '2026-10-05T00:00:00Z')`)
	execKept(t, dir, `INSERT INTO favorite_dir (dir, added_at) VALUES ('/srv/web', 'yesterday')`)

	// Act
	got := favoriteDirs(t, kept)

	// Assert
	if !slices.Equal(got, []string{apiDir}) {
		t.Errorf("Favorites = %v, want only %s", got, apiDir)
	}
}

func TestAKeptFileFromBeforeFavoritesReadsAsEmptyAndRefusesOne(t *testing.T) {
	t.Parallel()

	// Arrange
	// Favorites came in with the kept schema's second version; a file the
	// first made is left as it is until workflow db-clean --all.
	dir := t.TempDir()
	kept := store.New(dir, false)
	linkAna(t, kept)
	execKept(t, dir, "PRAGMA user_version = 1")

	// Act
	err := kept.Favor(t.Context(), apiDir, theTime())

	// Assert
	if !errors.Is(err, store.ErrKeptSchemaDiffers) {
		t.Errorf("Favor on a first-version file = %v, want ErrKeptSchemaDiffers", err)
	}
}

func TestADisabledStoreKeepsNoFavorite(t *testing.T) {
	t.Parallel()

	// Arrange
	kept := store.New(t.TempDir(), true)

	// Act
	err := kept.Favor(t.Context(), apiDir, theTime())

	// Assert
	if got := favoriteDirs(t, kept); err != nil || len(got) != 0 {
		t.Errorf("Favor = %v, Favorites = %v; want nothing kept and no error", err, got)
	}
}

func TestOnlyAStoreThatIsOnAndWritableWrites(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	cases := map[string]struct {
		kept store.Store
		want bool
	}{
		"on":        {kept: store.New(dir, false), want: true},
		"read-only": {kept: store.New(dir, false).ReadOnly(), want: false},
		"off":       {kept: store.New(dir, true), want: false},
		"nowhere":   {kept: store.New("", false), want: false},
	}

	for name, which := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Act & Assert
			if got := which.kept.Writes(); got != which.want {
				t.Errorf("Writes = %v, want %v", got, which.want)
			}
		})
	}
}

func TestAStoreThatIsOnKeepsWhetherOrNotItWrites(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	cases := map[string]struct {
		kept store.Store
		want bool
	}{
		"on":        {kept: store.New(dir, false), want: true},
		"read-only": {kept: store.New(dir, false).ReadOnly(), want: true},
		"off":       {kept: store.New(dir, true), want: false},
		"nowhere":   {kept: store.New("", false), want: false},
	}

	for name, which := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Act & Assert
			if got := which.kept.Keeps(); got != which.want {
				t.Errorf("Keeps = %v, want %v", got, which.want)
			}
		})
	}
}
