// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package wiring

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/jacob-delgado/workflow/internal/proc"
)

// Token source descriptions doctor reports, so the reader knows where a
// credential came from without the credential being shown.
const (
	sourceFile    = "the configuration file"
	sourceCommand = "token_command"
	sourceNone    = "no token configured"
)

// ResolveToken finds a credential from the literal in the file, an environment
// variable, or a command, in that order, and names where it came from — so a
// token need never be written into the file.
//
// A command is split on spaces and run as a program, no shell, so it stays pure
// Go and works on every platform; wrap a pipeline in a script if one is needed.
func ResolveToken(ctx context.Context, literal, command, envVar string) (string, string, error) {
	switch {
	case literal != "":
		return literal, sourceFile, nil
	case envVar != "":
		return strings.TrimSpace(os.Getenv(envVar)), "the " + envVar + " environment variable", nil
	case command != "":
		return fromCommand(ctx, command)
	default:
		return "", sourceNone, nil
	}
}

// fromCommand runs the token command and returns its trimmed output. Its error
// is deliberately generic: a failed command's message is not the token, but it
// is not worth risking in output either.
func fromCommand(ctx context.Context, command string) (string, string, error) {
	fields := strings.Fields(command)
	if len(fields) == 0 {
		return "", sourceCommand, nil
	}

	out, err := proc.Run(ctx, fields[0], fields[1:]...)
	if err != nil {
		return "", sourceCommand, fmt.Errorf("running the token command: %w", err)
	}

	return strings.TrimSpace(string(out)), sourceCommand, nil
}
