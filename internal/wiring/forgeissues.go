// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package wiring

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/jira"
	"github.com/jacob-delgado/workflow/internal/seams"
)

// The status a forge issue has, mapped onto the two Jira strings the pane reads:
// a name to show, and a category that chooses the glyph. A listed issue is open,
// because the listing asks only for open ones; an issue read in full says which
// it is, so one the loop has closed reads as closed.
const (
	forgeOpenStatus   = "Open"
	forgeOpenCategory = "new"
	forgeClosedStatus = "Closed"
	forgeDoneCategory = "done"
	// closeTransitionID names the one state change a forge issue has.
	closeTransitionID = "close"
)

// errNotAnIssueNumber is a tracker key that is not a forge issue number. The
// Issues pane only offers numbers, but the web reads whatever key it is handed.
var errNotAnIssueNumber = errors.New("not a forge issue number")

// trackerDeps is what backs the Issues pane: Jira, reached through jiraClient,
// when it is configured — beside the repository's own forge issues, reached
// through connect, when the repository asks for them — and otherwise the
// forge's issues alone, so a project without Jira still has a tracker to run
// the loop against.
func trackerDeps(
	ctx context.Context, cfg config.Config,
	jiraClient func() (jira.Client, error), connect func() (forgeConnection, error),
) seams.Jira {
	if !cfg.Jira.Configured() {
		return forgeIssuesDeps(ctx, connect)
	}

	if cfg.Issues.Forge {
		return combinedTracker(cfg.Jira, jiraDeps(ctx, cfg.Jira, jiraClient), forgeIssuesDeps(ctx, connect))
	}

	return jiraDeps(ctx, cfg.Jira, jiraClient)
}

// forgeIssuesDeps adapts the forge's issues to the tracker seam the Issues pane
// reads. The pane, the detail, the status change, assigning, commenting and the
// link run unchanged; logged work and a remote link, which a forge issue has no
// equivalent for here, are left nil so those features simply do not appear.
func forgeIssuesDeps(ctx context.Context, connect func() (forgeConnection, error)) seams.Jira {
	return seams.Jira{
		Search:        func(string, int) (jira.SearchResult, error) { return listForgeIssues(ctx, connect) },
		SearchLenient: func(string, int) (jira.SearchResult, error) { return listForgeIssues(ctx, connect) },
		Issue:         func(issueKey jira.Key) (jira.IssueDetail, error) { return readForgeIssue(ctx, connect, issueKey) },
		Transitions:   func(jira.Key) ([]jira.Transition, error) { return forgeIssueTransitions(), nil },
		Transition: func(issueKey jira.Key, _ jira.Transition, _ []jira.FieldValue) error {
			return closeForgeIssue(ctx, connect, issueKey)
		},
		Assign: func(issueKey jira.Key, assignee string) error {
			return assignForgeIssue(ctx, connect, issueKey, assignee)
		},
		Comment: func(issueKey jira.Key, text string) (jira.Comment, error) {
			return commentOnForgeIssue(ctx, connect, issueKey, text)
		},
		BrowseURL: func(issueKey jira.Key) string { return browseForgeIssue(connect, issueKey) },
	}
}

// commentOnForgeIssue posts text as a comment on the issue behind a tracker
// key.
func commentOnForgeIssue(
	ctx context.Context, connect func() (forgeConnection, error), issueKey jira.Key, text string,
) (jira.Comment, error) {
	connection, number, err := connectToIssue(connect, issueKey)
	if err != nil {
		return jira.Comment{}, err
	}

	posted, err := connection.client.CommentOnIssue(ctx, connection.repo, number, text)
	if err != nil {
		return jira.Comment{}, err
	}

	return forgeComment(posted), nil
}

// forgeComment is a forge issue's comment as the tracker seam carries one.
func forgeComment(comment forge.IssueComment) jira.Comment {
	return jira.Comment{Author: comment.Author, Body: comment.Body, Created: comment.Created}
}

// assignForgeIssue gives the issue behind a tracker key to assignee.
func assignForgeIssue(
	ctx context.Context, connect func() (forgeConnection, error), issueKey jira.Key, assignee string,
) error {
	connection, number, err := connectToIssue(connect, issueKey)
	if err != nil {
		return err
	}

	return connection.client.AssignIssue(ctx, connection.repo, number, assignee)
}

// browseForgeIssue links the issue behind a tracker key for someone to click,
// or is empty when the key names no issue or there is no forge to link to.
func browseForgeIssue(connect func() (forgeConnection, error), issueKey jira.Key) string {
	connection, number, err := connectToIssue(connect, issueKey)
	if err != nil {
		return ""
	}

	return connection.repo.IssueURL(number)
}

