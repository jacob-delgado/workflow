// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package store_test

// The store's files can be listed — each database with its size and what it
// holds — and cleaned: the cache alone, or the kept data too. A clean refuses
// anything that is not a plain file in the store directory, and the next
// write makes the file again.

import (
	"database/sql"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/jacob-delgado/workflow/internal/store"
)

// cacheName is the cache database's file name.
const cacheName = "workflow.db"

// keepBoth leaves a scope in the cache and ana's link in the kept file.
func keepBoth(t *testing.T, dir string) store.Store {
	t.Helper()

	kept := store.New(dir, false)
	linkAna(t, kept)

	err := kept.RecordScope(t.Context(), repo, recorded, theTime())
	if err != nil {
		t.Fatalf("keeping a scope: %v", err)
	}

	return kept
}

// exists reports a file at path.
func exists(path string) bool {
	_, err := os.Lstat(path)

	return err == nil
}

// heldCount is how many of what a file holds, -1 where it says nothing.
func heldCount(file store.DataFile, what string) int {
	index := slices.IndexFunc(file.Holds, func(held store.Held) bool { return held.What == what })
	if index < 0 {
		return -1
	}

	return file.Holds[index].Count
}

func TestNoDatabaseIsListedWhereThereIsNone(t *testing.T) {
	t.Parallel()

	cases := map[string]string{
		"an empty directory": t.TempDir(),
		"no directory":       filepath.Join(t.TempDir(), "workflow"),
	}

	for name, dir := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Act
			files, err := store.Files(t.Context(), dir)

			// Assert
			if err != nil || len(files) != 0 {
				t.Errorf("Files = %+v, %v; want none and no error", files, err)
			}
		})
	}
}

// listed is what a test checks of a listed file: its name, kind, whether it
// has a size, and the count of one thing it holds.
type listed struct {
	name  string
	kind  store.DataKind
	sized bool
	held  int
}

func TestEachDatabaseIsListedWithItsKindSizeAndContents(t *testing.T) {
	t.Parallel()

	// Arrange
	dir := t.TempDir()
	keepBoth(t, dir)

	// Act
	files, err := store.Files(t.Context(), dir)

	// Assert
	if err != nil || len(files) != 2 {
		t.Fatalf("Files = %+v, %v; want the cache and the kept file", files, err)
	}

	got := []listed{
		{files[0].Name, files[0].Kind, files[0].Bytes > 0, heldCount(files[0], "scopes")},
		{files[1].Name, files[1].Kind, files[1].Bytes > 0, heldCount(files[1], "owner decisions")},
	}
	want := []listed{{cacheName, store.DataCache, true, 1}, {"kept.db", store.DataKept, true, 1}}

	if !slices.Equal(got, want) {
		t.Errorf("Files listed %+v, want %+v", got, want)
	}
}

func TestEveryTableIsCountedInWhatItsFileHolds(t *testing.T) {
	t.Parallel()

	// Arrange
	// Each file is made whole with its first write, so every table its schema
	// creates is there to count, and a summary leaving one out is short.
	dir := t.TempDir()
	keepBoth(t, dir)

	// Act
	files, err := store.Files(t.Context(), dir)

	// Assert
	if err != nil || len(files) != 2 {
		t.Fatalf("Files = %+v, %v; want the cache and the kept file", files, err)
	}

	for _, file := range files {
		if tables := tablesIn(t, filepath.Join(dir, file.Name)); len(file.Holds) != tables {
			t.Errorf("%s says it holds %d kinds of thing, %+v, but has %d tables",
				file.Name, len(file.Holds), file.Holds, tables)
		}
	}
}

// tablesIn is how many tables the database at path has, read as another
// program would.
func tablesIn(t *testing.T, path string) int {
	t.Helper()

	database, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("opening %s: %v", path, err)
	}

	t.Cleanup(func() { _ = database.Close() })

	var count int

	err = database.QueryRowContext(t.Context(), `SELECT COUNT(*) FROM sqlite_master WHERE type = 'table'`).Scan(&count)
	if err != nil {
		t.Fatalf("counting the tables of %s: %v", path, err)
	}

	return count
}

