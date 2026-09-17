// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package editor_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/jacob-delgado/workflow/internal/editor"
)

func TestLocateNamesAFileInFull(t *testing.T) {
	t.Parallel()

	// A place a tool printed is a path from the repository's root, and may
	// begin with anything a file name can. Named in full it begins with the
	// root, whatever it began with before.
	cases := map[string]string{
		"an ordinary file":         sourceFile,
		"a name beginning with +":  "+draft.go",
		"a name beginning with -":  "-c.go",
		"a file in a subdirectory": filepath.Join("internal", "x.go"),
	}

	for name, file := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			dir := repositoryWith(t)

			err := os.MkdirAll(filepath.Join(dir, filepath.Dir(file)), 0o700)
			if err == nil {
				err = os.WriteFile(filepath.Join(dir, file), nil, 0o600)
			}

			if err != nil {
				t.Fatal(err)
			}

			// Act
			got, err := editor.Locate(dir, file)

			// Assert
			if err != nil || got != filepath.Join(dir, file) || !filepath.IsAbs(got) {
				t.Errorf("Locate(%q) = %q, %v; want %q", file, got, err, filepath.Join(dir, file))
			}
		})
	}
}

func TestLocateKeepsAPathAlreadyInFull(t *testing.T) {
	t.Parallel()

	// Arrange
	elsewhere := filepath.Join(repositoryWith(t, sourceFile), sourceFile)

	// Act
	got, err := editor.Locate(repositoryWith(t), elsewhere)

	// Assert
	if err != nil || got != elsewhere {
		t.Errorf("Locate(%q) = %q, %v; want it unchanged", elsewhere, got, err)
	}
}

func TestLocateRefusesWhatIsNotAFile(t *testing.T) {
	t.Parallel()

	cases := map[string]string{
		// go test prints a place relative to its package, which from the root
		// names nothing.
		"a file that is not there": "foo_test.go",
		"a directory":              "internal",
	}

	for name, file := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			dir := repositoryWith(t)

			err := os.Mkdir(filepath.Join(dir, "internal"), 0o700)
			if err != nil {
				t.Fatal(err)
			}

			// Act
			got, err := editor.Locate(dir, file)

			// Assert
			if !errors.Is(err, editor.ErrNoSuchFile) || got != "" {
				t.Errorf("Locate(%q) = %q, %v; want %v", file, got, err, editor.ErrNoSuchFile)
			}
		})
	}
}

func TestOpenRefusesAPlaceThatIsNotThere(t *testing.T) {
	t.Parallel()

	// Arrange
	env := environment(map[string]string{editorVariable: installedEditor})

	// Act
	reported, ok := editor.Open(env, repositoryWith(t), "foo_test.go", 4, func(err error) tea.Msg {
		return failure{err: err}
	})().(failure)

	// Assert
	// An editor handed a file that is not there opens an empty buffer, and
	// saving it leaves a stray file behind.
	if !ok || !errors.Is(reported.err, editor.ErrNoSuchFile) {
		t.Errorf("Open reported %v, want %v and no editor started", reported.err, editor.ErrNoSuchFile)
	}
}
