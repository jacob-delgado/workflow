// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package console_test

import (
	"slices"
	"strings"
	"testing"
	"testing/synctest"
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

// stillRun is what a wait watched in a still look came to: what it printed,
// what Watched returned, and the panic its bubble ended in, if any.
type stillRun struct {
	printed string
	err     error
	ended   any
}

// watchedStill watches wait in look, in a bubble that does not outlast the
// mark's next turn as watchingAs's does. synctest panics when the bubble's
// goroutine ends with another still blocked in it, as one waiting on the
// mark's timer is, and the run keeps that panic for the test to judge.
func watchedStill(t *testing.T, look console.Look, wait func(*standIn, console.Wait) error) stillRun {
	t.Helper()

	var run stillRun

	func() {
		defer func() { run.ended = recover() }()

		synctest.Test(t, func(*testing.T) {
			terminal := new(standIn)

			notes := console.New(terminal, console.Options{})
			run.err = notes.Watched(look, func(watched console.Wait) error { return wait(terminal, watched) })
			run.printed = terminal.String()
		})
	}()

	return run
}

func TestAStillLiveLineStartsNoTimerForItsMark(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		look   console.Look
		drawn  string
		turned []string
	}{
		"held in Unicode": {look: console.Look{Still: true}, drawn: "◐ Checking Jira", turned: []string{"◓", "◑", "◒"}},
		"held in ASCII": {
			look: console.Look{ASCII: true, Still: true}, drawn: "* Checking Jira", turned: asciiMarks(),
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Act
			run := watchedStill(t, tt.look, func(terminal *standIn, wait console.Wait) error {
				wait.Doing("Checking Jira")
				terminal.shows(t, "Checking Jira")
				// Three seconds on the bubble's clock: twelve frames, had the
				// mark a timer to turn on.
				time.Sleep(3 * time.Second)
				terminal.shows(t, "Checking Jira")

				return nil
			})

			// Assert
			if run.ended != nil || run.err != nil {
				t.Errorf("watched = %v, its bubble ending in %v; want nil, and no panic, as a timer the "+
					"line left waiting makes it", run.err, run.ended)
			}

			shown := ansi.Strip(run.printed)
			if !strings.Contains(shown, tt.drawn) || strings.ContainsAny(shown, strings.Join(tt.turned, "")) {
				t.Errorf("showing %q, want %q alone, never turned", shown, tt.drawn)
			}
		})
	}
}