func TestAChoiceOfGroupsIsCountedInWhatTheKeptFileHolds(t *testing.T) {
	t.Parallel()

	// Arrange
	dir := t.TempDir()
	kept := store.New(dir, false)
	listBoth(t, kept)

	err := kept.RecordGroups(t.Context(), repo, workspaceA, []string{}, theTime())
	if err != nil {
		t.Fatalf("RecordGroups returned %v, want nil", err)
	}

	// Act
	files, err := store.Files(t.Context(), dir)

	// Assert
	index := slices.IndexFunc(files, func(file store.DataFile) bool { return file.Kind == store.DataKept })
	if err != nil || index < 0 || heldCount(files[index], "group choices") != 1 {
		t.Errorf("Files = %+v, %v; want the kept file holding one group choice", files, err)
	}
}

func TestAFileThatIsNotADatabaseIsListedBySizeAlone(t *testing.T) {
	t.Parallel()

	// Arrange
	// Cleaning is the remedy for a broken file, so listing it must not fail.
	dir := t.TempDir()

	err := os.WriteFile(filepath.Join(dir, cacheName), []byte("not a database"), 0o600)
	if err != nil {
		t.Fatalf("writing the file: %v", err)
	}

	// Act
	files, err := store.Files(t.Context(), dir)

	// Assert
	if err != nil || len(files) != 1 || files[0].Bytes != int64(len("not a database")) || len(files[0].Holds) != 0 {
		t.Errorf("Files = %+v, %v; want workflow.db by its size, holding nothing said", files, err)
	}
}

// leaveAside writes the files an interrupted clean would leave set aside.
func leaveAside(t *testing.T, dir string, names ...string) {
	t.Helper()

	for _, name := range names {
		err := os.WriteFile(filepath.Join(dir, name), []byte("left"), 0o600)
		if err != nil {
			t.Fatalf("writing %s: %v", name, err)
		}
	}
}

func TestFilesSetAsideByAnInterruptedCleanAreListed(t *testing.T) {
	t.Parallel()

	// Arrange
	dir := t.TempDir()
	leaveAside(t, dir, "workflow.db.cleaning", "workflow.db-wal.cleaning")

	// Act
	files, err := store.Files(t.Context(), dir)

	// Assert
	want := []store.DataFile{{Name: "workflow.db.cleaning", Kind: store.DataCache, Bytes: 8, Holds: nil}}
	if err != nil || len(files) != 1 || files[0].Name != want[0].Name || files[0].Kind != want[0].Kind ||
		files[0].Bytes != want[0].Bytes {
		t.Errorf("Files = %+v, %v; want %+v", files, err, want)
	}
}

func TestTheNextCleanRemovesWhatAnInterruptedOneSetAside(t *testing.T) {
	t.Parallel()

	// Arrange
	dir := t.TempDir()
	keepBoth(t, dir)
	leaveAside(t, dir, "workflow.db.cleaning", "workflow.db-shm.cleaning", "kept.db.cleaning")

	// Act
	err := store.Clean(dir, store.CleanCache)
	// Assert
	if err != nil {
		t.Fatalf("Clean returned %v, want nil", err)
	}

	for _, name := range []string{cacheName, "workflow.db.cleaning", "workflow.db-shm.cleaning"} {
		if exists(filepath.Join(dir, name)) {
			t.Errorf("%s is still there after cleaning the cache", name)
		}
	}

	if !exists(filepath.Join(dir, "kept.db.cleaning")) {
		t.Error("cleaning the cache removed what a clean of the kept data set aside")
	}
}

func TestCleaningTheCacheKeepsTheKeptData(t *testing.T) {
	t.Parallel()

	// Arrange
	dir := t.TempDir()
	kept := keepBoth(t, dir)

	// Act
	err := store.Clean(dir, store.CleanCache)
	// Assert
	if err != nil {
		t.Fatalf("Clean returned %v, want nil", err)
	}

	for _, name := range []string{cacheName, "workflow.db-wal", "workflow.db-shm"} {
		if exists(filepath.Join(dir, name)) {
			t.Errorf("%s is still there after cleaning the cache", name)
		}
	}

	if links, _ := kept.OwnerLinks(t.Context(), forgeHost, workspaceA); len(links) != 1 {
		t.Errorf("OwnerLinks after cleaning the cache = %+v, want ana still linked", links)
	}
}

