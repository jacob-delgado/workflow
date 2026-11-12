// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

//go:build linux

package hooks_test

// Write creates lefthook.yml only once every script it names is whole, so a
// write stopped at any point by workflow killed or crashing, where no
// rollback can follow, leaves either no configuration, and the offer to try
// again, or one whose scripts are all there. The kernel's own record of what
// was created and written in the repository, kept in the order it happened,
// shows the order; a power loss, with nothing synced, is not covered.

import (
	"encoding/binary"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"syscall"
	"testing"

	"github.com/jacob-delgado/workflow/internal/hooks"
)

// inotifyHeader is the size of the fixed part of an inotify event, before the
// name that follows it.
const inotifyHeader = 16

// fileEvents records, in order, the files created in one directory and the
// files written whole in another, by the kernel's inotify.
type fileEvents struct {
	descriptor int
	// says words each watch's events: "created" or "wrote".
	says map[int32]string
}

// watchFiles starts recording the files created in created and written whole
// in written, until the test ends.
func watchFiles(t *testing.T, created, written string) fileEvents {
	t.Helper()

	descriptor, err := syscall.InotifyInit1(syscall.IN_NONBLOCK | syscall.IN_CLOEXEC)
	if err != nil {
		t.Fatalf("starting inotify: %v", err)
	}

	t.Cleanup(func() { _ = syscall.Close(descriptor) })

	events := fileEvents{descriptor: descriptor, says: map[int32]string{}}

	for dir, mask := range map[string]uint32{created: syscall.IN_CREATE, written: syscall.IN_CLOSE_WRITE} {
		watch, err := syscall.InotifyAddWatch(descriptor, dir, mask)
		if err != nil {
			t.Fatalf("watching %s: %v", dir, err)
		}

		events.says[int32(watch)] = map[uint32]string{syscall.IN_CREATE: "created", syscall.IN_CLOSE_WRITE: "wrote"}[mask]
	}

	return events
}

// read is every event recorded so far, in order, as "created lefthook.yml" or
// "wrote pre-commit".
func (e fileEvents) read(t *testing.T) []string {
	t.Helper()

	buffer := make([]byte, 1<<16)

	count, err := syscall.Read(e.descriptor, buffer)
	if errors.Is(err, syscall.EAGAIN) {
		return nil
	}

	if err != nil {
		t.Fatalf("reading inotify: %v", err)
	}

	var said []string

	for rest := buffer[:count]; len(rest) >= inotifyHeader; {
		watch := int32(binary.NativeEndian.Uint32(rest[0:4]))
		nameLength := int(binary.NativeEndian.Uint32(rest[12:16]))
		name := string(rest[inotifyHeader : inotifyHeader+nameLength])

		said = append(said, e.says[watch]+" "+trimNUL(name))
		rest = rest[inotifyHeader+nameLength:]
	}

	return said
}

// trimNUL is an inotify name without the NUL bytes that pad it.
func trimNUL(name string) string {
	if end := slices.Index([]byte(name), 0); end >= 0 {
		return name[:end]
	}

	return name
}

func TestWriteCreatesTheConfigurationOnlyOnceEveryScriptIsWhole(t *testing.T) {
	t.Parallel()

	// Arrange
	dir := t.TempDir()
	scripts := filepath.Join(dir, ".lefthook", preCommit)

	err := os.MkdirAll(scripts, 0o750)
	if err != nil {
		t.Fatalf("making the scripts' directory: %v", err)
	}

	events := watchFiles(t, dir, scripts)
	generated := hooks.Generated{Config: shortConfig, Scripts: []hooks.File{
		{Path: ".lefthook/" + preCommit + "/" + preCommit, Contents: "#!/bin/sh\nmake lint\n"},
	}}

	// Act
	err = hooks.Write(dir, generated)
	// Assert
	if err != nil {
		t.Fatalf("Write returned %v, want nil", err)
	}

	said := events.read(t)
	wrote, created := slices.Index(said, "wrote "+preCommit), slices.Index(said, "created lefthook.yml")

	if wrote < 0 || created < wrote {
		t.Errorf("Write did %q, want the script written whole before lefthook.yml was created", said)
	}
}
