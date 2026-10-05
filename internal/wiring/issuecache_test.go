// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package wiring_test

import (
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
	deps.Store.CacheIssues("assigned", listed)

	// Assert
	cached, found := deps.Store.CachedIssues("assigned")
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

	err = store.New(dir, false).CacheIssues(t.Context(), hex.EncodeToString(sum[:]), "assigned", seeded, time.Now())
	if err != nil {
		t.Fatalf("seeding the store: %v", err)
	}

	deps := wired(t, cfg, wiring.Workspace{Root: t.TempDir()}, nil)

	// Act
	cached, found := deps.Store.CachedIssues("assigned")

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
	deps.Store.CacheIssues("assigned", []jira.Issue{{Key: "42", Summary: "the forge's issue"}})

	// Assert
	if cached, found := deps.Store.CachedIssues("assigned"); found {
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

	err = store.New(dir, false).CacheIssues(t.Context(), hex.EncodeToString(sum[:]), "assigned", seeded, time.Now())
	if err != nil {
		t.Fatalf("seeding the store: %v", err)
	}

	deps := wired(t, cfg, wiring.Workspace{Root: t.TempDir()}, nil)

	// Act
	cached, found := deps.Store.CachedIssues("assigned")

	// Assert
	if found {
		t.Errorf("CachedIssues = %v, true; want the view read as never cached", cached)
	}
}
