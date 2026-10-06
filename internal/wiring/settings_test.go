// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package wiring_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/store"
	"github.com/jacob-delgado/workflow/internal/wiring"
)

// madeCache makes the store's cache in dir, by remembering a scope there.
func madeCache(t *testing.T, dir string) {
	t.Helper()

	err := store.New(dir, false).RecordScope(t.Context(), "github.com/acme/api", "auth", time.Now())
	if err != nil {
		t.Fatalf("making the cache: %v", err)
	}
}

func TestLocalDataListsTheStoresFiles(t *testing.T) {
	// Arrange
	dir := isolatedStoreDir(t)
	madeCache(t, dir)
	deps := wired(t, config.Default(), wiring.Workspace{Root: t.TempDir(), Remote: ""}, nil)

	// Act
	listed, files, err := deps.Settings.LocalData()

	// Assert
	if err != nil || listed != dir || len(files) == 0 || files[0].Kind != store.DataCache {
		t.Errorf("LocalData() = %q, %+v, %v; want the cache listed in %q", listed, files, err, dir)
	}
}

func TestRemoveLocalDataRemovesTheCache(t *testing.T) {
	// Arrange
	dir := isolatedStoreDir(t)
	madeCache(t, dir)
	deps := wired(t, config.Default(), wiring.Workspace{Root: t.TempDir(), Remote: ""}, nil)

	// Act
	err := deps.Settings.RemoveLocalData(store.CleanCache)

	// Assert
	_, statErr := os.Stat(filepath.Join(dir, "workflow.db"))
	if err != nil || !os.IsNotExist(statErr) {
		t.Errorf("RemoveLocalData = %v, and the cache is still there (%v); want it removed", err, statErr)
	}
}
