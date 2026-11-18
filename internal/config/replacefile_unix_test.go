// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

//go:build unix

package config_test

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"syscall"
	"testing"

	"github.com/jacob-delgado/workflow/internal/config"
)

// turnIntoADirectory opens the pipe at path to write, which lets the read
// waiting on it go on, gives that read a file's worth, and puts a directory
// in the pipe's place before it closes, so the read ends only once the path
// names a directory.
func turnIntoADirectory(path string) error {
	pipe, err := os.OpenFile(path, os.O_WRONLY, 0)
	if err != nil {
		return fmt.Errorf("opening the pipe to write: %w", err)
	}

	_, err = pipe.WriteString("{}")

	return errors.Join(err, os.Remove(path), os.Mkdir(path, 0o700), pipe.Close())
}

func TestAFileThatTurnsIntoADirectoryAfterItsReadIsNotReplaced(t *testing.T) {
	t.Parallel()

	// Arrange
	// The file is a pipe, so the read of what it held ends when the test
	// says, once a directory has taken its place: the write meets what
	// another program made there after the read, and is refused.
	dir := t.TempDir()
	path := filepath.Join(dir, config.FileName)

	err := syscall.Mkfifo(path, 0o600)
	if err != nil {
		t.Fatalf("making the pipe: %v", err)
	}

	turned := make(chan error, 1)

	go func() { turned <- turnIntoADirectory(path) }()

	// Act
	_, err = config.ReplaceFile(config.SaveLocksIn(""), path, savedConfig())

	// Assert
	turnErr := <-turned
	info, statErr := os.Stat(path)

	if err == nil || turnErr != nil || statErr != nil || !info.IsDir() {
		t.Errorf("ReplaceFile = %v (turning: %v), the path %v; want the write refused and the directory left",
			err, turnErr, statErr)
	}

	if names := entryNames(t, dir); !slices.Equal(names, []string{config.FileName}) {
		t.Errorf("the directory holds %q after the refused write, want only what took the file's place", names)
	}
}
