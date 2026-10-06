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
	"github.com/jacob-delgado/workflow/internal/jira"
	"github.com/jacob-delgado/workflow/internal/setup"
)

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
	// layers are the home file the new one lies over, and the new one; empty
	// when it stands alone.
	layers config.Files
}

// write writes cfg to path over opts' layers: replacing what --force names,
// and otherwise only as a new file, so a file or link that appears after the
// refusal was checked is still never written through.
func (opts initOptions) write(path string, cfg config.Config, over config.Revision) error {
	if opts.force {
		return setup.Save(path, opts.layers, cfg, over)
	}

	return setup.Create(path, opts.layers, cfg, over)
}

// newConfigInitCmd builds `workflow config init`. It asks for each credential
// and checks the Jira token, saving a webhook unchecked, unless --template is
// given, which writes a blank file to edit by hand.
func newConfigInitCmd(prompt Prompt) *cobra.Command {
	var opts initOptions

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Set up the configuration file, asking for the Jira token and a Slack webhook and checking the Jira token",
		Long: "Ask for the Jira address and token and a Slack incoming webhook, check the\n" +
			"Jira token, and write a " + config.FileName + " with what passed. The webhook is\n" +
			"saved unchecked, since a webhook cannot be checked without posting.\n\n" +
			"By default it lands at the repository root, so every subdirectory sees it;\n" +
			"outside a repository it lands in the current directory. Use --global to\n" +
			"write it to your home directory instead, where every directory can see it.\n" +
			"Use --template to write a blank file to fill in by hand rather than being\n" +
			"asked. With --dry-run it runs the same checks, writes nothing and stores\n" +
			"nothing in the keychain, and prints the file it would write, masked.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			opts.dryRun = dryRunRequested(cmd)

			where, err := whereInit(opts.global)
			if err != nil {
				return err
			}

			place := placeFor(opts.global)
			path := where.Path(place)
			opts.layers = where.Layers(place)

			if opts.template {
				return runConfigInit(cmd, path, opts)
			}

			return runGuidedInit(cmd, path, opts, prompt)
		},
	}

	cmd.Flags().BoolVar(&opts.force, "force", false, "overwrite an existing file")
	cmd.Flags().BoolVar(&opts.global, "global", false, "write to the home directory instead of here")
	cmd.Flags().BoolVar(&opts.template, "template", false, "write a blank file to edit by hand instead of being asked")

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
			"exits 3, as doctor does.",
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

// whereInit is where `config init` runs: the working directory and home, or
// the home directory alone for --global.
func whereInit(global bool) (setup.Where, error) {
	if global {
		home, err := os.UserHomeDir()
		if err != nil {
			return setup.Where{}, fmt.Errorf("determining the home directory: %w", err)
		}

		return setup.Where{HomeDir: home}, nil
	}

	// Trade-off TRADE-18: only Linux's tests reach this, since macOS still names
	// a working directory once it is removed.
	workDir, err := os.Getwd()
	if err != nil {
		return setup.Where{}, fmt.Errorf("determining the working directory: %w", err)
	}

	return setup.Where{WorkDir: workDir, HomeDir: configHome()}, nil
}

// placeFor is where --global says the file goes.
func placeFor(global bool) setup.Place {
	if global {
		return setup.Home
	}

	return setup.Repository
}

