// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

// Command workflow runs your Jira, Git forge and messaging workflow from the
// terminal.
package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"golang.org/x/term"

	"github.com/jacob-delgado/workflow/internal/cli"
	"github.com/jacob-delgado/workflow/internal/editor"
	"github.com/jacob-delgado/workflow/internal/keychain"
	"github.com/jacob-delgado/workflow/internal/proc"
)

// standupHelp sits below the scissors line in the standup draft, and is cut
// away with everything under it when the editor closes.
const standupHelp = "Edit your standup above this line, then save and close.\n" +
	"Lines below the scissors are removed. An empty draft posts nothing."

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
		Compose:     composeInEditor,
	}
}

// composeInEditor opens draft in $EDITOR (or $VISUAL, else vi) and returns what
// was left above the scissors line. It lives here, in the untested main,
// because launching a real editor is what a test cannot do.
func composeInEditor(draft string) (string, error) {
	file, err := os.CreateTemp("", "workflow-standup-*.md")
	if err != nil {
		return "", fmt.Errorf("creating the draft: %w", err)
	}

	path := file.Name()
	defer func() { _ = os.Remove(path) }()

	_, err = file.WriteString(editor.Draft(draft, standupHelp))

	err = errors.Join(err, file.Close())
	if err != nil {
		return "", fmt.Errorf("writing the draft: %w", err)
	}

	command, err := proc.Interactive(editor.Invocation(os.Getenv, filepath.Dir(path), path, 0))
	if err != nil {
		return "", fmt.Errorf("starting the editor: %w", err)
	}

	command.Stdin, command.Stdout, command.Stderr = os.Stdin, os.Stdout, os.Stderr

	err = command.Run()
	if err != nil {
		return "", fmt.Errorf("editing the draft: %w", err)
	}

	edited, err := os.ReadFile(path) //nolint:gosec // path is our own os.CreateTemp file, not user input
	if err != nil {
		return "", fmt.Errorf("reading the draft: %w", err)
	}

	return editor.Parse(string(edited)), nil
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
