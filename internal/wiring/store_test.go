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
	"github.com/jacob-delgado/workflow/internal/tui"
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
	deps := wired(t, config.Default(), where, nil)

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

	sum := sha256.Sum256([]byte(jiraAddress))
	instance := hex.EncodeToString(sum[:])

	dir, err := store.DefaultDir()
	if err != nil {
		t.Fatalf("resolving the store directory: %v", err)
	}

	// Every field carries its own escape, so dropping sanitize.Line from any one
	// of the six on read-back fails this test rather than only the Summary.
	hostile := store.CachedIssue{
		Key:            "PROJ-1\x1b[2Jkey",
		Summary:        "clear\x1b[2Jsummary",
		Status:         "To\x1b[2JDo",
		StatusCategory: "ne\x1b[2Jw",
		Type:           "Ta\x1b[2Jsk",
		Priority:       "Hi\x1b[2Jgh",
	}

	err = store.New(dir, false).CacheIssues(t.Context(), instance, "assigned", []store.CachedIssue{hostile}, time.Now())
	if err != nil {
		t.Fatalf("seeding the store: %v", err)
	}

	cfg := config.Default()
	cfg.Jira.BaseURL = jiraAddress
	deps := wired(t, cfg, wiring.Workspace{Root: t.TempDir()}, nil)

	// Act
	issues, found := deps.Store.CachedIssues("assigned")

	// Assert
	if !found || len(issues) != 1 {
		t.Fatalf("CachedIssues = %v, %v; want the one seeded issue", issues, found)
	}

	got := issues[0]
	fields := map[string]string{
		"key":             string(got.Key),
		"summary":         got.Summary,
		"status":          got.Status,
		"status category": string(got.StatusCategory),
		"type":            got.Type,
		"priority":        got.Priority,
	}

	for name, value := range fields {
		if strings.ContainsRune(value, '\x1b') {
			t.Errorf("the %s read back still holds a terminal escape: %q", name, value)
		}
	}
}

func TestADryRunReadsWhatALiveSessionAnnounced(t *testing.T) {
	// Arrange
	// Both are keyed by the same repository, so what the live store recorded is
	// what the read-only one finds.
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_STATE_HOME", "")

	where := wiring.Workspace{Root: t.TempDir(), Remote: "https://github.com/org/repo.git"}
	wired(t, config.Default(), where, nil).Store.RecordAnnounce(tui.AnnouncedPost{Pull: 7, Moment: 1})

	// Act
	posts := wiring.ReadOnlyStore(t.Context(), config.Default(), where).Announced()

	// Assert
	if len(posts) != 1 || posts[0] != (tui.AnnouncedPost{Pull: 7, Moment: 1}) {
		t.Errorf("the dry run read %+v, want the one announcement the live session recorded", posts)
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
