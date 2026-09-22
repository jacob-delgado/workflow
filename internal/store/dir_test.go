// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package store_test

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/jacob-delgado/workflow/internal/store"
)

func TestDirIsOSNative(t *testing.T) {
	t.Parallel()

	const home = "/home/dev"

	cases := map[string]struct {
		goos string
		env  map[string]string
		want string
	}{
		"macOS uses Application Support": {
			goos: "darwin", want: filepath.Join(home, "Library", "Application Support", "workflow"),
		},
		"Windows uses AppData": {
			goos: "windows", env: map[string]string{"AppData": `C:\Users\dev\AppData\Roaming`},
			want: filepath.Join(`C:\Users\dev\AppData\Roaming`, "workflow"),
		},
		"Linux prefers XDG_STATE_HOME": {
			goos: "linux", env: map[string]string{"XDG_STATE_HOME": "/home/dev/.state"},
			want: filepath.Join("/home/dev/.state", "workflow"),
		},
		"Linux falls back to ~/.local/state": {
			goos: "linux", want: filepath.Join(home, ".local", "state", "workflow"),
		},
	}

	for name, testCase := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			lookup := func(key string) (string, bool) {
				value, ok := testCase.env[key]

				return value, ok
			}

			// Act
			got, err := store.Dir(testCase.goos, home, lookup)

			// Assert
			if err != nil || got != testCase.want {
				t.Errorf("Dir(%s) = %q, %v; want %q", testCase.goos, got, err, testCase.want)
			}
		})
	}
}

func TestDirNeedsSomewhereToPutIt(t *testing.T) {
	t.Parallel()

	// Arrange
	// No home and no environment variable to derive a directory from.
	noEnv := func(string) (string, bool) { return "", false }

	// Act
	_, err := store.Dir("darwin", "", noEnv)

	// Assert
	if !errors.Is(err, store.ErrNoDir) {
		t.Errorf("Dir with no home = %v, want ErrNoDir", err)
	}
}
