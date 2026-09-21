// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

// Package cli builds the workflow command tree.
package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/jacob-delgado/workflow/internal/buildinfo"
	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/tui"
	"github.com/jacob-delgado/workflow/internal/web"
	"github.com/jacob-delgado/workflow/internal/webserver"
	"github.com/jacob-delgado/workflow/internal/wiring"
)

const longHelp = `workflow ties Jira, Slack, and your Git forge into one terminal workflow.

CONFIGURATION

  workflow reads ` + config.FileName + ` from the current directory, and falls
  back to your home directory. A file in the current directory REPLACES the one
  in your home directory — the two are never merged, so a repository-local
  configuration is always the whole story.

  Write a starting file with:

      workflow config init

  Then fill in the two credentials below and check your work with:

      workflow doctor

JIRA TOKEN (on-premises / Data Center)

  1. Sign in to your Jira instance in a browser.
  2. Open your avatar menu, then Profile, then Personal Access Tokens.
  3. Create a token, give it a name, and copy the value.
  4. Put it in ` + config.FileName + ` as jira.token, with your instance's URL
     as jira.base_url.

  Leave jira.user empty to authenticate with that token as a bearer token,
  which is what Data Center expects. Set jira.user only if your instance needs
  HTTP Basic authentication, in which case the token is used as the password.

SLACK — PICK ONE OF TWO

  An incoming webhook is the two-minute option; a bot token is the capable one.
  Set either. If you set both, the bot token is used.

  Incoming webhook (simplest):

  1. Go to https://api.slack.com/apps and create an app in your workspace.
  2. Turn on Incoming Webhooks, then "Add New Webhook to Workspace" and pick
     the channel it will post to.
  3. Put the URL in ` + config.FileName + ` as slack.webhook_url.

  The webhook is bound to the channel you picked, so slack.channel does not
  apply. Treat the URL like a password: anyone holding it can post there.

  Bot token (choose the channel at runtime, and post richer messages):

  1. Go to https://api.slack.com/apps and create an app in your workspace.
  2. Under OAuth & Permissions, add the chat:write bot token scope.
  3. Install the app to the workspace, then copy the Bot User OAuth Token. It
     starts with "xoxb-".
  4. Put it in ` + config.FileName + ` as slack.token, and set slack.channel to
     the channel workflow should post to.

  Invite the bot to that channel, or it cannot post there.

SECURITY

  ` + config.FileName + ` holds live credentials. "workflow config init" writes
  it readable only by you, it is listed in .gitignore, and "workflow config
  show" masks every one of them — including slack.webhook_url, which is a
  credential in its own right rather than merely an address.`

// Execute runs the command tree with the given arguments and streams. It
// returns an error rather than exiting, so tests can drive it.
//
// SIGINT and SIGTERM cancel the context every command runs under, which flows to
// each HTTP request and subprocess: a hung `doctor --online` or a slow git
// command stops on the first Ctrl+C rather than needing a second, harder signal.
// The interface puts the terminal in raw mode, where Ctrl+C arrives as a key
// rather than a signal, so this does not fight Bubble Tea's own handling.
func Execute(args []string, stdout, stderr io.Writer, prompt Prompt) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	return execute(ctx, args, stdout, stderr, prompt)
}

// execute runs the command tree under ctx, so a test can pass a context it
// controls without raising a real signal.
func execute(ctx context.Context, args []string, stdout, stderr io.Writer, prompt Prompt) error {
	// The command tree carries no context; ctx reaches each command through
	// Cobra's ExecuteContext below, which is how cmd.Context() is set.
	root := NewRootCmd(prompt) //nolint:contextcheck // ctx is delivered by ExecuteContext, not the constructor.
	root.SetArgs(args)
	root.SetOut(stdout)
	root.SetErr(stderr)

	err := root.ExecuteContext(ctx)
	if err != nil {
		return fmt.Errorf("workflow: %w", err)
	}

	return nil
}

// runTUI starts the interface and blocks until the user quits. It is tui.Run in
// production and a fake in tests, so the root command's own wiring — loading the
// configuration, building the model, applying dry run — can be exercised without
// a real terminal.
type runTUI func(ctx context.Context, model tui.Model, out io.Writer) error

// runWeb starts the local web server and blocks until the context is canceled.
// It is serveWeb in production and a fake in tests, so the --web flag's wiring
// can be exercised without binding a port.
type runWeb func(ctx context.Context, cfg config.Config, deps webserver.Deps, info webserver.Info, out io.Writer) error

// NewRootCmd builds the command tree. Bare `workflow` opens the TUI. The prompt
// is how `config init` asks for credentials; a zero one is fine for a caller
// that only walks the tree, such as the reference generator.
func NewRootCmd(prompt Prompt) *cobra.Command {
	return newRootCmd(prompt, tui.Run, serveWeb)
}

