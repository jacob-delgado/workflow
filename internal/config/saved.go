// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// Saved is a write a save made, with what its file held before, so the write
// can be taken back: by a Settings save whose keychain or Slack then refuses.
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
