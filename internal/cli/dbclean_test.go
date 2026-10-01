// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package cli_test

// `workflow db-clean` lists the store's files and removes the cache, or with
// --all the kept associations too, once confirmed.

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/jacob-delgado/workflow/internal/store"
)

// storeDirIn is where a run with home as its home keeps its store.
func storeDirIn(t *testing.T, home string) string {
	t.Helper()

	env := isolatedEnvironment(home)

	dir, err := store.Dir(runtime.GOOS, home, func(name string) (string, bool) {
		value, ok := env[name]

		return value, ok
	})
	if err != nil {
		t.Fatalf("finding the store under %s: %v", home, err)
	}

	return dir
}

// storedHome is a home whose store holds a scope in its cache and a
// repository's group in its kept file, and the store's directory.
func storedHome(t *testing.T) (place, string) {
	t.Helper()

	home := t.TempDir()
	dir := storeDirIn(t, home)
	kept := store.New(dir, false)

	err := kept.RecordScope(t.Context(), "github.com/owner/repo", "api", time.Now())
	if err != nil {
		t.Fatalf("seeding the cache: %v", err)
	}

	err = kept.SetRepoGroups(t.Context(), "github.com/owner/repo",
		[]store.SlackTarget{{ID: "S0PLATFORM", Label: "platform"}}, time.Now())
	if err != nil {
		t.Fatalf("seeding the kept file: %v", err)
	}

	return place{dir: t.TempDir(), home: home}, dir
}

// present reports whether name is in the store directory dir.
func present(dir, name string) bool {
	_, err := os.Lstat(filepath.Join(dir, name))

	return err == nil
}

func TestDBCleanListsWhatEachFileHolds(t *testing.T) {
	// Arrange
	where, dir := storedHome(t)

	// Act
	printed, err := runStreamsAt(t, where, answering("n", new([]string)), "db-clean")
	// Assert
	if err != nil {
		t.Fatalf("db-clean: %v (%+v)", err, printed)
	}

	for _, want := range []string{dir, "workflow.db", "cache", "scopes: 1", "kept.db", "kept", "repository groups: 1"} {
		if !strings.Contains(printed.stdout, want) {
			t.Errorf("the listing does not show %q:\n%s", want, printed.stdout)
		}
	}

	if !strings.Contains(printed.stdout, "KiB") {
		t.Errorf("the listing does not give sizes in human units:\n%s", printed.stdout)
	}
}

func TestDBCleanRemovesTheCacheOnceConfirmed(t *testing.T) {
	// Arrange
	where, dir := storedHome(t)

	var asked []string

	// Act
	printed, err := runStreamsAt(t, where, answering("y", &asked), "db-clean")
	// Assert
	if err != nil {
		t.Fatalf("db-clean: %v (%+v)", err, printed)
	}

	if len(asked) != 1 || !strings.Contains(asked[0], "workflow.db") || strings.Contains(asked[0], "kept.db") {
		t.Errorf("asked %q, want one question naming the cache alone", asked)
	}

	if present(dir, "workflow.db") || !present(dir, "kept.db") {
		t.Errorf("after a clean: workflow.db there %t (want false), kept.db there %t (want true)",
			present(dir, "workflow.db"), present(dir, "kept.db"))
	}

	if !strings.Contains(printed.stdout, "Removed workflow.db") {
		t.Errorf("the clean is not reported:\n%s", printed.stdout)
	}
}

func TestDBCleanLeavesEverythingWhenDeclined(t *testing.T) {
	// Arrange
	where, dir := storedHome(t)

	// Act
	printed, err := runStreamsAt(t, where, answering("", new([]string)), "db-clean", "--all")
	// Assert
	if err != nil {
		t.Fatalf("db-clean --all: %v (%+v)", err, printed)
	}

	if !present(dir, "workflow.db") || !present(dir, "kept.db") {
		t.Error("a declined clean removed a file")
	}

	if !strings.Contains(printed.stderr, "Nothing removed.") {
		t.Errorf("the declined clean is not said:\n%s", printed.stderr)
	}
}

