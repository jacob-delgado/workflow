// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package wiring

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/proc"
)

// Token source descriptions doctor reports, so the reader knows where a
// credential came from without the credential being shown.
const (
	sourceFile    = "the configuration file"
	sourceCommand = "token_command"
	sourceNone    = "no token configured"
)

// errNoToken is a token source that is set but gives nothing to send.
var errNoToken = errors.New("no token")

// errTokenCommandFailed is a token command that ran and gave no token.
var errTokenCommandFailed = errors.New("the token command failed")

// ResolveToken finds a credential from the literal in the file, an environment
// variable, or a command, in that order, and names where it came from — so a
// token need never be written into the file.
//
// A command is split on spaces and run as a program, no shell, so it stays pure
// Go and works on every platform; wrap a pipeline in a script if one is needed.
func ResolveToken(ctx context.Context, literal config.Secret, command, envVar string) (config.Secret, string, error) {
	switch {
	case literal != "":
		return literal, sourceFile, nil
	case envVar != "":
		return config.Secret(strings.TrimSpace(os.Getenv(envVar))), "the " + envVar + " environment variable", nil
	case command != "":
		return fromCommand(ctx, command)
	default:
		return "", sourceNone, nil
	}
}

// fromCommand runs the token command and returns its trimmed output.
func fromCommand(ctx context.Context, command string) (config.Secret, string, error) {
	fields := strings.Fields(command)
	if len(fields) == 0 {
		return "", sourceCommand, nil
	}

	out, err := proc.Run(ctx, fields[0], fields[1:]...)
	if err != nil {
		return "", sourceCommand, commandFailure(fields[0], err)
	}

	return config.Secret(strings.TrimSpace(string(out))), sourceCommand, nil
}

// commandFailure is why the token command gave no token, told by the kind of
// failure alone and never in proc's words, which carry what the program wrote
// to standard error: a command that reads a secret can print it, or what leads
// to it, as it fails.
func commandFailure(program string, err error) error {
	for _, kind := range []error{proc.ErrNotFound, proc.ErrTimedOut} {
		if errors.Is(err, kind) {
			return fmt.Errorf("%w: %s: %w", errTokenCommandFailed, program, kind)
		}
	}

	return fmt.Errorf("%w: %s; run it yourself to see why", errTokenCommandFailed, program)
}

// resolveSetToken finds the token a source the configuration sets gives, and
// refuses one that gives nothing: sent, an empty credential would only be
// turned away, for a reason the user could not act on.
func resolveSetToken(ctx context.Context, literal config.Secret, command, envVar string) (config.Secret, error) {
	token, source, err := ResolveToken(ctx, literal, command, envVar)
	if err != nil {
		return "", err
	}

	if token == "" {
		return "", fmt.Errorf("%w from %s", errNoToken, source)
	}

	return token, nil
}
