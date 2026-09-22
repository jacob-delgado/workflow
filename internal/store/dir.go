// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package store

import (
	"os"
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
// taken as arguments so every platform's path can be exercised from one machine:
// %AppData%\workflow on Windows, ~/Library/Application Support/workflow on macOS,
// and $XDG_STATE_HOME or ~/.local/state/workflow elsewhere.
func Dir(goos, home string, lookupEnv func(string) (string, bool)) (string, error) {
	switch goos {
	case "windows":
		if appData, ok := lookupEnv("AppData"); ok && appData != "" {
			return filepath.Join(appData, "workflow"), nil
		}

		return "", ErrNoDir
	case "darwin":
		if home == "" {
			return "", ErrNoDir
		}

		return filepath.Join(home, "Library", "Application Support", "workflow"), nil
	default:
		if state, ok := lookupEnv("XDG_STATE_HOME"); ok && state != "" {
			return filepath.Join(state, "workflow"), nil
		}

		if home == "" {
			return "", ErrNoDir
		}

		return filepath.Join(home, ".local", "state", "workflow"), nil
	}
}
