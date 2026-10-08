// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package progress_test

import (
	"testing"

	"github.com/jacob-delgado/workflow/internal/progress"
)

func TestEachStateHasOneMarkAndOneASCIIStandIn(t *testing.T) {
	t.Parallel()

	// The terminal's spine and the command line's status line draw a stage
	// with the same mark, and ui.ascii swaps each for the same stand-in.
	cases := map[string]struct {
		state         progress.State
		mark, inASCII string
	}{
		"not started": {state: progress.NotStarted, mark: "○", inASCII: "o"},
		"in flight":   {state: progress.InFlight, mark: "◐", inASCII: "*"},
		"done":        {state: progress.Done, mark: "●", inASCII: "#"},
		"failed":      {state: progress.Failed, mark: "✗", inASCII: "x"},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Act
			mark, inASCII := tt.state.Glyph(false), tt.state.Glyph(true)

			// Assert
			if mark != tt.mark || inASCII != tt.inASCII {
				t.Errorf("Glyph = %q, and %q in ASCII; want %q and %q", mark, inASCII, tt.mark, tt.inASCII)
			}
		})
	}
}
