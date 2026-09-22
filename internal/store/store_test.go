// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package store_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/jacob-delgado/workflow/internal/store"
)

// theTime is a fixed moment the recording tests stamp with; none asserts on it.
func theTime() time.Time {
	return time.Date(2026, 9, 22, 12, 0, 0, 0, time.UTC)
}

const repo = "git@github.com:example/repo.git"

func TestALastScopeRoundTrips(t *testing.T) {
	t.Parallel()

	// Arrange
	kept := store.New(t.TempDir(), false)

	// Act
	err := kept.RecordScope(t.Context(), repo, "config", theTime())
	if err != nil {
		t.Fatalf("RecordScope returned %v, want nil", err)
	}

	scope, found, err := kept.LastScope(t.Context(), repo)

	// Assert
	if err != nil || !found || scope != "config" {
		t.Errorf("LastScope = %q, %v, %v; want the recorded scope", scope, found, err)
	}
}

func TestNoScopeIsRecordedYet(t *testing.T) {
	t.Parallel()

	// Arrange
	empty := store.New(t.TempDir(), false)

	// Act
	scope, found, err := empty.LastScope(t.Context(), repo)

	// Assert
	if err != nil || found || scope != "" {
		t.Errorf("LastScope = %q, %v, %v; want no scope found", scope, found, err)
	}
}

func TestRecordingReplacesTheEarlierScope(t *testing.T) {
	t.Parallel()

	// Arrange
	kept := store.New(t.TempDir(), false)
	_ = kept.RecordScope(t.Context(), repo, "config", theTime())

	// Act
	_ = kept.RecordScope(t.Context(), repo, "web", theTime())
	scope, _, _ := kept.LastScope(t.Context(), repo)

	// Assert
	if scope != "web" {
		t.Errorf("LastScope = %q, want the most recent scope", scope)
	}
}

func TestScopesAreKeptPerRepository(t *testing.T) {
	t.Parallel()

	// Arrange
	kept := store.New(t.TempDir(), false)
	_ = kept.RecordScope(t.Context(), repo, "config", theTime())

	// Act
	scope, found, _ := kept.LastScope(t.Context(), "git@github.com:other/repo.git")

	// Assert
	if found || scope != "" {
		t.Errorf("LastScope for another repo = %q, %v; want no scope shared across repositories", scope, found)
	}
}

func TestAnEmptyRepositoryKeyIsNoRepository(t *testing.T) {
	t.Parallel()

	// Arrange
	// An on store, but no repository to key by — outside a work tree with no
	// remote — records and reads nothing rather than keying state under "".
	dir := t.TempDir()
	kept := store.New(dir, false)

	// Act
	err := kept.RecordScope(t.Context(), "", "config", theTime())
	scope, found, _ := kept.LastScope(t.Context(), "")

	// Assert
	if err != nil || found || scope != "" {
		t.Errorf("an empty repository key returned %q, %v, %v; want it to no-op", scope, found, err)
	}
}

func TestADisabledStoreRecordsNothing(t *testing.T) {
	t.Parallel()

	// Arrange
	dir := t.TempDir()
	off := store.New(dir, true)

	// Act
	err := off.RecordScope(t.Context(), repo, "config", theTime())
	scope, found, _ := off.LastScope(t.Context(), repo)

	// Assert
	if err != nil || found || scope != "" {
		t.Errorf("a disabled store returned %q, %v, %v; want it to no-op", scope, found, err)
	}

	_, statErr := os.Stat(filepath.Join(dir, "workflow.db"))
	if statErr == nil {
		t.Error("a disabled store wrote a database file")
	}
}

func TestTheStoreIsReadableOnlyByItsOwner(t *testing.T) {
	t.Parallel()

	// Arrange
	dir := filepath.Join(t.TempDir(), "workflow")
	kept := store.New(dir, false)

	// Act
	_ = kept.RecordScope(t.Context(), repo, "config", theTime())

	// Assert
	dirInfo, err := os.Stat(dir)
	if err != nil || dirInfo.Mode().Perm() != 0o700 {
		t.Errorf("store directory mode = %v (err %v), want 0700", dirInfo.Mode().Perm(), err)
	}

	fileInfo, err := os.Stat(filepath.Join(dir, "workflow.db"))
	if err != nil || fileInfo.Mode().Perm() != 0o600 {
		t.Errorf("store file mode = %v (err %v), want 0600", fileInfo.Mode().Perm(), err)
	}
}

func TestTwoStoresShareTheOneFile(t *testing.T) {
	t.Parallel()

	// Arrange
	// Two stores on the same directory stand in for the interface and the web
	// server: one records, the other reads it back.
	dir := t.TempDir()
	writer := store.New(dir, false)
	reader := store.New(dir, false)

	// Act
	_ = writer.RecordScope(t.Context(), repo, "config", theTime())
	scope, found, err := reader.LastScope(t.Context(), repo)

	// Assert
	if err != nil || !found || scope != "config" {
		t.Errorf("a second store read = %q, %v, %v; want the shared scope", scope, found, err)
	}
}