// listForgeIssues reads the assigned issues and shapes them as a search result.
func listForgeIssues(ctx context.Context, connect func() (forgeConnection, error)) (jira.SearchResult, error) {
	connection, err := connect()
	if err != nil {
		return jira.SearchResult{}, err
	}

	issues, err := connection.client.AssignedIssues(ctx, connection.repo)
	if err != nil {
		return jira.SearchResult{}, err
	}

	rows := make([]jira.Issue, 0, len(issues))
	for _, issue := range issues {
		rows = append(rows, forgeIssueRow(issue))
	}

	return jira.SearchResult{Issues: rows, Total: len(rows)}, nil
}

// readForgeIssue reads one issue in full and shapes it as issue detail.
func readForgeIssue(
	ctx context.Context, connect func() (forgeConnection, error), issueKey jira.Key,
) (jira.IssueDetail, error) {
	connection, number, err := connectToIssue(connect, issueKey)
	if err != nil {
		return jira.IssueDetail{}, err
	}

	detail, err := connection.client.ReadIssue(ctx, connection.repo, number)
	if err != nil {
		return jira.IssueDetail{}, err
	}

	return jira.IssueDetail{
		Issue:        forgeDetailRow(detail),
		Description:  detail.Body,
		Reporter:     detail.Author,
		Comments:     forgeThread(ctx, connection, number, detail.CommentCount),
		CommentTotal: detail.CommentCount,
	}, nil
}

// forgeThread reads an issue's most recent comments, best effort: the issue is
// what every caller needs and the thread only the detail, so a thread that
// cannot be read is an empty one under the forge's own count, and an issue with
// no comments asks for none. Reading at most a page or two keeps a caller that
// wants only the issue, such as the branch or the announcement, from paging
// through a long thread against the forge's rate limit.
func forgeThread(ctx context.Context, connection forgeConnection, number, count int) []jira.Comment {
	if count == 0 {
		return nil
	}

	read, err := connection.client.RecentIssueComments(ctx, connection.repo, number, count)
	if err != nil {
		return nil
	}

	thread := make([]jira.Comment, 0, len(read))
	for _, comment := range read {
		thread = append(thread, forgeComment(comment))
	}

	return thread
}

// closeForgeIssue closes the issue behind a tracker key.
func closeForgeIssue(ctx context.Context, connect func() (forgeConnection, error), issueKey jira.Key) error {
	connection, number, err := connectToIssue(connect, issueKey)
	if err != nil {
		return err
	}

	return connection.client.CloseIssue(ctx, connection.repo, number)
}

// connectToIssue resolves both the issue number a tracker key names and the
// forge connection, the pair every single-issue call needs. The key is read
// first, so one that names no issue is reported as such whether or not the forge
// can be reached.
func connectToIssue(connect func() (forgeConnection, error), issueKey jira.Key) (forgeConnection, int, error) {
	number, err := strconv.Atoi(string(issueKey))
	if err != nil {
		// Also mark it not-found so the web API answers 404 rather than a 500: a
		// key the forge cannot resolve to an issue is a missing resource.
		return forgeConnection{}, 0, fmt.Errorf("%w: %w: %q", jira.ErrNotFound, errNotAnIssueNumber, issueKey)
	}

	connection, err := connect()
	if err != nil {
		return forgeConnection{}, 0, err
	}

	return connection, number, nil
}

// forgeIssueRow shapes a listed forge issue as a search row: its number as the
// key, its title as the summary, and the open state every listed issue has
// mapped onto the glyph's category.
func forgeIssueRow(issue forge.Issue) jira.Issue {
	return jira.Issue{
		Key:            jira.Key(strconv.Itoa(issue.Number)),
		Summary:        issue.Title,
		Status:         forgeOpenStatus,
		StatusCategory: forgeOpenCategory,
	}
}

// forgeDetailRow shapes an issue read in full as a row, closed when the forge
// says it is.
func forgeDetailRow(detail forge.IssueDetail) jira.Issue {
	row := forgeIssueRow(detail.Issue)
	if detail.Closed {
		row.Status, row.StatusCategory = forgeClosedStatus, forgeDoneCategory
	}

	return row
}

// forgeIssueTransitions is the one state change a forge issue has: close it. It
// carries no fields, so the status picker applies it without a form.
func forgeIssueTransitions() []jira.Transition {
	return []jira.Transition{{
		ID: closeTransitionID, Name: "Close", ToStatus: forgeClosedStatus, ToStatusCategory: forgeDoneCategory,
	}}
}
