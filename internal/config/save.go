// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"runtime"
)

// Save writes the configuration to path at FileMode.
func Save(path string, cfg Config) error {
	encoded, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("encoding configuration: %w", err)
	}

	err = writePrivate(path, append(encoded, '\n'))
	if err != nil {
		return fmt.Errorf("writing %s: %w", path, err)
	}

	return nil
}

// writePrivate writes a file only its owner can reach. Asking for FileMode when
// opening it is not enough: that is honored for a file being created, and one
// that already exists keeps the mode it had. So the mode is set on the open
// file, before anything is written into it.
func writePrivate(path string, contents []byte) error {
	//nolint:gosec // the path is the user's own config file, by design
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, FileMode)
	if err != nil {
		return fmt.Errorf("opening it: %w", err)
	}

	err = file.Chmod(FileMode)
	if err == nil {
		_, err = file.Write(contents)
	}

	return errors.Join(err, file.Close())
}

// othersMask is the permission bits that belong to anyone but a file's owner.
const othersMask os.FileMode = 0o077

// SharedMode reports the permissions of a file that someone other than its owner
// can read or write, which a file holding credentials must not be.
func SharedMode(path string) (os.FileMode, bool) {
	// Windows keeps no such bits, and Go reports 0666 for every file there,
	// which says nothing about who can read it.
	if runtime.GOOS == "windows" {
		return 0, false
	}

	info, err := os.Stat(path)
	if err != nil {
		return 0, false
	}

	mode := info.Mode().Perm()

	return mode, mode&othersMask != 0
}
