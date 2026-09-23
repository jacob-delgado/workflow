// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

import (
	"fmt"
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
			width: 80, kept: []string{changeStatusHint, "b branch for PROJ-412"}, cut: []string{"w log work"},
		},
		"with the rail folded": {
			width: 40, kept: []string{"enter read issue"}, cut: []string{changeStatusHint},
		},
		"with room for ? alone": {width: 12, kept: nil, cut: []string{"enter read"}},
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

func TestACutFooterEndsOnAWholeKeyAndAnEllipsis(t *testing.T) {
	t.Parallel()

	// Each screen's footer is cut somewhere across these widths: where it is,
	// whole keys are dropped and an ellipsis says so, never half a key or a
	// dangling separator; where it is not, its last key ends the row.
	screens := map[string]struct {
		keys []string
		last string
	}{
		"the Review pane":        {keys: []string{"4"}, last: "q quit"},
		"the Reviews pane":       {keys: []string{"6"}, last: "q quit"},
		"the messaging preview":  {keys: []string{"5", "p"}, last: "esc discard"},
		"a commit being written": {keys: []string{"3", "c"}, last: "esc close"},
	}

	for name, screen := range screens {
		for width := 30; width <= 130; width++ {
			t.Run(fmt.Sprintf("%s at %d columns", name, width), func(t *testing.T) {
				t.Parallel()

				// Arrange
				choosing := newWorld()
				choosing.cfg.Messaging.Channels = []string{"#dev", "#releases"}

				// Act
				footer := strings.TrimRight(footerLine(typing(t, choosing.live(t, width, 30), screen.keys...).View().Content), " ")

				// Assert
				if !strings.HasSuffix(footer, " …") && !strings.HasSuffix(footer, screen.last) {
					t.Errorf("the footer ends %q, neither on its last key nor on an ellipsis", footer)
				}
			})
		}
	}
}

func TestTheAnnouncementPreviewKeepsItsWayOutOnANarrowTerminal(t *testing.T) {
	t.Parallel()

	// Act
	// A pull request ready for review, with CI, to one channel: every key the
	// preview offers, down to the one that backs out before anything is sent.
	footer := footerLine(typing(t, newWorld().live(t, 62, 30), "5", "p").View().Content)

	// Assert
	requireScreen(t, footer, "enter announce now", "w when CI passes", "e edit", "esc discard")
}
