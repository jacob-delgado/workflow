// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package store_test

// The kept database, kept.db, sits beside workflow.db and holds what the user
// decided rather than what a session saw. It migrates forward and is never
// discarded: a schema bump of the cache leaves it alone, a file from a newer
// build reads as empty and refuses writes, one that is not a database is
// reported and left for the user to clean, and a dry run never makes it.

import (
	"bytes"
	"database/sql"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"testing"

	"github.com/jacob-delgado/workflow/internal/store"
)

// forgeHost is the forge host the kept tests associate owners on.
const forgeHost = "github.com"

// anaOwner is the forge owner the tests link to the Slack user Ana.
const anaOwner = "ana"

// ana is a Slack user the tests link a forge owner to.
func ana() *store.SlackTarget {
	return &store.SlackTarget{ID: "U012ABC", Label: "Ana Lima"}
}

// keptPath is the kept database's path in a store directory.
func keptPath(dir string) string {
	return filepath.Join(dir, "kept.db")
}

// openKeptFile opens the kept database directly, as another program would.
func openKeptFile(t *testing.T, dir string) *sql.DB {
	t.Helper()

	database, err := sql.Open("sqlite", keptPath(dir))
	if err != nil {
		t.Fatalf("opening the kept database: %v", err)
	}

	t.Cleanup(func() { _ = database.Close() })

	return database
}

// keptPragma reads one integer-valued pragma from the kept database.
func keptPragma(t *testing.T, dir, name string) int {
	t.Helper()

	var value int

	err := openKeptFile(t, dir).QueryRowContext(t.Context(), "PRAGMA "+name).Scan(&value)
	if err != nil {
		t.Fatalf("reading the kept %s: %v", name, err)
	}

	return value
}

// execKept runs one statement against the kept database directly.
func execKept(t *testing.T, dir, query string) {
	t.Helper()

	_, err := openKeptFile(t, dir).ExecContext(t.Context(), query)
	if err != nil {
		t.Fatalf("running %q on the kept database: %v", query, err)
	}
}

// linkAna links the forge owner ana to the Slack user Ana in a live store.
func linkAna(t *testing.T, kept store.Store) {
	t.Helper()

	err := kept.LinkOwner(t.Context(), forgeHost, workspaceA, decided(anaOwner, ana()), theTime())
	if err != nil {
		t.Fatalf("linking ana: %v", err)
	}
}

func TestAnOwnersSlackLinkRoundTrips(t *testing.T) {
	t.Parallel()

	// Arrange
	kept := store.New(t.TempDir(), false)

	// Act
	err := kept.LinkOwner(t.Context(), forgeHost, workspaceA, decided(anaOwner, ana()), theTime())
	if err != nil {
		t.Fatalf("LinkOwner returned %v, want nil", err)
	}

	links, err := kept.OwnerLinks(t.Context(), forgeHost, workspaceA)

	// Assert
	want := []store.OwnerLink{{Owner: anaOwner, OnSlack: true, Slack: *ana()}}
	if err != nil || !slices.Equal(links, want) {
		t.Errorf("OwnerLinks = %+v, %v; want %+v", links, err, want)
	}
}

func TestKeptDataIsAFileOfItsOwnBesideTheCache(t *testing.T) {
	t.Parallel()

	// Arrange
	dir := t.TempDir()

	// Act
	linkAna(t, store.New(dir, false))

	// Assert
	_, keptErr := os.Stat(keptPath(dir))
	_, cacheErr := os.Stat(filepath.Join(dir, "workflow.db"))

	if keptErr != nil || !errors.Is(cacheErr, fs.ErrNotExist) {
		t.Errorf("after a link: kept.db stat %v, workflow.db stat %v; want only kept.db made", keptErr, cacheErr)
	}
}

func TestKeptDataSurvivesTheCachesSchemaBump(t *testing.T) {
	t.Parallel()

	// Arrange
	// The cache at another version is discarded on its next open; the kept
	// file beside it must come through untouched.
	dir := t.TempDir()
	kept := store.New(dir, false)
	linkAna(t, kept)

	err := kept.RecordScope(t.Context(), repo, recorded, theTime())
	if err != nil {
		t.Fatalf("keeping a scope: %v", err)
	}

	stampVersion(t, dir, currentVersion+1)

	_, _, err = kept.LastScope(t.Context(), repo)
	if err != nil {
		t.Fatalf("reopening the cache at another version: %v", err)
	}

	// Act
	links, err := kept.OwnerLinks(t.Context(), forgeHost, workspaceA)

	// Assert
	if err != nil || len(links) != 1 {
		t.Errorf("OwnerLinks after the cache was discarded = %+v, %v; want ana still linked", links, err)
	}
}

func TestAKeptFileFromANewerBuildReadsAsEmpty(t *testing.T) {
	t.Parallel()

	// Arrange
	dir := t.TempDir()
	kept := store.New(dir, false)
	linkAna(t, kept)
	execKept(t, dir, "PRAGMA user_version = 99")

	// Act
	links, err := kept.OwnerLinks(t.Context(), forgeHost, workspaceA)

	// Assert
	if err != nil || len(links) != 0 {
		t.Errorf("OwnerLinks from a newer build's file = %+v, %v; want nothing and no error", links, err)
	}
}

