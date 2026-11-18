// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package store_test

// The kept database, kept.db, sits beside workflow.db and holds what the user
// decided rather than what a session saw. Its schema has one version, opened
// as workflow.db's is: a schema bump of the cache leaves it alone, a file at
// another version is discarded and made again, one that is not a database is
// reported and left for the user to clean, and a dry run never makes it.

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/jacob-delgado/workflow/internal/store"
)

// keptVersion is the kept schema version this build stamps; it moves with the
// store's own constant.
const keptVersion = 2

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

	database := openFile(t, keptPath(dir))
	t.Cleanup(func() { _ = database.Close() })

	return database
}

// keptUserVersion reads the schema version the kept database is stamped with.
func keptUserVersion(t *testing.T, dir string) int {
	t.Helper()

	var version int

	err := openKeptFile(t, dir).QueryRowContext(t.Context(), "PRAGMA user_version").Scan(&version)
	if err != nil {
		t.Fatalf("reading the kept user_version: %v", err)
	}

	return version
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
	kept := store.New(t.TempDir(), store.ModeWrites)

	// Act
	err := kept.LinkOwner(t.Context(), forgeHost, workspaceA, decided(anaOwner, ana()), theTime())
	if err != nil {
		t.Fatalf("LinkOwner returned %v, want nil", err)
	}

	links, err := kept.OwnerLinks(t.Context(), forgeHost, workspaceA)

	// Assert
	want := []store.OwnerLink{store.OnSlackAs(anaOwner, false, *ana())}
	if err != nil || !slices.Equal(links, want) {
		t.Errorf("OwnerLinks = %+v, %v; want %+v", links, err, want)
	}
}

