// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package issuecache_test

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/jacob-delgado/workflow/internal/issuecache"
	"github.com/jacob-delgado/workflow/internal/jira"
)

// saved is a small assigned-issue list to round-trip.
func saved() []jira.Issue {
	return []jira.Issue{
		{Key: "OPS-1", Summary: "Fix login", Status: "In Progress", StatusCategory: "indeterminate", Type: "Bug"},
		{Key: "OPS-2", Summary: "Rotate keys", Status: "To Do", StatusCategory: "new", Type: "Story"},
	}
}

func TestWriteThenReadRoundTripsTheList(t *testing.T) {
	t.Parallel()

	// Arrange
	path := filepath.Join(t.TempDir(), "sub", "issues.json")

	// Act
	err := issuecache.Write(path, saved())
	if err != nil {
		t.Fatalf("Write returned %v, want nil", err)
	}

	got := issuecache.Read(path)

	// Assert
	if !slices.Equal(got, saved()) {
		t.Errorf("Read returned %+v, want the written list", got)
	}
}

func TestReadOfAMissingCacheIsNil(t *testing.T) {
	t.Parallel()

	// Act
	got := issuecache.Read(filepath.Join(t.TempDir(), "absent.json"))

	// Assert
	if got != nil {
		t.Errorf("Read of a missing cache = %+v, want nil", got)
	}
}

func TestReadOfACorruptCacheIsNil(t *testing.T) {
	t.Parallel()

	// Arrange
	path := filepath.Join(t.TempDir(), "corrupt.json")

	err := os.WriteFile(path, []byte("{not json"), 0o600)
	if err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	// Act
	got := issuecache.Read(path)

	// Assert
	if got != nil {
		t.Errorf("Read of a corrupt cache = %+v, want nil", got)
	}
}

func TestWriteMakesTheCacheOwnerOnly(t *testing.T) {
	t.Parallel()

	// Arrange
	path := filepath.Join(t.TempDir(), "issues.json")

	// Act
	err := issuecache.Write(path, saved())
	if err != nil {
		t.Fatalf("Write returned %v, want nil", err)
	}

	// Assert
	info, err := os.Stat(path)
	if err != nil || info.Mode().Perm() != issuecache.FileMode {
		t.Errorf("cache mode = %v (%v), want %v", info.Mode().Perm(), err, issuecache.FileMode)
	}
}

func TestWriteReportsAnUnusablePath(t *testing.T) {
	t.Parallel()

	// Arrange
	// A file stands where the cache directory would be, so making it fails.
	blocker := filepath.Join(t.TempDir(), "blocker")

	err := os.WriteFile(blocker, nil, 0o600)
	if err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	// Act
	err = issuecache.Write(filepath.Join(blocker, "issues.json"), saved())

	// Assert
	if err == nil {
		t.Error("Write to an unusable path returned nil, want an error")
	}
}

func TestPathNamesEachInstanceItsOwnFile(t *testing.T) {
	t.Parallel()

	// Act
	one, err := issuecache.Path("https://jira.one.example")
	two, twoErr := issuecache.Path("https://jira.two.example")
	again, _ := issuecache.Path("https://jira.one.example")

	// Assert
	if err != nil || twoErr != nil || one == two || one != again {
		t.Errorf("Path gave %q and %q (%v, %v); want a stable, per-instance path", one, two, err, twoErr)
	}
}
