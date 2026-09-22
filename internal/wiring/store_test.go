// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package wiring_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/store"
	"github.com/jacob-delgado/workflow/internal/wiring"
)

func TestTheStoreNeverHoldsARemotesCredential(t *testing.T) {
	// Arrange
	// A credential embedded in an HTTPS remote must never reach the store: it is
	// keyed by the remote's parsed host and path, not the raw URL.
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_STATE_HOME", "")

	const token = "ghp_SECRETTOKEN123"

	where := wiring.Workspace{
		Root:   t.TempDir(),
		Remote: "https://alice:" + token + "@github.com/org/repo.git",
	}
	deps := wiring.Deps(t.Context(), config.Default(), where, nil)

	// Act
	deps.Store.RecordScope("api")

	// Assert
	dir, err := store.DefaultDir()
	if err != nil {
		t.Fatalf("resolving the store directory: %v", err)
	}

	if storeHoldsToken(t, dir, token) {
		t.Errorf("the store holds the remote's credential %q", token)
	}
}

func TestCachedIssuesReadBackAreSanitized(t *testing.T) {
	// Arrange
	// A hostile field written straight to the store — as a tampered or corrupt
	// file could hold — must be neutralized when the interface reads it back,
	// because the store on disk is untrusted.
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_STATE_HOME", "")

	const baseURL = "https://jira.example.com"

	sum := sha256.Sum256([]byte(baseURL))
	instance := hex.EncodeToString(sum[:])

	dir, err := store.DefaultDir()
	if err != nil {
		t.Fatalf("resolving the store directory: %v", err)
	}

	hostile := store.CachedIssue{
		Key: "PROJ-1", Summary: "clear\x1b[2Jthe screen", Status: "To Do",
		StatusCategory: "new", Type: "Task", Priority: "High",
	}

	err = store.New(dir, false).CacheIssues(t.Context(), instance, "assigned", []store.CachedIssue{hostile}, time.Now())
	if err != nil {
		t.Fatalf("seeding the store: %v", err)
	}

	cfg := config.Default()
	cfg.Jira.BaseURL = baseURL
	deps := wiring.Deps(t.Context(), cfg, wiring.Workspace{Root: t.TempDir()}, nil)

	// Act
	issues, found := deps.Store.CachedIssues("assigned")

	// Assert
	if !found || len(issues) != 1 {
		t.Fatalf("CachedIssues = %v, %v; want the one seeded issue", issues, found)
	}

	if strings.ContainsRune(issues[0].Summary, '\x1b') {
		t.Errorf("the summary read back still holds a terminal escape: %q", issues[0].Summary)
	}
}

// storeHoldsToken reports whether any file in the store directory — the database
// or its write-ahead log — contains token.
func storeHoldsToken(t *testing.T, dir, token string) bool {
	t.Helper()

	entries, err := os.ReadDir(dir)
	if err != nil {
		return false
	}

	for _, entry := range entries {
		data, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err == nil && bytes.Contains(data, []byte(token)) {
			return true
		}
	}

	return false
}
