// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

// Package status is `workflow status`: where the work in a repository stands
// along the loop, read from its branch, its forge, its tracker and the store,
// printed on one line or as JSON, for the current repository or for each
// directory named.
package status

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/jacob-delgado/workflow/internal/cli/cmdkit"
	"github.com/jacob-delgado/workflow/internal/cli/console"
	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/gitrepo"
	"github.com/jacob-delgado/workflow/internal/jira"
	"github.com/jacob-delgado/workflow/internal/loop"
	"github.com/jacob-delgado/workflow/internal/sanitize"
	"github.com/jacob-delgado/workflow/internal/wiring"
	"github.com/jacob-delgado/workflow/internal/workdirs"
)

// statusSeams are what `workflow status` reads to build its line. They are
// seams so a test can answer without a repository or a network.
type statusSeams struct {
	Branch      func(ctx context.Context) (gitrepo.Branch, error)
	Changes     func(ctx context.Context) ([]gitrepo.Change, error)
	FindPull    func(ctx context.Context, branch string) (forge.PullRequest, bool, error)
	CheckStatus func(ctx context.Context, pull forge.PullRequest, head string) (forge.CI, error)
	Issue       func(ctx context.Context, issueKey jira.Key) (jira.IssueDetail, error)
	// Memory is what the store remembers of the announcements made here.
	Memory loop.AnnounceMemory
	// Project is the configured Jira project; empty falls back to the shape guard.
	Project string
	// Service is the configured messaging service, which names the last stage.
	Service string
	// Forge is the forge the remote points at, which names it in a note.
	Forge forge.Kind
	// Tracker names the issue tracker in a note: Jira, or the forge when
	// its issues stand in for Jira's.
	Tracker string
}

// errPlainBesideJSON refuses --plain beside --json, which prints no status line
// for it to draw. The live line a JSON run of named directories keeps is drawn
// as ui.ascii asks, so the refusal names it.
var errPlainBesideJSON = errors.New("--plain draws the status line's marks and --json prints no status line: " +
	"use one or the other; ui.ascii draws a JSON run's live line in ASCII")

// statusOptions are status's flags: the JSON, and the line's marks drawn in
// ASCII.
type statusOptions struct {
	asJSON bool
	plain  bool
}

// ascii reports whether a line is drawn in ASCII: when --plain asks for it, or
// the configuration the line was read under does.
func (o statusOptions) ascii(configured bool) bool {
	return o.plain || configured
}

// look is how the live line is drawn while status reads named directories:
// as ui asks, and in ASCII when --plain does.
func (o statusOptions) look(ui config.UI) console.Look {
	look := cmdkit.LiveLook(ui)
	look.ASCII = o.ascii(ui.ASCII)

	return look
}

// NewCmd builds `workflow status [directory...]`.
func NewCmd(prompt cmdkit.Prompt) *cobra.Command {
	var opts statusOptions

	cmd := &cobra.Command{
		Use:   "status [directory...]",
		Short: "Print the current work's issue, stage and CI on one line",
		Long: "Print, on one line, the issue the branch is for, how far along the loop\n" +
			"the work has got, and how CI stands — the same progress the interface's\n" +
			"top row shows, for a shell prompt or a status bar. --json prints it as data;\n" +
			"--plain draws its marks, and the live line it keeps on a terminal while it\n" +
			"reads named directories, in ASCII, as ui.ascii does, for this run alone.\n" +
			"A service that refuses to answer leaves its stage as though there were nothing\n" +
			"to say — CI none — and is named on standard error, one line each; so is one that\n" +
			"is not set up, such as a forge with no token, with how to set it up.\n\n" +
			"Given one or more directories, it prints a line for each, labeled with its\n" +
			"name, or with its path where two share a name, so `workflow status ~/src/*`\n" +
			"reports every repository at once. Each reads its own configuration. A\n" +
			"directory that cannot be read still gets its line, saying why, and the\n" +
			"command then fails, as it does outside a repository: it exits 3 when a\n" +
			"directory's configuration does not load, otherwise 4 when one is not a git\n" +
			"repository — as bare status exits 4 outside one — otherwise 1.",
		Example: cmdkit.Examples(
			`workflow status --json        # where the work stands, as data`,
			`workflow status ~/src/*       # every repository at once, a line each`,
		),
		Args: cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runStatusCommand(cmd, prompt, opts, args)
		},
	}

	cmdkit.AddJSONFlag(cmd, &opts.asJSON, "print the status as JSON")
	cmd.Flags().BoolVar(&opts.plain, "plain", false,
		"draw the line's marks, and the live line's, in ASCII, as ui.ascii does, whatever the configuration says")

	return cmd
}

