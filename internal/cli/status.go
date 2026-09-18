// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"

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
	Issue       func(issueKey string) (jira.IssueDetail, error)
}

// newStatusCmd builds `workflow status`.
func newStatusCmd() *cobra.Command {
	var asJSON bool

	cmd := &cobra.Command{
		Use:   "status",
		Short: "Print the current work's issue, stage and CI on one line",
		Long: "Print, on one line, the issue the branch is for, how far along the loop\n" +
			"the work has got, and how CI stands — the same progress the interface's\n" +
			"top row shows, for a shell prompt or a status bar. --json prints it as data.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runStatusCommand(cmd, asJSON)
		},
	}

	cmd.Flags().BoolVar(&asJSON, "json", false, "print the status as JSON")

	return cmd
}

// runStatusCommand wires the real repository, forge and Jira to status.
func runStatusCommand(cmd *cobra.Command, asJSON bool) error {
	// A missing or broken configuration is not fatal: the repository stages
	// still read, and the service stages simply stay not-started.
	cfg, _ := loadFromEnvironment()
	ctx := cmd.Context()

	dir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("determining the working directory: %w", err)
	}

	where := wiring.Locate(ctx, dir)
	deps := wiring.Deps(ctx, cfg, where, nil)

	seams := statusSeams{
		Branch:      func() (gitrepo.Branch, error) { return gitrepo.ReadBranch(ctx, proc.Run, where.Root) },
		Changes:     func() ([]gitrepo.Change, error) { return gitrepo.Status(ctx, proc.Run, where.Root) },
		FindPull:    deps.Forge.FindPullRequest,
		CheckStatus: deps.Forge.CheckStatus,
		Issue:       deps.Jira.Issue,
	}

	return runStatus(cmd.OutOrStdout(), seams, cfg.UI.ASCII, asJSON)
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
	branch, err := seams.Branch()
	if err != nil {
		return fmt.Errorf("reading the branch: %w", err)
	}

	facts := gather(seams, branch)

	if asJSON {
		return renderStatusJSON(out, facts)
	}

	renderStatusLine(out, facts, ascii)

	return nil
}

// gather reads the issue, the pull request and its CI, and derives the stages.
// A service that will not answer leaves its stage not-started rather than
// failing the whole line.
func gather(seams statusSeams, branch gitrepo.Branch) statusFacts {
	issueKey, named := convention.IssueKey(branch.Name)
	facts := statusFacts{issue: issueKey}

	if named {
		detail, err := seams.Issue(issueKey)
		if err == nil {
			facts.summary = detail.Issue.Summary
		}
	}

	onFeature := branch.Name != "" && branch.Name != strings.TrimPrefix(branch.Base, "origin/")
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
	report := statusReport{
		Issue:   facts.issue,
		Summary: facts.summary,
		Stages:  stageReports(facts.stages),
		CI:      ciWord(facts.ci),
	}

	encoder := json.NewEncoder(out)
	encoder.SetIndent("", "  ")

	err := encoder.Encode(report)
	if err != nil {
		return fmt.Errorf("encoding the status: %w", err)
	}

	return nil
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
