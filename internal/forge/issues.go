// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package forge

import (
	"context"
	"net/http"
	"strconv"
)

// issuesSegment is where issues live under a repository's API path, shared so
// the listing, the read and the close spell it once.
const issuesSegment = "/issues"

// Issue is an open issue on the forge assigned to you, for the tracker to offer
// when there is no Jira. It is scoped to one repository, because the loop that
// resolves it — a branch and a pull request — lives in that repository.
type Issue struct {
	Number int
	URL    string
	Title  string
}

// IssueDetail is one issue read in full: its body, who opened it, and whether
// it has been closed since it was listed.
type IssueDetail struct {
	Issue  Issue
	Body   string
	Author string
	Closed bool
}

// AssignedIssues lists the open issues in the repository assigned to the token's
// owner, for the Issues pane when there is no Jira to ask.
func (c Client) AssignedIssues(ctx context.Context, repo Repo) ([]Issue, error) {
	speaks, err := dialectFor(repo.Kind)
	if err != nil {
		return nil, err
	}

	return speaks.issues(ctx, c, repo)
}

// ReadIssue reads one issue in full, for the detail the pane shows.
func (c Client) ReadIssue(ctx context.Context, repo Repo, number int) (IssueDetail, error) {
	speaks, err := dialectFor(repo.Kind)
	if err != nil {
		return IssueDetail{}, err
	}

	return speaks.readIssue(ctx, c, repo, number)
}

// CloseIssue closes an issue, the one state change a forge issue has, so picking
// one up and resolving it completes the loop.
func (c Client) CloseIssue(ctx context.Context, repo Repo, number int) error {
	speaks, err := dialectFor(repo.Kind)
	if err != nil {
		return err
	}

	return speaks.closeIssue(ctx, c, repo, number)
}

// AssignIssue gives an issue to the user named, as Jira's assign does: added to
// its assignees on GitHub, and made its assignee on GitLab, which keys users by
// id and so looks the name up first.
func (c Client) AssignIssue(ctx context.Context, repo Repo, number int, username string) error {
	speaks, err := dialectFor(repo.Kind)
	if err != nil {
		return err
	}

	return speaks.assign(ctx, c, repo, number, username)
}

// IssueURL is the issue's page on the forge, for a link to open or copy, or
// empty on a forge that could not be told.
func (r Repo) IssueURL(number int) string {
	var page string

	switch r.Kind {
	case KindGitHub:
		page = "/issues/"
	case KindGitLab:
		page = "/-/issues/"
	case KindUnknown:
		return ""
	}

	return "https://" + r.Host + "/" + r.Path + page + strconv.Itoa(number)
}

// githubAssignees is the body that adds assignees to a GitHub issue.
type githubAssignees struct {
	Assignees []string `json:"assignees"`
}

// githubAssignIssue adds username to the issue's assignees.
func githubAssignIssue(ctx context.Context, client Client, repo Repo, number int, username string) error {
	_, err := repoCall[githubIssue](ctx, client, repo, http.MethodPost,
		githubRepoPath(repo)+issuesSegment+"/"+strconv.Itoa(number)+"/assignees",
		githubAssignees{Assignees: []string{username}})

	return err
}

// gitlabAssignees is the body that sets a GitLab issue's assignees.
type gitlabAssignees struct {
	AssigneeIDs []int64 `json:"assignee_ids"`
}

// gitlabAssignIssue makes username the issue's assignee.
func gitlabAssignIssue(ctx context.Context, client Client, repo Repo, number int, username string) error {
	ids, err := gitlabUserIDs(ctx, client, []string{username})
	if err != nil {
		return err
	}

	_, err = repoCall[gitlabIssue](ctx, client, repo, http.MethodPut,
		gitlabProjectPath(repo)+issuesSegment+"/"+strconv.Itoa(number), gitlabAssignees{AssigneeIDs: ids})

	return err
}
