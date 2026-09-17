// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/jacob-delgado/workflow/internal/config"
)

// errConfigExists reports that init would have overwritten a file.
var errConfigExists = errors.New("configuration file already exists")

// newConfigCmd builds the `workflow config` subtree.
func newConfigCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Create and inspect the configuration file",
		Args:  cobra.NoArgs,
	}

	cmd.AddCommand(newConfigInitCmd(), newConfigShowCmd())

	return cmd
}

// newConfigInitCmd builds `workflow config init`.
func newConfigInitCmd() *cobra.Command {
	var (
		force  bool
		global bool
	)

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Write a starting configuration file",
		Long: "Write a starting " + config.FileName + " with empty credentials.\n\n" +
			"By default it lands in the current directory. Use --global to write it to\n" +
			"your home directory instead, where every directory can see it.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			dir, err := targetDir(global)
			if err != nil {
				return err
			}

			return runConfigInit(cmd, filepath.Join(dir, config.FileName), force)
		},
	}

	cmd.Flags().BoolVar(&force, "force", false, "overwrite an existing file")
	cmd.Flags().BoolVar(&global, "global", false, "write to the home directory instead of here")

	return cmd
}

// showLoadError guides a config show that found no file to the command that
// creates one; any other load failure is a real error.
func showLoadError(cmd *cobra.Command, err error) error {
	if !errors.Is(err, config.ErrNotFound) {
		return err
	}

	out := cmd.OutOrStdout()
	fmt.Fprintf(out, "%s\n\n%s\n%s\n", config.NoConfigHeadline, config.InitStep, config.DoctorStep)

	return nil
}

// newConfigShowCmd builds `workflow config show`.
func newConfigShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "show",
		Short: "Print the configuration in effect, with tokens masked",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := loadFromEnvironment()
			if err != nil {
				return showLoadError(cmd, err)
			}

			return runConfigShow(cmd, cfg)
		},
	}
}

// targetDir picks the directory `config init` writes to.
func targetDir(global bool) (string, error) {
	if global {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("determining the home directory: %w", err)
		}

		return home, nil
	}

	workDir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("determining the working directory: %w", err)
	}

	return workDir, nil
}

// runConfigInit writes the template, refusing to clobber an existing file
// unless force says otherwise — that file holds credentials that are not
// recoverable once overwritten.
func runConfigInit(cmd *cobra.Command, path string, force bool) error {
	_, err := os.Stat(path)
	if err == nil && !force {
		return fmt.Errorf("%w: %s (pass --force to overwrite)", errConfigExists, path)
	}

	err = config.Save(path, config.Template())
	if err != nil {
		return err
	}

	out := cmd.OutOrStdout()
	fmt.Fprintf(out, "Wrote %s (mode %#o).\n", path, config.FileMode)
	fmt.Fprintf(out, "\nNext: add your Jira and Slack tokens, then run `workflow doctor`.\n")
	fmt.Fprintf(out, "`workflow --help` explains how to create each token.\n")

	return nil
}

// runConfigShow prints the loaded configuration with both tokens masked.
func runConfigShow(cmd *cobra.Command, cfg config.Config) error {
	out := cmd.OutOrStdout()
	fmt.Fprintf(out, "# %s\n", cfg.Path)

	encoded, err := json.MarshalIndent(cfg.Redacted(), "", "  ")
	if err != nil {
		return fmt.Errorf("encoding configuration: %w", err)
	}

	fmt.Fprintf(out, "%s\n", encoded)

	return nil
}
