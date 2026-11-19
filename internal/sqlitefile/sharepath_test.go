// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

//go:build unix

package sqlitefile_test

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAPathStartingWithTwoSlashesNamesThatFile(t *testing.T) {
	t.Parallel()

	// Arrange
	// Two leading slashes name the root, so the URI must not read the first
	// part of such a path as a server.
	dir := t.TempDir()
	path := "/" + filepath.Join(dir, "made.db")

	// Act
	madeAt(t, path)

	// Assert
	_, err := os.Stat(filepath.Join(dir, "made.db"))
	if err != nil {
		t.Errorf("the database is not at %s: %v", path, err)
	}
}
