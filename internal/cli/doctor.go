// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"errors"
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/jacob-delgado/workflow/internal/config"
)

// errIncomplete reports a configuration that loaded but is missing fields.
var errIncomplete = errors.New("configuration is incomplete")

// newDoctorCmd builds `workflow doctor`.
func newDoctorCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "doctor",
		Short: "Report which configuration was found and what it is missing",
		Long: "Report which " + config.FileName + " is in effect and which required\n" +
			"fields are still empty. Makes no network calls, so it never\n" +
			"tells you a token is valid — only that one is present.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := loadFromEnvironment()

			return runDoctor(cmd.OutOrStdout(), cfg, err)
		},
	}
}

// runDoctor writes the report. A load error is part of the report rather than a
// failure to produce one: "there is no configuration file" is exactly what
// someone running doctor is asking about.
func runDoctor(out io.Writer, cfg config.Config, loadErr error) error {
	if loadErr != nil {
		return reportLoadError(out, loadErr)
	}

	fmt.Fprintf(out, "Configuration: %s\n", cfg.Path)
	fmt.Fprintf(out, "Jira:          %s (%s)\n", describe(cfg.Jira.BaseURL), cfg.Jira.AuthMode())
	fmt.Fprintf(out, "Slack:         %s\n", describe(cfg.Slack.Channel))

	missing := cfg.Missing()
	if len(missing) == 0 {
		fmt.Fprintf(out, "\nEverything required is set.\n")

		return nil
	}

	fmt.Fprintf(out, "\nMissing:\n")

	for _, field := range missing {
		fmt.Fprintf(out, "  - %s\n", field)
	}

	fmt.Fprintf(out, "\nEdit %s, then run `workflow doctor` again.\n", cfg.Path)
	fmt.Fprintf(out, "`workflow --help` explains how to create each token.\n")

	return fmt.Errorf("%w: %d field(s) missing", errIncomplete, len(missing))
}

// reportLoadError explains a configuration that could not be read, and says what
// to do about it.
func reportLoadError(out io.Writer, loadErr error) error {
	if errors.Is(loadErr, config.ErrNotFound) {
		fmt.Fprintf(out, "No %s found here or in your home directory.\n", config.FileName)
		fmt.Fprintf(out, "\nCreate one with `workflow config init`.\n")
		fmt.Fprintf(out, "`workflow --help` explains how to create each token.\n")

		return loadErr
	}

	return loadErr
}

// describe renders an unset value as something a reader can act on.
func describe(value string) string {
	if value == "" {
		return "(not set)"
	}

	return value
}
