// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/gitrepo"
	"github.com/jacob-delgado/workflow/internal/proc"
)

// errConfigExists reports that init would have overwritten a file.
var errConfigExists = errors.New("configuration file already exists")

// newConfigCmd builds the `workflow config` subtree.
func newConfigCmd(prompt Prompt) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Create and inspect the configuration file",
		Args:  cobra.NoArgs,
	}

	cmd.AddCommand(newConfigInitCmd(prompt), newConfigShowCmd())

	return cmd
}

// newConfigInitCmd builds `workflow config init`. It asks for each credential
// and checks it, unless --template is given, which writes a blank file to edit
// by hand.
func newConfigInitCmd(prompt Prompt) *cobra.Command {
	var (
		force    bool
		global   bool
		template bool
	)

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Set up the configuration file, asking for and checking each credential",
		Long: "Ask for the Jira and Slack credentials, check each one, and write a\n" +
			config.FileName + " with what passed.\n\n" +
			"By default it lands at the repository root, so every subdirectory sees it;\n" +
			"outside a repository it lands in the current directory. Use --global to\n" +
			"write it to your home directory instead, where every directory can see it.\n" +
			"Use --template to write a blank file to fill in by hand rather than being\n" +
			"asked.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			dir, err := targetDir(global)
			if err != nil {
				return err
			}

			path := filepath.Join(dir, config.FileName)
			if template {
				return runConfigInit(cmd, path, force)
			}

			return runGuidedInit(cmd, path, force, prompt)
		},
	}

	cmd.Flags().BoolVar(&force, "force", false, "overwrite an existing file")
	cmd.Flags().BoolVar(&global, "global", false, "write to the home directory instead of here")
	cmd.Flags().BoolVar(&template, "template", false, "write a blank file to edit by hand instead of being asked")

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

	return config.RepoRoot(workDir), nil
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

// runGuidedInit asks for each credential, checks it, and writes what passed,
// refusing to clobber an existing file unless force says otherwise.
func runGuidedInit(cmd *cobra.Command, path string, force bool, prompt Prompt) error {
	_, err := os.Stat(path)
	if err == nil && !force {
		return fmt.Errorf("%w: %s (pass --force to overwrite)", errConfigExists, path)
	}

	out := cmd.OutOrStdout()
	fmt.Fprintf(out, "Setting up %s. Leave a prompt blank to skip it.\n\n", path)

	cfg := config.Default()

	cfg.Jira, err = collectJira(cmd.Context(), out, prompt)
	if err != nil {
		return err
	}

	cfg.Slack, err = collectSlack(out, prompt)
	if err != nil {
		return err
	}

	err = config.Save(path, cfg)
	if err != nil {
		return err
	}

	fmt.Fprintf(out, "\nWrote %s (mode %#o). Run `workflow doctor` to check it again.\n", path, config.FileMode)
	warnIfNotIgnored(cmd.Context(), out, path)

	return nil
}

// collectJira asks for the Jira address and token, checks them, and keeps them
// only if the check passed or the user chose to save them regardless.
func collectJira(ctx context.Context, out io.Writer, prompt Prompt) (config.Jira, error) {
	baseURL, err := prompt.Line("Jira base URL (e.g. https://jira.example.com), blank to skip: ")
	if err != nil {
		return config.Jira{}, err
	}

	baseURL = strings.TrimSpace(baseURL)
	if baseURL == "" {
		return config.Jira{}, nil
	}

	token, err := prompt.Secret("Jira personal access token: ")
	if err != nil {
		return config.Jira{}, err
	}

	settings := config.Jira{BaseURL: baseURL, Token: config.Secret(strings.TrimSpace(token))}

	kept, err := keepIfChecked(prompt, "jira", checkJira(ctx, out, onlineDoer(config.Config{}), settings))
	if err != nil || !kept {
		return config.Jira{}, err
	}

	return keepTokenSafe(out, prompt, settings)
}

// keepTokenSafe offers to move the token into the OS keychain, so the file holds
// a token_command rather than the secret. It is a no-op where the keychain is
// not wired for the platform.
func keepTokenSafe(out io.Writer, prompt Prompt, jira config.Jira) (config.Jira, error) {
	if prompt.StoreSecret == nil {
		return jira, nil
	}

	store, err := confirm(prompt, "Store the Jira token in your keychain, keeping it out of the file?")
	if err != nil || !store {
		return jira, err
	}

	tokenCommand, err := prompt.StoreSecret(jira.Token.Reveal())
	if err != nil {
		return jira, fmt.Errorf("storing the token in the keychain: %w", err)
	}

	jira.Token, jira.TokenCommand = "", tokenCommand

	fmt.Fprintf(out, "  %-10s stored in the keychain; the file will hold a token_command\n", "jira")

	return jira, nil
}

// collectSlack asks for a Slack incoming webhook, the setup with nothing to
// check. A bot token, which is more involved, is left to the docs and a later
// hand edit.
func collectSlack(out io.Writer, prompt Prompt) (config.Slack, error) {
	webhook, err := prompt.Secret("Slack incoming webhook URL, blank to skip: ")
	if err != nil {
		return config.Slack{}, err
	}

	webhook = strings.TrimSpace(webhook)
	if webhook == "" {
		return config.Slack{}, nil
	}

	fmt.Fprintf(out, "  %-10s saved (a webhook cannot be checked without posting)\n", "slack")

	return config.Slack{WebhookURL: config.Secret(webhook)}, nil
}

// keepIfChecked decides whether to keep a credential: a passing check keeps it,
// and a failing one asks, so a service that is merely unreachable right now can
// still be saved.
func keepIfChecked(prompt Prompt, what string, checkErr error) (bool, error) {
	if checkErr == nil {
		return true, nil
	}

	return confirm(prompt, "The "+what+" check did not pass. Save it anyway?")
}

// warnIfNotIgnored says so when the file is inside a repository but not ignored
// by git, since it is about to hold credentials. Outside a repository there is
// nothing to warn about.
func warnIfNotIgnored(ctx context.Context, out io.Writer, path string) {
	ignored, err := gitrepo.At(proc.Run, filepath.Dir(path)).CheckIgnored(ctx, path)
	if err != nil || ignored {
		return
	}

	fmt.Fprintf(out, "\nWarning: %s is not ignored by git. It holds credentials —\n", filepath.Base(path))
	fmt.Fprintf(out, "add it to .gitignore so it is never committed.\n")
}

// runConfigShow prints the loaded configuration with every credential masked.
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
