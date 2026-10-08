// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

//go:build unix

package hooks_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/jacob-delgado/workflow/internal/hooks"
)

func TestWriteStaysInsideTheRepositoryPastALinkedScriptsDirectory(t *testing.T) {
	t.Parallel()

	// The repository's own tree decides what .lefthook is, so a clone can make
	// it a link to somewhere else entirely, such as a directory on PATH.
	cases := map[string]string{
		"the scripts directory": ".lefthook",
		"a hook's directory":    filepath.Join(".lefthook", preCommit),
	}

	for name, linked := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			repo, outside := t.TempDir(), t.TempDir()

			err := os.MkdirAll(filepath.Dir(filepath.Join(repo, linked)), 0o750)
			if err != nil {
				t.Fatal(err)
			}

			err = os.Symlink(outside, filepath.Join(repo, linked))
			if err != nil {
				t.Fatal(err)
			}

			// Act
			err = hooks.Write(repo, failingHook())

			// Assert
			if err == nil {
				t.Error("Write followed the link and succeeded, want it refused")
			}

			if left := entriesIn(t, outside); len(left) != 0 {
				t.Errorf("Write left %q outside the repository, want nothing", left)
			}
		})
	}
}
