// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package store

import (
	"os"
	"path/filepath"
)

// Mode is what a store does with its files.
type Mode int

const (
	// ModeOff keeps nothing: every method no-ops, so persistence turned off
	// behaves exactly as no store at all. It is the zero mode.
	ModeOff Mode = iota
	// ModeReadOnly is a dry run's: it reads what is already on disk, reads
	// nothing where there is no store yet rather than make one, and no-ops
	// every write.
	ModeReadOnly
	// ModeWrites reads and records what it is told.
	ModeWrites
)

// Store points at the on-disk store. Its zero value is off.
type Store struct {
	dir  string
	mode Mode
}

// New points a store at dir in the mode given. A store with no directory has
// nowhere to keep anything, so it is off whatever the mode, as is one a user
// who opts out of persistence asks for: the code path stays the same and
// every method no-ops.
func New(dir string, mode Mode) Store {
	if dir == "" {
		return Store{dir: "", mode: ModeOff}
	}

	return Store{dir: dir, mode: mode}
}

// ReadOnly is the store for a dry run: ModeReadOnly over the same directory.
// A store that is off stays off.
func (s Store) ReadOnly() Store {
	if s.mode == ModeOff {
		return s
	}

	return Store{dir: s.dir, mode: ModeReadOnly}
}

// Keeps reports a store that keeps anything: one that is not off. A read-only
// store keeps what is already on disk, so it keeps.
func (s Store) Keeps() bool {
	return !s.off()
}

// Writes reports a store that records what it is told: ModeWrites, with
// somewhere to keep it.
func (s Store) Writes() bool {
	return !s.writesNothing()
}

// off reports a store that should do nothing.
func (s Store) off() bool {
	return s.mode == ModeOff
}

// writesNothing reports a store whose writes no-op: one that is off, or read-only.
func (s Store) writesNothing() bool {
	return s.mode != ModeWrites
}

// nothingToReadIn reports a store with nothing it may read from the database
// named: one that is off, or a read-only one with no such file on disk, which
// it must not create by opening.
func (s Store) nothingToReadIn(name string) bool {
	if s.mode != ModeReadOnly {
		return s.off()
	}

	_, err := os.Stat(filepath.Join(s.dir, name))

	return err != nil
}
