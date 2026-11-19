// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package store

import (
	"os"
	"path"
	"path/filepath"
	"runtime"
)

// DefaultDir is the OS-native directory workflow keeps its store in, from the
// running machine's home and environment.
func DefaultDir() (string, error) {
	home, _ := os.UserHomeDir()

	return Dir(runtime.GOOS, home, os.LookupEnv)
}

// Dir is the OS-native store directory for a platform, home and environment,
// taken as arguments so both platforms' paths can be exercised from one
// machine: ~/Library/Application Support/workflow on macOS, and
// $XDG_STATE_HOME or ~/.local/state/workflow elsewhere. A variable that is
// not an absolute path is ignored, as the XDG specification says to: joined,
// it would put the store wherever workflow happened to start.
func Dir(goos, home string, lookupEnv func(string) (string, bool)) (string, error) {
	switch goos {
	case "darwin":
		if home == "" {
			return "", ErrNoDir
		}

		return filepath.Join(home, "Library", "Application Support", "workflow"), nil
	default:
		if state, ok := lookupEnv("XDG_STATE_HOME"); ok && path.IsAbs(state) {
			return filepath.Join(state, "workflow"), nil
		}

		if home == "" {
			return "", ErrNoDir
		}

		return filepath.Join(home, ".local", "state", "workflow"), nil
	}
}
