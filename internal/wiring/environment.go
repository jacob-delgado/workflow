// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package wiring

import (
	"context"
	"time"

	"github.com/jacob-delgado/workflow/internal/proc"
)

// Environment is what the wiring reads of the process it serves: the home
// directory, the environment's variables, the directory the store keeps its
// files in, and where a program is found. The command line builds it from the
// operating system and hands it over, so the wiring never reads the process's
// own, and a test hands each run one of its own.
type Environment struct {
	// Home is the home directory, or "" where there is none.
	Home string
	// Getenv reads an environment variable, "" when it is unset.
	Getenv func(name string) string
	// StateDir is the directory the store keeps its files in, or why there is
	// none, as store.DefaultDir says.
	StateDir func() (string, error)
	// LookPath finds a program on the environment's PATH, or answers
	// proc.ErrNotFound, as proc.LookPath does.
	LookPath func(name string) (string, error)
}

// Available reports whether name can be found on the environment's PATH.
func (e Environment) Available(name string) bool {
	_, err := e.LookPath(name)

	return err == nil
}

// Run runs a program found on the environment's PATH and returns its standard
// output, as proc.Run does; a gitrepo.Runner and a forge.Run both.
func (e Environment) Run(ctx context.Context, name string, args ...string) ([]byte, error) {
	return e.runCommand(ctx, proc.Command{Name: name, Args: args})
}

// CaptureWithin is proc.CaptureWithin over a program found on the
// environment's PATH, as Taskwarrior is asked.
func (e Environment) CaptureWithin(
	ctx context.Context, timeout time.Duration, program proc.Command, input []byte,
) ([]byte, error) {
	found, err := e.found(program)
	if err != nil {
		return nil, err
	}

	return proc.CaptureWithin(ctx, timeout, found, input)
}

// capture is proc.Capture over a program found on the environment's PATH.
func (e Environment) capture(ctx context.Context, program proc.Command, input []byte) ([]byte, error) {
	return e.CaptureWithin(ctx, 0, program, input)
}

// runCommand is proc.RunCommand over a program found on the environment's
// PATH.
func (e Environment) runCommand(ctx context.Context, program proc.Command) ([]byte, error) {
	found, err := e.found(program)
	if err != nil {
		return nil, err
	}

	return proc.RunCommand(ctx, found)
}

// start is proc.Start over a program found on the environment's PATH.
func (e Environment) start(ctx context.Context, program proc.Command) (proc.Output, error) {
	found, err := e.found(program)
	if err != nil {
		return proc.Output{}, err
	}

	return proc.Start(ctx, found)
}

// git runs git, found on the environment's PATH, with its terminal prompts
// turned off. What it runs is quick and local — the fetch, pull and push that
// reach the network stream through start instead — but nothing inside the
// interface could answer a credential prompt, so should one ask, git fails
// rather than seize the terminal. It is the Runner every repository this
// package builds goes through.
func (e Environment) git(ctx context.Context, name string, args ...string) ([]byte, error) {
	return e.runCommand(ctx, proc.Command{Name: name, Args: args, Env: []string{"GIT_TERMINAL_PROMPT=0"}})
}

// found is program with the executable the environment's PATH holds for it.
func (e Environment) found(program proc.Command) (proc.Command, error) {
	path, err := e.LookPath(program.Name)
	if err != nil {
		return proc.Command{}, err
	}

	program.Path = path

	return program, nil
}
