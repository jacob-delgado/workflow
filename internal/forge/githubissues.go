// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package forge

import (
	"context"
	"net/http"
	"strconv"
)

// githubIssues lists the open issues in the repository assigned to the token
// owner. It searches with a repo filter, so @me needs no username, and with
// is:issue so the pull requests that search also returns are left out.
func githubIssues(ctx context.Context, client Client, repo Repo) ([]Issue, error) {
	found, _, err := githubSearch[githubIssue](ctx, client, "is:issue is:open assignee:@me repo:"+repo.Path)
	if err != nil {
		return nil, err
	}

	issues := make([]Issue, 0, len(found))
	for _, item := range found {
		issues = append(issues, item.issue())
	}

	return issues, nil
}

// githubIssue is an issue as GitHub sends it, from the search or a single read.
type githubIssue struct {
	Number int    `json:"number"`
	URL    string `json:"html_url"`
	Title  string `json:"title"`
	Body   string `json:"body"`
	State  string `json:"state"`
	User   struct {
		Login string `json:"login"`
	} `json:"user"`
	Comments int `json:"comments"`
}

func (g githubIssue) issue() Issue {
	return Issue{Number: g.Number, URL: g.URL, Title: g.Title}
}

func (g githubIssue) detail() IssueDetail {
	return IssueDetail{
		Issue: g.issue(), Body: g.Body, Author: g.User.Login, Closed: g.State == wireClosed, CommentCount: g.Comments,
	}
}

// githubReadIssue reads one issue's body and author.
func githubReadIssue(ctx context.Context, client Client, repo Repo, number int) (IssueDetail, error) {
	read, err := repoCall[githubIssue](ctx, client, repo, http.MethodGet,
		githubRepoPath(repo)+issuesSegment+"/"+strconv.Itoa(number), nil)
	if err != nil {
		return IssueDetail{}, err
	}

	return read.detail(), nil
}

// githubIssueState is the PATCH body that closes an issue.
type githubIssueState struct {
	State string `json:"state"`
}

// githubCloseIssue closes an issue by setting its state to closed.
func githubCloseIssue(ctx context.Context, client Client, repo Repo, number int) error {
	_, err := repoCall[githubIssue](ctx, client, repo, http.MethodPatch,
		githubRepoPath(repo)+issuesSegment+"/"+strconv.Itoa(number), githubIssueState{State: wireClosed})

	return err
}
