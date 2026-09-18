// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

// Command workflow runs your Jira, Slack, and Git forge workflow from the
// terminal.
package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"golang.org/x/term"

	"github.com/jacob-delgado/workflow/internal/cli"
)

// exitFailure is the status returned when a command reports an error.
const exitFailure = 1

func main() {
	err := cli.Execute(os.Args[1:], os.Stdout, os.Stderr, terminalPrompt())
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(exitFailure)
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
	}
}
