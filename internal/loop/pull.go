// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package loop

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"slices"

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
// leaves that part out of the proposal, as Owners left zero leaves out the
// reviewers.
type PullSeams struct {
	Branch    func(ctx context.Context) (gitrepo.Branch, error)
	FindPull  func(ctx context.Context, branch string) (forge.PullRequest, bool, error)
	Templates func() []forge.Template
	Issue     func(ctx context.Context, issueKey jira.Key) (jira.IssueDetail, error)
	BrowseURL func(jira.Key) string
	Owners    OwnerSeams
}

// PullOptions are the configuration a pull request is composed under: the
// tracker's project, which picks the issue key out of the branch name, and where
// the title comes from.
type PullOptions struct {
	Project     string
	TitleSource convention.TitleSource
	// Template is the template the body starts from, by name, as the
	// terminal's ctrl+t chooses one; empty starts from the first.
	Template string
	// Base is the branch it merges into, as a surface chose it, by the name
	// the forge knows; the code owners asked to review are read against it.
	// Empty is the base the branch was read to have left. The title and body
	// are drafted from the branch's commits as it was read, whichever base
	// is chosen.
	Base string
}

// ComposePull proposes the pull request for the checked-out branch — a title and
// body from its commits, its issue and the repository's template opts names, or
// its first, the base it would merge into, and the code owners of its changes
// as reviewers — and returns the branch it read. It refuses with
// ErrNothingToOpen when there is nothing to propose, with ErrNoSuchTemplate
// when the repository has no template of the name opts gives, and with a
// PullAlreadyOpenError when an open pull request already stands for the branch.
// A merged or closed one does not stand in the way, and neither does a forge
// that cannot say: the open is where that forge's own answer lands.
func ComposePull(
	ctx context.Context, seams PullSeams, opts PullOptions,
) (forge.NewPullRequest, gitrepo.Branch, error) {
	branch, err := branchToOpen(ctx, seams.Branch)
	if err != nil {
		return forge.NewPullRequest{}, gitrepo.Branch{}, err
	}

	err = refuseAnOpenPull(ctx, seams.FindPull, branch.Name)
	if err != nil {
		return forge.NewPullRequest{}, gitrepo.Branch{}, err
	}

	template, err := chosenTemplate(seams.Templates, opts.Template)
	if err != nil {
		return forge.NewPullRequest{}, gitrepo.Branch{}, err
	}

	return draft(ctx, seams, opts, branch, template), branch, nil
}

// branchToOpen reads the checked-out branch, refusing one that could not carry a
// pull request: a detached tree, or a branch with no commits of its own. The
// branch is read under ctx.
func branchToOpen(ctx context.Context, read func(context.Context) (gitrepo.Branch, error)) (gitrepo.Branch, error) {
	if read == nil {
		return gitrepo.Branch{}, ErrNothingToOpen
	}

	branch, err := read(ctx)
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
func refuseAnOpenPull(
	ctx context.Context, find func(ctx context.Context, branch string) (forge.PullRequest, bool, error), name string,
) error {
	if find == nil {
		return ErrNothingToOpen
	}

	pull, found, err := find(ctx, name)
	if err == nil && found && pull.IsOpen() {
		return PullAlreadyOpenError{Pull: pull}
	}

	return nil
}

// draft composes the pull request for branch, reading its issue under ctx.
func draft(
	ctx context.Context, seams PullSeams, opts PullOptions, branch gitrepo.Branch, template string,
) forge.NewPullRequest {
	key, _ := IssueOf(branch, opts.Project)
	issueKey := jira.Key(key.Key)

	title, body := Draft(DraftInput{
		Subjects:     Subjects(branch.Commits),
		IssueKey:     issueKey,
		IssueSummary: issueSummary(ctx, seams.Issue, issueKey),
		IssueURL:     issueURL(seams.BrowseURL, issueKey),
		Template:     template,
		TitleSource:  opts.TitleSource,
	})

	base := cmp.Or(opts.Base, branch.BaseName())
	users, teams := SplitReviewers(ProposedReviewers(ctx, seams.Owners, base))

	return forge.NewPullRequest{
		Title: title, Body: body, Head: branch.Name, Base: base, Reviewers: users, TeamReviewers: teams,
	}
}

// DraftInput is what a pull request's title and body are proposed from: the
// branch's commit subjects, oldest first; the issue it names, with the
// summary and link the tracker gave — empty where it gave none; the template
// the body starts from, empty for none; and where the title comes from.
type DraftInput struct {
	Subjects     []string
	IssueKey     jira.Key
	IssueSummary string
	IssueURL     string
	Template     string
	TitleSource  convention.TitleSource
}

// Draft proposes a pull request's title, then its body, from what a surface
// already holds. ComposePull reads the input through seams and the template
// its options name; a surface that holds it already, as the terminal's
// composer does, passes it here.
func Draft(input DraftInput) (string, string) {
	key := string(input.IssueKey)

	return convention.PullRequestTitleFrom(input.TitleSource, input.Subjects, key, input.IssueSummary),
		convention.PullRequestBody(input.Template, input.Subjects, key, input.IssueURL)
}

// ErrNoSuchTemplate refuses a pull request template asked for by a name the
// repository has none of.
var ErrNoSuchTemplate = errors.New("the repository has no pull request template of that name")

// chosenTemplate is the body of the repository's template named, or of its
// first when the name is empty, or empty when it has none — the body then
// lists the branch's commits. A name it has none of is ErrNoSuchTemplate.
func chosenTemplate(read func() []forge.Template, name string) (string, error) {
	var templates []forge.Template
	if read != nil {
		templates = read()
	}

	if name == "" {
		if len(templates) == 0 {
			return "", nil
		}

		return templates[0].Body, nil
	}

	at := slices.IndexFunc(templates, func(template forge.Template) bool { return template.Name == name })
	if at < 0 {
		return "", ErrNoSuchTemplate
	}

	return templates[at].Body, nil
}

// JiraIssue is the Jira issue the branch is for, by its link or its name, and
// whether it has one. The bare forge issue number a branch can carry instead,
// 42, is no Jira issue even with Jira as the tracker.
func JiraIssue(branch gitrepo.Branch, project string) (jira.Key, bool) {
	key, named := IssueOf(branch, project)
	if !named || key.Tracker != convention.TrackerJira {
		return "", false
	}

	return jira.Key(key.Key), true
}