// runStatusCommand prints the status of the current repository, or of each
// named directory, noting on a terminal which directory it is reading.
func runStatusCommand(cmd *cobra.Command, prompt cmdkit.Prompt, opts statusOptions, dirs []string) error {
	if opts.plain && opts.asJSON {
		return fmt.Errorf("%w: %w", cmdkit.ErrUsage, errPlainBesideJSON)
	}

	if len(dirs) == 0 {
		return statusHere(cmd, opts)
	}

	return statusAcross(cmd, prompt, dirs, opts)
}

// statusHere prints the status of the current directory: a bare line, and a
// directory that is no repository is an error.
func statusHere(cmd *cobra.Command, opts statusOptions) error {
	// A missing configuration is not fatal: the repository stages still read,
	// and the service stages simply stay not-started.
	conn, err := cmdkit.Connect(cmd)
	if err != nil {
		return err
	}
	defer conn.CloseLog()

	return runStatus(cmd.Context(), cmdkit.OutputOf(cmd), seamsFor(conn), opts.ascii(conn.Config.UI.ASCII), opts.asJSON)
}

// statusAcross prints one labeled status per named directory, once each has
// been read, keeping a live line of which it is reading while a person
// watches the terminal. A directory that cannot be read is noted in its row
// rather than stopping the rest, and then fails the command, as bare status
// fails outside a repository.
func statusAcross(cmd *cobra.Command, prompt cmdkit.Prompt, dirs []string, opts statusOptions) error {
	requestLog, closeLog, err := cmdkit.RequestLogFor(cmd)
	if err != nil {
		return err
	}
	defer closeLog()

	// The live line is the command's own, drawn as the configuration where it
	// runs asks. One that cannot be read draws it as the defaults do: each
	// directory reads, and reports, its own.
	here, _ := cmdkit.LoadFromEnvironment(cmd)

	var statuses []directoryStatus

	watchErr := cmdkit.Waiting(cmd, prompt, opts.look(here.UI), func(_ cmdkit.Output, live console.Wait) error {
		statuses = statusesOf(cmd, live, dirs, requestLog)

		return nil
	})

	out := cmdkit.OutputOf(cmd)

	if opts.asJSON {
		err = statusesJSON(out.Artifact, statuses)
	} else {
		statusLines(out.Artifact, statuses, opts)
	}

	for _, status := range statuses {
		writeNotes(out.Notes, status.label+": ", status.facts.notes)
	}

	return errors.Join(err, unreadDirectories(statuses), watchErr)
}

// unreadDirectories joins the error of every directory that could not be read,
// each named, so the exit status tells a script one of them failed.
func unreadDirectories(statuses []directoryStatus) error {
	var failures []error

	for _, status := range statuses {
		if status.err != nil {
			failures = append(failures, fmt.Errorf("%s: %w", status.label, status.err))
		}
	}

	return errors.Join(failures...)
}

// directoryStatus is one named directory's status, or why it has none.
type directoryStatus struct {
	// dir is the directory as it was given, found where it leads from the
	// working directory, and label what names it in the output.
	dir   string
	found string
	label string
	facts statusFacts
	ascii bool
	err   error
}

