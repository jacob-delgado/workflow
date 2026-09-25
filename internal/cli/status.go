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

	"github.com/jacob-delgado/workflow/internal/convention"
	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/gitrepo"
	"github.com/jacob-delgado/workflow/internal/jira"
	"github.com/jacob-delgado/workflow/internal/loop"
	"github.com/jacob-delgado/workflow/internal/progress"
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
}

// newStatusCmd builds `workflow status [directory...]`.
func newStatusCmd() *cobra.Command {
	var asJSON bool

	cmd := &cobra.Command{
		Use:   "status [directory...]",
		Short: "Print the current work's issue, stage and CI on one line",
		Long: "Print, on one line, the issue the branch is for, how far along the loop\n" +
			"the work has got, and how CI stands — the same progress the interface's\n" +
			"top row shows, for a shell prompt or a status bar. --json prints it as data.\n\n" +
			"Given one or more directories, it prints a labeled line for each, so\n" +
			"`workflow status ~/src/*` reports every repository at once. Each reads its\n" +
			"own configuration. A directory that cannot be read still gets its line,\n" +
			"saying why, and the command then fails, as it does outside a repository.",
		Args: cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runStatusCommand(cmd, asJSON, args)
		},
	}

	cmd.Flags().BoolVar(&asJSON, "json", false, "print the status as JSON")

	return cmd
}

// runStatusCommand prints the status of the current repository, or of each
// named directory.
func runStatusCommand(cmd *cobra.Command, asJSON bool, dirs []string) error {
	if len(dirs) == 0 {
		return statusHere(cmd, asJSON)
	}

	return statusAcross(cmd, dirs, asJSON)
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

	return runStatus(cmd.OutOrStdout(), seamsFor(conn), conn.cfg.UI.ASCII, asJSON)
}

// statusAcross prints one labeled status per named directory. A directory that
// cannot be read is noted in its row rather than stopping the rest, and then
// fails the command, as bare status fails outside a repository.
func statusAcross(cmd *cobra.Command, dirs []string, asJSON bool) error {
	requestLog, closeLog, err := requestLogFor(cmd)
	if err != nil {
		return err
	}
	defer closeLog()

	out := cmd.OutOrStdout()
	statuses := statusesOf(cmd.Context(), dirs, requestLog)

	if asJSON {
		err = statusesJSON(out, statuses)
	} else {
		statusLines(out, statuses)
	}

	return errors.Join(err, unreadDirectories(statuses))
}

// statusLines prints each directory's labeled line, or why it has none.
func statusLines(out io.Writer, statuses []directoryStatus) {
	for _, status := range statuses {
		fmt.Fprintf(out, "%s  ", status.label)

		if status.err != nil {
			fmt.Fprintln(out, unreadReason(status.err))

			continue
		}

		renderStatusLine(out, status.facts, status.ascii)
	}
}