func TestKeptDataIsAFileOfItsOwnBesideTheCache(t *testing.T) {
	t.Parallel()

	// Arrange
	dir := t.TempDir()

	// Act
	linkAna(t, store.New(dir, store.ModeWrites))

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
	kept := store.New(dir, store.ModeWrites)
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

func TestAReadOfAKeptFileAtAnotherSchemaVersionMakesItAgain(t *testing.T) {
	t.Parallel()

	// Arrange
	dir := t.TempDir()
	kept := store.New(dir, store.ModeWrites)
	linkAna(t, kept)
	execKept(t, dir, "PRAGMA user_version = 99")

	// Act
	links, err := kept.OwnerLinks(t.Context(), forgeHost, workspaceA)

	// Assert
	if err != nil || len(links) != 0 {
		t.Errorf("OwnerLinks from a file at another version = %+v, %v; want nothing and no error", links, err)
	}

	if got := keptUserVersion(t, dir); got != keptVersion {
		t.Errorf("the file is at version %d after the read, want it made again at %d", got, keptVersion)
	}
}

func TestAKeptFileAtAnotherSchemaVersionIsMadeAgain(t *testing.T) {
	t.Parallel()

	// Arrange
	dir := t.TempDir()
	kept := store.New(dir, store.ModeWrites)
	linkAna(t, kept)
	execKept(t, dir, "PRAGMA user_version = 1")

	// Act
	err := kept.LinkOwner(t.Context(), forgeHost, workspaceA, decided("ben", nil), theTime())
	// Assert
	if err != nil {
		t.Fatalf("LinkOwner on a file at another version = %v, want it kept in a file made again", err)
	}

	if got := keptUserVersion(t, dir); got != keptVersion {
		t.Errorf("the file is at version %d, want %d", got, keptVersion)
	}

	links, err := kept.OwnerLinks(t.Context(), forgeHost, workspaceA)
	if want := []store.OwnerLink{decided("ben", nil)}; err != nil || !slices.Equal(links, want) {
		t.Errorf("OwnerLinks = %+v, %v; want ben alone, ana's link gone with the file", links, err)
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
	err := store.New(dir, store.ModeWrites).
		LinkOwner(t.Context(), forgeHost, workspaceA, decided(anaOwner, ana()), theTime())

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
	links, err := store.New(dir, store.ModeWrites).OwnerLinks(t.Context(), forgeHost, workspaceA)

	// Assert
	if len(links) != 0 || err == nil {
		t.Errorf("OwnerLinks from a kept.db that is not a database = %+v, %v; want nothing, and why", links, err)
	}
}

func TestAReadOnlyStoreNeverMakesTheKeptFile(t *testing.T) {
	t.Parallel()

	// Arrange
	dir := t.TempDir()
	readOnly := store.New(dir, store.ModeWrites).ReadOnly()

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
	linkAna(t, store.New(dir, store.ModeWrites))

	// Act
	links, err := store.New(dir, store.ModeWrites).ReadOnly().OwnerLinks(t.Context(), forgeHost, workspaceA)

	// Assert
	if err != nil || len(links) != 1 {
		t.Errorf("a read-only OwnerLinks = %+v, %v; want ana's link", links, err)
	}
}

func TestAReadOnlyStoreReadsAKeptFileAtAnotherSchemaVersionAsEmpty(t *testing.T) {
	t.Parallel()

	// Arrange
	// The live open would remake this file, so what it holds is not what a
	// run would read: ana's link is there, and a dry run must not show it.
	dir := t.TempDir()
	linkAna(t, store.New(dir, store.ModeWrites))
	execKept(t, dir, "PRAGMA user_version = 99")

	before, err := os.ReadFile(keptPath(dir))
	if err != nil {
		t.Fatal(err)
	}

	// Act
	links, err := store.New(dir, store.ModeWrites).ReadOnly().OwnerLinks(t.Context(), forgeHost, workspaceA)

	// Assert
	if err != nil || len(links) != 0 {
		t.Errorf("a read-only OwnerLinks = %+v, %v; want nothing read, and no error, from a file at another version",
			links, err)
	}

	after, err := os.ReadFile(keptPath(dir))
	if err != nil || !bytes.Equal(before, after) {
		t.Errorf("a read-only read changed kept.db (err %v), want it left as it was", err)
	}

	if got := keptUserVersion(t, dir); got != 99 {
		t.Errorf("kept.db is at version %d after a read-only read, want it left at 99", got)
	}
}

func TestADisabledStoreKeepsNoLink(t *testing.T) {
	t.Parallel()

	// Arrange
	dir := t.TempDir()
	disabled := store.New(dir, store.ModeOff)

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

func TestStoresOpeningAFreshKeptFileTogetherAllPrepareIt(t *testing.T) {
	t.Parallel()

	// Arrange
	// Each process makes the schema in one immediate transaction that re-reads
	// the version, so none makes a table another already has.
	dir := t.TempDir()
	owners := []string{anaOwner, "ben", "carla", "dan", "eve", "fay"}
	failures := make([]error, len(owners))

	var group sync.WaitGroup

	// Act
	for index, owner := range owners {
		group.Go(func() {
			failures[index] = store.New(dir, store.ModeWrites).
				LinkOwner(t.Context(), forgeHost, workspaceA, decided(owner, nil), theTime())
		})
	}

	group.Wait()

	// Assert
	for index, err := range failures {
		if err != nil {
			t.Errorf("deciding %s alongside the others: %v", owners[index], err)
		}
	}

	links, err := store.New(dir, store.ModeWrites).OwnerLinks(t.Context(), forgeHost, workspaceA)
	if err != nil || len(links) != len(owners) {
		t.Errorf("OwnerLinks = %+v, %v; want every owner decided", links, err)
	}
}

func TestAKeptWriteWhoseContextEndedWritesNothing(t *testing.T) {
	t.Parallel()

	// Arrange
	dir := t.TempDir()
	linkAna(t, store.New(dir, store.ModeWrites))

	ended, cancel := context.WithCancel(t.Context())
	cancel()

	// Act
	err := store.New(dir, store.ModeWrites).LinkOwner(ended, forgeHost, workspaceA, decided("bo", ana()), theTime())

	// Assert
	links, readErr := store.New(dir, store.ModeWrites).OwnerLinks(t.Context(), forgeHost, workspaceA)
	if !errors.Is(err, context.Canceled) || readErr != nil || len(links) != 1 {
		t.Errorf("LinkOwner = %v; links %+v (%v); want the ended context reported and nothing written",
			err, links, readErr)
	}
}

func TestAKeptWriteWhileAnotherWriterHoldsTheFileWritesNothing(t *testing.T) {
	t.Parallel()

	// Arrange
	dir := t.TempDir()
	linkAna(t, store.New(dir, store.ModeWrites))

	writer, err := openKeptFile(t, dir).Conn(t.Context())
	if err != nil {
		t.Fatalf("taking a connection: %v", err)
	}
	defer func() { _ = writer.Close() }()

	_, err = writer.ExecContext(t.Context(), "BEGIN IMMEDIATE")
	if err != nil {
		t.Fatalf("holding the write lock: %v", err)
	}
	defer func() { _, _ = writer.ExecContext(t.Context(), "ROLLBACK") }()

	// Act
	// The write waits out the store's busy timeout for the lock, and is then
	// refused as its transaction begins.
	err = store.New(dir, store.ModeWrites).LinkOwner(t.Context(), forgeHost, workspaceA, decided("bo", ana()), theTime())

	// Assert
	links, readErr := store.New(dir, store.ModeWrites).OwnerLinks(t.Context(), forgeHost, workspaceA)
	if err == nil || !strings.Contains(err.Error(), "writing the kept data") || readErr != nil || len(links) != 1 {
		t.Errorf("LinkOwner = %v; links %+v (%v); want the write refused as it begins and nothing written",
			err, links, readErr)
	}
}
