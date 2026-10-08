// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package cli_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestStatusCountsNoChangesItCannotRead(t *testing.T) {
	t.Parallel()

	// Arrange
	// There is a change to count, but git cannot read the index to count it,
	// while the branch and its commits still read.
	repo := t.TempDir()
	gitInit(t, repo)
	commit(t, repo, "init")

	err := os.WriteFile(filepath.Join(repo, "notes.txt"), []byte("draft\n"), 0o600)
	if err != nil {
		t.Fatal(err)
	}

	err = os.WriteFile(filepath.Join(repo, ".git", "index"), []byte("not an index"), 0o600)
	if err != nil {
		t.Fatal(err)
	}

	// Act
	output, err := run(t, repo, "status")

	// Assert
	// The line still comes, with the commit stage not started rather than in
	// flight, as though the tree were clean.
	if err != nil || !strings.Contains(output, "○ Commits") {
		t.Errorf("status = %v, want the line with the commit stage not started:\n%s", err, output)
	}
}
