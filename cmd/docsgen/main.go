// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

// Command docsgen writes the command reference for the documentation site from
// the Cobra command tree into the directory it is given:
//
//	task docs:gen
//
// The work is internal/docsgen's, where its tests are.
package main

import (
	"os"

	"github.com/jacob-delgado/workflow/internal/docsgen"
)

func main() {
	os.Exit(docsgen.Run(os.Args[1:], os.Stdout, os.Stderr))
}
