// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

//go:build unix

package fileowner_test

import (
	"io/fs"
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"

	"github.com/jacob-delgado/workflow/internal/fileowner"
)

func TestOfNamesTheUserWhoMadeTheFile(t *testing.T) {
	t.Parallel()

	// Arrange
	path := filepath.Join(t.TempDir(), "made")

	err := os.WriteFile(path, nil, 0o600)
	if err != nil {
		t.Fatalf("making the file: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("reading the file's mode: %v", err)
	}

	// Act
	owner, known := fileowner.Of(info)

	// Assert
	if !known || owner.User != os.Geteuid() {
		t.Errorf("Of = %+v, %t; want the file owned by user %d", owner, known, os.Geteuid())
	}
}

func TestOfKnowsNoOwnerOfAFileTheSystemDidNotDescribe(t *testing.T) {
	t.Parallel()

	// Arrange
	info, err := fs.Stat(fstest.MapFS{"made": {Data: nil}}, "made")
	if err != nil {
		t.Fatalf("reading the file's mode: %v", err)
	}

	// Act
	_, known := fileowner.Of(info)

	// Assert
	if known {
		t.Error("Of knew an owner of a file no system described")
	}
}
