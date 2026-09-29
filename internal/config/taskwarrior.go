// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"errors"
	"fmt"
	"strings"
)

// ErrInvalidTaskwarriorProgram reports a taskwarrior.program that is not one
// line: a name or a path cannot hold a line break or a NUL.
var ErrInvalidTaskwarriorProgram = errors.New("taskwarrior.program must be a program name or a path on one line")

// Taskwarrior configures the optional Taskwarrior integration, on whenever a
// Taskwarrior 3.5.0 or newer is found.
type Taskwarrior struct {
	// Program is the task program to run: a name looked up on PATH, or a path.
	// Empty tries every task in an absolute PATH directory, in order, and keeps
	// the first that answers as Taskwarrior — go-task, the Taskfile runner, is
	// also called task.
	Program string `json:"program"`
	// Disabled turns the integration off even where Taskwarrior is installed.
	Disabled bool `json:"disabled"`
}

// validateTaskwarrior refuses a program with a line break or a NUL in it. It
// trims nothing: a path may begin or end with a space.
func (c Config) validateTaskwarrior() error {
	if strings.ContainsAny(c.Taskwarrior.Program, "\n\r\x00") {
		return fmt.Errorf("%w: %q", ErrInvalidTaskwarriorProgram, c.Taskwarrior.Program)
	}

	return nil
}
