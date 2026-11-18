// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// Saved is a write a save made, with what its file held before, so the write
// can be taken back: by a Settings save whose keychain or Slack then refuses,
// and by a first run whose keychain refuses the token the file reads.
type Saved struct {
	files  Files
	before layer
	at     Revision
}

// Revision is the revision of the files the write left.
func (s Saved) Revision() Revision {
	return s.at
}

// targetOf is the layer of the file files saves to, of home and repo as read.
func targetOf(files Files, home, repo layer) layer {
	if files.Repo != "" {
		return repo
	}

	return home
}

// TakeBack takes the write back, while the files are still at the revision it
// left: the file it saved to holds what it held before again, written as every
// save writes, at FileMode and owned by the user running this, or, where there
// was no file, the one the write made goes from where the write followed its
// links to. Files changed since are left as they are, refused with
// ErrChangedOnDisk. It holds locks from the revision check through the write,
// as the save did.
func (s Saved) TakeBack(locks SaveLocks) error {
	unlock, err := locks.Hold(s.files)
	if err != nil {
		return fmt.Errorf("taking the write back: %w", err)
	}
	defer unlock()

	now, err := RevisionOfLayers(s.files)
	if err != nil || now != s.at {
		return fmt.Errorf("taking the write back: %s: %w", s.files, ErrChangedOnDisk)
	}

	if s.before.exists {
		return writePrivate(s.files.Target(), s.before.contents)
	}

	made, err := filepath.EvalSymlinks(s.files.Target())

	return errors.Join(err, os.Remove(made))
}

// ReplaceFile writes cfg to a file of its own at path, at FileMode, replacing
// whatever is there, or the file a link there points at, and returns the write
// with what that file held before, so the write can be taken back. The new
// file is made in the directory of the file it replaces, so that directory
// must be writable too. It holds the file's save lock from its read through
// the rename, so a save another workflow makes meanwhile is waited for, as
// every save is.
func ReplaceFile(locks SaveLocks, path string, cfg Config) (Saved, error) {
	files := Files{Home: path}

	unlock, err := locks.Hold(files)
	if err != nil {
		return Saved{}, err
	}
	defer unlock()

	before, err := readAsFound(path)
	if err != nil {
		return Saved{}, err
	}

	err = writePrivate(path, indented(cfg))
	if err != nil {
		return Saved{}, fmt.Errorf("writing %s: %w", path, err)
	}

	at, err := RevisionOfLayers(files)

	return Saved{files: files, before: before, at: at}, err
}

// readAsFound is the file at path as it is, through its links, whoever could
// have written it, since a file replaced is given back as it was; a file not
// there is a layer that does not exist.
func readAsFound(path string) (layer, error) {
	//nolint:gosec // the path is the user's own config file, by design
	contents, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return layer{path: path}, nil
	}

	if err != nil {
		return layer{}, fmt.Errorf("reading %s: %w", path, err)
	}

	return layer{path: path, contents: contents, exists: true}, nil
}
