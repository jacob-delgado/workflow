// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

import (
	"testing"

	"github.com/jacob-delgado/workflow/internal/jira"
)

func TestAKnownErrorReadsAsASentenceAndAnUnknownOneAsRawText(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		err  error
		want string
	}{
		"a known sentinel becomes a sentence": {
			err:  jira.ErrUnreachable,
			want: "Jira did not answer within 10 seconds",
		},
		"an unknown error keeps its raw text": {
			err:  errUnreadable,
			want: "permission denied",
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			failing := newWorld()
			failing.detailErr = tt.err

			// Act
			view := failing.live(t, 120, 40).View()

			// Assert
			requireScreen(t, view, tt.want)
		})
	}
}
