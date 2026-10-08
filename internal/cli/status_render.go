// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/jacob-delgado/workflow/internal/gitrepo"
	"github.com/jacob-delgado/workflow/internal/progress"
)

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
		marks[index] = stage.State.Glyph(ascii) + " " + stage.Name
	}

	line.WriteString(strings.Join(marks, "  "))
	line.WriteString("  CI " + facts.ci.Word())

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
		CI:      facts.ci.Word(),
	})
}

// repoStatus is one repository's status in the array `status DIR...` prints.
type repoStatus struct {
	Repository string        `json:"repository"`
	Dir        string        `json:"dir"`
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
		report := repoStatus{Repository: status.label, Dir: status.dir}

		if status.err != nil {
			report.Error = unreadReason(status.err)
		} else {
			facts := status.facts
			report.Issue, report.Summary = facts.issue, facts.summary
			report.Stages, report.CI = stageReports(facts.stages), facts.ci.Word()
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

// stateWord names a stage state for JSON.
func stateWord(state progress.State) string {
	return map[progress.State]string{
		progress.Done:       "done",
		progress.InFlight:   "in_flight",
		progress.Failed:     "failed",
		progress.NotStarted: "not_started",
	}[state]
}
