// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
)

// ErrInvalidTaskwarriorProgram reports a taskwarrior.program that is not a
// name looked up on PATH or an absolute path, on one line: a path relative to
// the working directory names another program in each directory workflow
// starts in, and a name or a path cannot hold a line break or a NUL.
var ErrInvalidTaskwarriorProgram = errors.New(
	"taskwarrior.program must be a program name or an absolute path, on one line")

// Taskwarrior configures the optional Taskwarrior integration, on whenever a
// Taskwarrior 3.5.0 or newer is found.
type Taskwarrior struct {
	// Program is the task program to run: a name looked up on PATH, or a path.
	// Only the home directory's file may set it.
	// Empty tries every task in an absolute PATH directory, in order, and keeps
	// the first that answers as Taskwarrior — go-task, the Taskfile runner, is
	// also called task.
	Program string `json:"program"`
	// Disabled turns the integration off even where Taskwarrior is installed.
	Disabled bool `json:"disabled"`
}

// validateTaskwarrior refuses a program with a line break or a NUL in it, or
// named by a path relative to the working directory. It trims nothing: a
// path may end with a space, and one that begins with a space is relative.
func (c Config) validateTaskwarrior() error {
	program := c.Taskwarrior.Program

	relative := program != "" && !filepath.IsAbs(program) && filepath.Base(program) != program
	if relative || strings.ContainsAny(program, "\n\r\x00") {
		return fmt.Errorf("%w: %q", ErrInvalidTaskwarriorProgram, program)
	}

	return nil
}
