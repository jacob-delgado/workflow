// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

// Package cli builds the workflow command tree.
package cli

import (
	"context"
	"errors"
	"fmt"
	"image/color"
	"io"
	"os"
	"os/signal"
	"syscall"

	"charm.land/fang/v2"
	"charm.land/lipgloss/v2"
	"github.com/spf13/cobra"

	"github.com/jacob-delgado/workflow/internal/buildinfo"
	"github.com/jacob-delgado/workflow/internal/cli/console"
	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/lifetime"
	"github.com/jacob-delgado/workflow/internal/seams"
	"github.com/jacob-delgado/workflow/internal/tui"
	"github.com/jacob-delgado/workflow/internal/workdirs"
)

const longHelp = `workflow ties Jira, your Git forge and your team's messaging service into one
workflow, in the terminal or a browser.

CONFIGURATION

  workflow reads ` + config.FileName + ` from the current directory or, in a
  repository, the nearest directory above it up to the repository root, and
  falls back to your home directory. A file found there is LAYERED over the
  one in your home directory, setting by setting: the repository's file holds
  only what that repository changes, and inherits the rest, tokens included.

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
  it to .gitignore), and "workflow config show" masks every one of them —
  including messaging.webhook_url, which is a credential in its own right
  rather than merely an address.`

// Execute runs the command tree with the given arguments and streams, in env:
// the directory, home, variables, store and programs of the process it runs
// in, which no command reads for itself. A command that fails has its error
// said once, on stderr, tagged ERROR; Execute returns it rather than exiting,
// so tests can drive it and the caller can exit with its ExitStatus.
//
// SIGINT and SIGTERM cancel the context every command runs under, which flows to
// each HTTP request and subprocess: a hung `doctor --online` or a slow git
// command stops on the first Ctrl+C rather than needing a second, harder signal.
// The interface handles no signal itself: it puts the terminal in raw mode,
// where Ctrl+C arrives as a key, which quits it, and a signal ends it through
// this context, as it ends every other command. Once the last interface has
// quit, workflow waits, up to timing.request_timeout, for the writes it left
// in flight; a Ctrl+C then, which the restored terminal sends as SIGINT, or
// SIGTERM ends those writes' contexts and exits at once, waiting for none.
//
// A little work detaches from its asker's context, bounded, since stopping it
// halfway would lose what it was for: Settings' refresh of a Slack user
// token's secrets and their keeping in the keychain
// (wiring.PlaceSlackCredentials), as Slack spends the old pair when it
// answers, within timing.request_timeout and security's 30 seconds; and a
// post's record (loop.Deliver) and the store's open, local SQLite writes that
// wait at most five seconds for its lock. It runs to its end only where
// something waits for it, as a command's own call does, and as --web's drain
// does for a request it serves, within its five-second grace. In the terminal
// a quit waits, up to timing.request_timeout, for the refresh, which runs
// inside the settings save it is part of, for a post's record, itself a
// counted write, and for the store's open when a write to the store begins
// it, as part of that write; it does not wait for an open a read of the store
// begins (the announcements, the cached issue list, the last scope), as it
// waits for no read. A signal exits at once under all of it, and ends a post's
// record there as it ends every write the interface sends (TRADE-38 says what
// that can cost).
//
// The editor the interface hands the terminal to runs under no context:
// it is yours until you leave it, and a signal while it is open ends workflow
// once you do.
func Execute(args []string, stdout, stderr io.Writer, prompt Prompt, env Environment) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	// stop cancels ctx as well as the signals' diversion, once the last
	// interface has quit and the writes it left in flight have landed or been
	// given up on: whileOpen keeps a write still running out of that.
	defer stop()

	// The command tree carries no context; ctx reaches each command through
	// Cobra's ExecuteContext, which is how cmd.Context() is set.
	return execute(ctx, NewRootCmdOver(prompt, tui.Run, WebServerAt, env), invocation{
		args: args, stdout: stdout, stderr: stderr, getenv: env.Process.Getenv,
	})
}

