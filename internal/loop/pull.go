// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package loop

import (
	"errors"
	"fmt"
	"strings"

	"github.com/jacob-delgado/workflow/internal/convention"
	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/gitrepo"
	"github.com/jacob-delgado/workflow/internal/jira"
)

var (
	// ErrNothingToOpen refuses composing a pull request when there is nothing to
	// propose: the tree is not on a branch, the branch has no commits, or there
	// is no way to read either.
	ErrNothingToOpen = errors.New("there is nothing to open a pull request for")
	// ErrPullAlreadyOpen refuses a second pull request for a branch that already
	// has an open one. The error returned is a PullAlreadyOpenError, which carries
	// that pull request.
	ErrPullAlreadyOpen = errors.New("a pull request is already open for this branch")
)

// PullAlreadyOpenError is ErrPullAlreadyOpen with the pull request that is
// open, so a surface can point at it.
type PullAlreadyOpenError struct {
	Pull forge.PullRequest
}

var _ error = PullAlreadyOpenError{}

// Error says what was refused, in words any surface can show.
func (e PullAlreadyOpenError) Error() string {
	return ErrPullAlreadyOpen.Error()
}

// Unwrap lets errors.Is match ErrPullAlreadyOpen.
func (PullAlreadyOpenError) Unwrap() error {
	return ErrPullAlreadyOpen
}

// PullSeams are what composing a pull request reads. A nil Branch or FindPull
// means there is nothing to open; a nil Templates, Issue or BrowseURL only
// leaves that part out of the proposal.
type PullSeams struct {
	Branch    func() (gitrepo.Branch, error)
	FindPull  func(branch string) (forge.PullRequest, bool, error)
	Templates func() []forge.Template
	Issue     func(jira.Key) (jira.IssueDetail, error)
	BrowseURL func(jira.Key) string
}

// PullOptions are the configuration a pull request is composed under: the
// tracker's project, which picks the issue key out of the branch name, and where
// the title comes from.
type PullOptions struct {
	Project     string
	TitleSource convention.TitleSource
}

// ComposePull proposes the pull request for the checked-out branch — a title and
// body from its commits, its issue and the repository's first template, and the
// base it would merge into — and returns the branch it read. It refuses with
// ErrNothingToOpen when there is nothing to propose, and with a
// PullAlreadyOpenError when an open pull request already stands for the branch.
// A merged or closed one does not stand in the way, and neither does a forge
// that cannot say: the open is where that forge's own answer lands.
func ComposePull(seams PullSeams, opts PullOptions) (forge.NewPullRequest, gitrepo.Branch, error) {
	branch, err := branchToOpen(seams.Branch)
	if err != nil {
		return forge.NewPullRequest{}, gitrepo.Branch{}, err
	}

	err = refuseAnOpenPull(seams.FindPull, branch.Name)
	if err != nil {
		return forge.NewPullRequest{}, gitrepo.Branch{}, err
	}

	return draft(seams, opts, branch), branch, nil
}

// branchToOpen reads the checked-out branch, refusing one that could not carry a
// pull request: a detached tree, or a branch with no commits of its own.
func branchToOpen(read func() (gitrepo.Branch, error)) (gitrepo.Branch, error) {
	if read == nil {
		return gitrepo.Branch{}, ErrNothingToOpen
	}

	branch, err := read()
	if err != nil {
		return gitrepo.Branch{}, fmt.Errorf("reading the branch: %w", err)
	}

	if branch.Name == "" || len(branch.Commits) == 0 {
		return gitrepo.Branch{}, ErrNothingToOpen
	}

	return branch, nil
}

// refuseAnOpenPull refuses a branch whose pull request is open. FindPull also
// returns a merged one, whose branch may carry new commits worth a fresh pull
// request, so only the open state refuses.
func refuseAnOpenPull(find func(branch string) (forge.PullRequest, bool, error), name string) error {
	if find == nil {
		return ErrNothingToOpen
	}

	pull, found, err := find(name)
	if err == nil && found && pull.IsOpen() {
		return PullAlreadyOpenError{Pull: pull}
	}

	return nil
}

