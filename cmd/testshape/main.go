// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

// Command testshape checks that every test marks its Arrange, Act and Assert,
// and that each Assert reaches a failure. Name test files; each one's whole
// package is checked:
//
//	go run ./cmd/testshape internal/tui/*_test.go
//
// task lint runs it over every test file, and the pre-commit hook over the
// staged ones.
package main

import (
	"os"

	"github.com/jacob-delgado/workflow/internal/testshape"
)

func main() {
	os.Exit(testshape.Run(os.Args[1:], os.Stdout, os.Stderr))
}
