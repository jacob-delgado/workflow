// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"github.com/spf13/cobra"
)

// jsonFlag names the flag every command that prints data takes to print it as
// JSON.
const jsonFlag = "json"

// addJSONFlag declares --json on cmd, held in asJSON, with help that says what
// it prints: the one way every command's data is asked for.
func addJSONFlag(cmd *cobra.Command, asJSON *bool, help string) {
	cmd.Flags().BoolVar(asJSON, jsonFlag, false, help)
}
