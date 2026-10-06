// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

//go:build unix

package editor_test

// A draft goes to a private file before the editor opens it. When that file
// cannot be written whole, Edit reports the reason and the part written is
// removed, before any editor is started. A limit on the size of the files the
// process may write is what fails the write here: it holds for every write the
// process makes, so the test runs alone and lifts it the moment the draft is
// written.
//
// The limit is far past the size of anything else the process writes meanwhile,
// go test's own record of the files a test opens among them: at a byte, a flush
// of that record while the limit held would fail the whole run.

import (
	"errors"
	"os"
	"strings"
	"syscall"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/jacob-delgado/workflow/internal/editor"
	"github.com/jacob-delgado/workflow/internal/rlimit"
)

// fileSizeLimit is the size no file the process writes may pass while a test
// holds the limit.
const fileSizeLimit = 1 << 20

//nolint:paralleltest // the file size limit holds for every write the process makes, so this runs alone.
func TestEditReportsADraftCutShortAndLeavesNoneBehind(t *testing.T) {
	// Arrange
	// The draft runs past the limit. No editor is installed either: were the
	// failed write missed, Edit would fail for want of the editor instead.
	tmpdir := t.TempDir()
	env := environment(map[string]string{editorVariable: missingEditor, tmpdirVariable: tmpdir})
	tooLong := strings.Repeat("a", fileSizeLimit+1)

	lift := rlimit.Lower(t, syscall.RLIMIT_FSIZE, fileSizeLimit)

	// Act
	reported, _ := editor.Edit(env, t.TempDir(), tooLong, "help", func(_ string, err error) tea.Msg {
		return failure{err: err}
	})().(failure)

	lift()

	// Assert
	if !errors.Is(reported.err, syscall.EFBIG) || !strings.Contains(reported.err.Error(), "writing the draft") {
		t.Errorf("Edit reported %v, want the draft's own failure", reported.err)
	}

	left, err := os.ReadDir(tmpdir)
	if err != nil || len(left) != 0 {
		t.Errorf("$TMPDIR holds %v (%v), want nothing left behind", left, err)
	}
}