func TestCleaningEverythingRemovesTheKeptDataToo(t *testing.T) {
	t.Parallel()

	// Arrange
	dir := t.TempDir()
	keepBoth(t, dir)

	// Act
	err := store.Clean(dir, store.CleanAll)
	// Assert
	if err != nil {
		t.Fatalf("Clean returned %v, want nil", err)
	}

	entries, _ := os.ReadDir(dir)
	if len(entries) != 0 {
		t.Errorf("the store directory still holds %v, want nothing", entries)
	}
}

func TestAWriteAfterACleanMakesTheFileAgain(t *testing.T) {
	t.Parallel()

	// Arrange
	dir := t.TempDir()
	kept := keepBoth(t, dir)

	err := store.Clean(dir, store.CleanAll)
	if err != nil {
		t.Fatalf("cleaning: %v", err)
	}

	// Act
	scopeErr := kept.RecordScope(t.Context(), repo, "web", theTime())
	linkErr := kept.LinkOwner(t.Context(), forgeHost, workspaceA, decided("ben", nil), theTime())

	// Assert
	if scopeErr != nil || linkErr != nil {
		t.Fatalf("writing after a clean: scope %v, link %v; want both kept", scopeErr, linkErr)
	}

	scope, found, _ := kept.LastScope(t.Context(), repo)
	links, _ := kept.OwnerLinks(t.Context(), forgeHost, workspaceA)

	if !found || scope != "web" || len(links) != 1 {
		t.Errorf("after a clean and a write: scope %q, %v, links %+v; want the new ones alone", scope, found, links)
	}
}

func TestACleanRefusesWhatIsNotAPlainFileAndRemovesNothing(t *testing.T) {
	t.Parallel()

	cases := map[string]func(t *testing.T, dir string) string{
		"a symlinked database": func(t *testing.T, dir string) string {
			t.Helper()

			outside := filepath.Join(t.TempDir(), "elsewhere.db")

			err := os.WriteFile(outside, []byte("someone else's"), 0o600)
			if err != nil {
				t.Fatalf("writing the file outside: %v", err)
			}

			err = os.Symlink(outside, filepath.Join(dir, "kept.db-shm"))
			if err != nil {
				t.Skipf("symlinks unavailable: %v", err)
			}

			return outside
		},
		"a directory where a file is set aside": func(t *testing.T, dir string) string {
			t.Helper()

			aside := filepath.Join(dir, "kept.db.cleaning")

			err := os.Mkdir(aside, 0o700)
			if err != nil {
				t.Fatalf("making the directory: %v", err)
			}

			return aside
		},
		"a directory in a companion's place": func(t *testing.T, dir string) string {
			t.Helper()

			companion := filepath.Join(dir, "kept.db-wal")
			_ = os.Remove(companion)

			err := os.Mkdir(companion, 0o700)
			if err != nil {
				t.Fatalf("making the directory: %v", err)
			}

			return companion
		},
	}

	for name, plant := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			dir := t.TempDir()
			keepBoth(t, dir)

			_ = os.Remove(filepath.Join(dir, "kept.db-shm"))
			planted := plant(t, dir)

			// Act
			err := store.Clean(dir, store.CleanAll)

			// Assert
			if !errors.Is(err, store.ErrCleanRefused) {
				t.Errorf("Clean = %v, want ErrCleanRefused", err)
			}

			for _, path := range []string{filepath.Join(dir, cacheName), keptPath(dir), planted} {
				if !exists(path) {
					t.Errorf("%s was removed by a refused clean", path)
				}
			}
		})
	}
}

func TestACleanRefusesARelativeDirectory(t *testing.T) {
	t.Parallel()

	// Act
	err := store.Clean("workflow", store.CleanAll)

	// Assert
	if !errors.Is(err, store.ErrCleanRefused) {
		t.Errorf("Clean of a relative directory = %v, want ErrCleanRefused", err)
	}
}

func TestCleaningWhereThereIsNothingIsNoError(t *testing.T) {
	t.Parallel()

	// Arrange
	dir := filepath.Join(t.TempDir(), "workflow")

	// Act
	err := store.Clean(dir, store.CleanAll)
	// Assert
	if err != nil {
		t.Errorf("Clean of nothing = %v, want nil", err)
	}

	_, statErr := os.Stat(dir)
	if !errors.Is(statErr, fs.ErrNotExist) {
		t.Errorf("Clean made %s (stat: %v), want nothing made", dir, statErr)
	}
}
