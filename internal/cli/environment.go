// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"

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
	// operating system cannot name it.
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
// command's own work starts.
func handOver(cmd *cobra.Command, env Environment) {
	cmd.SetContext(context.WithValue(cmd.Context(), environmentKey{}, env))
}

// environmentOf is the Environment the command cmd runs in.
func environmentOf(cmd *cobra.Command) Environment {
	env, _ := cmd.Context().Value(environmentKey{}).(Environment)

	return env
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
// cmd runs in was run from when it is relative, as a shell would read it there.
func fromWorkingDir(cmd *cobra.Command, path string) (string, error) {
	if filepath.IsAbs(path) {
		return path, nil
	}

	dir, err := workingDir(cmd)
	if err != nil {
		return "", err
	}

	return filepath.Join(dir, path), nil
}
