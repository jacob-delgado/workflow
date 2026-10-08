// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package store_test

import (
	"strings"
	"testing"

	"github.com/jacob-delgado/workflow/internal/store"
)

func TestASizeIsShownInBytesKiBOrMiB(t *testing.T) {
	t.Parallel()

	cases := map[int64]string{0: "0 B", 1023: "1023 B", 1536: "1.5 KiB", 3 << 20: "3.0 MiB"}

	for size, want := range cases {
		t.Run(want, func(t *testing.T) {
			t.Parallel()

			// Act & Assert
			if got := store.HumanBytes(size); got != want {
				t.Errorf("HumanBytes(%d) = %q, want %q", size, got, want)
			}
		})
	}
}

func TestEachCleanSaysWhatGoesWithIt(t *testing.T) {
	t.Parallel()

	// Removing the cache loses only what a session makes again; removing
	// everything loses what was decided, which is asked for again.
	cases := map[string]struct {
		scope store.CleanScope
		says  string
	}{
		"the cache":  {scope: store.CleanCache, says: "made again as you work"},
		"everything": {scope: store.CleanAll, says: "will be asked again"},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Act & Assert
			if got := tt.scope.Consequence(); !strings.Contains(got, tt.says) {
				t.Errorf("Consequence = %q, want it to say %q", got, tt.says)
			}
		})
	}
}