// invocation is one run of the command tree: its arguments, the streams it
// writes, and the variables that decide how its notes are said.
type invocation struct {
	args           []string
	stdout, stderr io.Writer
	getenv         func(name string) string
}

// execute runs root as run asks under ctx, its help, usage and version said
// through fang, and says the error it ended with, if any, once. A run that
// succeeded but whose output went nowhere ends with that.
//
// Trade-off TRADE-44: fang capitalizes its titles and each flag's first word,
// and links mango and roff though the man page is off.
func execute(ctx context.Context, root *cobra.Command, run invocation) error {
	checked := prepare(root, run)

	var said error

	fail := func(err error) {
		said = checked.failed(interrupted(ctx, err))
		console.New(run.stderr, voiceOf(root, run.getenv)).Fail(said, ExitStatus(said))
	}

	err := fang.Execute(ctx, root, fang.WithVersion(buildinfo.Current()), fang.WithoutManpage(),
		fang.WithColorSchemeFunc(ansiScheme), fang.WithErrorHandler(func(_ io.Writer, _ fang.Styles, err error) {
			fail(err)
		}))
	if err == nil && checked.err != nil {
		fail(nil)
	}

	return said
}

// prepare readies root to run as run asks, writing its artifact to stdout
// through the check it returns.
func prepare(root *cobra.Command, run invocation) *checkedOutput {
	checked := &checkedOutput{out: run.stdout}

	root.SetArgs(run.args)
	root.SetOut(checked.stream())
	root.SetErr(console.NewStream(run.stderr))
	// cobra adds its help and completion commands only as the tree runs, after
	// any walk could meet them. Adding them here lets markMisuse reach them too,
	// and only once the streams are set: a completion script keeps the stdout
	// its command was added under.
	root.InitDefaultHelpCmd()
	root.InitDefaultCompletionCmd()
	markMisuse(root)

	return checked
}

// interrupted is err with ctx's error put back once ctx has ended. A git or
// gh child stopped by the interrupt fails with its own exit status, and a
// failed git read is then reported as no repository at all; neither carries
// ctx's error, so it is put back for ExitStatus to find.
func interrupted(ctx context.Context, err error) error {
	if err == nil || ctx.Err() == nil {
		return err
	}

	return fmt.Errorf("%w: %w", ctx.Err(), err)
}

// ansiScheme is the help's colors, from the terminal's own sixteen: its text
// in the terminal's default color, and descriptions, comments and flag
// defaults in its dim gray. Nothing else is colored, since none of it is
// identity or state; the errors are the notes', in red.
func ansiScheme(lipgloss.LightDarkFunc) fang.ColorScheme {
	plain, dim := lipgloss.NoColor{}, lipgloss.BrightBlack

	return fang.ColorScheme{
		Base: plain, Title: plain, Description: dim, Codeblock: plain, Program: plain, DimmedArgument: plain,
		Comment: dim, Flag: plain, FlagDefault: dim, Command: plain, QuotedString: plain, Argument: plain,
		Help: plain, Dash: plain, ErrorHeader: [2]color.Color{lipgloss.Red, plain}, ErrorDetails: lipgloss.Red,
	}
}

// RunInterface starts the interface and blocks until the user quits, or asks
// to switch directory, which the Next it returns says. It is tui.Run in
// production and a fake in tests, so the root command's own wiring — loading
// the configuration, building the model, applying dry run and color off, and
// opening the next interface where a switch asks — can be exercised without a
// real terminal.
type RunInterface func(ctx context.Context, model tui.Model, in io.Reader, out io.Writer) (tui.Next, error)

// NewRootCmd builds the command tree to walk, as the reference generator
// does: it runs no command, so it is handed no Environment to run one in, and
// a command run from it anyway says so. The prompt is how `config init` asks
// for credentials; a zero one is fine.
func NewRootCmd(prompt Prompt) *cobra.Command {
	return NewRootCmdOver(prompt, tui.Run, WebServerAt, noEnvironment())
}

