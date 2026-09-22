// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package wiring_test

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

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