// refuseOverwrite refuses to clobber an existing file unless force says
// otherwise — that file holds credentials that are not recoverable once
// overwritten. A dry run is refused the same way, since it previews what would
// happen.
func refuseOverwrite(path string, force bool) error {
	if force {
		return nil
	}

	err := setup.RefuseExisting(path)
	if err != nil {
		return fmt.Errorf("%w; pass --force to overwrite", err)
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

// runConfigInit writes the template, unless the file is already there. Over a
// home file it writes an empty layer instead: the template's blanks would hide
// every setting the home file makes.
func runConfigInit(cmd *cobra.Command, path string, opts initOptions) error {
	err := refuseOverwrite(path, opts.force)
	if err != nil {
		return err
	}

	if opts.layers.Home != "" {
		return writeEmptyLayer(cmd, path, opts)
	}

	if opts.dryRun {
		return previewConfig(cmd, path, config.Template())
	}

	err = opts.write(path, config.Template(), config.Revision{})
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

// writeEmptyLayer writes the repository's file over the home one with nothing
// in it yet, and says what belongs there.
func writeEmptyLayer(cmd *cobra.Command, path string, opts initOptions) error {
	beneath, over, err := setup.Beneath(opts.layers)
	if err != nil {
		return err
	}

	if opts.dryRun {
		return previewConfig(cmd, path, beneath)
	}

	err = opts.write(path, beneath, over)
	if err != nil {
		return err
	}

	fmt.Fprintf(cmd.ErrOrStderr(), "Wrote %s (mode %#o), over %s.\n"+
		"\nNext: add only the settings this repository changes; the rest come from %s.\n",
		path, config.FileMode, opts.layers.Home, opts.layers.Home)

	return nil
}

// runGuidedInit asks for each credential, checks the Jira token, saves a
// webhook unchecked, and writes what it kept, unless the file is already
// there. A dry run asks and checks the same, but offers no keychain — which
// would store the token — and prints the file rather than writing it.
func runGuidedInit(cmd *cobra.Command, path string, opts initOptions, prompt Prompt) error {
	err := refuseOverwrite(path, opts.force)
	if err != nil {
		return err
	}

	if opts.dryRun {
		prompt.StoreSecret = nil
	}

	requestLog, closeLog, err := requestLogFor(cmd)
	if err != nil {
		return err
	}
	defer closeLog()

	out := cmd.ErrOrStderr()
	fmt.Fprintf(out, "Setting up %s. Leave a prompt blank to skip it.\n\n", path)

	beneath, over, err := setup.Beneath(opts.layers)
	if err != nil {
		return err
	}

	answers, err := askAnswers(cmd.Context(), out, prompt, requestLog.Wrap("jira", onlineDoer(config.Config{})))
	if errors.Is(err, io.EOF) || errors.Is(err, errNoTerminal) {
		return fmt.Errorf("%w; pass --template to write a file to edit by hand", errNoTerminal)
	}

	if err != nil {
		return err
	}

	cfg := answers.Over(beneath)
	if opts.dryRun {
		return previewConfig(cmd, path, cfg)
	}

	err = opts.write(path, cfg, over)
	if err != nil {
		return err
	}

	fmt.Fprintf(out, "\nWrote %s (mode %#o). Run `workflow doctor` to check it again.\n", path, config.FileMode)
	warnIfNotIgnored(cmd.Context(), out, path)

	return nil
}

// askAnswers asks setup's questions in turn: Jira's, then messaging's.
func askAnswers(ctx context.Context, out io.Writer, prompt Prompt, doer jira.Doer) (setup.Answers, error) {
	asked, err := collectJira(ctx, out, prompt, doer)
	if err != nil {
		return setup.Answers{}, err
	}

	webhook, err := collectMessaging(out, prompt)
	if err != nil {
		return setup.Answers{}, err
	}

	return setup.Answers{Jira: asked, Webhook: webhook}, nil
}

// collectJira asks for the Jira address and token, checks them over doer, and
// keeps them only if the check passed or the user chose to save them regardless.
func collectJira(ctx context.Context, out io.Writer, prompt Prompt, doer jira.Doer) (config.Jira, error) {
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

	kept, err := keepIfChecked(prompt, "jira", checkTyped(ctx, out, doer, settings))
	if err != nil || !kept {
		return config.Jira{}, err
	}

	return keepTokenSafe(out, prompt, settings)
}

// checkTyped checks the address and token typed, saying how it went.
func checkTyped(ctx context.Context, out io.Writer, doer jira.Doer, settings config.Jira) error {
	who, err := setup.Check(ctx, doer, settings)
	if err != nil {
		fmt.Fprintf(out, "  %-10s %v\n", "jira", err)

		return err
	}

	fmt.Fprintf(out, "  %-10s authenticates as %s\n", "jira", who)

	return nil
}

// keepTokenSafe offers to move the token into the OS keychain, so the file holds
// a token_command rather than the secret. It is a no-op where the keychain is
// not wired for the platform.
func keepTokenSafe(out io.Writer, prompt Prompt, settings config.Jira) (config.Jira, error) {
	if prompt.StoreSecret == nil {
		return settings, nil
	}

	store, err := confirm(prompt, "Store the Jira token in your keychain, keeping it out of the file?")
	if err != nil || !store {
		return settings, err
	}

	settings, err = setup.Keep(prompt.StoreSecret, settings)
	if err != nil {
		return settings, err
	}

	fmt.Fprintf(out, "  %-10s stored in the keychain; the file will hold a token_command\n", "jira")

	return settings, nil
}

// collectMessaging asks for a Slack incoming webhook, the setup with nothing to
// check. A blank answer leaves Slack to a user token, which `workflow slack
// login` sets up, since it needs the app's client ID and a refresh token; the
// other services' webhooks are left to the docs and a later hand edit of the
// messaging block.
func collectMessaging(out io.Writer, prompt Prompt) (config.Secret, error) {
	webhook, err := prompt.Secret("Slack incoming webhook URL, blank to post with your user token instead: ")
	if err != nil {
		return "", err
	}

	webhook = strings.TrimSpace(webhook)
	if webhook == "" {
		fmt.Fprintf(out, "  %-10s run `workflow slack login` to post with your Slack user token\n", "slack")

		return "", nil
	}

	fmt.Fprintf(out, "  %-10s saved (a webhook cannot be checked without posting)\n", "slack")

	return config.Secret(webhook), nil
}

// keepIfChecked decides whether to keep a credential: a passing check keeps it,
// and a failing one asks, so a service that is merely unreachable right now can
// still be saved. An address that is no address is never kept, so it is not
// asked about: the setup stops on it.
func keepIfChecked(prompt Prompt, what string, checkErr error) (bool, error) {
	if checkErr == nil {
		return true, nil
	}

	if !setup.Keepable(checkErr) {
		return false, checkErr
	}

	return confirm(prompt, "The "+what+" check did not pass. Save it anyway?")
}

// warnIfNotIgnored says so when the file is inside a repository but not ignored
// by git, since it is about to hold credentials. Outside a repository there is
// nothing to warn about.
func warnIfNotIgnored(ctx context.Context, out io.Writer, path string) {
	if !setup.NotIgnored(ctx, path) {
		return
	}

	fmt.Fprintf(out, "\nWarning: %s is not ignored by git. It holds credentials —\n", filepath.Base(path))
	fmt.Fprintf(out, "add it to .gitignore so it is never committed.\n")
}

// runConfigShow prints the loaded configuration with every credential masked:
// the JSON alone on stdout, so it pipes into jq, and which file it came from on
// stderr.
func runConfigShow(cmd *cobra.Command, cfg config.Config) error {
	fmt.Fprintf(cmd.ErrOrStderr(), "# %s\n", cfg.Layers())

	return encodeJSON(cmd.OutOrStdout(), cfg.Redacted())
}
