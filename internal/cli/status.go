// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/convention"
	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/gitrepo"
	"github.com/jacob-delgado/workflow/internal/jira"
	"github.com/jacob-delgado/workflow/internal/proc"
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
	// Project is the configured Jira project; empty falls back to the shape guard.
	Project string
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
			"own configuration.",
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
	ctx := cmd.Context()
	out := cmd.OutOrStdout()

	// An unknown home directory just means no home-directory fallback for a
	// repository's configuration, not a failure.
	home, _ := os.UserHomeDir()

	if len(dirs) == 0 {
		return statusHere(ctx, out, home, asJSON)
	}

	return statusAcross(ctx, out, home, dirs, asJSON)
}

// statusHere prints the status of the current directory: a bare line, and a
// directory that is no repository is an error.
func statusHere(ctx context.Context, out io.Writer, home string, asJSON bool) error {
	dir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("determining the working directory: %w", err)
	}

	seams, ascii := seamsFor(ctx, dir, home)

	return runStatus(out, seams, ascii, asJSON)
}

// statusAcross prints one labeled status per named directory. A directory that
// is no repository is noted rather than failing the rest.
func statusAcross(ctx context.Context, out io.Writer, home string, dirs []string, asJSON bool) error {
	if asJSON {
		return statusesJSON(ctx, out, home, dirs)
	}

	for _, dir := range dirs {
		facts, ascii, err := statusOf(ctx, dir, home)
		if err != nil {
			fmt.Fprintf(out, "%s  not a git repository\n", repoLabel(dir))

			continue
		}

		fmt.Fprintf(out, "%s  ", repoLabel(dir))
		renderStatusLine(out, facts, ascii)
	}

	return nil
}

// seamsFor wires the real repository, forge and Jira for the directory at dir,
// which reads its own configuration.
func seamsFor(ctx context.Context, dir, home string) (statusSeams, bool) {
	// A missing or broken configuration is not fatal: the repository stages
	// still read, and the service stages simply stay not-started.
	cfg, _ := config.Load(dir, home)
	where := wiring.Locate(ctx, dir)
	deps := wiring.Deps(ctx, cfg, where, nil)

	repo := gitrepo.At(proc.Run, where.Root)
	seams := statusSeams{
		Branch:      func() (gitrepo.Branch, error) { return repo.ReadBranch(ctx) },
		Changes:     func() ([]gitrepo.Change, error) { return repo.Status(ctx) },
		FindPull:    deps.Forge.FindPullRequest,
		CheckStatus: deps.Forge.CheckStatus,
		Issue:       deps.Jira.Issue,
		Project:     cfg.Jira.Project,
	}

	return seams, cfg.UI.ASCII
}

// statusOf gathers the status of the repository at dir.
func statusOf(ctx context.Context, dir, home string) (statusFacts, bool, error) {
	seams, ascii := seamsFor(ctx, dir, home)
	facts, err := statusFromSeams(seams)

	return facts, ascii, err
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

// gather reads the issue, the pull request and its CI, and derives the stages.
// A service that will not answer leaves its stage not-started rather than
// failing the whole line.
func gather(seams statusSeams, branch gitrepo.Branch) statusFacts {
	issueKey, named := convention.IssueKey(branch.Name, seams.Project)
	facts := statusFacts{issue: issueKey}

	if named {
		detail, err := seams.Issue(jira.Key(issueKey))
		if err == nil {
			facts.summary = detail.Issue.Summary
		}
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
	})

	return facts
}

// gatherReview looks for the branch's pull request and its CI, on a feature
// branch with a forge to ask.
func gatherReview(seams statusSeams, branch gitrepo.Branch, onFeature bool) (forge.PullRequest, bool, forge.CIState) {
	if !onFeature {
		return forge.PullRequest{}, false, forge.CINone
	}

	pull, found, err := seams.FindPull(branch.Name)
	if err != nil || !found {
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

// encodeJSON writes value as indented JSON, the one place a command encodes it.
func encodeJSON(out io.Writer, value any) error {
	encoder := json.NewEncoder(out)
	encoder.SetIndent("", "  ")

	err := encoder.Encode(value)
	if err != nil {
		return fmt.Errorf("encoding the output: %w", err)
	}

	return nil
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
// per repository, so a directory that is no repository is a row with an error
// rather than a failure of the whole command.
func statusesJSON(ctx context.Context, out io.Writer, home string, dirs []string) error {
	reports := make([]repoStatus, 0, len(dirs))

	for _, dir := range dirs {
		report := repoStatus{Repository: repoLabel(dir)}

		facts, _, err := statusOf(ctx, dir, home)
		if err != nil {
			report.Error = "not a git repository"
		} else {
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
