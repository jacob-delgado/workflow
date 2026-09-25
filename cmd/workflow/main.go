// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

// Command workflow runs your Jira, Git forge and messaging workflow from the
// terminal.
package main

import (
	"bufio"
	"fmt"
	"os"
	"os/user"
	"runtime"
	"strings"

	"golang.org/x/term"

	"github.com/jacob-delgado/workflow/internal/cli"
	"github.com/jacob-delgado/workflow/internal/editor"
	"github.com/jacob-delgado/workflow/internal/keychain"
	"github.com/jacob-delgado/workflow/internal/proc"
)

func main() {
	err := cli.Execute(os.Args[1:], os.Stdout, os.Stderr, terminalPrompt())
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(cli.ExitStatus(err))
	}
}

// terminalPrompt answers a command's questions from the terminal: a visible
// line through a reader shared across prompts, and a secret read without echo
// through x/term. The two reads live here, in the untested main, because
// reading a real terminal is what a test cannot do; keeping a secret in the
// keychain and composing in the editor come from their own packages.
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
		StoreSecret: keychain.Storer(runtime.GOOS, proc.Capture, user.Current, os.Getenv),
		Compose: func(draft, help string) (string, error) {
			return editor.Compose(os.Getenv, draft, help)
		},
	}
}
