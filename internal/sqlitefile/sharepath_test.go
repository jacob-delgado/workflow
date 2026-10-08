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
	// A share's path on Windows, \\server\share\..., slashes to
	// //server/share/...; on Unix two leading slashes name the root, so the
	// same spelling of a path names a file a test can make.
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