// NewRootCmdOver builds the command tree over the interface and web server a
// caller hands it, so a test can see what bare `workflow` and `workflow --web`
// open without a terminal or a port, and has every command run in env.
func NewRootCmdOver(prompt Prompt, run RunInterface, serveAt RunWebAt, env Environment) *cobra.Command {
	var flags rootFlags

	opening := surfaces{run: run, serveAt: serveAt}
	root := &cobra.Command{
		Use:               "workflow",
		Short:             "Run your Jira, Git forge and messaging workflow, in the terminal or a browser",
		Long:              longHelp,
		Version:           buildinfo.Current(),
		SilenceUsage:      true,
		SilenceErrors:     true,
		Args:              cobra.NoArgs,
		PersistentPreRunE: func(cmd *cobra.Command, _ []string) error { return handOver(cmd, env) },
		PreRunE:           func(cmd *cobra.Command, _ []string) error { return checkPort(cmd, flags.web, flags.port) },
		RunE:              func(cmd *cobra.Command, _ []string) error { return opening.open(cmd, prompt, flags) },
	}

	flags.declare(root)
	root.AddCommand(subcommands(prompt)...)

	return root
}

// surfaces are what bare workflow can open: the terminal interface, or the
// web server on a port.
type surfaces struct {
	run     RunInterface
	serveAt RunWebAt
}

// open wires the working directory, offering a first run where it works, and
// opens the web server where flags ask for it and the interface otherwise.
func (s surfaces) open(cmd *cobra.Command, prompt Prompt, flags rootFlags) error {
	conn, err := connectLeniently(cmd)
	if err != nil {
		return err
	}
	defer conn.closeLog()

	conn = conn.withSetup(cmd, prompt.StoreSecret)

	if flags.web {
		return serveWeb(cmd, conn, s.serveAt, flags)
	}

	return runInterfaces(cmd, s.run, conn, flags.dryRun)
}

// interfaceInput bundles what opening the terminal interface needs, so the
// launcher does not take a long list of positional arguments.
type interfaceInput struct {
	cfg          config.Config
	loadErr      error
	deps         tui.Deps
	resolveAhead func(ctx context.Context)
	dryRun       bool
	noColorEnv   string
	in           io.Reader
	out          io.Writer
	// arrive is what the model is told before it runs: nothing for the first
	// interface, and where a switch went, or why it could not, after one.
	arrive func(tui.Model) tui.Model
}

// inputFor is what opening the interface wired by conn needs.
func inputFor(cmd *cobra.Command, conn connection, dryRun bool) interfaceInput {
	return interfaceInput{
		cfg: conn.cfg, loadErr: conn.loadErr, deps: conn.deps, resolveAhead: conn.controls.ResolveAhead,
		dryRun: dryRun, noColorEnv: environmentOf(cmd).Process.Getenv("NO_COLOR"), in: cmd.InOrStdin(),
		out:    terminalOut(cmd),
		arrive: func(model tui.Model) tui.Model { return model },
	}
}

