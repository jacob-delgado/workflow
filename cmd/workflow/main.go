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
	"github.com/jacob-delgado/workflow/internal/store"
	"github.com/jacob-delgado/workflow/internal/wiring"
)

func main() {
	err := cli.Execute(os.Args[1:], os.Stdout, os.Stderr, terminalPrompt(), processEnvironment())
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(cli.ExitStatus(err))
	}
}

// processEnvironment is the process workflow runs in, as the operating system
// tells it: the one place a command's working directory, home, variables,
// store and programs are read from it.
func processEnvironment() cli.Environment {
	// A machine without a home directory is unusual but not a failure: the
	// working directory alone is still a valid place to find a configuration.
	home, _ := os.UserHomeDir()

	return cli.Environment{
		WorkingDir: os.Getwd,
		Chdir:      os.Chdir,
		Process: wiring.Environment{
			Home: home, Getenv: os.Getenv, StateDir: store.DefaultDir, LookPath: proc.LookPath,
		},
	}
}

// terminalPrompt answers a command's questions from the terminal: a visible
// line through a reader shared across prompts, a secret read without echo, and
// whether a stream is a terminal, by x/term. The check lives here, in the
// untested main, because a real terminal is what a test cannot make; keeping
// a secret in the keychain and composing in the editor come from their own
// packages.
func terminalPrompt() cli.Prompt {
	reader := bufio.NewReader(os.Stdin)

	return cli.Prompt{
		Line:        cli.LineReader(reader, os.Stderr),
		Secret:      cli.SecretReader(os.Stdin, os.Stderr),
		StoreSecret: keychain.Storer(runtime.GOOS, proc.Capture, user.Current, os.Getenv),
		Input:       reader,
		IsTerminal:  isTerminal,
	}
}

// isTerminal reports whether stream is a terminal that can erase a line: a
// file x/term says is one, under any TERM but dumb, which has no erase.
func isTerminal(stream io.Writer) bool {
	file, ok := stream.(*os.File)

	return ok && term.IsTerminal(int(file.Fd())) && os.Getenv("TERM") != "dumb"
}
