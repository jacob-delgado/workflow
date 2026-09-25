// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package forge

import "context"

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
