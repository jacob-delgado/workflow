// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

// Package proc runs external programs.
//
// It is the only place in this module that spawns a subprocess. Concentrating
// that here means the gosec exception for a program name held in a variable is
// written once, with its reason, instead of being repeated at every call site
// where it would gradually stop being read.
package proc

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"

	"github.com/jacob-delgado/workflow/internal/sanitize"
)

// ErrNotFound reports a program that is not on PATH.
var ErrNotFound = errors.New("program not found on PATH")

// Run executes name with args and returns what it wrote to standard output.
//
// A non-zero exit becomes an error carrying the program's standard error. That
// matters more than it looks: "git failed" without the reason sends the reader
// back to a terminal to run the command again by hand.
func Run(ctx context.Context, name string, args ...string) ([]byte, error) {
	command, err := build(ctx, Command{Dir: "", Name: name, Args: args, Env: nil})
	if err != nil {
		return nil, err
	}

	var stderr bytes.Buffer

	command.Stderr = &stderr

	output, err := command.Output()
	if err != nil {
		return nil, fmt.Errorf("%s: %w: %s", name, err, strings.TrimSpace(sanitize.Text(stderr.String())))
	}

	return output, nil
}

// Capture runs a program with input on its standard input and returns what it
// wrote to standard output.
//
// Unlike Run it feeds a body in, and it returns the output even on a non-zero
// exit: a tool that answers with data on standard output and signals an HTTP
// error only through its exit status — gh api, glab api — is read either way,
// with its standard error folded into the returned error.
func Capture(ctx context.Context, program Command, input []byte) ([]byte, error) {
	command, err := build(ctx, program)
	if err != nil {
		return nil, err
	}

	var stdout, stderr bytes.Buffer

	command.Stdout, command.Stderr = &stdout, &stderr

	if len(input) > 0 {
		command.Stdin = bytes.NewReader(input)
	}

	err = command.Run()
	if err != nil {
		return stdout.Bytes(), fmt.Errorf("%s: %w: %s",
			program.Name, err, strings.TrimSpace(sanitize.Text(stderr.String())))
	}

	return stdout.Bytes(), nil
}

// LookPath reports where a program is, or an error if it is not on PATH. It is
// exec.LookPath, re-exported so callers wiring a seam do not reach past this
// package for one of its two halves.
func LookPath(name string) (string, error) {
	path, err := exec.LookPath(name)
	if err != nil {
		return "", fmt.Errorf("%w: %s", ErrNotFound, name)
	}

	return path, nil
}

// Available reports whether name can be found on PATH.
func Available(name string) bool {
	_, err := exec.LookPath(name)

	return err == nil
}
