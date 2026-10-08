// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package wiring_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/proc"
	"github.com/jacob-delgado/workflow/internal/store"
	"github.com/jacob-delgado/workflow/internal/tui"
	"github.com/jacob-delgado/workflow/internal/wiring"
)

// nothingOnPath is an Environment whose PATH holds no program at all, though
// this process's own PATH has git.
func nothingOnPath(t *testing.T) wiring.Environment {
	t.Helper()

	return wiring.Environment{
		Home:     t.TempDir(),
		Getenv:   func(string) string { return "" },
		StateDir: func() (string, error) { return "", store.ErrNoDir },
		LookPath: func(name string) (string, error) { return "", fmt.Errorf("%w: %s", proc.ErrNotFound, name) },
	}
}

// programRun is one way the wiring runs a program, over env and the seams it
// wired.
type programRun func(t *testing.T, env wiring.Environment, deps tui.Deps) error

// programRuns are the ways the wiring runs a program: a read, a streamed run,
// a run by name and a captured program.
func programRuns() map[string]programRun {
	return map[string]programRun{
		"a read": func(_ *testing.T, _ wiring.Environment, deps tui.Deps) error {
			_, err := deps.Git.Branch()

			return err
		},
		"a streamed run": func(_ *testing.T, _ wiring.Environment, deps tui.Deps) error {
			_, err := deps.Git.Amend()

			return err
		},
		"a run by name": func(t *testing.T, env wiring.Environment, _ tui.Deps) error {
			t.Helper()

			_, err := env.Run(t.Context(), "git", "--version")

			return err
		},
		"a captured program": func(t *testing.T, env wiring.Environment, _ tui.Deps) error {
			t.Helper()

			_, err := env.CaptureWithin(t.Context(), 0, proc.Command{Name: "git"}, nil)

			return err
		},
	}
}

func TestEveryProgramIsFoundOnTheEnvironmentsPathAlone(t *testing.T) {
	for name, run := range programRuns() {
		t.Run(name, func(t *testing.T) {
			// Arrange
			env := nothingOnPath(t)
			where := wiring.Workspace{Root: repository(t), Remote: "", Dir: "", Repository: true}
			deps, _ := env.Deps(t.Context(), config.Default(), where, nil)

			// Act
			err := run(t, env, deps)

			// Assert
			if !errors.Is(err, proc.ErrNotFound) {
				t.Errorf("%s with no git on the environment's PATH = %v, want %v", name, err, proc.ErrNotFound)
			}
		})
	}
}

func TestAProgramIsAvailableOnlyWhereTheEnvironmentFindsIt(t *testing.T) {
	// Act
	available := nothingOnPath(t).Available("git")

	// Assert
	if available {
		t.Error("git is available though the environment's PATH holds no program")
	}
}
