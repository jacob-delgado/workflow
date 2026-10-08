// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"errors"
	"fmt"
	"slices"

	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/gitrepo"
	"github.com/jacob-delgado/workflow/internal/jira"
	"github.com/jacob-delgado/workflow/internal/loop"
	"github.com/jacob-delgado/workflow/internal/progress"
	"github.com/jacob-delgado/workflow/internal/report"
	"github.com/jacob-delgado/workflow/internal/sanitize"
)

// statusFacts is everything the line and the JSON are built from, and the
// notes saying why each service that did not answer did not.
type statusFacts struct {
	issue   string
	summary string
	stages  []progress.Stage
	ci      forge.CIState
	notes   []string
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
// failing the whole line, and is named among the facts' notes.
func gather(seams statusSeams, branch gitrepo.Branch) statusFacts {
	issueRef, named := loop.IssueOf(branch, seams.Project)
	facts := statusFacts{issue: issueRef.Key}

	var issueErr error
	if named {
		facts.summary, issueErr = issueSummary(seams, issueRef.Key)
	}

	onFeature := branch.Name != "" && branch.Name != branch.BaseName()
	review := gatherReview(seams, branch, onFeature)
	changes, changesErr := seams.Changes()
	facts.ci = review.ci
	facts.notes = serviceNotes([]serviceRead{
		{name: seams.Tracker, err: issueErr},
		{name: forgeName(seams.Forge), err: review.err},
		{name: "The working tree", err: changesErr},
	})

	facts.stages = progress.Stages(progress.Work{
		OnFeatureBranch:    onFeature,
		IssueNamed:         named,
		Commits:            len(branch.Commits),
		UncommittedChanges: len(changes),
		PullRequest:        review.state,
		CI:                 review.ci,
		ChangesRequested:   review.pull.ChangesRequested,
		Announced:          review.state != progress.NoPullRequest && announcedNow(seams.Memory, review.pull, review.ci),
	}, seams.Service)

	return facts
}

// serviceRead is one service's read, by the name a note gives it, and why it
// failed, or nil.
type serviceRead struct {
	name string
	err  error
}

// serviceNotes says why each service that did not answer did not, in its
// name: first each that refused, in its own words, which may not drive the
// terminal; then each that is not set up — no credential, no forge the origin
// names — with how to set it up, in the words the web gives it, once however
// many reads met it. A tracker that says the branch's issue does not exist
// has answered, and is left out.
func serviceNotes(reads []serviceRead) []string {
	var unread, notSetUp []string

	for _, read := range reads {
		switch {
		case read.err == nil || errors.Is(read.err, jira.ErrNotFound):
		case loop.NotSetUp(read.err):
			note := notSetUpNote(read.name, read.err)
			if !slices.Contains(notSetUp, note) {
				notSetUp = append(notSetUp, note)
			}
		default:
			unread = append(unread, read.name+" could not be read: "+sanitize.Line(read.err.Error()))
		}
	}

	return append(unread, notSetUp...)
}

// notSetUpNote says a service is not set up, and how to set it up, in the
// words the web's problem gives it.
func notSetUpNote(name string, err error) string {
	return name + " is not set up: " + report.FaultDetail(err)
}

// forgeName names the forge as the subject of a note: by its name when the
// remote says which, and as the forge otherwise.
func forgeName(kind forge.Kind) string {
	if kind == forge.KindUnknown {
		return "The forge"
	}

	return kind.String()
}

// issueSummary is the named issue's summary, or none, with why, when Jira
// will not answer.
func issueSummary(seams statusSeams, issueKey string) (string, error) {
	detail, err := seams.Issue(jira.Key(issueKey))
	if err != nil {
		return "", err
	}

	return detail.Issue.Summary, nil
}

// announcedNow reports that the store remembers the pull request announced at
// the moment it is at now, which is what the interface's top row reads too.
func announcedNow(memory loop.AnnounceMemory, pull forge.PullRequest, ciState forge.CIState) bool {
	return memory.Holds(loop.Announced{Pull: pull.Number, Moment: loop.AnnounceMoment(pull, forge.CI{State: ciState})})
}

// reviewRead is what the forge said of the branch's pull request: the pull,
// where its review stands, its CI, and why the forge would not say, or nil.
type reviewRead struct {
	pull  forge.PullRequest
	state progress.PullState
	ci    forge.CIState
	err   error
}

// gatherReview looks for the branch's pull request, where it stands and its
// CI, on a feature branch with a forge to ask. A read that fails reads as no
// pull request, or no CI, with its error beside.
func gatherReview(seams statusSeams, branch gitrepo.Branch, onFeature bool) reviewRead {
	none := reviewRead{state: progress.NoPullRequest, ci: forge.CINone}
	if !onFeature {
		return none
	}

	pull, found, err := seams.FindPull(branch.Name)
	if err != nil || !found {
		none.err = err

		return none
	}

	if !pull.IsOpen() {
		// A merged pull request has no live CI to poll: its review is over, as
		// the interface's spine and rail say too.
		return reviewRead{pull: pull, state: progress.PullStateOf(pull.State), ci: forge.CINone}
	}

	status, err := seams.CheckStatus(pull, branch.Head)
	if err != nil {
		return reviewRead{pull: pull, state: progress.PullRequestOpen, ci: forge.CINone, err: err}
	}

	return reviewRead{pull: pull, state: progress.PullRequestOpen, ci: status.State}
}
