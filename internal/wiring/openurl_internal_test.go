// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package wiring

// browserCommand chooses a platform's opener; the choice cannot be reached
// black-box because runtime.GOOS is fixed for a run. This test lives in package
// wiring — testpackage's default skip covers *_internal_test.go — to hand it
// each platform in turn.

import (
	"errors"
	"slices"
	"testing"
)

func TestBrowserCommandForEachPlatform(t *testing.T) {
	t.Parallel()

	const url = "https://ci.example.com/checks/42"

	cases := map[string]struct {
		goos string
		name string
		args []string
	}{
		"macOS":   {goos: "darwin", name: "open", args: []string{url}},
		"Windows": {goos: "windows", name: "cmd", args: []string{"/c", "start", "", url}},
		"Linux":   {goos: "linux", name: "xdg-open", args: []string{url}},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Act
			command, args := browserCommand(tt.goos, url)

			// Assert
			if command != tt.name || !slices.Equal(args, tt.args) {
				t.Errorf("browserCommand(%q) = %q %q, want %q %q", tt.goos, command, args, tt.name, tt.args)
			}
		})
	}
}

// The URL a check carries comes from the forge, so the opener takes only what a
// browser understands and never a value a program could read as a flag.
func TestBrowserOnlyOpensWebURLs(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		raw string
		ok  bool
	}{
		"http":       {raw: "http://ci.example.com/checks/42", ok: true},
		"https":      {raw: "https://ci.example.com/checks/42", ok: true},
		"flag":       {raw: "-rf", ok: false},
		"file":       {raw: "file:///etc/passwd", ok: false},
		"javascript": {raw: "javascript:alert(1)", ok: false},
		"empty":      {raw: "", ok: false},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Act
			got, err := safeBrowserURL(tt.raw)

			// Assert
			if tt.ok && (err != nil || got != tt.raw) {
				t.Errorf("safeBrowserURL(%q) = %q, %v; want it allowed unchanged", tt.raw, got, err)
			}

			if !tt.ok && !errors.Is(err, errUnsafeBrowserURL) {
				t.Errorf("safeBrowserURL(%q) = %q, %v; want it refused", tt.raw, got, err)
			}
		})
	}
}