// unreadReason says why a directory has no status: that it is no repository,
// or, for a repository that could not be read, the error itself.
func unreadReason(err error) string {
	if errors.Is(err, gitrepo.ErrNotARepository) {
		return gitrepo.ErrNotARepository.Error()
	}

	return err.Error()
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
// reads its own configuration, recording every request in one log.
func statusesOf(ctx context.Context, dirs []string, requestLog *wiring.RequestLog) []directoryStatus {
	// An unknown home directory just means no home-directory fallback for a
	// repository's configuration, not a failure.
	home, _ := os.UserHomeDir()
	statuses := make([]directoryStatus, 0, len(dirs))

	for _, dir := range dirs {
		conn := connectAt(ctx, dir, home, requestLog)
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
		Memory:      announceMemory(conn.deps.Store),
		Project:     conn.cfg.Jira.Project,
		Service:     conn.cfg.Messaging.Service(),
	}
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

// statusFacts is everything the line and the JSON are built from.
type statusFacts struct {
	issue   string
	summary string
	stages  []progress.Stage
	ci      forge.CIState
}

// runStatus gathers the current state and prints it, as a line or as JSON.
func runStatus(out io.Writer, seams statusSeams, ascii, asJSON bool) error {
	facts, err := statusFromSeams(seams)
	if err != nil {
		return err
	}

	if asJSON {
		return renderStatusJSON(out, facts)
	}

	renderStatusLine(out, facts, ascii)

	return nil
}

// statusFromSeams reads the branch and derives the facts, or fails when the
// branch cannot be read at all.
func statusFromSeams(seams statusSeams) (statusFacts, error) {
	branch, err := seams.Branch()
	if err != nil {
		return statusFacts{}, fmt.Errorf("reading the branch: %w", err)
	}

	return gather(seams, branch), nil
}

// gather reads the issue, the pull request, its CI and whether it was
// announced, and derives the stages.
// A service that will not answer leaves its stage not-started rather than
// failing the whole line.
func gather(seams statusSeams, branch gitrepo.Branch) statusFacts {
	issueKey, named := convention.IssueKey(branch.Name, seams.Project)
	facts := statusFacts{issue: issueKey}

	if named {
		facts.summary = issueSummary(seams, issueKey)
	}

	onFeature := branch.Name != "" && branch.Name != branch.BaseName()
	pull, found, ciState := gatherReview(seams, branch, onFeature)
	facts.ci = ciState

	facts.stages = progress.Stages(progress.Work{
		OnFeatureBranch:    onFeature,
		IssueNamed:         named,
		Commits:            len(branch.Commits),
		UncommittedChanges: countChanges(seams),
		PullRequestFound:   found,
		CI:                 ciState,
		ChangesRequested:   pull.ChangesRequested,
		Announced:          found && announcedNow(seams.Memory, pull, ciState),
	}, seams.Service)

	return facts
}

// issueSummary is the named issue's summary, or none when Jira will not answer.
func issueSummary(seams statusSeams, issueKey string) string {
	detail, err := seams.Issue(jira.Key(issueKey))
	if err != nil {
		return ""
	}

	return detail.Issue.Summary
}

// announcedNow reports that the store remembers the pull request announced at
// the moment it is at now, which is what the interface's top row reads too.
func announcedNow(memory loop.AnnounceMemory, pull forge.PullRequest, ciState forge.CIState) bool {
	return memory.Holds(loop.Announced{Pull: pull.Number, Moment: loop.AnnounceMoment(pull, forge.CI{State: ciState})})
}

// gatherReview looks for the branch's pull request and its CI, on a feature
// branch with a forge to ask.
func gatherReview(seams statusSeams, branch gitrepo.Branch, onFeature bool) (forge.PullRequest, bool, forge.CIState) {
	if !onFeature {
		return forge.PullRequest{}, false, forge.CINone
	}

	pull, found, err := seams.FindPull(branch.Name)
	if err != nil || !found || !pull.IsOpen() {
		// A merged pull request is found but has no live CI to poll, so the status
		// line treats a merged branch as having no open review.
		return forge.PullRequest{}, false, forge.CINone
	}

	status, err := seams.CheckStatus(pull, branch.Head)
	if err != nil {
		return pull, true, forge.CINone
	}

	return pull, true, status.State
}

// countChanges is how many files have uncommitted changes, or zero when they
// cannot be read.
func countChanges(seams statusSeams) int {
	changes, err := seams.Changes()
	if err != nil {
		return 0
	}

	return len(changes)
}

// renderStatusLine prints the issue, the stage glyphs and the CI state.
func renderStatusLine(out io.Writer, facts statusFacts, ascii bool) {
	var line strings.Builder

	if facts.issue != "" {
		line.WriteString(facts.issue)

		if facts.summary != "" {
			line.WriteString(" " + facts.summary)
		}

		line.WriteString("  ")
	}

	marks := make([]string, len(facts.stages))
	for index, stage := range facts.stages {
		marks[index] = statusGlyph(stage.State, ascii) + " " + stage.Name
	}

	line.WriteString(strings.Join(marks, "  "))
	line.WriteString("  CI " + ciWord(facts.ci))

	fmt.Fprintln(out, line.String())
}

// statusReport is the JSON shape of the status.
type statusReport struct {
	Issue   string        `json:"issue,omitempty"`
	Summary string        `json:"summary,omitempty"`
	Stages  []stageReport `json:"stages"`
	CI      string        `json:"ci"`
}

// stageReport is one stage as data.
type stageReport struct {
	Name  string `json:"name"`
	State string `json:"state"`
}

// renderStatusJSON prints the same facts as indented JSON.
func renderStatusJSON(out io.Writer, facts statusFacts) error {
	return encodeJSON(out, statusReport{
		Issue:   facts.issue,
		Summary: facts.summary,
		Stages:  stageReports(facts.stages),
		CI:      ciWord(facts.ci),
	})
}

// repoStatus is one repository's status in the array `status DIR...` prints.
type repoStatus struct {
	Repository string        `json:"repository"`
	Issue      string        `json:"issue,omitempty"`
	Summary    string        `json:"summary,omitempty"`
	Stages     []stageReport `json:"stages,omitempty"`
	CI         string        `json:"ci,omitempty"`
	Error      string        `json:"error,omitempty"`
}

// statusesJSON prints the status of each directory as a JSON array, one object
// per repository, so a directory that cannot be read is a row with an error
// rather than a gap in the array.
func statusesJSON(out io.Writer, statuses []directoryStatus) error {
	reports := make([]repoStatus, 0, len(statuses))

	for _, status := range statuses {
		report := repoStatus{Repository: status.label}

		if status.err != nil {
			report.Error = unreadReason(status.err)
		} else {
			facts := status.facts
			report.Issue, report.Summary = facts.issue, facts.summary
			report.Stages, report.CI = stageReports(facts.stages), ciWord(facts.ci)
		}

		reports = append(reports, report)
	}

	return encodeJSON(out, reports)
}

// stageReports turns the stages into their JSON shape.
func stageReports(stages []progress.Stage) []stageReport {
	reports := make([]stageReport, len(stages))
	for index, stage := range stages {
		reports[index] = stageReport{Name: stage.Name, State: stateWord(stage.State)}
	}

	return reports
}

// statusGlyph is the mark for a stage state, plain by default or ASCII.
func statusGlyph(state progress.State, ascii bool) string {
	switch state {
	case progress.Done:
		return pickGlyph(ascii, "●", "#")
	case progress.InFlight:
		return pickGlyph(ascii, "◐", "*")
	case progress.Failed:
		return pickGlyph(ascii, "✗", "x")
	case progress.NotStarted:
		return pickGlyph(ascii, "○", "o")
	default:
		return pickGlyph(ascii, "○", "o")
	}
}

// pickGlyph chooses the plain-ASCII mark where ascii is set.
func pickGlyph(ascii bool, unicode, plain string) string {
	if ascii {
		return plain
	}

	return unicode
}

// stateWord names a stage state for JSON.
func stateWord(state progress.State) string {
	switch state {
	case progress.Done:
		return "done"
	case progress.InFlight:
		return "in_flight"
	case progress.Failed:
		return "failed"
	case progress.NotStarted:
		return "not_started"
	default:
		return "not_started"
	}
}

// ciWord names how CI stands for the line and the JSON.
func ciWord(state forge.CIState) string {
	switch state {
	case forge.CIRunning:
		return "running"
	case forge.CIPassed:
		return "passed"
	case forge.CIFailed:
		return "failed"
	case forge.CINone:
		return "none"
	default:
		return "none"
	}
}