// statusesOf gathers the status of the repository at each directory, which
// reads its own configuration, recording every request in one log and saying
// on the live line which directory it is reading, and how many it has read.
func statusesOf(
	cmd *cobra.Command, live console.Wait, dirs []string, requestLog *wiring.RequestLog,
) []directoryStatus {
	statuses := placed(cmd, dirs)

	for index, status := range statuses {
		live.Doing("Reading " + sanitize.Line(status.label))
		live.Counted(index, len(statuses))

		if status.err != nil {
			continue
		}

		conn := cmdkit.ConnectAt(cmd.Context(), cmd, status.found, requestLog)
		statuses[index].facts, statuses[index].err = statusOf(cmd.Context(), conn)
		statuses[index].ascii = conn.Config.UI.ASCII
	}

	return statuses
}

// placed finds where each directory given leads from the working directory,
// or keeps why it cannot, and labels each by where it leads, so that "." is
// named for the working directory rather than for the dot.
func placed(cmd *cobra.Command, dirs []string) []directoryStatus {
	statuses := make([]directoryStatus, len(dirs))
	leads := make([]string, len(dirs))

	for index, dir := range dirs {
		found, err := cmdkit.FromWorkingDir(cmd, dir)
		statuses[index] = directoryStatus{dir: dir, found: found, err: err}
		// An unnamed working directory leaves nothing found: the lead is then
		// the directory as given.
		leads[index] = filepath.Clean(cmp.Or(found, dir))
	}

	for index, label := range directoryLabels(leads, cmdkit.ConfigHome(cmd)) {
		statuses[index].label = label
	}

	return statuses
}

// statusOf gathers the status of the repository a connection wired under ctx,
// refusing one whose configuration file could not be read.
func statusOf(ctx context.Context, conn cmdkit.Connection) (statusFacts, error) {
	err := conn.UnreadConfiguration()
	if err != nil {
		return statusFacts{}, err
	}

	return statusFromSeams(ctx, seamsFor(conn))
}

// seamsFor reads the repository, forge, Jira and store a connection wired.
func seamsFor(conn cmdkit.Connection) statusSeams {
	return statusSeams{
		Branch:      conn.Deps.Git.Branch,
		Changes:     conn.Deps.Git.Changes,
		FindPull:    conn.Deps.Forge.FindPullRequest,
		CheckStatus: conn.Deps.Forge.CheckStatus,
		Issue:       conn.Deps.Jira.Issue,
		Memory:      loop.AnnounceMemory{Recorded: conn.Deps.Store.Announced},
		Project:     conn.Config.Jira.Project,
		Service:     conn.Config.Messaging.Service(),
		Forge:       conn.Deps.Forge.Kind,
		Tracker:     trackerName(conn),
	}
}

// trackerName names the tracker the issue is read from: Jira when it is
// configured, and otherwise the forge, whose issues stand in for it.
func trackerName(conn cmdkit.Connection) string {
	if conn.Config.Jira.Configured() {
		return "Jira"
	}

	return forgeName(conn.Deps.Forge.Kind)
}

// directoryLabels names each directory in the output by its base name or,
// where another of dirs has a base name the same, by its path, written from
// home.
func directoryLabels(dirs []string, home string) []string {
	named := make(map[string]int, len(dirs))
	for _, dir := range dirs {
		named[filepath.Base(dir)]++
	}

	labels := make([]string, len(dirs))

	for index, dir := range dirs {
		base := filepath.Base(dir)
		if named[base] > 1 {
			labels[index] = workdirs.Shown(dir, home)

			continue
		}

		labels[index] = base
	}

	return labels
}

// runStatus gathers the current state and prints it, as a line or as JSON,
// with a note on stderr for each service that refused to answer or is not set
// up. The store is read under ctx.
func runStatus(ctx context.Context, out cmdkit.Output, seams statusSeams, ascii, asJSON bool) error {
	facts, err := statusFromSeams(ctx, seams)
	if err != nil {
		return err
	}

	writeNotes(out.Notes, "", facts.notes)

	if asJSON {
		return renderStatusJSON(out.Artifact, facts)
	}

	renderStatusLine(out.Artifact, facts, ascii)

	return nil
}

// writeNotes says each note on a service, one apiece after label.
func writeNotes(notes console.Notes, label string, said []string) {
	for _, note := range said {
		notes.Note(label + note)
	}
}
