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

// writeUnder creates an empty file at a path below dir, making its directories.
func writeUnder(t *testing.T, dir, rel string) {
	t.Helper()

	full := filepath.Join(dir, filepath.FromSlash(rel))

	err := os.MkdirAll(filepath.Dir(full), 0o700)
	if err == nil {
		err = os.WriteFile(full, nil, 0o600)
	}

	if err != nil {
		t.Fatal(err)
	}
}

func TestResolveKeepsAPlaceThatIsThereAsPrinted(t *testing.T) {
	t.Parallel()

	// Arrange
	dir := repositoryWith(t, sourceFile)

	// Act
	got, ok := editor.Resolve(dir, sourceFile)

	// Assert
	if !ok || got != sourceFile {
		t.Errorf("Resolve(%q) = %q, %v; want it unchanged and found", sourceFile, got, ok)
	}
}

func TestResolveFindsAPlacePrintedRelativeToItsPackage(t *testing.T) {
	t.Parallel()

	// Arrange
	// go test prints a bare file name; the file lives below the root.
	dir := repositoryWith(t)
	writeUnder(t, dir, "internal/tui/run_test.go")

	// Act
	got, ok := editor.Resolve(dir, "run_test.go")

	// Assert
	if want := filepath.Join("internal", "tui", "run_test.go"); !ok || got != want {
		t.Errorf("Resolve(%q) = %q, %v; want %q", "run_test.go", got, ok, want)
	}
}

func TestResolveDropsAPlaceThatMatchesNothing(t *testing.T) {
	t.Parallel()

	// Arrange
	dir := repositoryWith(t, sourceFile)

	// Act
	got, ok := editor.Resolve(dir, "absent_test.go")

	// Assert
	if ok || got != "" {
		t.Errorf("Resolve(%q) = %q, %v; want it dropped", "absent_test.go", got, ok)
	}
}

func TestResolveDropsAnAmbiguousPlace(t *testing.T) {
	t.Parallel()

	// Arrange
	// The same name below two packages: opening either would be a guess.
	dir := repositoryWith(t)
	writeUnder(t, dir, "a/shared_test.go")
	writeUnder(t, dir, "b/shared_test.go")

	// Act
	got, ok := editor.Resolve(dir, "shared_test.go")

	// Assert
	if ok || got != "" {
		t.Errorf("Resolve(%q) = %q, %v; want it dropped as ambiguous", "shared_test.go", got, ok)
	}
}

func TestResolveIgnoresTheGitDirectory(t *testing.T) {
	t.Parallel()

	// Arrange
	// git keeps copies of tracked files under .git; a place must resolve to a
	// working file, so the one below .git must not be the match that wins.
	dir := repositoryWith(t)
	writeUnder(t, dir, ".git/hooks/pre-commit.go")
	writeUnder(t, dir, "internal/pre-commit.go")

	// Act
	got, ok := editor.Resolve(dir, "pre-commit.go")

	// Assert
	if want := filepath.Join("internal", "pre-commit.go"); !ok || got != want {
		t.Errorf("Resolve(%q) = %q, %v; want %q, skipping .git", "pre-commit.go", got, ok, want)
	}
}

func TestResolveSkipsADirectoryItCannotRead(t *testing.T) {
	t.Parallel()

	// Arrange
	if os.Geteuid() == 0 {
		t.Skip("root reads every directory, so the sealed one would not be skipped")
	}

	dir := repositoryWith(t)
	writeUnder(t, dir, "reachable/run_test.go")
	writeUnder(t, dir, "sealed/run_test.go")

	sealed := filepath.Join(dir, "sealed")

	err := os.Chmod(sealed, 0)
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() { _ = os.Chmod(sealed, 0o700) })

	// Act
	got, ok := editor.Resolve(dir, "run_test.go")

	// Assert
	// The sealed copy cannot be read, so the reachable one is the only match.
	if want := filepath.Join("reachable", "run_test.go"); !ok || got != want {
		t.Errorf("Resolve(%q) = %q, %v; want %q", "run_test.go", got, ok, want)
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