// newRootCmd builds the command tree over an injected interface runner and web
// server.
func newRootCmd(prompt Prompt, run runTUI, serve runWeb) *cobra.Command {
	var (
		dryRun  bool
		logFile string
		web     bool
	)

	root := &cobra.Command{
		Use:           "workflow",
		Short:         "Run your Jira, Slack, and Git forge workflow from the terminal",
		Long:          longHelp,
		Version:       buildinfo.Current(),
		SilenceUsage:  true,
		SilenceErrors: true,
		Args:          cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, loadErr := loadFromEnvironment()
			ctx := cmd.Context()

			dir, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("determining the working directory: %w", err)
			}

			requestLog, closeLog, err := openRequestLog(logFile)
			if err != nil {
				return err
			}
			defer closeLog()

			deps := wiring.Deps(ctx, cfg, wiring.Locate(ctx, dir), requestLog)

			if web {
				if loadErr != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "workflow web: configuration did not load cleanly: %v\n", loadErr)
				}

				info := webserver.Info{Version: buildinfo.Current(), DryRun: dryRun}

				return serve(ctx, cfg, webDeps(deps), info, cmd.OutOrStdout())
			}

			model := tui.New(cfg, loadErr, deps)
			if dryRun {
				model = model.WithDryRun()
			}

			return run(ctx, model, cmd.OutOrStdout())
		},
	}

	root.Flags().BoolVar(&dryRun, "dry-run", false,
		"hold back every write to Jira, the forge, Slack, git and files, and say what it would have done")
	root.Flags().StringVar(&logFile, "log", "",
		"append a one-line outline of each request (method, path, status, duration) to FILE, for a bug report")
	root.Flags().BoolVar(&web, "web", false,
		"serve the web interface on http://"+webserver.LoopbackAddr+" instead of opening the terminal interface")

	root.AddCommand(newConfigCmd(prompt), newDoctorCmd(), newStatusCmd(), newStandupCmd(prompt), newReviewsCmd())

	return root
}

// serveWeb builds the web server over the seams and serves it on the loopback
// interface until the context is canceled. Handler fails only when the embedded
// spec cannot load, which is a build defect rather than a runtime condition.
func serveWeb(
	ctx context.Context, cfg config.Config, deps webserver.Deps, info webserver.Info, out io.Writer,
) error {
	assets, _ := web.Assets()

	handler, err := webserver.Handler(deps, cfg, info, assets)
	if err != nil {
		return fmt.Errorf("building the web server: %w", err)
	}

	fmt.Fprintf(out, "workflow web: serving http://%s — press Ctrl+C to stop\n", webserver.LoopbackAddr)

	return webserver.Serve(ctx, webserver.LoopbackAddr, handler)
}

// webDeps adapts the interface's dependency bundle to the web server's narrower
// one. They are the same seams, which is why the web server is another consumer
// of the wiring rather than a second implementation.
func webDeps(deps tui.Deps) webserver.Deps {
	return webserver.Deps{
		Search:       deps.Jira.Search,
		Issue:        deps.Jira.Issue,
		BrowseURL:    deps.Jira.BrowseURL,
		Branch:       deps.Git.Branch,
		Branches:     deps.Git.Branches,
		Checkout:     deps.Git.Checkout,
		CreateBranch: deps.Git.CreateBranch,
		Commit:       deps.Git.Commit,
		Push:         deps.Git.Push,
		Changes:      deps.Git.Changes,
		FindPull:     deps.Forge.FindPullRequest,
		CheckCI:      deps.Forge.CheckStatus,
		Author:       deps.Forge.Author,
		Post:         deps.Slack.Post,
	}
}

// logFileMode is the permission a request log is created with. Like the
// configuration, it is the user's own file and nobody else's to read.
const logFileMode os.FileMode = 0o600

// openRequestLog opens the request-outline log named by path. An empty path
// means no logging: the log is nil and the returned close is a no-op, so the
// caller wires and closes it the same way either way.
func openRequestLog(path string) (*wiring.RequestLog, func(), error) {
	if path == "" {
		return nil, func() {}, nil
	}

	//nolint:gosec // the path is the user's own --log argument, by design.
	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, logFileMode)
	if err != nil {
		return nil, func() {}, fmt.Errorf("opening the request log: %w", err)
	}

	return wiring.NewRequestLog(file, nil), func() { _ = file.Close() }, nil
}

// loadFromEnvironment loads the configuration that applies to this process,
// searching the working directory and then the home directory. A missing or
// unreadable file is reported through the error; the zero Config is still
// usable, which is what lets doctor and the TUI explain what is wrong.
func loadFromEnvironment() (config.Config, error) {
	workDir, err := os.Getwd()
	if err != nil {
		return config.Config{}, fmt.Errorf("determining the working directory: %w", err)
	}

	// A machine without a home directory is unusual but not fatal: the working
	// directory alone is still a valid place to find a configuration.
	homeDir, err := os.UserHomeDir()
	if err != nil {
		homeDir = ""
	}

	return config.Load(workDir, homeDir)
}
