// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"context"
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
		// Runnable, so NoArgs refuses an unknown subcommand as misuse; declared
		// here, not left to markMisuse, so the generated reference shows it too.
		RunE: func(cmd *cobra.Command, _ []string) error { return cmd.Help() },
	}

	cmd.AddCommand(newConfigInitCmd(prompt), newConfigShowCmd())

	return cmd
}

// initOptions are config init's flags.
type initOptions struct {
	force    bool
	global   bool
	template bool
	dryRun   bool
}

// newConfigInitCmd builds `workflow config init`. It asks for each credential
// and checks it, unless --template is given, which writes a blank file to edit
// by hand.
func newConfigInitCmd(prompt Prompt) *cobra.Command {
	var opts initOptions

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Set up the configuration file, asking for and checking each credential",
		Long: "Ask for the Jira and Slack credentials, check each one, and write a\n" +
			config.FileName + " with what passed.\n\n" +
			"By default it lands at the repository root, so every subdirectory sees it;\n" +
			"outside a repository it lands in the current directory. Use --global to\n" +
			"write it to your home directory instead, where every directory can see it.\n" +
			"Use --template to write a blank file to fill in by hand rather than being\n" +
			"asked. With --dry-run it runs the same checks, writes nothing and stores\n" +
			"nothing in the keychain, and prints the file it would write, masked.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			dir, err := targetDir(opts.global)
			if err != nil {
				return err
			}

			path := filepath.Join(dir, config.FileName)
			if opts.template {
				return runConfigInit(cmd, path, opts)
			}

			return runGuidedInit(cmd, path, opts, prompt)
		},
	}

	cmd.Flags().BoolVar(&opts.force, "force", false, "overwrite an existing file")
	cmd.Flags().BoolVar(&opts.global, "global", false, "write to the home directory instead of here")
	cmd.Flags().BoolVar(&opts.template, "template", false, "write a blank file to edit by hand instead of being asked")
	cmd.Flags().BoolVar(&opts.dryRun, "dry-run", false, "print the file it would write, masked, and write nothing")

	return cmd
}

// showLoadError fails a config show whose configuration did not load, as
// doctor does. One that found no file is also guided to the command that
// creates one, on stderr, where a script reading the JSON will not take it
// for any.
func showLoadError(cmd *cobra.Command, err error) error {
	if errors.Is(err, config.ErrNotFound) {
		fmt.Fprintf(cmd.ErrOrStderr(), "%s\n\n%s\n%s\n", config.NoConfigHeadline, config.InitStep, config.DoctorStep)
	}

	return err
}

// newConfigShowCmd builds `workflow config show`.
func newConfigShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "show",
		Short: "Print the configuration in effect, with tokens masked",
		Long: "Print the configuration in effect as JSON, with every credential masked.\n" +
			"The JSON alone goes to stdout, so it pipes into jq; the file it came from is\n" +
			"named on stderr. With no configuration file it says how to create one and\n" +
			"fails, as doctor does.",
		Args: cobra.NoArgs,
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

// refuseOverwrite refuses to clobber an existing file unless force says
// otherwise — that file holds credentials that are not recoverable once
// overwritten. A dry run is refused the same way, since it previews what would
// happen.
func refuseOverwrite(path string, force bool) error {
	_, err := os.Stat(path)
	if err == nil && !force {
		return fmt.Errorf("%w: %s (pass --force to overwrite)", errConfigExists, path)
	}

	return nil
}

// previewConfig is what a dry run of config init prints instead of writing: the
// file, with every credential masked, as the artifact on stdout, and where it
// would have gone on stderr.
func previewConfig(cmd *cobra.Command, path string, cfg config.Config) error {
	fmt.Fprintf(cmd.ErrOrStderr(), "dry run: would write %s (mode %#o)\n", path, config.FileMode)

	return encodeJSON(cmd.OutOrStdout(), cfg.Redacted())
}

// runConfigInit writes the template, unless the file is already there.
func runConfigInit(cmd *cobra.Command, path string, opts initOptions) error {
	err := refuseOverwrite(path, opts.force)
	if err != nil {
		return err
	}

	if opts.dryRun {
		return previewConfig(cmd, path, config.Template())
	}

	err = config.Save(path, config.Template())
	if err != nil {
		return err
	}

	// The file is what config init makes; everything it says is about the file.
	out := cmd.ErrOrStderr()
	fmt.Fprintf(out, "Wrote %s (mode %#o).\n", path, config.FileMode)
	fmt.Fprintf(out, "\nNext: add your Jira and Slack tokens, then run `workflow doctor`.\n")
	fmt.Fprintf(out, "`workflow --help` explains how to create each token.\n")

	return nil
}

// runGuidedInit asks for each credential, checks it, and writes what passed,
// unless the file is already there. A dry run asks and checks the same, but
// offers no keychain — which would store the token — and prints the file
// rather than writing it.
func runGuidedInit(cmd *cobra.Command, path string, opts initOptions, prompt Prompt) error {
	err := refuseOverwrite(path, opts.force)
	if err != nil {
		return err
	}

	if opts.dryRun {
		prompt.StoreSecret = nil
	}

	out := cmd.ErrOrStderr()
	fmt.Fprintf(out, "Setting up %s. Leave a prompt blank to skip it.\n\n", path)

	cfg := config.Default()

	cfg.Jira, err = collectJira(cmd.Context(), out, prompt)
	if err != nil {
		return err
	}

	cfg.Messaging, err = collectMessaging(out, prompt)
	if err != nil {
		return err
	}

	if opts.dryRun {
		return previewConfig(cmd, path, cfg)
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

// collectMessaging asks for a Slack incoming webhook, the setup with nothing to
// check. A bot token, and the other services' webhooks, are more involved and
// are left to the docs and a later hand edit of the messaging block.
func collectMessaging(out io.Writer, prompt Prompt) (config.Messaging, error) {
	webhook, err := prompt.Secret("Slack incoming webhook URL, blank to skip: ")
	if err != nil {
		return config.Messaging{}, err
	}

	webhook = strings.TrimSpace(webhook)
	if webhook == "" {
		return config.Messaging{}, nil
	}

	fmt.Fprintf(out, "  %-10s saved (a webhook cannot be checked without posting)\n", "slack")

	return config.Messaging{Kind: config.KindSlack, WebhookURL: config.Secret(webhook)}, nil
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

// runConfigShow prints the loaded configuration with every credential masked:
// the JSON alone on stdout, so it pipes into jq, and which file it came from on
// stderr.
func runConfigShow(cmd *cobra.Command, cfg config.Config) error {
	fmt.Fprintf(cmd.ErrOrStderr(), "# %s\n", cfg.Path)

	return encodeJSON(cmd.OutOrStdout(), cfg.Redacted())
}
