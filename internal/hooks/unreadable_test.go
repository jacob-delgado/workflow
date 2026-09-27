// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package hooks_test

import (
	"fmt"
	"io/fs"
	"testing"
	"testing/fstest"

	"github.com/jacob-delgado/workflow/internal/hooks"
)

// unreadableHooks is a hooks directory that lists one of its files, runnable,
// but will not open it, as a hook only its owner may read looks to anyone else.
type unreadableHooks struct {
	fstest.MapFS

	unreadable string
}

var _ fs.ReadFileFS = unreadableHooks{}

// ReadFile refuses the unreadable file and reads any other.
func (dir unreadableHooks) ReadFile(name string) ([]byte, error) {
	if name == dir.unreadable {
		return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrPermission}
	}

	contents, err := dir.MapFS.ReadFile(name)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", name, err)
	}

	return contents, nil
}

func TestAHookThatCannotBeReadIsLeftOut(t *testing.T) {
	t.Parallel()

	// Arrange
	dir := unreadableHooks{
		MapFS: fstest.MapFS{
			preCommit: executable(simplePreCommit),
			prePush:   executable("#!/bin/sh\ngo test ./...\n"),
		},
		unreadable: preCommit,
	}

	// Act
	found := hooks.ExistingHooks(dir, "linux")

	// Assert
	if len(found) != 1 || found[0].Name != prePush {
		t.Errorf("ExistingHooks = %+v, want only the pre-push hook it could read", found)
	}
}
