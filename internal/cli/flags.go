// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"github.com/spf13/cobra"
)

// jsonFlag names the flag every command that prints data takes to print it as
// JSON.
const jsonFlag = "json"

// The one-letter spellings of the flags the scriptable commands share, each
// the same on every command that takes its flag, so a script learns them
// once. No other flag is spelled short: a command's own flag taking one of
// these letters would clash with the root's -n wherever cobra merges them.
const (
	dryRunShorthand = "n"
	yesShorthand    = "y"
	jsonShorthand   = "j"
)

// addJSONFlag declares --json, or -j, on cmd, held in asJSON, with help that
// says what it prints: the one way every command's data is asked for.
func addJSONFlag(cmd *cobra.Command, asJSON *bool, help string) {
	cmd.Flags().BoolVarP(asJSON, jsonFlag, jsonShorthand, false, help)
}
