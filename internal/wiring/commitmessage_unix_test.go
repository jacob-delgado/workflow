// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

//go:build unix

package wiring_test

// A commit's message goes to a private file before git reads it. When that
// file cannot be written whole, the commit is refused with the reason and the
// part written is removed, before git is ever started. A limit on the size of
// the files the process may write is what fails the write here: it holds for
// every write the process makes, so it is lifted the moment the commit returns.
//
// The limit is far past the size of anything else the process writes meanwhile,
// go test's own record of the files a test opens among them: at a byte, a flush
// of that record while the limit held would fail the whole run. The message is
// what passes it.

import (
	"errors"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/wiring"
)

// fileSizeLimit is the size no file the process writes may pass while the test
// holds the limit.
const fileSizeLimit = 1 << 20

// limitFileSize caps the size of every file the process writes at limit bytes,
// and returns what lifts the cap again, which the test also runs at its end.
func limitFileSize(t *testing.T, limit uint64) func() {
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

	err = syscall.Setrlimit(syscall.RLIMIT_FSIZE, &syscall.Rlimit{Cur: limit, Max: was.Max})
	if err != nil {
		t.Fatalf("limiting the file size: %v", err)
	}

	return lift
}

func TestACommitWhoseMessageCannotBeWrittenWholeLeavesNothingBehind(t *testing.T) {
	// Arrange
	// No git on PATH either: were the failed write missed, the commit would
	// fail for want of git instead.
	drafts := t.TempDir()
	t.Setenv("TMPDIR", drafts)
	t.Setenv("PATH", t.TempDir())

	repo := wired(t, config.Default(), wiring.Workspace{Root: t.TempDir(), Remote: ""}, nil).Git

	message := "feat: x\n\n" + strings.Repeat("#", fileSizeLimit+1) + "\n"
	lift := limitFileSize(t, fileSizeLimit)

	// Act
	_, err := repo.Commit(message)

	lift()

	// Assert
	if !errors.Is(err, syscall.EFBIG) || !strings.Contains(err.Error(), "writing the commit message") {
		t.Errorf("Commit = %v, want the message file's own failure", err)
	}

	left, _ := filepath.Glob(filepath.Join(drafts, "*"))
	if len(left) != 0 {
		t.Errorf("the part of the message written outlived the refused commit: %q", left)
	}
}
