// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package store_test

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/jacob-delgado/workflow/internal/store"
)

// The platforms Dir is asked about, by their GOOS names.
const (
	darwin  = "darwin"
	windows = "windows"
	linux   = "linux"
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
			goos: darwin, want: filepath.Join(home, "Library", "Application Support", "workflow"),
		},
		"Windows uses AppData": {
			goos: windows, env: map[string]string{"AppData": `C:\Users\dev\AppData\Roaming`},
			want: filepath.Join(`C:\Users\dev\AppData\Roaming`, "workflow"),
		},
		"Linux prefers XDG_STATE_HOME": {
			goos: linux, env: map[string]string{"XDG_STATE_HOME": "/home/dev/.state"},
			want: filepath.Join("/home/dev/.state", "workflow"),
		},
		"Linux falls back to ~/.local/state": {
			goos: linux, want: filepath.Join(home, ".local", "state", "workflow"),
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

	// No home, and no environment variable to derive a directory from.
	cases := map[string]struct {
		goos string
		env  map[string]string
	}{
		"macOS with no home":                 {goos: darwin, env: nil},
		"Windows with no AppData":            {goos: windows, env: nil},
		"Windows with an empty AppData":      {goos: windows, env: map[string]string{"AppData": ""}},
		"Linux with no XDG_STATE_HOME":       {goos: linux, env: nil},
		"Linux with an empty XDG_STATE_HOME": {goos: linux, env: map[string]string{"XDG_STATE_HOME": ""}},
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
			_, err := store.Dir(testCase.goos, "", lookup)

			// Assert
			if !errors.Is(err, store.ErrNoDir) {
				t.Errorf("Dir(%s) with no home = %v, want ErrNoDir", testCase.goos, err)
			}
		})
	}
}