func TestDBCleanAllWarnsAndRemovesBothWithYes(t *testing.T) {
	// Arrange
	where, dir := storedHome(t)

	// Act
	printed, err := runStreamsAt(t, where, unusedPrompt(t), "db-clean", "--all", "--yes")
	// Assert
	if err != nil {
		t.Fatalf("db-clean --all --yes: %v (%+v)", err, printed)
	}

	if present(dir, "workflow.db") || present(dir, "kept.db") {
		t.Error("db-clean --all --yes left a database file")
	}

	if !strings.Contains(printed.stderr, "people and group associations will be asked again") {
		t.Errorf("--all does not warn that associations are asked again:\n%s", printed.stderr)
	}
}

func TestDBCleanDryRunOnlySaysWhatItWouldRemove(t *testing.T) {
	// Arrange
	where, dir := storedHome(t)

	// Act
	printed, err := runStreamsAt(t, where, unusedPrompt(t), "db-clean", "--all", "--dry-run")
	// Assert
	if err != nil {
		t.Fatalf("db-clean --all --dry-run: %v (%+v)", err, printed)
	}

	if !present(dir, "workflow.db") || !present(dir, "kept.db") {
		t.Error("a dry run removed a file")
	}

	if !strings.Contains(printed.stderr, "dry run: would remove workflow.db and kept.db") {
		t.Errorf("the dry run does not say what it would remove:\n%s", printed.stderr)
	}
}

func TestDBCleanSaysWhenThereIsNothingToRemove(t *testing.T) {
	// Arrange
	where := place{dir: t.TempDir(), home: t.TempDir()}

	// Act
	printed, err := runStreamsAt(t, where, unusedPrompt(t), "db-clean")
	// Assert
	if err != nil {
		t.Fatalf("db-clean: %v (%+v)", err, printed)
	}

	if !strings.Contains(printed.stdout, "Nothing to remove") {
		t.Errorf("an empty store is not said to have nothing to remove:\n%s", printed.stdout)
	}
}

func TestDBCleanRefusesAFileThatIsNotTheStores(t *testing.T) {
	// Arrange
	home := t.TempDir()
	dir := storeDirIn(t, home)
	// A symlink where the cache belongs points outside the store.
	err := os.MkdirAll(dir, 0o700)
	if err != nil {
		t.Fatalf("making the store directory: %v", err)
	}

	err = os.Symlink(filepath.Join(home, "elsewhere"), filepath.Join(dir, "workflow.db"))
	if err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}

	// Act
	_, err = runStreamsAt(t, place{dir: t.TempDir(), home: home}, unusedPrompt(t), "db-clean", "--yes")
	// Assert
	if !errors.Is(err, store.ErrCleanRefused) {
		t.Fatalf("db-clean over a symlink: %v, want the clean refused", err)
	}

	wantExit(t, err, 1)

	if !present(dir, "workflow.db") {
		t.Error("the refused clean removed the symlink")
	}
}

func TestDBCleanExitsAsRefusedWhenAFileCannotBeRemoved(t *testing.T) {
	// Arrange
	where, dir := storedHome(t)
	// A directory nothing can be renamed in stands for a file another program
	// holds open on Windows.
	err := os.Chmod(dir, 0o500)
	if err != nil {
		t.Fatalf("making the store directory read-only: %v", err)
	}

	t.Cleanup(func() { _ = os.Chmod(dir, 0o700) })

	// Act
	_, err = runStreamsAt(t, where, unusedPrompt(t), "db-clean", "--yes")
	// Assert
	if !errors.Is(err, store.ErrNotCleaned) {
		t.Fatalf("db-clean in a read-only directory: %v, want the file not cleaned", err)
	}

	wantExit(t, err, 4)

	if !present(dir, "workflow.db") {
		t.Error("a failed clean removed the cache")
	}
}
