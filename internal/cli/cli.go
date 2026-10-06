// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

// Package cli builds the workflow command tree.
package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"os/signal"
	"strconv"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/jacob-delgado/workflow/internal/buildinfo"
	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/seams"
	"github.com/jacob-delgado/workflow/internal/tui"
	"github.com/jacob-delgado/workflow/internal/webserver"
	"github.com/jacob-delgado/workflow/internal/workdirs"
)

const longHelp = `workflow ties Jira, your Git forge and your team's messaging service into one
workflow, in the terminal or a browser.

CONFIGURATION

  workflow reads ` + config.FileName + ` from the current directory or the
  nearest directory above it, no higher than the repository root, and falls
  back to your home directory. A file found there is LAYERED over the one in
  your home directory, setting by setting: the repository's file holds only
  what that repository changes, and inherits the rest, tokens included.

  Set it up, answering the prompts, with:

      workflow config init

  (--template writes a blank file to edit instead, with the credentials
  below.) Then check your work with:

      workflow doctor

  Jira is optional. Leave jira.base_url empty and no Jira token is needed:
  your forge's issues are the tracker instead, and the Issues pane lists the
  open issues assigned to you there.

JIRA TOKEN (on-premises / Data Center)

  1. Sign in to your Jira instance in a browser.
  2. Open your avatar menu, then Profile, then Personal Access Tokens.
  3. Create a token, give it a name, and copy the value.
  4. Put it in ` + config.FileName + ` as jira.token, with your instance's URL
     as jira.base_url.

  Leave jira.user empty to authenticate with that token as a bearer token,
  which is what Data Center expects. Set jira.user only if your instance needs
  HTTP Basic authentication, in which case the token is used as the password.

MESSAGING — SLACK, TEAMS, DISCORD OR A PLAIN WEBHOOK

  messaging.kind picks the service: slack (the default when empty), teams,
  discord or webhook. Slack posts with a rotating user token or over an
  incoming webhook — one or the other, never both; the others post over an
  incoming webhook.

  Slack incoming webhook (simplest):

  1. Go to https://api.slack.com/apps and create an app in your workspace.
  2. Turn on Incoming Webhooks, then "Add New Webhook to Workspace" and pick
     the channel it will post to.
  3. Put the URL in ` + config.FileName + ` as messaging.webhook_url.

  The webhook is bound to the channel you picked, so messaging.channel does not
  apply. Treat the URL like a password: anyone holding it can post there.

  Slack user token (post as you, to the channel you choose at runtime):

  1. Go to https://api.slack.com/apps and create an app in your workspace.
  2. Under OAuth & Permissions, add the chat:write user token scope, and turn
     on token rotation. Tagging reviewers and user groups is optional and also
     needs users:read, channels:read, groups:read and usergroups:read; without
     them an announcement posts untagged.
  3. Install the app to the workspace. Its access token starts "xoxe.xoxp-"
     and lasts twelve hours; its refresh token starts "xoxe-1-".
  4. Set messaging.channel, then run "workflow slack login" with the app's
     client ID and secret (Basic Information) and the refresh token.

  workflow refreshes the token before it runs out and keeps each new one in
  the macOS keychain, or in ` + config.FileName + ` on Linux and Windows.

  Teams, Discord or a plain webhook: create an incoming webhook in the service,
  set messaging.kind, and put the URL in messaging.webhook_url.

SECURITY

  ` + config.FileName + ` holds live credentials. "workflow config init" writes
  it readable only by you and warns when the file is not ignored by git (add
  it to .gitignore), and "workflow config show" masks every one of them — including messaging.webhook_url, which is a
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
	// cobra adds its help and completion commands only as the tree runs, after
	// any walk could meet them. Adding them here lets markMisuse reach them too,
	// and only once the streams are set: a completion script keeps the stdout
	// its command was added under.
	root.InitDefaultHelpCmd()
	root.InitDefaultCompletionCmd()
	markMisuse(root)

	err := root.ExecuteContext(ctx)
	if err == nil {
		return nil
	}

	// A git or gh child stopped by the interrupt fails with its own exit
	// status, and a failed git read is then reported as no repository at all;
	// neither carries ctx's error, so put it back for ExitStatus to find.
	if ctx.Err() != nil {
		return fmt.Errorf("workflow: %w: %w", ctx.Err(), err)
	}

	return fmt.Errorf("workflow: %w", err)
}

// RunInterface starts the interface and blocks until the user quits, or asks
// to switch directory, which the Next it returns says. It is tui.Run in
// production and a fake in tests, so the root command's own wiring — loading
// the configuration, building the model, applying dry run and color off, and
// opening the next interface where a switch asks — can be exercised without a
// real terminal.
type RunInterface func(ctx context.Context, model tui.Model, out io.Writer) (tui.Next, error)

// RunWeb starts the local web server and blocks until the context is canceled.
// What it says about the server goes to notes, stderr: the server has no
// artifact for stdout to carry.
type RunWeb func(
	ctx context.Context, cfg config.Config, deps webserver.Deps, info webserver.Info, notes io.Writer,
) error

// RunWebAt is the web server that serves on addr. It is WebServerAt in
// production and a fake in tests, so the --web and --port flags' wiring can be
// exercised without binding a port.
type RunWebAt func(addr string) RunWeb

// NewRootCmd builds the command tree. Bare `workflow` opens the TUI. The prompt
// is how `config init` asks for credentials; a zero one is fine for a caller
// that only walks the tree, such as the reference generator.
func NewRootCmd(prompt Prompt) *cobra.Command {
	return NewRootCmdOver(prompt, tui.Run, WebServerAt)
}

// NewRootCmdOver builds the command tree over the interface and web server a
// caller hands it, so a test can see what bare `workflow` and `workflow --web`
// open without a terminal or a port.
func NewRootCmdOver(prompt Prompt, run RunInterface, serveAt RunWebAt) *cobra.Command {
	var (
		dryRun bool
		web    bool
		port   int
	)

	root := &cobra.Command{
		Use:           "workflow",
		Short:         "Run your Jira, Git forge and messaging workflow, in the terminal or a browser",
		Long:          longHelp,
		Version:       buildinfo.Current(),
		SilenceUsage:  true,
		SilenceErrors: true,
		Args:          cobra.NoArgs,
		PreRunE:       func(cmd *cobra.Command, _ []string) error { return checkPort(cmd, web, port) },
		RunE: func(cmd *cobra.Command, _ []string) error {
			conn, err := connectLeniently(cmd)
			if err != nil {
				return err
			}
			defer conn.closeLog()

			conn = conn.withSetup(cmd, prompt.StoreSecret)

			if web {
				return serveWeb(cmd, conn, serveAt(webserver.LoopbackAddr(port)), dryRun)
			}

			return runInterfaces(cmd, run, conn, dryRun)
		},
	}

	// Declared once, on the root, for every command: a script passes them before
	// the command's name or after it. Each command reads them back by name.
	root.PersistentFlags().BoolVar(&dryRun, dryRunFlag, false,
		"hold back every write to Jira, the forge, the messaging service, Taskwarrior, git and files, "+
			"and say what it would have done")
	root.PersistentFlags().String(logFlag, "",
		"append a one-line outline of each request (method, path, status, duration) to `FILE`, for a bug report")
	root.Flags().BoolVar(&web, "web", false,
		"serve the web interface on http://127.0.0.1 instead of opening the terminal interface")
	root.Flags().IntVar(&port, portFlag, webserver.DefaultPort, "the port --web serves on, from 1 to 65535")

	root.AddCommand(subcommands(prompt)...)

	return root
}

// interfaceInput bundles what opening the terminal interface needs, so the
// launcher does not take a long list of positional arguments.
type interfaceInput struct {
	cfg          config.Config
	loadErr      error
	deps         tui.Deps
	resolveAhead func()
	dryRun       bool
	noColorEnv   string
	out          io.Writer
	// arrive is what the model is told before it runs: nothing for the first
	// interface, and where a switch went, or why it could not, after one.
	arrive func(tui.Model) tui.Model
}

// inputFor is what opening the interface wired by conn needs.
func inputFor(cmd *cobra.Command, conn connection, dryRun bool) interfaceInput {
	return interfaceInput{
		cfg: conn.cfg, loadErr: conn.loadErr, deps: conn.deps, resolveAhead: conn.controls.ResolveAhead,
		dryRun: dryRun, noColorEnv: os.Getenv("NO_COLOR"), out: cmd.OutOrStdout(),
		arrive: func(model tui.Model) tui.Model { return model },
	}
}

// runInterfaces opens the interface where conn is wired, and again wherever it
// asks to switch to, until one ends by quitting. A switch that cannot be made
// reopens the interface where it was, saying why, so it never exits.
//
// Trade-off TRADE-34: a switch ends one program and starts the next, wired
// afresh, rather than rewiring the one running.
func runInterfaces(cmd *cobra.Command, run RunInterface, conn connection, dryRun bool) error {
	input := inputFor(cmd, conn, dryRun)

	for {
		next, err := openInterface(cmd.Context(), run, input)
		if err != nil || next.Dir == "" {
			return err
		}

		moved, err := switchTo(cmd, conn, next.Dir)
		if err != nil {
			input.arrive = func(model tui.Model) tui.Model { return model.StayedAfter(next, err) }

			continue
		}

		conn = moved
		input = inputFor(cmd, conn, dryRun)
		input.arrive = func(model tui.Model) tui.Model { return model.Arrived(next) }
	}
}

// errKeysRefused is a directory whose ui.keys the interface would refuse.
var errKeysRefused = errors.New("the configuration there binds keys workflow refuses")

// switchTo wires the directory a switch asked for, and moves there once its
// configuration is one the interface can open on: a directory not there, or
// keys the interface would refuse, leave the working directory as it was.
func switchTo(cmd *cobra.Command, from connection, dir string) (connection, error) {
	conn, err := wireAt(cmd, from, dir)
	if err != nil {
		return connection{}, err
	}

	return conn, moveTo(dir)
}

// wireAt wires dir as from was wired, to the same request log, refusing a
// directory that is not there and keys the interface would refuse. It moves
// nothing.
func wireAt(cmd *cobra.Command, from connection, dir string) (connection, error) {
	err := workdirs.Check(dir)
	if err != nil {
		return connection{}, err
	}

	conn := connectAt(cmd, dir, configHome(), from.requestLog)

	err = tui.CheckKeys(conn.cfg.UI.Keys)
	if err != nil {
		return connection{}, fmt.Errorf("%w: %w", errKeysRefused, err)
	}

	conn.closeLog = from.closeLog

	return conn.withSetup(cmd, from.keychain), nil
}

// moveTo makes dir the process's working directory, so whatever the new
// session runs — an editor, a hook — starts there.
func moveTo(dir string) error {
	err := os.Chdir(dir)
	if err != nil {
		return fmt.Errorf("moving to the directory: %w", err)
	}

	return nil
}

// openInterface refuses a broken ui.keys map before building anything — a keymap
// CheckKeys refuses should say so and stop, as a configuration problem, and not
// open an interface that answers the wrong keys — then builds the model, applies
// dry run and color off, finds the services' tokens while a token command can
// still ask on the terminal, and runs it.
func openInterface(ctx context.Context, run RunInterface, input interfaceInput) (tui.Next, error) {
	err := tui.CheckKeys(input.cfg.UI.Keys)
	if err != nil {
		return tui.Next{}, err
	}

	deps := input.deps
	if input.dryRun {
		// A dry-run interface opens no cache, and New would seed the issue
		// list from it before WithDryRun could drop it. It keeps the kept
		// reads, bound to the read-only store, so the announcement preview
		// says whom a post would tag and the Repositories pane lists the
		// favorites.
		deps.Store = seams.Store{
			OwnerLinks: deps.Store.OwnerLinks, RepoGroups: deps.Store.RepoGroups, LastGroups: deps.Store.LastGroups,
			Favorites: deps.Store.Favorites,
		}
	}

	model := tui.New(input.cfg, input.loadErr, deps)
	if input.dryRun {
		model = model.WithDryRun()
	}

	if !input.cfg.UI.DrawColor(input.noColorEnv) {
		model = model.WithoutColor()
	}

	input.resolveAhead()

	return run(ctx, input.arrive(model), input.out)
}

// subcommands are every `workflow` subcommand: the read commands, the guided
// config, and the scriptable write commands.
func subcommands(prompt Prompt) []*cobra.Command {
	return []*cobra.Command{
		newConfigCmd(prompt), newDoctorCmd(), newStatusCmd(), newSummaryCmd(prompt), newReviewsCmd(),
		newRepositoriesCmd(),
		newBranchCmd(prompt), newPRCmd(prompt), newAnnounceCmd(prompt), newCommentCmd(prompt),
		newSlackCmd(prompt), newDBCleanCmd(prompt),
	}
}

// checkPort refuses, as a mistake in the call, --port without --web, where
// nothing listens on it, and a port no listener can take.
func checkPort(cmd *cobra.Command, web bool, port int) error {
	var why string

	switch {
	case !web && cmd.Flags().Changed(portFlag):
		why = "it takes effect only with --web"
	case port < 1 || port > math.MaxUint16:
		why = "it must be from 1 to 65535"
	default:
		return nil
	}

	return fmt.Errorf(`%w %q for "--port" flag: %s`, errUsage, strconv.Itoa(port), why)
}

// loadFromEnvironment loads the configuration that applies to this process,
// searching up from the working directory to the repository root and then the
// home directory. A missing or unreadable file is reported through the error;
// the zero Config is still usable, which is what lets doctor explain what is
// wrong.
func loadFromEnvironment() (config.Config, error) {
	// Trade-off TRADE-18: only Linux's tests reach this, since macOS still names
	// a working directory once it is removed.
	workDir, err := os.Getwd()
	if err != nil {
		return config.Config{}, fmt.Errorf("determining the working directory: %w", err)
	}

	return config.Load(workDir, configHome())
}

// configHome is the home directory a configuration falls back to, or "" for
// none. A machine without a home directory is unusual but not a failure: the
// working directory alone is still a valid place to find a configuration, and
// the lookup answers "" beside its error.
func configHome() string {
	home, _ := os.UserHomeDir()

	return home
}
