// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

// Package keychain stores a secret in the operating system's keychain and names
// the command that reads it back, so a token need never sit in the
// configuration file. It links nothing: it drives the platform's own tool,
// which is what keeps the build pure Go.
package keychain

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jacob-delgado/workflow/internal/proc"
)

// ErrSecretNotOneLine reports a secret that cannot travel on one line of
// security's input: one holding a line break or a NUL byte, or one longer than
// the line security reads.
var ErrSecretNotOneLine = errors.New("the secret cannot be handed to security on one line")

// jiraService is the name the Jira token is stored under. A token_command
// already saved in a configuration file names it, so it stays as it is.
const jiraService = "workflow-jira"

// lookupCommand is the token_command that reads the Jira token back out of the
// keychain, printing it and nothing else.
const lookupCommand = "security find-generic-password -s " + jiraService + " -w"

// maxLine is the longest command line security reads whole from its input, its
// newline included; it would read the rest of a longer one as a second command.
const maxLine = 4095

// Runner runs a program with input on its standard input and returns what it
// wrote to standard output. proc.Capture satisfies it.
type Runner func(ctx context.Context, program proc.Command, input []byte) ([]byte, error)

// Storer is how config init keeps the Jira token in the keychain on goos: a
// function that stores a secret through run and returns the token_command that
// reads it back. It is nil where storing is not wired for goos, so the guided
// flow keeps the token in the file there. Reading a secret back through a
// token_command works more widely — anywhere its own tool is installed — but
// storing is wired for macOS here, through the built-in `security`.
//
// security runs with -i, reading the command that stores the secret from its
// standard input, so the secret is never one of the program's arguments.
func Storer(goos string, run Runner) func(secret string) (string, error) {
	if goos != "darwin" {
		return nil
	}

	return func(secret string) (string, error) {
		line, err := storeLine(secret)
		if err != nil {
			return "", err
		}

		// Bounded as proc.Run bounds a quick read, so a security that never
		// answers cannot hold config init open.
		ctx, cancel := context.WithTimeout(context.Background(), proc.DefaultRunTimeout)
		defer cancel()

		_, err = run(ctx, proc.Command{Name: "security", Args: []string{"-i"}}, []byte(line))
		if err != nil {
			return "", err
		}

		return lookupCommand, nil
	}
}

// storeLine is security's command line that saves secret under the Jira
// service, updating an existing entry (-U) rather than adding a second one.
func storeLine(secret string) (string, error) {
	if strings.ContainsAny(secret, "\n\x00") {
		return "", fmt.Errorf("%w: it holds a line break or a NUL byte", ErrSecretNotOneLine)
	}

	line := "add-generic-password -U -s " + jiraService + " -w " + quoted(secret) + "\n"
	if len(line) > maxLine {
		return "", fmt.Errorf("%w: it is longer than the %d bytes security reads", ErrSecretNotOneLine, maxLine)
	}

	return line, nil
}

// quoted is value as one argument on security's command line: in double
// quotes, with each backslash and double quote escaped by a backslash, which
// security's own parser removes again.
func quoted(value string) string {
	return `"` + strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(value) + `"`
}
