// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/spf13/cobra"

	"github.com/jacob-delgado/workflow/internal/proc"
	"github.com/jacob-delgado/workflow/internal/wiring"
)

// Environment is what a command reads of the process it runs in: the
// directory it was run from and how to move the process to another, and what
// the wiring reads — the home directory, the environment's variables, the
// store's directory and where a program is found. Execute's caller builds it
// from the operating system, so no command reads the process's own, and a
// test hands each run one of its own.
type Environment struct {
	// WorkingDir is the directory the command was run from, or why the
	// operating system cannot name it. A command reads it once, whatever
	// reads it after: its sections, its request log and each directory it
	// names relatively are all read from the one directory.
	WorkingDir func() (string, error)
	// Chdir makes dir the process's working directory, so whatever a session
	// switched there runs — an editor, a hook — starts there.
	Chdir func(dir string) error
	// Process is what the wiring reads of the process.
	Process wiring.Environment
}

// environmentKey is what a command's context keeps its Environment under.
type environmentKey struct{}

// handOver puts env in the context of the command cmd runs, which every
// command under the root reads it from: the root does this before any
// command's own work starts. The working directory is read at the first ask
// and answered from then on as it was then.
func handOver(cmd *cobra.Command, env Environment) {
	env.WorkingDir = sync.OnceValues(env.WorkingDir)
	cmd.SetContext(context.WithValue(cmd.Context(), environmentKey{}, env))
}

// errNoEnvironment is a command run without the environment it runs in: from
// a tree built only to walk, or under a persistent hook of its own, which
// keeps the root's from handing it over.
var errNoEnvironment = errors.New("the command was run without the environment it runs in")

// environmentOf is the Environment the command cmd runs in, or, when none was
// handed over, one that answers every question with errNoEnvironment.
func environmentOf(cmd *cobra.Command) Environment {
	env, handed := cmd.Context().Value(environmentKey{}).(Environment)
	if !handed {
		return noEnvironment()
	}

	return env
}

// noEnvironment is the environment of a command that has none: it names no
// directory, home or variable, finds no program, and moves nowhere.
func noEnvironment() Environment {
	return Environment{
		WorkingDir: func() (string, error) { return "", errNoEnvironment },
		Chdir:      func(string) error { return errNoEnvironment },
		Process: wiring.Environment{
			Home:     "",
			Getenv:   func(string) string { return "" },
			StateDir: func() (string, error) { return "", errNoEnvironment },
			LookPath: func(string) (string, error) { return "", fmt.Errorf("%w: %w", proc.ErrNotFound, errNoEnvironment) },
		},
	}
}

// workingDir is the directory the command cmd runs in was run from, or why it
// cannot be named: the one place a command reads it.
func workingDir(cmd *cobra.Command) (string, error) {
	dir, err := environmentOf(cmd).WorkingDir()
	if err != nil {
		return "", fmt.Errorf("determining the working directory: %w", err)
	}

	return dir, nil
}

// fromWorkingDir is path, or where it leads from the directory the command
// cmd runs in was run from when it is relative to that, as a shell would read
// it there.
func fromWorkingDir(cmd *cobra.Command, path string) (string, error) {
	if !relativeToWorkingDir(path) {
		return path, nil
	}

	dir, err := workingDir(cmd)
	if err != nil {
		return "", err
	}

	return filepath.Join(dir, path), nil
}

// relativeToWorkingDir reports that path leads from the working directory. An
// absolute path does not, and on Windows neither does one rooted on the
// current drive (\logs) nor one relative to another drive's own directory
// (D:logs): the operating system reads those from elsewhere.
func relativeToWorkingDir(path string) bool {
	rooted := path != "" && os.IsPathSeparator(path[0])

	return !filepath.IsAbs(path) && filepath.VolumeName(path) == "" && !rooted
}
