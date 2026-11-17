// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package wiring_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"testing"
	"time"

	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/jira"
	"github.com/jacob-delgado/workflow/internal/store"
	"github.com/jacob-delgado/workflow/internal/wiring"
)

// cachedJiraConfig is a configuration whose Jira the issue cache is keyed by,
// in a home of the test's own so the store starts empty.
func cachedJiraConfig(t *testing.T) config.Config {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_STATE_HOME", "")

	cfg := config.Default()
	cfg.Jira.BaseURL = jiraAddress

	return cfg
}

func TestTheIssueCacheNeverKeepsAForgeRow(t *testing.T) {
	// Arrange
	// The cache is keyed by the Jira instance and the view, not the repository,
	// so a forge issue's number kept there would seed another repository's list
	// with an issue that is not its own.
	deps := wired(t, cachedJiraConfig(t), wiring.Workspace{Root: t.TempDir()}, nil)
	listed := []jira.Issue{
		{Key: "42", Summary: "the forge's issue in this repository"},
		{Key: "PROJ-1", Summary: "a Jira issue"},
	}

	// Act
	err := deps.Store.CacheIssues(t.Context(), "assigned", listed, time.Now())
	if err != nil {
		t.Fatalf("caching the issues: %v", err)
	}

	// Assert
	cached, found := deps.Store.CachedIssues(t.Context(), "assigned")
	if !found || len(cached) != 1 || cached[0].Key != "PROJ-1" {
		t.Errorf("CachedIssues = %v, %v; want only PROJ-1, never the forge's #42", cached, found)
	}
}

func TestAForgeRowAlreadyInTheCacheIsNotReadBack(t *testing.T) {
	// Arrange
	// The store is a file on disk: a row an earlier build wrote, or a tampered
	// file holds, is not trusted to be one this repository may show.
	cfg := cachedJiraConfig(t)
	sum := sha256.Sum256([]byte(jiraAddress))

	dir, err := store.DefaultDir()
	if err != nil {
		t.Fatalf("resolving the store directory: %v", err)
	}

	seeded := []store.CachedIssue{{Key: "#7", Summary: "another repository's issue"}, {Key: "PROJ-2", Summary: "kept"}}

	err = store.New(dir, store.ModeWrites).
		CacheIssues(t.Context(), hex.EncodeToString(sum[:]), "assigned", seeded, time.Now())
	if err != nil {
		t.Fatalf("seeding the store: %v", err)
	}

	deps := wired(t, cfg, wiring.Workspace{Root: t.TempDir()}, nil)

	// Act
	cached, found := deps.Store.CachedIssues(t.Context(), "assigned")

	// Assert
	if !found || len(cached) != 1 || cached[0].Key != "PROJ-2" {
		t.Errorf("CachedIssues = %v, %v; want only PROJ-2, never the seeded forge #7", cached, found)
	}
}

func TestAListOfOnlyForgeIssuesIsNotCachedAsAnEmptyOne(t *testing.T) {
	// Arrange
	// A view cached as empty opens the next session on "no issues", settled; a
	// view whose every row was the forge's has nothing to keep, which is not
	// the same as having been seen empty.
	deps := wired(t, cachedJiraConfig(t), wiring.Workspace{Root: t.TempDir()}, nil)

	// Act
	err := deps.Store.CacheIssues(t.Context(), "assigned",
		[]jira.Issue{{Key: "42", Summary: "the forge's issue"}}, time.Now())
	if err != nil {
		t.Fatalf("caching the issues: %v", err)
	}

	// Assert
	if cached, found := deps.Store.CachedIssues(t.Context(), "assigned"); found {
		t.Errorf("CachedIssues = %v, true; want nothing cached", cached)
	}
}

func TestACachedListOfOnlyForgeRowsReadsAsNotCached(t *testing.T) {
	// Arrange
	cfg := cachedJiraConfig(t)
	sum := sha256.Sum256([]byte(jiraAddress))

	dir, err := store.DefaultDir()
	if err != nil {
		t.Fatalf("resolving the store directory: %v", err)
	}

	seeded := []store.CachedIssue{{Key: "#7", Summary: "another repository's issue"}}

	err = store.New(dir, store.ModeWrites).
		CacheIssues(t.Context(), hex.EncodeToString(sum[:]), "assigned", seeded, time.Now())
	if err != nil {
		t.Fatalf("seeding the store: %v", err)
	}

	deps := wired(t, cfg, wiring.Workspace{Root: t.TempDir()}, nil)

	// Act
	cached, found := deps.Store.CachedIssues(t.Context(), "assigned")

	// Assert
	if found {
		t.Errorf("CachedIssues = %v, true; want the view read as never cached", cached)
	}
}

func TestAListReadLaterWithinTheSameSecondIsKeptOverOneReadBefore(t *testing.T) {
	// Arrange
	// Two reads of one view half a second apart, within one second, whose
	// writes land out of order: the later read's first.
	deps := wired(t, cachedJiraConfig(t), wiring.Workspace{Root: t.TempDir()}, nil)
	earlier := time.Date(2026, time.November, 17, 9, 0, 0, 100*int(time.Millisecond), time.UTC)
	newer := []jira.Issue{{Key: "PROJ-11", Summary: "read later"}}

	err := deps.Store.CacheIssues(t.Context(), "assigned", newer, earlier.Add(500*time.Millisecond))
	if err != nil {
		t.Fatalf("caching the list read later: %v", err)
	}

	// Act
	err = deps.Store.CacheIssues(t.Context(), "assigned", []jira.Issue{{Key: "PROJ-10", Summary: "read before"}}, earlier)

	// Assert
	cached, found := deps.Store.CachedIssues(t.Context(), "assigned")
	if err != nil || !found || len(cached) != 1 || cached[0].Key != newer[0].Key {
		t.Errorf("CachedIssues = %v, %v (%v); want %s, the list read later", cached, found, err, newer[0].Key)
	}
}

func TestAListWhoseWriteFailedDoesNotKeepOneReadBeforeItOut(t *testing.T) {
	// Arrange
	// The list read later fails to land, its asker gone, so the list read
	// half a second before it is the newest there is to keep.
	deps := wired(t, cachedJiraConfig(t), wiring.Workspace{Root: t.TempDir()}, nil)
	earlier := time.Date(2026, time.November, 17, 9, 0, 0, 100*int(time.Millisecond), time.UTC)
	before := []jira.Issue{{Key: "PROJ-10", Summary: "read before"}}
	gone, cancel := context.WithCancel(t.Context())
	cancel()

	err := deps.Store.CacheIssues(gone, "assigned", []jira.Issue{{Key: "PROJ-11", Summary: "read later"}},
		earlier.Add(500*time.Millisecond))
	if err == nil {
		t.Fatal("caching the list read later for an asker gone returned nil, want it to fail")
	}

	// Act
	err = deps.Store.CacheIssues(t.Context(), "assigned", before, earlier)

	// Assert
	cached, found := deps.Store.CachedIssues(t.Context(), "assigned")
	if err != nil || !found || len(cached) != 1 || cached[0].Key != before[0].Key {
		t.Errorf("CachedIssues = %v, %v (%v); want %s, the only list written", cached, found, err, before[0].Key)
	}
}
