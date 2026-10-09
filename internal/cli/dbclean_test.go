// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package cli_test

// `workflow db-clean` lists the store's files and removes the cache, or with
// --all the kept associations too, once confirmed.

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/jacob-delgado/workflow/internal/cli"
	"github.com/jacob-delgado/workflow/internal/store"
)

// storeDirIn is where a run with home as its home keeps its store.
func storeDirIn(t *testing.T, home string) string {
	t.Helper()

	env := isolatedEnvironment(place{home: home})

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

	err = kept.SetRepoGroups(t.Context(), "github.com/owner/repo", "T0OWNER",
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
	t.Parallel()

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
	t.Parallel()

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

	if !strings.Contains(printed.stderr, "Removed workflow.db.") || strings.Contains(printed.stdout, "Removed") {
		t.Errorf("the clean is not reported on stderr alone:\n%+v", printed)
	}
}

func TestDBCleanLeavesEverythingWhenDeclined(t *testing.T) {
	t.Parallel()

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
	t.Parallel()

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
	t.Parallel()

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
	t.Parallel()

	// Arrange
	where := place{dir: t.TempDir(), home: t.TempDir()}

	// Act
	printed, err := runStreamsAt(t, where, unusedPrompt(t), "db-clean")
	// Assert
	if err != nil {
		t.Fatalf("db-clean: %v (%+v)", err, printed)
	}

	if !strings.Contains(printed.stderr, "Nothing to remove.") || strings.Contains(printed.stdout, "Nothing to remove") {
		t.Errorf("an empty store is not said, on stderr alone, to have nothing to remove:\n%+v", printed)
	}

	if !strings.HasPrefix(printed.stdout, "Local data in ") {
		t.Errorf("stdout does not carry the listing:\n%s", printed.stdout)
	}
}

func TestDBCleanRefusesAFileThatIsNotTheStores(t *testing.T) {
	t.Parallel()

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
	t.Parallel()

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

func TestTheWebListsAndCleansTheStoreDBCleanDoes(t *testing.T) {
	t.Parallel()

	// Arrange
	where, dir := storedHome(t)

	ran := runRootAt(t, where, "--web")
	if ran.err != nil || ran.servers != 1 {
		t.Fatalf("workflow --web = %v, served %d times; want the server", ran.err, ran.servers)
	}

	local := ran.deps.Settings

	// Act
	cleanErr := local.RemoveLocalData(store.CleanCache)
	listed, files, listErr := local.LocalData()

	// Assert
	if cleanErr != nil || listErr != nil {
		t.Fatalf("clean: %v, list: %v", cleanErr, listErr)
	}

	if listed != dir || len(files) != 1 || files[0].Name != "kept.db" {
		t.Errorf("listed %s %+v after cleaning the cache, want kept.db alone in %s", listed, files, dir)
	}
}

func TestDBCleanIsSummarizedAsRemovingTheLocalData(t *testing.T) {
	t.Parallel()

	// Act
	output, err := run(t, t.TempDir(), "--help")

	// Assert
	if err != nil || !strings.Contains(output, "Remove workflow's local data") {
		t.Errorf("--help = %v, want db-clean summarized as removing workflow's local data:\n%s", err, output)
	}
}

func TestDBCleanWithNowhereToKeepTheStoreSaysSo(t *testing.T) {
	t.Parallel()

	// Arrange
	// No home and no $XDG_STATE_HOME leave nowhere the store could be.
	where := place{dir: t.TempDir(), home: ""}

	// Act
	printed, err := runStreamsAt(t, where, unusedPrompt(t), "db-clean", "--yes")

	// Assert
	if err == nil || !strings.Contains(err.Error(), "finding the local data") || printed.stdout != "" {
		t.Errorf("db-clean with no home = %v, printed %q; want it to say it found no local data to list",
			err, printed.stdout)
	}
}

func TestDBCleanThatCannotReadTheStoreSaysSo(t *testing.T) {
	t.Parallel()

	// Arrange
	if runtime.GOOS == "windows" {
		t.Skip("Windows reads a path through a file as one that is not there")
	}

	// A file stands where the store's directory's parent belongs, so nothing
	// in the store can be looked at.
	home := t.TempDir()
	parent := filepath.Dir(storeDirIn(t, home))

	err := os.MkdirAll(filepath.Dir(parent), 0o700)
	if err != nil {
		t.Fatal(err)
	}

	err = os.WriteFile(parent, []byte("not a directory\n"), 0o600)
	if err != nil {
		t.Fatal(err)
	}

	where := place{dir: t.TempDir(), home: home}

	// Act
	printed, err := runStreamsAt(t, where, unusedPrompt(t), "db-clean", "--yes")

	// Assert
	if err == nil || !strings.Contains(err.Error(), "reading the local data") || printed.stdout != "" {
		t.Errorf("db-clean over an unreadable store = %v, printed %q; want it to say it could not read it",
			err, printed.stdout)
	}
}

func TestDBCleanWithNoTerminalToAskRemovesNothingAndNamesYes(t *testing.T) {
	t.Parallel()

	// Arrange
	where, dir := storedHome(t)
	ended := func(string) (string, error) { return "", io.EOF }

	// Act
	_, err := runStreamsAt(t, where, cli.Prompt{Line: ended, Secret: ended}, "db-clean")

	// Assert
	if err == nil || !strings.Contains(err.Error(), "--yes") {
		t.Errorf("db-clean with nothing to answer = %v, want it to name --yes", err)
	}

	wantExit(t, err, 2)

	if !present(dir, "workflow.db") {
		t.Error("db-clean removed the cache with no answer to remove it")
	}
}
