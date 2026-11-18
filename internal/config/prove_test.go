// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package config_test

// A Slack refresh spends its refresh token as Slack answers, so the file the
// new pair goes into is proved writable first, by a file made and removed
// beside it, without a byte of the file itself changing.

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/jacob-delgado/workflow/internal/config"
)

// provedContents is a configuration file a prove must leave as it is.
const provedContents = `{"jira": {"project": "PROVED"}}`

// entries are the names in dir.
func entries(t *testing.T, dir string) []string {
	t.Helper()

	found, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("listing %s: %v", dir, err)
	}

	names := make([]string, 0, len(found))
	for _, entry := range found {
		names = append(names, entry.Name())
	}

	return names
}

// agedDirectory sets dir's modification time a day back, and is that time,
// so a file made or removed in dir afterwards shows in its modification time.
func agedDirectory(t *testing.T, dir string) time.Time {
	t.Helper()

	aged := time.Now().Add(-24 * time.Hour).Truncate(time.Second)

	err := os.Chtimes(dir, aged, aged)
	if err != nil {
		t.Fatalf("aging %s: %v", dir, err)
	}

	return aged
}

// modifiedSince reports dir modified after aged: a file made or removed in it.
func modifiedSince(t *testing.T, dir string, aged time.Time) bool {
	t.Helper()

	info, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("reading %s: %v", dir, err)
	}

	return info.ModTime().After(aged)
}

func TestProveWritableMakesAProbeAndLeavesTheFileAsItWas(t *testing.T) {
	t.Parallel()

	// Arrange
	dir := t.TempDir()
	path := write(t, dir, provedContents)
	aged := agedDirectory(t, dir)

	// Act
	err := config.ProveWritable(path)

	// Assert
	contents, readErr := os.ReadFile(path)
	if err != nil || readErr != nil || string(contents) != provedContents {
		t.Errorf("ProveWritable = %v; the file reads %q (%v); want it proved and unchanged", err, contents, readErr)
	}

	if names := entries(t, dir); len(names) != 1 || !modifiedSince(t, dir, aged) {
		t.Errorf("the directory holds %q, want a probe made beside the file and removed", names)
	}
}

func TestProveWritableInADirectoryNothingCanBeWrittenInIsRefused(t *testing.T) {
	t.Parallel()

	// Arrange
	if os.Geteuid() == 0 {
		t.Skip("root writes in any directory")
	}

	dir := t.TempDir()
	path := write(t, dir, provedContents)

	err := os.Chmod(dir, 0o500)
	if err != nil {
		t.Fatalf("making the directory read-only: %v", err)
	}

	t.Cleanup(func() { _ = os.Chmod(dir, 0o700) })

	// Act
	err = config.ProveWritable(path)

	// Assert
	if !errors.Is(err, fs.ErrPermission) {
		t.Errorf("ProveWritable = %v, want the directory's refusal", err)
	}
}

func TestProveWritableFollowsALinkToItsTargetsDirectory(t *testing.T) {
	t.Parallel()

	// Arrange
	target := write(t, t.TempDir(), provedContents)
	linkDir := t.TempDir()
	link := filepath.Join(linkDir, config.FileName)

	err := os.Symlink(target, link)
	if err != nil {
		t.Skipf("making a link: %v", err)
	}

	err = os.Chmod(linkDir, 0o500)
	if err != nil {
		t.Fatalf("making the link's directory read-only: %v", err)
	}

	t.Cleanup(func() { _ = os.Chmod(linkDir, 0o700) })

	targetAged, linkAged := agedDirectory(t, filepath.Dir(target)), agedDirectory(t, linkDir)

	// Act
	err = config.ProveWritable(link)
	// Assert
	if err != nil {
		t.Errorf("ProveWritable = %v, want the target's directory proved, whatever the link's allows", err)
	}

	targetDir := filepath.Dir(target)
	if names := entries(t, targetDir); len(names) != 1 || !modifiedSince(t, targetDir, targetAged) {
		t.Errorf("the target's directory holds %q, want a probe made beside the target and removed", names)
	}

	if modifiedSince(t, linkDir, linkAged) {
		t.Error("the link's directory changed, want the probe made beside the target, not the link")
	}
}

func TestProveWritableRefusesALinkThatLoops(t *testing.T) {
	t.Parallel()

	// Arrange
	dir := t.TempDir()
	path := filepath.Join(dir, config.FileName)

	err := os.Symlink(path, path)
	if err != nil {
		t.Skipf("making a link: %v", err)
	}

	// Act
	err = config.ProveWritable(path)

	// Assert
	if err == nil {
		t.Error("ProveWritable = nil, want a link that loops refused")
	}
}

func TestProveWritableRefusesAPathThatNamesNoFile(t *testing.T) {
	t.Parallel()

	// Act
	err := config.ProveWritable("")

	// Assert
	if !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("ProveWritable(\"\") = %v, want no file named", err)
	}
}
