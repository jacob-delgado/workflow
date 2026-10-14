// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package progress_test

import (
	"slices"
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

func TestTheInFlightMarkTurnsFromItsOwnGlyph(t *testing.T) {
	t.Parallel()

	// The terminal's panes and the command line's live line turn the same
	// mark; ASCII has no shape that turns, so its stand-in holds still.
	cases := map[string]struct {
		ascii bool
		want  []string
	}{
		"in Unicode": {ascii: false, want: []string{"◐", "◓", "◑", "◒"}},
		"in ASCII":   {ascii: true, want: []string{"*"}},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Act
			frames := progress.Turn(tt.ascii)

			// Assert
			if !slices.Equal(frames, tt.want) || frames[0] != progress.InFlight.Glyph(tt.ascii) {
				t.Errorf("Turn(%v) = %q, want %q, from the in-flight glyph", tt.ascii, frames, tt.want)
			}
		})
	}
}
