// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package cli_test

// The Repositories pane switches directory by ending the interface and
// asking for the next one to open elsewhere; the root command opens it there,
// wired to that directory, or, when it cannot, reopens where it was and says
// why.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/jacob-delgado/workflow/internal/tui"
)

// twoDirectories are a directory to start in and one to switch to, each named
// so the interface's top row tells them apart.
func twoDirectories(t *testing.T) (string, string) {
	t.Helper()

	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	start, other := filepath.Join(root, "start"), filepath.Join(root, "elsewhere")
	for _, dir := range []string{start, other} {
		err = os.Mkdir(dir, 0o750)
		if err != nil {
			t.Fatal(err)
		}
	}

	return start, other
}

// shown is an interface's screen as the user would read it.
func shown(model tui.Model) string {
	return ansi.Strip(model.View().Content)
}

func TestASwitchOpensTheInterfaceAgainWhereItWasAskedToGo(t *testing.T) {
	// Arrange
	start, other := twoDirectories(t)

	// Act
	ran := runRootSwitching(t, place{dir: start, home: t.TempDir()}, []tui.Next{{Dir: other}})

	// Assert
	if ran.err != nil || ran.interfaces != 2 {
		t.Fatalf("workflow = %v, opened %d interfaces; want two", ran.err, ran.interfaces)
	}

	if spine := ran.spine(); !strings.Contains(spine, "elsewhere") {
		t.Errorf("the second interface's top row = %q, want it working in elsewhere", spine)
	}

	if !strings.Contains(shown(ran.model), "switched to") {
		t.Errorf("the second interface does not say it switched:\n%s", shown(ran.model))
	}
}

func TestASwitchMovesTheWorkingDirectory(t *testing.T) {
	// Arrange
	start, other := twoDirectories(t)

	// Act
	runRootSwitching(t, place{dir: start, home: t.TempDir()}, []tui.Next{{Dir: other}})

	// Assert
	// Whatever the new session runs — an editor, a hook — starts there too.
	here, err := os.Getwd()
	if err != nil || here != other {
		t.Errorf("working directory = %q, %v; want %s", here, err, other)
	}
}

func TestASwitchThatCannotBeMadeStaysWhereItWasAndSaysWhy(t *testing.T) {
	cases := map[string]func(t *testing.T, other string) string{
		"a directory not there": func(_ *testing.T, other string) string {
			return filepath.Join(other, "gone")
		},
		"keys the interface cannot use there": func(t *testing.T, other string) string {
			t.Helper()

			err := os.WriteFile(filepath.Join(other, ".workflow.json"),
				[]byte(`{"ui": {"keys": {"no-such-action": "x"}}}`), 0o600)
			if err != nil {
				t.Fatal(err)
			}

			return other
		},
	}

	for name, target := range cases {
		t.Run(name, func(t *testing.T) {
			// Arrange
			start, other := twoDirectories(t)
			destination := target(t, other)

			// Act
			ran := runRootSwitching(t, place{dir: start, home: t.TempDir()}, []tui.Next{{Dir: destination}})

			// Assert
			if ran.err != nil || ran.interfaces != 2 {
				t.Fatalf("workflow = %v, opened %d interfaces; want the first reopened", ran.err, ran.interfaces)
			}

			if spine := ran.spine(); !strings.Contains(spine, "start") {
				t.Errorf("the reopened interface's top row = %q, want it still in start", spine)
			}

			if !strings.Contains(shown(ran.model), "could not switch to") {
				t.Errorf("the reopened interface does not say why:\n%s", shown(ran.model))
			}

			here, err := os.Getwd()
			if err != nil || here != start {
				t.Errorf("working directory = %q, %v; want it left in %s", here, err, start)
			}
		})
	}
}
