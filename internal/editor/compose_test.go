// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package editor_test

import (
	"errors"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/jacob-delgado/workflow/internal/editor"
	"github.com/jacob-delgado/workflow/internal/proc"
)

// scriptedEditor is an editor setting that runs script, a POSIX shell body
// handed the draft's path as $1, in place of a person at a terminal. The
// script is read by sh rather than run itself, so no test executes a file
// another parallel test may still hold open for writing.
func scriptedEditor(t *testing.T, script string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "editor.sh")

	err := os.WriteFile(path, []byte(script+"\n"), 0o600)
	if err != nil {
		t.Fatalf("writing the editor script: %v", err)
	}

	return "sh " + path
}

// requireEmpty fails the test if anything was left behind in dir.
func requireEmpty(t *testing.T, dir string) {
	t.Helper()

	left, err := os.ReadDir(dir)
	if err != nil || len(left) != 0 {
		t.Errorf("$TMPDIR holds %v (%v), want nothing left behind", left, err)
	}
}

func TestComposeRoundTripsADraftThroughTheEditor(t *testing.T) {
	t.Parallel()

	// Arrange
	// The editor writes a line above everything it was handed, so what comes
	// back shows the draft went out, was edited, and lost its help.
	tmpdir := t.TempDir()
	env := environment(map[string]string{
		editorVariable: scriptedEditor(t, `{ echo 'Shipped the login fix.'; cat "$1"; } >"$1.edited" && mv "$1.edited" "$1"`),
		tmpdirVariable: tmpdir,
	})

	// Act
	got, err := editor.Compose(env, "# Standup", "Edit above the line.")

	// Assert
	if err != nil || got != "Shipped the login fix.\n# Standup" {
		t.Errorf("Compose = %q, %v; want the edit above the draft, without its help", got, err)
	}

	requireEmpty(t, tmpdir)
}

func TestComposeAfterTheEditorFailedKeepsNothingAndCleansUp(t *testing.T) {
	t.Parallel()

	// Arrange
	tmpdir := t.TempDir()
	env := environment(map[string]string{
		editorVariable: scriptedEditor(t, `echo 'half an edit' >"$1"; exit 1`),
		tmpdirVariable: tmpdir,
	})

	// Act
	got, err := editor.Compose(env, "# Standup", "help")

	// Assert
	var exit *exec.ExitError
	if !errors.As(err, &exit) || got != "" {
		t.Errorf("Compose after a failed editor = %q, %v; want nothing kept and the editor's exit", got, err)
	}

	requireEmpty(t, tmpdir)
}

func TestComposeWithAnEditorThatIsNotInstalledSaysSoAndLeavesNoDraft(t *testing.T) {
	t.Parallel()

	// Arrange
	tmpdir := t.TempDir()
	env := environment(map[string]string{editorVariable: missingEditor, tmpdirVariable: tmpdir})

	// Act
	_, err := editor.Compose(env, "# Standup", "help")

	// Assert
	if !errors.Is(err, proc.ErrNotFound) {
		t.Errorf("Compose = %v, want ErrNotFound for an editor not on PATH", err)
	}

	requireEmpty(t, tmpdir)
}

func TestComposeReportsADraftThatCannotBeWritten(t *testing.T) {
	t.Parallel()

	// Arrange
	missingDir := filepath.Join(t.TempDir(), "missing")
	env := environment(map[string]string{editorVariable: installedEditor, tmpdirVariable: missingDir})

	// Act
	_, err := editor.Compose(env, "# Standup", "help")

	// Assert
	if !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("Compose = %v, want the failure to create a draft in a missing $TMPDIR", err)
	}
}