// draft composes the pull request for branch.
func draft(seams PullSeams, opts PullOptions, branch gitrepo.Branch) forge.NewPullRequest {
	subjects := Subjects(branch.Commits)
	key, _ := convention.IssueKey(branch.Name, opts.Project)
	issueKey := jira.Key(key)

	return forge.NewPullRequest{
		Title: convention.PullRequestTitleFrom(opts.TitleSource, subjects, key, issueSummary(seams.Issue, issueKey)),
		Body:  convention.PullRequestBody(firstTemplate(seams.Templates), subjects, key, issueURL(seams.BrowseURL, issueKey)),
		Head:  branch.Name,
		Base:  branch.BaseName(),
	}
}

// firstTemplate is the repository's first pull request template's body, or
// empty when it has none — the body then lists the branch's commits.
func firstTemplate(read func() []forge.Template) string {
	if read == nil {
		return ""
	}

	templates := read()
	if len(templates) == 0 {
		return ""
	}

	return templates[0].Body
}

// JiraIssue is the Jira issue the branch names, and whether it names one: a key
// with its project, PROJ-42. The bare forge issue number a branch can carry
// instead, 42, is no Jira issue even with Jira as the tracker: Jira would refuse
// that number, or read it as the id of an unrelated issue.
func JiraIssue(branch gitrepo.Branch, project string) (jira.Key, bool) {
	key, named := convention.IssueKey(branch.Name, project)
	if !named || !strings.Contains(key, "-") {
		return "", false
	}

	return jira.Key(key), true
}

// Why no move to the review status is offered, for a surface that answers a
// request for one rather than offering it.
var (
	// ErrNoReviewStatus is a configuration with no jira.review_status to move to.
	ErrNoReviewStatus = errors.New("no review status is configured (jira.review_status)")
	// ErrNoReviewTransition is an issue Jira offers no move to the review status
	// for, from where it stands — or no issue, or no tracker to ask.
	ErrNoReviewTransition = errors.New("jira offers no move to the review status")
	// ErrReviewNeedsFields is a move to the review status that asks for fields,
	// which a move made without a form cannot fill.
	ErrReviewNeedsFields = errors.New("the move to the review status asks for fields")
)

// ReviewTransition is the move to the configured review status for an issue
// whose pull request is now open, and whether there is one worth offering: the
// status is configured, the branch names an issue, Jira offers a move to that
// status, and the move needs no fields. A tracker that cannot be read offers
// nothing — the pull request is already open.
func ReviewTransition(
	transitions func(jira.Key) ([]jira.Transition, error), key jira.Key, status string,
) (jira.Transition, bool) {
	move, err := FindReviewTransition(transitions, key, status)

	return move, err == nil
}

// FindReviewTransition is ReviewTransition saying why there is no move: no
// review status configured, no move Jira offers — or no issue or tracker to ask
// — a move that asks for fields, or the tracker's own error when it cannot be
// read. With no status configured the tracker is not asked at all.
func FindReviewTransition(
	transitions func(jira.Key) ([]jira.Transition, error), key jira.Key, status string,
) (jira.Transition, error) {
	if status == "" {
		return jira.Transition{}, ErrNoReviewStatus
	}

	if key == "" || transitions == nil {
		return jira.Transition{}, ErrNoReviewTransition
	}

	moves, err := transitions(key)
	if err != nil {
		return jira.Transition{}, fmt.Errorf("reading the transitions of %s: %w", key, err)
	}

	index, found := jira.FindTransition(moves, status)
	if !found {
		return jira.Transition{}, ErrNoReviewTransition
	}

	if len(moves[index].Fields) > 0 {
		return jira.Transition{}, ErrReviewNeedsFields
	}

	return moves[index], nil
}
