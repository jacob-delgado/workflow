// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

// Package progress works out how far along the developer loop a piece of work
// is — pick an issue, branch, commit, open a review, announce it — from what
// the repository and the services report. Nothing is stored, so every stage is
// derived fresh. Both the terminal interface's spine and `workflow status`
// read it, so the rule lives in one place.
package progress

import "github.com/jacob-delgado/workflow/internal/forge"

// State is how far a stage has got.
type State int

const (
	// NotStarted is a stage not begun.
	NotStarted State = iota
	// InFlight is a stage under way.
	InFlight
	// Done is a stage complete.
	Done
	// Failed is a stage to go back to: a CI failure, or changes requested.
	Failed
)

// Stage is one step of the loop and how far it has got.
type Stage struct {
	Name  string
	State State
}

// Work is everything the stages are derived from. The three session fields are
// knowledge the terminal interface has and a one-shot command does not; a
// command leaves them false, so its Slack stage reads not-started and its Issue
// stage never reads in-flight.
type Work struct {
	// OnFeatureBranch is a named branch other than the base.
	OnFeatureBranch bool
	// IssueNamed is that the branch name carries an issue key.
	IssueNamed bool
	// IssueSelected is that an issue is picked but not yet branched for. Session
	// knowledge.
	IssueSelected bool
	// Commits is how many commits the branch has that the base does not.
	Commits int
	// UncommittedChanges is how many files have uncommitted changes.
	UncommittedChanges int
	// PullRequestFound is that the branch has an open pull request.
	PullRequestFound bool
	// CI is how the pull request's CI stands.
	CI forge.CIState
	// ChangesRequested is that a reviewer asked for changes.
	ChangesRequested bool
	// Announced is that the pull request was posted to Slack. Session knowledge.
	Announced bool
	// PostPending is that a post is waiting for CI. Session knowledge.
	PostPending bool
}

// Stages is the loop's five stages, in order, each with how far it has got.
func Stages(work Work) []Stage {
	return []Stage{
		{Name: "Issue", State: issueState(work)},
		{Name: "Branch", State: branchState(work)},
		{Name: "Commits", State: commitState(work)},
		{Name: "Review", State: reviewState(work)},
		{Name: "Slack", State: slackState(work)},
	}
}

// issueState is done once the branch names an issue, and in flight while one is
// selected.
func issueState(work Work) State {
	switch {
	case work.IssueNamed:
		return Done
	case work.IssueSelected:
		return InFlight
	default:
		return NotStarted
	}
}

// branchState is done on a feature branch.
func branchState(work Work) State {
	if work.OnFeatureBranch {
		return Done
	}

	return NotStarted
}

// commitState is done once the branch has commits, and in flight while there
// are changes to commit.
func commitState(work Work) State {
	switch {
	case work.OnFeatureBranch && work.Commits > 0:
		return Done
	case work.UncommittedChanges > 0:
		return InFlight
	default:
		return NotStarted
	}
}

// reviewState follows the pull request, its CI and its review: changes still
// asked for stop it reading as done, the same as a CI failure, because both are
// something to go back and address.
func reviewState(work Work) State {
	switch {
	case !work.PullRequestFound:
		return NotStarted
	case work.CI == forge.CIFailed || work.ChangesRequested:
		return Failed
	case work.CI == forge.CIPassed:
		return Done
	default:
		return InFlight
	}
}

// slackState is done once the pull request is posted, and in flight while a
// post waits for CI.
func slackState(work Work) State {
	switch {
	case work.Announced:
		return Done
	case work.PostPending:
		return InFlight
	default:
		return NotStarted
	}
}
