// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

// Command workflow runs your Jira, Git forge and messaging workflow from the
// terminal.
package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"runtime"
	"strings"

	"golang.org/x/term"

	"github.com/jacob-delgado/workflow/internal/cli"
	"github.com/jacob-delgado/workflow/internal/editor"
	"github.com/jacob-delgado/workflow/internal/keychain"
	"github.com/jacob-delgado/workflow/internal/proc"
)

// keychainService is the name the Jira token is stored under in the keychain.
const keychainService = "workflow-jira"

func main() {
	err := cli.Execute(os.Args[1:], os.Stdout, os.Stderr, terminalPrompt())
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(cli.ExitStatus(err))
	}
}

// terminalPrompt reads guided-init answers from the terminal: a visible line
// through a reader shared across prompts, and a secret read without echo through
// x/term. It lives here, in the untested main, because reading a real terminal
// is what a test cannot do.
func terminalPrompt() cli.Prompt {
	reader := bufio.NewReader(os.Stdin)

	return cli.Prompt{
		Line: func(prompt string) (string, error) {
			fmt.Fprint(os.Stderr, prompt)

			line, err := reader.ReadString('\n')

			return strings.TrimRight(line, "\r\n"), err
		},
		Secret: func(prompt string) (string, error) {
			fmt.Fprint(os.Stderr, prompt)

			secret, err := term.ReadPassword(int(os.Stdin.Fd()))

			fmt.Fprintln(os.Stderr)

			return string(secret), err
		},
		StoreSecret: keychainStore(runtime.GOOS),
		Compose: func(draft, help string) (string, error) {
			return editor.Compose(os.Getenv, draft, help)
		},
	}
}

// keychainStore stores a secret in the OS keychain and returns the token_command
// that reads it back, or nil where storing is not wired for the platform, so
// the guided flow keeps the token in the file there.
func keychainStore(goos string) func(string) (string, error) {
	if !keychain.Supported(goos) {
		return nil
	}

	return func(secret string) (string, error) {
		command := keychain.StoreCommand(keychainService, secret)

		_, err := proc.Run(context.Background(), command.Name, command.Args...)
		if err != nil {
			return "", err
		}

		return keychain.LookupCommand(keychainService), nil
	}
}
