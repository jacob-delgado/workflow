// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

// Package progress works out how far along the developer loop a piece of work
// is — pick an issue, branch, commit, open a review, announce it — from what
// the repository and the services report. It keeps no state of its own: every
// stage is derived fresh, not read from the store. It is the one place the
// stage rules are written: the terminal interface's spine and `workflow status`
// read it, and the web server ships its stages in every frame of the stream,
// which the web's work story draws.
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

// Glyph is the mark a stage in the state is drawn with — the terminal's spine
// and the command line's status line alike — or, with ascii, the stand-in
// ui.ascii draws instead. A map, not a switch, so there is no last-case arm
// gobco can never see; exhaustive keeps it complete.
func (s State) Glyph(ascii bool) string {
	marks := map[State]struct{ unicode, plain string }{
		NotStarted: {unicode: "○", plain: "o"},
		InFlight:   {unicode: "◐", plain: "*"},
		Done:       {unicode: "●", plain: "#"},
		Failed:     {unicode: "✗", plain: "x"},
	}[s]

	if ascii {
		return marks.plain
	}

	return marks.unicode
}

// Turn is the frames the in-flight mark turns through while work is out — the
// terminal's panes and the command line's live line alike, each at its own
// rate whichever frames it draws: a quarter turn a frame from InFlight's own
// glyph; with ascii, which has no shape that turns by quarters, a line
// spinning. Held still, by ui.motion, the mark is InFlight's glyph instead,
// which in Unicode is the first frame.
func Turn(ascii bool) []string {
	if ascii {
		return []string{"|", "/", "-", `\`}
	}

	return []string{InFlight.Glyph(false), "◓", "◑", "◒"}
}

// System is where a stage's work happens, which the terminal's spine colors it
// by.
type System int

const (
	// Tracker is the issue tracker: Jira, or the forge's own issues.
	Tracker System = iota
	// Git is the repository.
	Git
	// Forge is the pull request's host: GitHub or GitLab.
	Forge
	// Messaging is the service the work is announced on.
	Messaging
)

// Step is which stage of the loop a stage is, whatever it is named: the last
// is named for the messaging service the work is announced on.
type Step int

const (
	// StepIssue is the issue the work is for.
	StepIssue Step = iota
	// StepBranch is the branch made for it.
	StepBranch
	// StepCommits is the work committed on it.
	StepCommits
	// StepReview is its pull request's review and CI.
	StepReview
	// StepAnnounce is the pull request announced on the messaging service.
	StepAnnounce
)

// Stage is one step of the loop, as it is named, the system its work happens
// in, and how far it has got.
type Stage struct {
	Step   Step
	Name   string
	System System
	State  State
}

// PullState is where the branch's pull request stands, as far as the loop is
// concerned: there is none to follow, it is open for review, it is merging, or
// it has merged.
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
	// PullRequestMerging is a pull request the forge is merging: no longer
	// open, its merge in flight.
	PullRequestMerging
)

// PullStateOf is where a found pull request stands for the loop. One closed
// without merging counts as none: its review neither goes on nor ended in a
// merge. A map, so exhaustive keeps it complete and gobco sees no arm unrun.
func PullStateOf(state forge.PullState) PullState {
	return map[forge.PullState]PullState{
		forge.StateOpen:    PullRequestOpen,
		forge.StateMerging: PullRequestMerging,
		forge.StateMerged:  PullRequestMerged,
		forge.StateClosed:  NoPullRequest,
	}[state]
}

// IssueWork is how far the work's issue has got: none yet, one picked but
// not yet branched for, or one the branch name carries. A picked issue is
// session knowledge, which a one-shot command does not have.
type IssueWork int

const (
	// IssueNone is no issue picked, and none the branch names.
	IssueNone IssueWork = iota
	// IssuePicked is an issue picked but not yet branched for.
	IssuePicked
	// IssueNamed is an issue the branch name carries.
	IssueNamed
)

// AnnounceWork is how far the announcement has got: none, one under way —
// waiting for CI or on its way, which is session knowledge — or one made at
// the moment the pull request is at now, in this session or, as the store
// remembers, an earlier one.
type AnnounceWork int

const (
	// AnnounceNone is no announcement made or under way.
	AnnounceNone AnnounceWork = iota
	// AnnounceUnderWay is an announcement waiting for CI or on its way.
	AnnounceUnderWay
	// AnnounceMade is the pull request announced at its moment.
	AnnounceMade
)

// Review is where the branch's pull request stands, how its CI stands and
// whether a reviewer asked for changes. With no pull request to follow, it
// holds neither CI nor changes asked for: NoReview.
type Review struct {
	pull             PullState
	ci               forge.CIState
	changesRequested bool
}

// NoReview is a branch with no pull request to follow.
func NoReview() Review {
	return Review{pull: NoPullRequest, ci: forge.CINone, changesRequested: false}
}

// ReviewOf is the review of pull, when found, with its CI in ci. One not
// found, or closed without merging, is NoReview, whatever its CI or its
// reviewers last said.
func ReviewOf(pull forge.PullRequest, found bool, ci forge.CIState) Review {
	if !found || PullStateOf(pull.State) == NoPullRequest {
		return NoReview()
	}

	return Review{pull: PullStateOf(pull.State), ci: ci, changesRequested: pull.ChangesRequested}
}

// Work is everything the stages are derived from. A one-shot command knows
// no picked issue and no announcement under way, so neither its Issue stage
// nor its messaging stage ever reads in flight.
type Work struct {
	// Issue is how far the work's issue has got.
	Issue IssueWork
	// OnFeatureBranch is a named branch other than the base.
	OnFeatureBranch bool
	// Commits is how many commits the branch has that the base does not.
	Commits int
	// UncommittedChanges is how many files have uncommitted changes.
	UncommittedChanges int
	// Review is where the branch's pull request stands.
	Review Review
	// Announce is how far the announcement has got.
	Announce AnnounceWork
}

// Stages is the loop's five stages, in order, each with its system and how far
// it has got. The last is named for the messaging service the work is announced
// on — Slack, Teams, Discord or Webhook — as the configuration names it.
func Stages(work Work, messagingService string) []Stage {
	return []Stage{
		{Step: StepIssue, Name: "Issue", System: Tracker, State: issueState(work)},
		{Step: StepBranch, Name: "Branch", System: Git, State: branchState(work)},
		{Step: StepCommits, Name: "Commits", System: Git, State: commitState(work)},
		{Step: StepReview, Name: "Review", System: Forge, State: reviewState(work)},
		{Step: StepAnnounce, Name: messagingService, System: Messaging, State: announceState(work)},
	}
}

// issueState is done once the branch names an issue, and in flight while one is
// picked. A map, so exhaustive keeps it complete.
func issueState(work Work) State {
	return map[IssueWork]State{IssueNone: NotStarted, IssuePicked: InFlight, IssueNamed: Done}[work.Issue]
}

// branchState is done on a feature branch.
func branchState(work Work) State {
	if work.OnFeatureBranch {
		return Done
	}

	return NotStarted
}

// commitState is done once the branch has commits, or its pull request has
// merged them, and in flight while there are changes to commit on it. Commits
// counts what the base lacks, so after a merge and a fetch it is none, though
// the work was committed and is in. Changes on the base branch are not yet
// work the loop follows: no branch has been made for them to be committed on.
func commitState(work Work) State {
	switch {
	case !work.OnFeatureBranch:
		return NotStarted
	case work.Commits > 0 || work.Review.pull == PullRequestMerged:
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
// whatever was asked of it before the merge, and one merging is in flight
// until the merge lands, whatever its CI last read.
func reviewState(work Work) State {
	review := work.Review

	switch {
	case review.pull == PullRequestMerged:
		return Done
	case review.pull == NoPullRequest:
		return NotStarted
	case review.pull == PullRequestMerging:
		return InFlight
	case review.ci == forge.CIFailed || review.changesRequested:
		return Failed
	case review.ci == forge.CIPassed:
		return Done
	default:
		return InFlight
	}
}

// announceState is done once the pull request is announced, and in flight while
// an announcement waits for CI or is on its way. A map, so exhaustive keeps it
// complete.
func announceState(work Work) State {
	return map[AnnounceWork]State{AnnounceNone: NotStarted, AnnounceUnderWay: InFlight, AnnounceMade: Done}[work.Announce]
}