func TestAKeptFileFromANewerBuildRefusesWritesAndIsLeftAlone(t *testing.T) {
	t.Parallel()

	// Arrange
	dir := t.TempDir()
	kept := store.New(dir, false)
	linkAna(t, kept)
	execKept(t, dir, "PRAGMA user_version = 99")
	execKept(t, dir, "PRAGMA application_id = 42")

	// Act
	err := kept.LinkOwner(t.Context(), forgeHost, workspaceA, decided("ben", nil), theTime())

	// Assert
	if !errors.Is(err, store.ErrKeptFromNewerBuild) {
		t.Errorf("LinkOwner on a newer build's file = %v, want ErrKeptFromNewerBuild", err)
	}

	if got := keptPragma(t, dir, "user_version"); got != 99 {
		t.Errorf("the newer file is now at version %d, want it left at 99", got)
	}

	if got := keptPragma(t, dir, "application_id"); got != 42 {
		t.Errorf("the newer file's application_id is %d, want the file that was there", got)
	}
}

// writeNotADatabase puts a kept.db in dir that is not a database, and returns
// what it holds.
func writeNotADatabase(t *testing.T, dir string) []byte {
	t.Helper()

	notADatabase := bytes.Repeat([]byte("this is not a database\n"), 45)

	err := os.WriteFile(keptPath(dir), notADatabase, 0o600)
	if err != nil {
		t.Fatalf("writing a kept.db that is not a database: %v", err)
	}

	return notADatabase
}

func TestAKeptFileThatIsNotADatabaseRefusesWritesAndIsLeftAlone(t *testing.T) {
	t.Parallel()

	// Arrange
	dir := t.TempDir()
	notADatabase := writeNotADatabase(t, dir)

	// Act
	err := store.New(dir, false).LinkOwner(t.Context(), forgeHost, workspaceA, decided(anaOwner, ana()), theTime())

	// Assert
	if err == nil {
		t.Error("LinkOwner over a kept.db that is not a database = nil, want it refused")
	}

	left, readErr := os.ReadFile(keptPath(dir))
	if readErr != nil || !bytes.Equal(left, notADatabase) {
		t.Errorf("kept.db now holds %d bytes (err %v), want its %d bytes left for the user to clean",
			len(left), readErr, len(notADatabase))
	}
}

func TestAKeptFileThatIsNotADatabaseReadsAsNothingAndSaysWhy(t *testing.T) {
	t.Parallel()

	// Arrange
	dir := t.TempDir()
	writeNotADatabase(t, dir)

	// Act
	links, err := store.New(dir, false).OwnerLinks(t.Context(), forgeHost, workspaceA)

	// Assert
	if len(links) != 0 || err == nil {
		t.Errorf("OwnerLinks from a kept.db that is not a database = %+v, %v; want nothing, and why", links, err)
	}
}

func TestAReadOnlyStoreNeverMakesTheKeptFile(t *testing.T) {
	t.Parallel()

	// Arrange
	dir := t.TempDir()
	readOnly := store.New(dir, false).ReadOnly()

	// Act
	linkErr := readOnly.LinkOwner(t.Context(), forgeHost, workspaceA, decided(anaOwner, ana()), theTime())
	links, readErr := readOnly.OwnerLinks(t.Context(), forgeHost, workspaceA)

	// Assert
	if linkErr != nil || readErr != nil || len(links) != 0 {
		t.Errorf("a read-only store: link %v, read %+v, %v; want nothing and no error", linkErr, links, readErr)
	}

	_, statErr := os.Stat(keptPath(dir))
	if !errors.Is(statErr, fs.ErrNotExist) {
		t.Errorf("a read-only store made kept.db (stat: %v), want nothing on disk", statErr)
	}
}

func TestAReadOnlyStoreReadsTheKeptLinks(t *testing.T) {
	t.Parallel()

	// Arrange
	dir := t.TempDir()
	linkAna(t, store.New(dir, false))

	// Act
	links, err := store.New(dir, false).ReadOnly().OwnerLinks(t.Context(), forgeHost, workspaceA)

	// Assert
	if err != nil || len(links) != 1 {
		t.Errorf("a read-only OwnerLinks = %+v, %v; want ana's link", links, err)
	}
}

func TestADisabledStoreKeepsNoLink(t *testing.T) {
	t.Parallel()

	// Arrange
	dir := t.TempDir()
	disabled := store.New(dir, true)

	// Act
	linkErr := disabled.LinkOwner(t.Context(), forgeHost, workspaceA, decided(anaOwner, ana()), theTime())
	links, readErr := disabled.OwnerLinks(t.Context(), forgeHost, workspaceA)

	// Assert
	if linkErr != nil || readErr != nil || len(links) != 0 {
		t.Errorf("a disabled store: link %v, read %+v, %v; want nothing and no error", linkErr, links, readErr)
	}

	_, statErr := os.Stat(keptPath(dir))
	if !errors.Is(statErr, fs.ErrNotExist) {
		t.Errorf("a disabled store made kept.db (stat: %v), want nothing on disk", statErr)
	}
}

func TestStoresOpeningAFreshKeptFileTogetherAllMigrateIt(t *testing.T) {
	t.Parallel()

	// Arrange
	// Each process migrates in one immediate transaction that re-reads the
	// version, so none applies a migration another already has.
	dir := t.TempDir()
	owners := []string{anaOwner, "ben", "carla", "dan", "eve", "fay"}
	failures := make([]error, len(owners))

	var group sync.WaitGroup

	// Act
	for index, owner := range owners {
		group.Go(func() {
			failures[index] = store.New(dir, false).LinkOwner(t.Context(), forgeHost, workspaceA, decided(owner, nil), theTime())
		})
	}

	group.Wait()

	// Assert
	for index, err := range failures {
		if err != nil {
			t.Errorf("deciding %s alongside the others: %v", owners[index], err)
		}
	}

	links, err := store.New(dir, false).OwnerLinks(t.Context(), forgeHost, workspaceA)
	if err != nil || len(links) != len(owners) {
		t.Errorf("OwnerLinks = %+v, %v; want every owner decided", links, err)
	}
}