// runInterfaces opens the interface where conn is wired, and again wherever it
// asks to switch to, until one ends by quitting. A switch that cannot be made
// reopens the interface where it was, saying why, so it never exits. Every
// interface counts the writes it sends in one lifetime.InFlight, and once the
// last has quit, runInterfaces waits for those still in flight to land: a
// switch does not wait, and its writes are waited for at the quit.
//
// Trade-off TRADE-34: a switch ends one program and starts the next, wired
// afresh, rather than rewiring the one running.
func runInterfaces(cmd *cobra.Command, run RunInterface, conn connection, dryRun bool) error {
	program, closed := whileOpen(cmd.Context())
	defer closed()

	writes := lifetime.NewInFlight(writesSettle)
	program = lifetime.Counting(program, writes)
	input := inputFor(cmd, conn, dryRun)

	for {
		next, err := openInterface(program, run, input)
		if err != nil || next.Dir == "" {
			return errors.Join(err, finishWrites(cmd, writes, requestTimeout(conn.cfg)))
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

// whileOpen is the lifetime of the interfaces and of every write they send,
// which tui.Run ends with it: ctx's values, ending when ctx does until closed
// is called, and never after. A signal while an interface is open, or while
// workflow waits for its writes after the last has quit, stops the program
// and its writes, as it stops every command; Execute's own end, once that
// wait is over, cancels ctx too, and must not cut a write it gave up on.
func whileOpen(ctx context.Context) (context.Context, func() bool) {
	// Trade-off TRADE-38: a write the last interface left running goes on past
	// its quit, and workflow waits for it, up to timing.request_timeout.
	lifetime, end := context.WithCancelCause(context.WithoutCancel(ctx))

	return lifetime, context.AfterFunc(ctx, func() { end(context.Cause(ctx)) })
}

// errKeysRefused is a directory whose ui.keys the interface would refuse.
var errKeysRefused = errors.New("the configuration there binds keys workflow refuses")

// switchTo wires the directory a switch asked for, and moves there once its
// configuration is one the interface can open on: a directory not there, or
// keys the interface would refuse, leave the working directory as it was.
func switchTo(cmd *cobra.Command, from connection, dir string) (connection, error) {
	conn, err := wireAt(cmd.Context(), cmd, from, dir)
	if err != nil {
		return connection{}, err
	}

	return conn, moveTo(cmd, dir)
}

// wireAt wires dir as from was wired, under ctx, to the same request log,
// refusing a directory that is not there and keys the interface would refuse.
// It moves nothing.
func wireAt(ctx context.Context, cmd *cobra.Command, from connection, dir string) (connection, error) {
	err := workdirs.Check(dir)
	if err != nil {
		return connection{}, err
	}

	conn := connectAt(ctx, cmd, dir, from.requestLog)

	err = tui.CheckKeys(conn.cfg.UI.Keys)
	if err != nil {
		return connection{}, fmt.Errorf("%w: %w", errKeysRefused, err)
	}

	conn.closeLog = from.closeLog

	return conn.withSetup(cmd, from.keychain), nil
}

// moveTo makes dir the process's working directory, so whatever the new
// session runs — an editor, a hook — starts there.
func moveTo(cmd *cobra.Command, dir string) error {
	err := environmentOf(cmd).Chdir(dir)
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

	input.resolveAhead(ctx)

	return run(ctx, input.arrive(model), input.in, input.out)
}

// subcommands are every `workflow` subcommand: the read commands, the guided
// config, and the scriptable write commands.
func subcommands(prompt Prompt) []*cobra.Command {
	return []*cobra.Command{
		newConfigCmd(prompt), newDoctorCmd(prompt), newStatusCmd(prompt), newSummaryCmd(prompt), newReviewsCmd(),
		newRepositoriesCmd(), newIssuesCmd(), newIssueCmd(),
		newBranchCmd(prompt), newPRCmd(prompt), newFinishCmd(prompt), newAnnounceCmd(prompt), newCommentCmd(prompt),
		newTransitionCmd(prompt), newAssignCmd(prompt), newWorklogCmd(prompt),
		newSlackCmd(prompt), newDBCleanCmd(prompt),
	}
}

// loadFromEnvironment loads the configuration that applies where cmd runs,
// searching up from its working directory to the repository root and then
// the home directory. A missing or unreadable file, or a working directory
// that cannot be named, is reported through the error.
func loadFromEnvironment(cmd *cobra.Command) (config.Config, error) {
	workDir, err := workingDir(cmd)
	if err != nil {
		return config.Config{}, err
	}

	return config.Load(workDir, configHome(cmd))
}

// configHome is the home directory a configuration falls back to, or "" for
// none. A machine without a home directory is unusual but not a failure: the
// working directory alone is still a valid place to find a configuration.
func configHome(cmd *cobra.Command) string {
	return environmentOf(cmd).Process.Home
}
