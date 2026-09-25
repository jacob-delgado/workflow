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
	"github.com/jacob-delgado/workflow/internal/tui"
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
// when it is configured, and otherwise the forge's own issues, reached through
// connect, so a project without Jira still has a tracker to run the loop
// against.
func trackerDeps(
	ctx context.Context, settings config.Jira,
	jiraClient func() (jira.Client, error), connect func() (forgeConnection, error),
) tui.JiraDeps {
	if settings.Configured() {
		return jiraDeps(ctx, settings, jiraClient)
	}

	return forgeIssuesDeps(ctx, connect)
}

// forgeIssuesDeps adapts the forge's issues to the tracker seam the Issues pane
// reads. The pane, the detail and the status change run unchanged; a comment and
// a remote link, which a forge issue has no equivalent for, are left nil so
// those features simply do not appear.
func forgeIssuesDeps(ctx context.Context, connect func() (forgeConnection, error)) tui.JiraDeps {
	return tui.JiraDeps{
		Search:      func(string, int) (jira.SearchResult, error) { return listForgeIssues(ctx, connect) },
		Issue:       func(issueKey jira.Key) (jira.IssueDetail, error) { return readForgeIssue(ctx, connect, issueKey) },
		Transitions: func(jira.Key) ([]jira.Transition, error) { return forgeIssueTransitions(), nil },
		Transition: func(issueKey jira.Key, _ jira.Transition, _ []jira.FieldValue) error {
			return closeForgeIssue(ctx, connect, issueKey)
		},
	}
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
		Issue:       forgeDetailRow(detail),
		Description: detail.Body,
		Reporter:    detail.Author,
	}, nil
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
