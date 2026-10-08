// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"errors"
	"fmt"
	"io"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/gitrepo"
	"github.com/jacob-delgado/workflow/internal/jira"
	"github.com/jacob-delgado/workflow/internal/loop"
	"github.com/jacob-delgado/workflow/internal/wiring"
)

// statusSeams are what `workflow status` reads to build its line. They are
// seams so a test can answer without a repository or a network.
type statusSeams struct {
	Branch      func() (gitrepo.Branch, error)
	Changes     func() ([]gitrepo.Change, error)
	FindPull    func(branch string) (forge.PullRequest, bool, error)
	CheckStatus func(pull forge.PullRequest, head string) (forge.CI, error)
	Issue       func(issueKey jira.Key) (jira.IssueDetail, error)
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

// newStatusCmd builds `workflow status [directory...]`.
func newStatusCmd(prompt Prompt) *cobra.Command {
	var asJSON bool

	cmd := &cobra.Command{
		Use:   "status [directory...]",
		Short: "Print the current work's issue, stage and CI on one line",
		Long: "Print, on one line, the issue the branch is for, how far along the loop\n" +
			"the work has got, and how CI stands — the same progress the interface's\n" +
			"top row shows, for a shell prompt or a status bar. --json prints it as data.\n" +
			"A service that refuses to answer leaves its stage as though there were nothing\n" +
			"to say — CI none — and is named on standard error, one line each; so is one that\n" +
			"is not set up, such as a forge with no token, with how to set it up.\n\n" +
			"Given one or more directories, it prints a labeled line for each, so\n" +
			"`workflow status ~/src/*` reports every repository at once. Each reads its\n" +
			"own configuration. A directory that cannot be read still gets its line,\n" +
			"saying why, and the command then fails, as it does outside a repository: it\n" +
			"exits 3 when a directory's configuration does not load, otherwise 4 when one\n" +
			"is not a git repository — as bare status exits 4 outside one — otherwise 1.",
		Example: examples(
			`workflow status --json        # where the work stands, as data`,
			`workflow status ~/src/*       # every repository at once, a line each`,
		),
		Args: cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runStatusCommand(cmd, prompt, asJSON, args)
		},
	}

	cmd.Flags().BoolVar(&asJSON, "json", false, "print the status as JSON")

	return cmd
}

// runStatusCommand prints the status of the current repository, or of each
// named directory, noting on a terminal which directory it is reading.
func runStatusCommand(cmd *cobra.Command, prompt Prompt, asJSON bool, dirs []string) error {
	if len(dirs) == 0 {
		return statusHere(cmd, asJSON)
	}

	note := newProgressNote(cmd.ErrOrStderr(), prompt.IsTerminal)
	defer note.clear()

	return statusAcross(cmd, note, dirs, asJSON)
}

// statusHere prints the status of the current directory: a bare line, and a
// directory that is no repository is an error.
func statusHere(cmd *cobra.Command, asJSON bool) error {
	// A missing configuration is not fatal: the repository stages still read,
	// and the service stages simply stay not-started.
	conn, err := connect(cmd)
	if err != nil {
		return err
	}
	defer conn.closeLog()

	return runStatus(outputOf(cmd), seamsFor(conn), conn.cfg.UI.ASCII, asJSON)
}

// statusAcross prints one labeled status per named directory. A directory that
// cannot be read is noted in its row rather than stopping the rest, and then
// fails the command, as bare status fails outside a repository.
func statusAcross(cmd *cobra.Command, note *progressNote, dirs []string, asJSON bool) error {
	requestLog, closeLog, err := requestLogFor(cmd)
	if err != nil {
		return err
	}
	defer closeLog()

	out := note.around(outputOf(cmd))
	statuses := statusesOf(cmd, note, dirs, requestLog)

	if asJSON {
		err = statusesJSON(out.artifact, statuses)
	} else {
		statusLines(out.artifact, statuses)
	}

	for _, status := range statuses {
		writeNotes(out.notes, status.label+": ", status.facts.notes)
	}

	return errors.Join(err, unreadDirectories(statuses))
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
	label string
	facts statusFacts
	ascii bool
	err   error
}

// statusesOf gathers the status of the repository at each directory, which
// reads its own configuration, recording every request in one log and noting
// which directory it is reading.
func statusesOf(
	cmd *cobra.Command, note *progressNote, dirs []string, requestLog *wiring.RequestLog,
) []directoryStatus {
	home := configHome()
	statuses := make([]directoryStatus, 0, len(dirs))

	for _, dir := range dirs {
		note.show("Reading", repoLabel(dir))

		conn := connectAt(cmd, dir, home, requestLog)
		facts, err := statusOf(conn)

		statuses = append(statuses, directoryStatus{
			label: repoLabel(dir), facts: facts, ascii: conn.cfg.UI.ASCII, err: err,
		})
	}

	return statuses
}

// statusOf gathers the status of the repository a connection wired, refusing
// one whose configuration file could not be read.
func statusOf(conn connection) (statusFacts, error) {
	err := conn.unreadConfiguration()
	if err != nil {
		return statusFacts{}, err
	}

	return statusFromSeams(seamsFor(conn))
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

// repoLabel names a directory in the output: its base name, or the path itself
// when the base name would not say which repository it is.
func repoLabel(dir string) string {
	base := filepath.Base(dir)
	if base == "." {
		return dir
	}

	return base
}

// runStatus gathers the current state and prints it, as a line or as JSON,
// with a note on stderr for each service that refused to answer or is not set
// up.
func runStatus(out output, seams statusSeams, ascii, asJSON bool) error {
	facts, err := statusFromSeams(seams)
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

// writeNotes writes each note on a service, one line apiece after label.
func writeNotes(out io.Writer, label string, notes []string) {
	for _, note := range notes {
		fmt.Fprintln(out, label+note)
	}
}
