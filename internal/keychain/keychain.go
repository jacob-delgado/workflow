// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

// Package keychain stores a secret in the operating system's keychain and names
// the command that reads it back, so a token need never sit in the
// configuration file. It links nothing: it drives the platform's own tool,
// which is what keeps the build pure Go.
package keychain

import "context"

// jiraService is the name the Jira token is stored under. A token_command
// already saved in a configuration file names it, so it stays as it is.
const jiraService = "workflow-jira"

// lookupCommand is the token_command that reads the Jira token back out of the
// keychain, printing it and nothing else.
const lookupCommand = "security find-generic-password -s " + jiraService + " -w"

// Runner runs a program and returns what it wrote to standard output.
// proc.Run satisfies it.
type Runner func(ctx context.Context, name string, args ...string) ([]byte, error)

// Storer is how config init keeps the Jira token in the keychain on goos: a
// function that stores a secret through run and returns the token_command that
// reads it back. It is nil where storing is not wired for goos, so the guided
// flow keeps the token in the file there. Reading a secret back through a
// token_command works more widely — anywhere its own tool is installed — but
// storing is wired for macOS here, through the built-in `security`.
func Storer(goos string, run Runner) func(secret string) (string, error) {
	if goos != "darwin" {
		return nil
	}

	return func(secret string) (string, error) {
		// -U updates an existing entry rather than adding a second one for the
		// same service.
		_, err := run(context.Background(), "security", "add-generic-password", "-U", "-s", jiraService, "-w", secret)
		if err != nil {
			return "", err
		}

		return lookupCommand, nil
	}
}
