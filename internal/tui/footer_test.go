// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

import (
	"strings"
	"testing"
)

func TestANarrowFooterGivesUpThePanesVerbsBeforeTheWayToEveryKey(t *testing.T) {
	t.Parallel()

	// The Issues pane offers more verbs than any of these widths holds beside
	// ? keys, so its last verbs give way rather than the one key that lists the
	// rest — down to none of them where ? alone fits.
	cases := map[string]struct {
		width int
		kept  []string
		cut   []string
	}{
		"beside the rail": {
			width: 80, kept: []string{"t change status", "b branch for PROJ-412"}, cut: []string{"r refresh"},
		},
		"with the rail folded": {
			width: 40, kept: []string{"t change status", "c comment"}, cut: []string{"b branch for"},
		},
		"with room for ? alone": {width: 12, kept: nil, cut: []string{"t change"}},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Act
			footer := strings.TrimRight(footerLine(newWorld().live(t, tt.width, 30).View().Content), " ")

			// Assert
			requireScreen(t, footer, append(tt.kept, "? keys …")...)
			refuseScreen(t, footer, append(tt.cut, "tab next pane")...)
		})
	}
}
