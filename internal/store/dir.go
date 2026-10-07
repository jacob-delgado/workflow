// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package store

import (
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"unicode"
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
// and $XDG_STATE_HOME or ~/.local/state/workflow elsewhere. A variable that is
// not an absolute path is ignored, as the XDG specification says to: joined,
// it would put the store wherever workflow happened to start.
func Dir(goos, home string, lookupEnv func(string) (string, bool)) (string, error) {
	switch goos {
	case "windows":
		if appData, ok := lookupEnv("AppData"); ok && windowsAbsolute(appData) {
			return filepath.Join(appData, "workflow"), nil
		}

		return "", ErrNoDir
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

// windowsAbsolute reports a Windows path that starts at the root of a named
// drive, or a UNC share, read the same whichever system Dir runs on.
func windowsAbsolute(name string) bool {
	if strings.HasPrefix(name, `\\`) {
		return true
	}

	drive, rest, named := strings.Cut(name, ":")
	if !named || len(drive) != 1 || !strings.HasPrefix(rest, `\`) && !strings.HasPrefix(rest, "/") {
		return false
	}

	letter := unicode.ToUpper(rune(drive[0]))

	return 'A' <= letter && letter <= 'Z'
}
