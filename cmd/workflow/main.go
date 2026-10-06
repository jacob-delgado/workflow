// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

// Command workflow runs your Jira, Git forge and messaging workflow from the
// terminal.
package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/user"
	"runtime"

	"golang.org/x/term"

	"github.com/jacob-delgado/workflow/internal/cli"
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
// through x/term, and whether a stream is a terminal, by x/term too. The two
// reads and the check live here, in the untested main, because
// reading a real terminal is what a test cannot do; keeping a secret in the
// keychain and composing in the editor come from their own packages.
func terminalPrompt() cli.Prompt {
	reader := bufio.NewReader(os.Stdin)

	return cli.Prompt{
		Line: cli.LineReader(reader, os.Stderr),
		Secret: func(prompt string) (string, error) {
			fmt.Fprint(os.Stderr, prompt)

			secret, err := term.ReadPassword(int(os.Stdin.Fd()))

			fmt.Fprintln(os.Stderr)

			return string(secret), err
		},
		StoreSecret: keychain.Storer(runtime.GOOS, proc.Capture, user.Current, os.Getenv),
		Input:       reader,
		IsTerminal:  isTerminal,
	}
}

// isTerminal reports whether stream is a terminal: a file x/term says is one.
func isTerminal(stream io.Writer) bool {
	file, ok := stream.(*os.File)

	return ok && term.IsTerminal(int(file.Fd()))
}
