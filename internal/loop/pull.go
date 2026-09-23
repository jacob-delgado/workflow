// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package loop

import (
	"errors"
	"fmt"

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

// ReviewTransition is the move to the configured review status for an issue
// whose pull request is now open, and whether there is one worth offering: the
// status is configured, the branch names an issue, Jira offers a move to that
// status, and the move needs no fields. A tracker that cannot be read offers
// nothing — the pull request is already open.
func ReviewTransition(
	transitions func(jira.Key) ([]jira.Transition, error), key jira.Key, status string,
) (jira.Transition, bool) {
	if status == "" || key == "" || transitions == nil {
		return jira.Transition{}, false
	}

	moves, err := transitions(key)
	if err != nil {
		return jira.Transition{}, false
	}

	index, found := jira.FindTransition(moves, status)
	if !found || len(moves[index].Fields) > 0 {
		return jira.Transition{}, false
	}

	return moves[index], true
}
