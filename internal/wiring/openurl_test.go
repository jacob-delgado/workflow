// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package wiring_test

// The OpenURL seam hands a check's address to the platform's own browser
// opener, but only a web address: a value a program could read as a flag, or one
// naming a local file, is refused before any opener runs.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/wiring"
)

// recordingOpeners installs a stand-in for every platform browser opener — open,
// xdg-open and cmd — on PATH, each recording the arguments it was handed, and
// returns the path they record to. Whichever opener this platform reaches, the
// test can then see what OpenURL passed it.
func recordingOpeners(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()
	record := filepath.Join(dir, "opened")
	script := "#!/bin/sh\nprintf '%s\\n' \"$@\" >> \"" + record + "\"\n"

	for _, name := range []string{"open", "xdg-open", "cmd"} {
		write(t, filepath.Join(dir, name), script, 0o755)
	}

	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))

	return record
}

func TestOpenURLOpensOnlyWebAddresses(t *testing.T) {
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
			// Arrange
			record := recordingOpeners(t)
			open := wiring.Deps(t.Context(), config.Default(), wiring.Workspace{Root: t.TempDir(), Remote: ""}, nil).OpenURL

			// Act
			err := open(tt.raw)

			// Assert
			opened, _ := os.ReadFile(record)

			if tt.ok && (err != nil || !strings.Contains(string(opened), tt.raw)) {
				t.Errorf("OpenURL(%q) = %v, opener saw %q; want it opened with the URL", tt.raw, err, opened)
			}

			if !tt.ok && (err == nil || !strings.Contains(err.Error(), "non-http(s)") || len(opened) != 0) {
				t.Errorf("OpenURL(%q) = %v, opener saw %q; want it refused and nothing run", tt.raw, err, opened)
			}
		})
	}
}
