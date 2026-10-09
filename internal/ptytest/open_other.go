// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

//go:build !linux && !darwin

package ptytest

import "os"

// openPair opens nothing: this package knows how Linux and macOS make a
// pseudo-terminal, and a test typing at one is skipped elsewhere.
func openPair() (*os.File, *os.File, error) {
	return nil, nil, errNoPseudoTerminal
}
