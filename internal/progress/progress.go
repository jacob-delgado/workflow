// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

// Package progress works out how far along the developer loop a piece of work
// is — pick an issue, branch, commit, open a review, announce it — from what
// the repository and the services report. It keeps no state of its own: every
// stage is derived fresh, not read from the store. Both the terminal interface's
// spine and `workflow status` read it, so the rule lives in one place.
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

// PullState is where the branch's pull request stands, as far as the loop is
// concerned: there is none to follow, it is open for review, or it has merged.
type PullState int

const (
	// NoPullRequest is a branch with no pull request to follow: none was found,
	// or the one found was closed without merging.
	NoPullRequest PullState = iota
	// PullRequestOpen is a pull request open for review.
	PullRequestOpen
	// PullRequestMerged is a pull request that has merged, so its review is over
	// and it has no CI left to pass.
	PullRequestMerged
)

// PullStateOf is where a found pull request stands for the loop. One closed
// without merging counts as none: its review neither goes on nor ended in a
// merge. A map, so exhaustive keeps it complete and gobco sees no arm unrun.
func PullStateOf(state forge.PullState) PullState {
	return map[forge.PullState]PullState{
		forge.StateOpen:   PullRequestOpen,
		forge.StateMerged: PullRequestMerged,
		forge.StateClosed: NoPullRequest,
	}[state]
}

// Work is everything the stages are derived from. The two session fields,
// IssueSelected and PostPending, are knowledge the terminal interface has and a
// one-shot command does not; a command leaves them false, so neither its Issue
// stage nor its messaging stage ever reads in-flight.
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
	// PullRequest is where the branch's pull request stands.
	PullRequest PullState
	// CI is how the pull request's CI stands.
	CI forge.CIState
	// ChangesRequested is that a reviewer asked for changes.
	ChangesRequested bool
	// Announced is that the pull request was announced on the messaging
	// service at the moment it is at now, in this session or, as the store
	// remembers, an earlier one.
	Announced bool
	// PostPending is that an announcement is waiting for CI. Session knowledge.
	PostPending bool
}

// Stages is the loop's five stages, in order, each with how far it has got. The
// last is named for the messaging service the work is announced on — Slack,
// Teams, Discord or Webhook — as the configuration names it.
func Stages(work Work, messagingService string) []Stage {
	return []Stage{
		{Name: "Issue", State: issueState(work)},
		{Name: "Branch", State: branchState(work)},
		{Name: "Commits", State: commitState(work)},
		{Name: "Review", State: reviewState(work)},
		{Name: messagingService, State: announceState(work)},
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
// something to go back and address. A merged pull request's review is over,
// whatever was asked of it before the merge.
func reviewState(work Work) State {
	switch {
	case work.PullRequest == PullRequestMerged:
		return Done
	case work.PullRequest == NoPullRequest:
		return NotStarted
	case work.CI == forge.CIFailed || work.ChangesRequested:
		return Failed
	case work.CI == forge.CIPassed:
		return Done
	default:
		return InFlight
	}
}

// announceState is done once the pull request is announced, and in flight while
// an announcement waits for CI.
func announceState(work Work) State {
	switch {
	case work.Announced:
		return Done
	case work.PostPending:
		return InFlight
	default:
		return NotStarted
	}
}
