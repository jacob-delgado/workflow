// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package forge

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"time"
)

// issuesSegment is where issues live under a repository's API path, shared so
// the listing, the read and the close spell it once.
const issuesSegment = "/issues"

// The segments under an issue where its comments live on each forge.
const (
	commentsSegment = "/comments"
	notesSegment    = "/notes"
)

// Issue is an open issue on the forge assigned to you, for the tracker to offer
// when there is no Jira. It is scoped to one repository, because the loop that
// resolves it — a branch and a pull request — lives in that repository.
type Issue struct {
	Number int
	URL    string
	Title  string
}

// IssueDetail is one issue read in full: its body, who opened it, whether it
// has been closed since it was listed, and how many comments it has.
type IssueDetail struct {
	Issue        Issue
	Body         string
	Author       string
	Closed       bool
	CommentCount int
}

// IssueComment is one comment on an issue: who wrote it, what it says, and
// when.
type IssueComment struct {
	Author  string
	Body    string
	Created time.Time
}

// ghostAuthor is who GitHub shows a comment from a deleted account as.
const ghostAuthor = "ghost"

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

// RecentIssueComments reads the most recent of an issue's comments, as many as
// one page holds, oldest first: total is how many the issue has, which GitHub,
// listing a thread only oldest first, needs to find its last page. It costs
// one request on GitLab and at most two on GitHub, however long the thread.
// GitLab's notes about the issue's own changes, such as a new label, are left
// out.
func (c Client) RecentIssueComments(ctx context.Context, repo Repo, number, total int) ([]IssueComment, error) {
	speaks, err := dialectFor(repo.Kind)
	if err != nil {
		return nil, err
	}

	return speaks.issueComments(ctx, c, repo, number, total)
}

// CommentOnIssue posts text as a comment on an issue, as written: the forge
// renders it as Markdown. GitLab answers a comment made only of quick actions
// by running them and keeping no note, which is the zero comment here, and not
// a failure to try again.
func (c Client) CommentOnIssue(ctx context.Context, repo Repo, number int, text string) (IssueComment, error) {
	speaks, err := dialectFor(repo.Kind)
	if err != nil {
		return IssueComment{}, err
	}

	return speaks.commentOnIssue(ctx, c, repo, number, text)
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

// issueCommentBody is the body that posts a comment, the same on both forges.
type issueCommentBody struct {
	Body string `json:"body"`
}

// githubComment is an issue comment as GitHub sends it. A deleted account's
// comment comes with no user.
type githubComment struct {
	User *struct {
		Login string `json:"login"`
	} `json:"user"`
	Body    string    `json:"body"`
	Created time.Time `json:"created_at"`
}

func (g githubComment) comment() IssueComment {
	author := ghostAuthor
	if g.User != nil && g.User.Login != "" {
		author = g.User.Login
	}

	return IssueComment{Author: author, Body: g.Body, Created: g.Created}
}

// githubIssueComments reads the newest page's worth of an issue's comments:
// the last page, and the one before it when the last is short.
func githubIssueComments(ctx context.Context, client Client, repo Repo, number, total int) ([]IssueComment, error) {
	path := githubRepoPath(repo) + issuesSegment + "/" + strconv.Itoa(number) + commentsSegment + "?"
	page := func(at int) ([]githubComment, error) {
		return repoCall[[]githubComment](ctx, client, repo, http.MethodGet, path+pageQuery(url.Values{}, at), nil)
	}

	last := max(1, (total+perPage-1)/perPage)

	read, err := page(last)
	if err != nil {
		return nil, err
	}

	if last > 1 && len(read) < perPage {
		before, err := page(last - 1)
		if err != nil {
			return nil, err
		}

		read = append(before, read...)
	}

	read = read[max(0, len(read)-perPage):]

	comments := make([]IssueComment, 0, len(read))
	for _, one := range read {
		comments = append(comments, one.comment())
	}

	return comments, nil
}

// githubCommentOnIssue posts a comment on an issue.
func githubCommentOnIssue(
	ctx context.Context, client Client, repo Repo, number int, text string,
) (IssueComment, error) {
	posted, err := repoCall[githubComment](ctx, client, repo, http.MethodPost,
		githubRepoPath(repo)+issuesSegment+"/"+strconv.Itoa(number)+commentsSegment, issueCommentBody{Body: text})
	if err != nil {
		return IssueComment{}, err
	}

	return posted.comment(), nil
}

// gitlabNote is an issue note as GitLab sends it. A system note records a
// change to the issue rather than something someone wrote.
type gitlabNote struct {
	Author struct {
		Username string `json:"username"`
	} `json:"author"`
	Body    string    `json:"body"`
	Created time.Time `json:"created_at"`
	System  bool      `json:"system"`
}

func (g gitlabNote) comment() IssueComment {
	return IssueComment{Author: g.Author.Username, Body: g.Body, Created: g.Created}
}

// gitlabIssueComments reads one page of an issue's notes, newest first, and
// turns it oldest first, leaving out the system notes.
func gitlabIssueComments(ctx context.Context, client Client, repo Repo, number, _ int) ([]IssueComment, error) {
	query := url.Values{"sort": {"desc"}, "order_by": {"created_at"}}
	path := gitlabProjectPath(repo) + issuesSegment + "/" + strconv.Itoa(number) + notesSegment + "?"

	read, err := repoCall[[]gitlabNote](ctx, client, repo, http.MethodGet, path+pageQuery(query, 1), nil)
	if err != nil {
		return nil, err
	}

	comments := make([]IssueComment, 0, len(read))
	for _, one := range slices.Backward(read) {
		if !one.System {
			comments = append(comments, one.comment())
		}
	}

	return comments, nil
}

// gitlabCommentOnIssue posts a note on an issue. A note of only quick actions
// is answered with no note, which reads as the zero comment.
func gitlabCommentOnIssue(
	ctx context.Context, client Client, repo Repo, number int, text string,
) (IssueComment, error) {
	posted, err := repoCall[gitlabNote](ctx, client, repo, http.MethodPost,
		gitlabProjectPath(repo)+issuesSegment+"/"+strconv.Itoa(number)+notesSegment, issueCommentBody{Body: text})
	if errors.Is(err, errAccepted) {
		return IssueComment{}, nil
	}

	if err != nil {
		return IssueComment{}, err
	}

	return posted.comment(), nil
}
