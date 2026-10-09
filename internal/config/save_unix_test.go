// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

//go:build unix

package config_test

// A save writes a new file beside the configuration and only then puts it in
// the configuration's place. When that new file cannot be written whole, the
// save is refused with the reason, the part written is removed, and the
// configuration is left as it was. A limit on the size of the files the process
// may write is what fails the write here: it holds for every write the process
// makes, so the test runs alone and lifts it the moment Save returns.
//
// The limit is far past the size of anything else the process writes meanwhile,
// go test's own record of the files a test opens among them: at a byte, a flush
// of that record while the limit held would fail the whole run.

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"syscall"
	"testing"

	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/rlimit"
)

// fileSizeLimit is the size no file the process writes may pass while a test
// holds the limit.
const fileSizeLimit = 1 << 20

//nolint:paralleltest // the file size limit holds for every write the process makes, so this runs alone.
func TestASaveCutShortLeavesTheConfigurationAsItWas(t *testing.T) {
	// Arrange
	// The saved configuration encodes past the limit.
	const kept = `{"jira": {"base_url": "https://kept.example.com"}}`

	dir := t.TempDir()
	path := write(t, dir, kept)

	tooLong := savedConfig()
	tooLong.Jira.BaseURL += "/" + strings.Repeat("a", fileSizeLimit)

	lift := rlimit.Lower(t, syscall.RLIMIT_FSIZE, fileSizeLimit)

	// Act
	err := config.Save(path, tooLong)

	lift()

	// Assert
	if !errors.Is(err, syscall.EFBIG) || !strings.Contains(err.Error(), config.FileName) {
		t.Errorf("Save = %v, want the write's own failure, naming the file", err)
	}

	contents, readErr := os.ReadFile(path)
	if readErr != nil || string(contents) != kept {
		t.Errorf("the configuration reads %q (%v) after the refused save, want it as it was", contents, readErr)
	}

	if names := entryNames(t, dir); !slices.Equal(names, []string{config.FileName}) {
		t.Errorf("the directory holds %q after the refused save, want only the configuration", names)
	}
}

//nolint:paralleltest // the file size limit holds for every write the process makes, so this runs alone.
func TestACreateCutShortLeavesNoFile(t *testing.T) {
	// Arrange
	// The new configuration encodes past the limit.
	dir := t.TempDir()
	path := filepath.Join(dir, config.FileName)

	tooLong := savedConfig()
	tooLong.Jira.BaseURL += "/" + strings.Repeat("a", fileSizeLimit)

	lift := rlimit.Lower(t, syscall.RLIMIT_FSIZE, fileSizeLimit)

	// Act
	err := config.Create(path, tooLong)

	lift()

	// Assert
	if !errors.Is(err, syscall.EFBIG) || !strings.Contains(err.Error(), config.FileName) {
		t.Errorf("Create = %v, want the write's own failure, naming the file", err)
	}

	if names := entryNames(t, dir); len(names) != 0 {
		t.Errorf("the directory holds %q after the refused create, want nothing", names)
	}
}
