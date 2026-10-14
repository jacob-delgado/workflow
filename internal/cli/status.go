// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/jacob-delgado/workflow/internal/cli/console"
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

// errPlainBesideJSON refuses --plain beside --json, which prints no line for
// it to draw.
var errPlainBesideJSON = errors.New("--plain draws the line's marks and --json prints no line: " +
	"use one or the other")

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

// newStatusCmd builds `workflow status [directory...]`.
func newStatusCmd(prompt Prompt) *cobra.Command {
	var opts statusOptions

	cmd := &cobra.Command{
		Use:   "status [directory...]",
		Short: "Print the current work's issue, stage and CI on one line",
		Long: "Print, on one line, the issue the branch is for, how far along the loop\n" +
			"the work has got, and how CI stands — the same progress the interface's\n" +
			"top row shows, for a shell prompt or a status bar. --json prints it as data;\n" +
			"--plain draws its marks in ASCII, as ui.ascii does, for this run alone.\n" +
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
		Example: examples(
			`workflow status --json        # where the work stands, as data`,
			`workflow status ~/src/*       # every repository at once, a line each`,
		),
		Args: cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runStatusCommand(cmd, prompt, opts, args)
		},
	}

	addJSONFlag(cmd, &opts.asJSON, "print the status as JSON")
	cmd.Flags().BoolVar(&opts.plain, "plain", false,
		"draw the line's marks in ASCII, as ui.ascii does, whatever the configuration says")

	return cmd
}

// runStatusCommand prints the status of the current repository, or of each
// named directory, noting on a terminal which directory it is reading.
func runStatusCommand(cmd *cobra.Command, prompt Prompt, opts statusOptions, dirs []string) error {
	if opts.plain && opts.asJSON {
		return fmt.Errorf("%w: %w", errUsage, errPlainBesideJSON)
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
	conn, err := connect(cmd)
	if err != nil {
		return err
	}
	defer conn.closeLog()

	return runStatus(cmd.Context(), outputOf(cmd), seamsFor(conn), opts.ascii(conn.cfg.UI.ASCII), opts.asJSON)
}

// statusAcross prints one labeled status per named directory, once each has
// been read, keeping a live line of which it is reading while a person
// watches the terminal. A directory that cannot be read is noted in its row
// rather than stopping the rest, and then fails the command, as bare status
// fails outside a repository.
func statusAcross(cmd *cobra.Command, prompt Prompt, dirs []string, opts statusOptions) error {
	requestLog, closeLog, err := requestLogFor(cmd)
	if err != nil {
		return err
	}
	defer closeLog()

	var statuses []directoryStatus

	watchErr := waiting(cmd, prompt, func(_ output, live console.Wait) error {
		statuses = statusesOf(cmd, live, dirs, requestLog)

		return nil
	})

	out := outputOf(cmd)

	if opts.asJSON {
		err = statusesJSON(out.artifact, statuses)
	} else {
		statusLines(out.artifact, statuses, opts)
	}

	for _, status := range statuses {
		writeNotes(out.notes, status.label+": ", status.facts.notes)
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

		conn := connectAt(cmd.Context(), cmd, status.found, requestLog)
		statuses[index].facts, statuses[index].err = statusOf(cmd.Context(), conn)
		statuses[index].ascii = conn.cfg.UI.ASCII
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
		found, err := fromWorkingDir(cmd, dir)
		statuses[index] = directoryStatus{dir: dir, found: found, err: err}
		// An unnamed working directory leaves nothing found: the lead is then
		// the directory as given.
		leads[index] = filepath.Clean(cmp.Or(found, dir))
	}

	for index, label := range directoryLabels(leads, configHome(cmd)) {
		statuses[index].label = label
	}

	return statuses
}

// statusOf gathers the status of the repository a connection wired under ctx,
// refusing one whose configuration file could not be read.
func statusOf(ctx context.Context, conn connection) (statusFacts, error) {
	err := conn.unreadConfiguration()
	if err != nil {
		return statusFacts{}, err
	}

	return statusFromSeams(ctx, seamsFor(conn))
}

// seamsFor reads the repository, forge, Jira and store a connection wired.
func seamsFor(conn connection) statusSeams {
	return statusSeams{
		Branch:      conn.deps.Git.Branch,
		Changes:     conn.deps.Git.Changes,
		FindPull:    conn.deps.Forge.FindPullRequest,
		CheckStatus: conn.deps.Forge.CheckStatus,
		Issue:       conn.deps.Jira.Issue,
		Memory:      loop.AnnounceMemory{Recorded: conn.deps.Store.Announced},
		Project:     conn.cfg.Jira.Project,
		Service:     conn.cfg.Messaging.Service(),
		Forge:       conn.deps.Forge.Kind,
		Tracker:     trackerName(conn),
	}
}

// trackerName names the tracker the issue is read from: Jira when it is
// configured, and otherwise the forge, whose issues stand in for it.
func trackerName(conn connection) string {
	if conn.cfg.Jira.Configured() {
		return "Jira"
	}

	return forgeName(conn.deps.Forge.Kind)
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
func runStatus(ctx context.Context, out output, seams statusSeams, ascii, asJSON bool) error {
	facts, err := statusFromSeams(ctx, seams)
	if err != nil {
		return err
	}

	writeNotes(out.notes, "", facts.notes)

	if asJSON {
		return renderStatusJSON(out.artifact, facts)
	}

	renderStatusLine(out.artifact, facts, ascii)

	return nil
}

// writeNotes says each note on a service, one apiece after label.
func writeNotes(notes console.Notes, label string, said []string) {
	for _, note := range said {
		notes.Note(label + note)
	}
}
