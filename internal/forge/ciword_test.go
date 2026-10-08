// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package forge_test

import (
	"testing"

	"github.com/jacob-delgado/workflow/internal/forge"
)

func TestEachCIStateHasTheOneWordEverySurfaceWritesItIn(t *testing.T) {
	t.Parallel()

	// The review filter, the command line's status and the API all write CI in
	// these words, so they are the forge's to say.
	const (
		noneWord   = "none"
		failedWord = "failed"
	)

	cases := map[forge.CIState]string{
		forge.CINone: noneWord, forge.CIRunning: "running", forge.CIPassed: "passed", forge.CIFailed: failedWord,
	}

	for state, want := range cases {
		t.Run(want, func(t *testing.T) {
			t.Parallel()

			// Act & Assert
			if got := state.Word(); got != want {
				t.Errorf("Word() = %q, want %q", got, want)
			}
		})
	}
}
