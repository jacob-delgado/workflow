// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

//go:build !linux && !darwin

package ptytest

import "os"

// openPair opens nothing: this package knows how Linux and macOS make a
// pseudo-terminal, and a test typing at one is skipped elsewhere.
//
// Trade-off TRADE-35: no CI job runs this; only tests import ptytest, so
// only the Lint job's lint:crossvet compiles it, for Windows.
func openPair() (*os.File, *os.File, error) {
	return nil, nil, errNoPseudoTerminal
}
