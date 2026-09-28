// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

//go:build unix

package editor_test

// A draft goes to a private file before the editor opens it. When that file
// cannot be written whole, Compose is refused with the reason and the part
// written is removed, before any editor is started. A limit on the size of the
// files the process may write is what fails the write here: it holds for every
// write the process makes, so the test runs alone and lifts it the moment
// Compose returns.
//
// The limit is far past the size of anything else the process writes meanwhile,
// go test's own record of the files a test opens among them: at a byte, a flush
// of that record while the limit held would fail the whole run.

import (
	"errors"
	"strings"
	"syscall"
	"testing"

	"github.com/jacob-delgado/workflow/internal/editor"
)

// fileSizeLimit is the size no file the process writes may pass while a test
// holds the limit.
const fileSizeLimit = 1 << 20

// limitFileSize caps the size of every file the process writes at
// fileSizeLimit, and returns what lifts the cap again, which the test also runs
// at its end.
func limitFileSize(t *testing.T) func() {
	t.Helper()

	var was syscall.Rlimit

	err := syscall.Getrlimit(syscall.RLIMIT_FSIZE, &was)
	if err != nil {
		t.Fatalf("reading the file size limit: %v", err)
	}

	lift := func() {
		err := syscall.Setrlimit(syscall.RLIMIT_FSIZE, &was)
		if err != nil {
			t.Fatalf("lifting the file size limit: %v", err)
		}
	}

	t.Cleanup(lift)

	err = syscall.Setrlimit(syscall.RLIMIT_FSIZE, &syscall.Rlimit{Cur: fileSizeLimit, Max: was.Max})
	if err != nil {
		t.Fatalf("limiting the file size: %v", err)
	}

	return lift
}

//nolint:paralleltest // the file size limit holds for every write the process makes, so this runs alone.
func TestComposeReportsADraftCutShortAndLeavesNoneBehind(t *testing.T) {
	// Arrange
	// The draft runs past the limit. No editor is installed either: were the
	// failed write missed, Compose would fail for want of the editor instead.
	tmpdir := t.TempDir()
	env := environment(map[string]string{editorVariable: missingEditor, tmpdirVariable: tmpdir})
	tooLong := strings.Repeat("a", fileSizeLimit+1)

	lift := limitFileSize(t)

	// Act
	_, err := editor.Compose(env, tooLong, "help")

	lift()

	// Assert
	if !errors.Is(err, syscall.EFBIG) || !strings.Contains(err.Error(), "writing the draft") {
		t.Errorf("Compose = %v, want the draft's own failure", err)
	}

	requireEmpty(t, tmpdir)
}
