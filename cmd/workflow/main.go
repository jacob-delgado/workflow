// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

// Command workflow runs your Jira, Slack, and Git forge workflow from the
// terminal.
package main

import (
	"fmt"
	"os"

	"github.com/jacob-delgado/workflow/internal/cli"
)

// exitFailure is the status returned when a command reports an error.
const exitFailure = 1

func main() {
	err := cli.Execute(os.Args[1:], os.Stdout, os.Stderr)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(exitFailure)
	}
}
