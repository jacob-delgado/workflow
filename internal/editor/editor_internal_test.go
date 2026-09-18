// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package editor

// defaultEditor picks the fallback by platform, which chosen reads from
// runtime.GOOS. Testing both branches from one machine means calling it with the
// platform as an argument, which is what this reaches for.

import "testing"

func TestDefaultEditorIsPlatformSpecific(t *testing.T) {
	t.Parallel()

	cases := map[string]string{
		"windows": "notepad",
		"linux":   "vi",
		"darwin":  "vi",
	}

	for goos, want := range cases {
		t.Run(goos, func(t *testing.T) {
			t.Parallel()

			// Act & Assert
			if got := defaultEditor(goos); got != want {
				t.Errorf("defaultEditor(%q) = %q, want %q", goos, got, want)
			}
		})
	}
}
