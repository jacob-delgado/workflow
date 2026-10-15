// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package console_test

import (
	"slices"
	"strings"
	"testing"
	"time"
	"unicode"

	"github.com/charmbracelet/x/ansi"

	"github.com/jacob-delgado/workflow/internal/cli/console"
)

// tooLong is what a wait is doing, in more words than an 80-column line holds.
const tooLong = "Reading every repository under the directory named on the command line, " +
	"one after another, until each has answered"

// asciiMarks are the frames the live line's mark turns through in ASCII.
func asciiMarks() []string {
	return []string{"|", "/", "-", `\`}
}

// markedBefore reports whether shown draws text behind one of marks, the
// frame the line's mark had turned to.
func markedBefore(shown string, marks []string, text string) bool {
	return slices.ContainsFunc(marks, func(mark string) bool { return strings.Contains(shown, mark+" "+text) })
}

// nonASCII reports whether shown holds a character outside ASCII.
func nonASCII(shown string) bool {
	return strings.ContainsFunc(shown, func(r rune) bool { return r > unicode.MaxASCII })
}

func TestALiveLineDrawsItsBarAndCutInTheCharactersItsLookAsks(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		look      console.Look
		marks     []string
		bar, cut  string
		asciiOnly bool
	}{
		"by default": {
			look: console.Look{}, marks: liveMarks(),
			bar: "Checking Jira ━━━━━━━━──────────────── 1 of 3", cut: "…",
		},
		"in ASCII": {
			look: console.Look{ASCII: true}, marks: asciiMarks(),
			bar: "Checking Jira ========---------------- 1 of 3", cut: "...", asciiOnly: true,
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Act
			printed, err := watchingAs(t, tt.look, func(terminal *standIn, wait console.Wait) error {
				wait.Doing("Checking Jira")
				wait.Counted(1, 3)
				terminal.shows(t, "1 of 3")
				wait.Doing(tooLong)
				terminal.shows(t, "Reading every")

				return nil
			})

			// Assert
			shown := ansi.Strip(printed)
			if err != nil || !markedBefore(shown, tt.marks, tt.bar) || !strings.Contains(shown, tt.cut) {
				t.Errorf("watched = %v, showing %q; want %q behind one of %q, and the long line cut with %q",
					err, shown, tt.bar, tt.marks, tt.cut)
			}

			if tt.asciiOnly && nonASCII(shown) {
				t.Errorf("showing %q, want ASCII alone", shown)
			}
		})
	}
}

func TestALiveLinesMarkTurnsOnlyWhereItsLookLetsItMove(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		look         console.Look
		drawn, never []string
	}{
		"by default": {look: console.Look{}, drawn: liveMarks(), never: asciiMarks()},
		"still":      {look: console.Look{Still: true}, drawn: []string{"◐"}, never: []string{"◓", "◑", "◒"}},
		"in ASCII": {
			look: console.Look{ASCII: true}, drawn: asciiMarks(), never: append(liveMarks(), "*"),
		},
		"still in ASCII": {
			look: console.Look{ASCII: true, Still: true}, drawn: []string{"*"}, never: asciiMarks(),
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Act
			printed, _ := watchingAs(t, tt.look, func(terminal *standIn, wait console.Wait) error {
				wait.Doing("Checking Jira")
				terminal.shows(t, "Checking Jira")
				// A second on the bubble's clock: four frames, where the
				// mark turns at all.
				time.Sleep(time.Second)
				terminal.shows(t, "Checking Jira")

				return nil
			})

			// Assert
			shown := ansi.Strip(printed)
			for _, mark := range tt.drawn {
				if !strings.Contains(shown, mark) {
					t.Errorf("showing %q, want the mark drawn as %q", shown, mark)
				}
			}

			for _, mark := range tt.never {
				if strings.Contains(shown, mark) {
					t.Errorf("showing %q, want no %q", shown, mark)
				}
			}
		})
	}
}
