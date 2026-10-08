// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package sqlitefile_test

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	"github.com/jacob-delgado/workflow/internal/sqlitefile"
)

// madeAt makes a table in the database file opened at path, failing the test
// unless it could.
func madeAt(t *testing.T, path string) {
	t.Helper()

	database := sqlitefile.Open(path, "mode=rwc")

	t.Cleanup(func() { closed(t, database) })

	_, err := database.ExecContext(t.Context(), "CREATE TABLE made (id INTEGER PRIMARY KEY) STRICT")
	if err != nil {
		t.Fatalf("making a table in %s: %v", path, err)
	}
}

// closed closes database, failing the test unless it could.
func closed(t *testing.T, database *sql.DB) {
	t.Helper()

	err := database.Close()
	if err != nil {
		t.Errorf("closing the database: %v", err)
	}
}

func TestAPathHoldingWhatAURIMeansSomethingByNamesThatFile(t *testing.T) {
	t.Parallel()

	// Arrange
	dir := filepath.Join(t.TempDir(), "a?b#c%25d")

	err := os.Mkdir(dir, 0o700)
	if err != nil {
		t.Fatalf("making the directory: %v", err)
	}

	path := filepath.Join(dir, "made.db")

	// Act
	madeAt(t, path)

	// Assert
	_, err = os.Stat(path)
	if err != nil {
		t.Errorf("the database is not at %s: %v", path, err)
	}
}
