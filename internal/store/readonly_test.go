// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package store_test

// A read-only store is the one a dry run reads: what an earlier session kept,
// when there is a store on disk, and nothing made or written either way.

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/jacob-delgado/workflow/internal/store"
)

func TestAReadOnlyStoreMakesNothingWhereThereIsNone(t *testing.T) {
	t.Parallel()

	cases := map[string]storeCall{
		"LastScope": lastScope, "RecordScope": recordScope,
		"RecordAnnounce": recordAnnounce, "Announces": announces,
		"CachedIssues": cachedIssues, "CacheIssues": cacheIssues,
	}

	for name, call := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			dir := filepath.Join(t.TempDir(), "workflow")

			// Act
			readBack, err := call(t.Context(), store.New(dir, false).ReadOnly())

			// Assert
			if err != nil || readBack {
				t.Errorf("%s = %v, read back %v; want nothing, and no error, from no store", name, err, readBack)
			}

			_, statErr := os.Stat(dir)
			if !errors.Is(statErr, fs.ErrNotExist) {
				t.Errorf("%s on a read-only store made %s (stat: %v), want nothing on disk", name, dir, statErr)
			}
		})
	}
}

// keptAndRead pairs a write with the read that sees it.
type keptAndRead struct {
	write storeCall
	read  storeCall
}

// eachKind is each kind of thing the store keeps, by the read that sees it.
func eachKind() map[string]keptAndRead {
	return map[string]keptAndRead{
		"a scope":         {write: recordScope, read: lastScope},
		"an announcement": {write: recordAnnounce, read: announces},
		"an issue list":   {write: cacheIssues, read: cachedIssues},
	}
}

func TestAReadOnlyStoreReadsWhatAnEarlierSessionKept(t *testing.T) {
	t.Parallel()

	for name, kind := range eachKind() {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			dir := t.TempDir()

			_, err := kind.write(t.Context(), store.New(dir, false))
			if err != nil {
				t.Fatalf("keeping %s: %v", name, err)
			}

			// Act
			readBack, err := kind.read(t.Context(), store.New(dir, false).ReadOnly())

			// Assert
			if err != nil || !readBack {
				t.Errorf("reading %s = %v, read back %v; want what was kept", name, err, readBack)
			}
		})
	}
}

func TestAReadOnlyStoreWritesNothing(t *testing.T) {
	t.Parallel()

	for name, kind := range eachKind() {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			// A read through a live store makes the database, so the read-only
			// write below meets a store that is there.
			dir := t.TempDir()

			_, err := kind.read(t.Context(), store.New(dir, false))
			if err != nil {
				t.Fatalf("making the store: %v", err)
			}

			// Act
			_, err = kind.write(t.Context(), store.New(dir, false).ReadOnly())
			// Assert
			if err != nil {
				t.Errorf("writing %s to a read-only store = %v, want it held back quietly", name, err)
			}

			readBack, readErr := kind.read(t.Context(), store.New(dir, false))
			if readErr != nil || readBack {
				t.Errorf("after a read-only write, reading %s = %v, read back %v; want nothing kept",
					name, readErr, readBack)
			}
		})
	}
}

func TestAReadOnlyStoreReadsADirectoryNamedWithURICharacters(t *testing.T) {
	t.Parallel()

	// Arrange
	// The read-only open names the database by URI, where a percent sign
	// escapes and a hash ends the path, so the name must be escaped to reach it.
	dir := filepath.Join(t.TempDir(), "50% #1 Support", "workflow")

	_, err := recordAnnounce(t.Context(), store.New(dir, false))
	if err != nil {
		t.Fatalf("keeping an announcement: %v", err)
	}

	// Act
	readBack, err := announces(t.Context(), store.New(dir, false).ReadOnly())

	// Assert
	if err != nil || !readBack {
		t.Errorf("Announces under %q = %v, read back %v; want the kept announcement", dir, err, readBack)
	}
}

func TestAReadOnlyStoreLeavesTheDirectoryAsItFoundIt(t *testing.T) {
	t.Parallel()

	// Arrange
	// A live open narrows the directory to its owner every time; a read-only
	// one changes nothing it finds, so a looser mode set since stays.
	dir := filepath.Join(t.TempDir(), "workflow")

	_, err := recordAnnounce(t.Context(), store.New(dir, false))
	if err != nil {
		t.Fatalf("keeping an announcement: %v", err)
	}

	err = os.Chmod(dir, 0o755)
	if err != nil {
		t.Fatalf("loosening the store directory: %v", err)
	}

	// Act
	readBack, err := announces(t.Context(), store.New(dir, false).ReadOnly())

	// Assert
	if err != nil || !readBack {
		t.Fatalf("Announces = %v, read back %v; want the kept announcement", err, readBack)
	}

	info, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("reading the store directory: %v", err)
	}

	if info.Mode().Perm() != 0o755 {
		t.Errorf("after a read-only read the directory is %v, want the 0755 it was", info.Mode().Perm())
	}
}
