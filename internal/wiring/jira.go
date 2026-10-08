// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package wiring

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/convention"
	"github.com/jacob-delgado/workflow/internal/httpx"
	"github.com/jacob-delgado/workflow/internal/jira"
	"github.com/jacob-delgado/workflow/internal/seams"
)

// errNotAJiraKey is a tracker key with no project part, such as the forge issue
// number 42 a branch can name. Jira would read that number as the id of an
// unrelated issue, so it is never asked.
var errNotAJiraKey = errors.New("not a Jira issue key")

// jiraDeps is what a surface asks of Jira, each seam reaching it through
// jiraClient, which finds the token the first time Jira is asked. It needs no
// guard for a missing or malformed configuration: the client refuses before
// sending anything, and the pane shows why.
func jiraDeps(ctx context.Context, settings config.Jira, jiraClient func() (jira.Client, error)) seams.Jira {
	// A link reads only the address, so it comes from a client that never sends
	// and so never needs the token.
	addresses := jira.New(nil, settings)

	return seams.Jira{
		Search: func(jql string, startAt int) (jira.SearchResult, error) {
			return ask(jiraClient, func(client jira.Client) (jira.SearchResult, error) {
				return client.Search(ctx, jql, startAt)
			})
		},
		SearchLenient: func(jql string, startAt int) (jira.SearchResult, error) {
			return ask(jiraClient, func(client jira.Client) (jira.SearchResult, error) {
				return client.SearchLenient(ctx, jql, startAt)
			})
		},
		Issue: func(issueKey jira.Key) (jira.IssueDetail, error) {
			return askJiraIssue(jiraClient, issueKey, func(client jira.Client) (jira.IssueDetail, error) {
				return client.Issue(ctx, issueKey)
			})
		},
		Transitions: func(issueKey jira.Key) ([]jira.Transition, error) {
			return askJiraIssue(jiraClient, issueKey, func(client jira.Client) ([]jira.Transition, error) {
				return client.Transitions(ctx, issueKey)
			})
		},
		Transition: func(issueKey jira.Key, to jira.Transition, values []jira.FieldValue) error {
			return tellJiraIssue(jiraClient, issueKey, func(client jira.Client) error {
				return client.ApplyTransition(ctx, issueKey, to, values)
			})
		},
		Comment: func(issueKey jira.Key, text string) (jira.Comment, error) {
			return askJiraIssue(jiraClient, issueKey, func(client jira.Client) (jira.Comment, error) {
				return client.AddComment(ctx, issueKey, text)
			})
		},
		Activity: func(start, end time.Time) (jira.Activity, error) {
			return ask(jiraClient, func(client jira.Client) (jira.Activity, error) {
				return client.Activity(ctx, start, end)
			})
		},
		Assign: func(issueKey jira.Key, assignee string) error {
			return tellJiraIssue(jiraClient, issueKey, func(client jira.Client) error {
				return client.Assign(ctx, issueKey, assignee)
			})
		},
		AddWorklog: func(issueKey jira.Key, timeSpent, comment string) (jira.Worklog, error) {
			return askJiraIssue(jiraClient, issueKey, func(client jira.Client) (jira.Worklog, error) {
				return client.AddWorklog(ctx, issueKey, timeSpent, comment)
			})
		},
		LinkPullRequest: func(issueKey jira.Key, pullURL, title string) error {
			return tellJiraIssue(jiraClient, issueKey, func(client jira.Client) error {
				return client.LinkPullRequest(ctx, issueKey, pullURL, title)
			})
		},
		BrowseURL: func(issueKey jira.Key) string { return browseJiraIssue(addresses, issueKey) },
	}
}

// connectJira finds the Jira token, running its command when one is set, and
// builds the client that carries it. A token source that gives none is no
// credential at all, and is reported as such before anything is sent.
func connectJira(
	ctx context.Context, settings config.Jira, httpTransport httpx.Doer, log *RequestLog,
) (jira.Client, error) {
	if settings.AuthMode() != config.AuthNone {
		token, err := resolveSetToken(ctx, settings.Token, settings.TokenCommand, settings.TokenEnv)
		if err != nil {
			return jira.Client{}, fmt.Errorf("%w: %w", jira.ErrNoCredential, err)
		}

		settings.Token = token
	}

	//nolint:bodyclose // Wrap only relays the response; the jira client reads and closes its body.
	return jira.New(log.Wrap("jira", httpTransport), settings), nil
}

// askJiraIssue asks Jira about the issue issueKey names. A key with no project
// part is refused as missing without asking, or finding the token to ask with:
// Jira reads a bare number as an issue's id, so a forge issue's 42 would reach
// whichever Jira issue has that id. The forge's issues refuse a key that is
// not a number the same way, so every surface falls back as it does for an
// issue the tracker lacks.
func askJiraIssue[T any](
	jiraClient func() (jira.Client, error), issueKey jira.Key, question func(jira.Client) (T, error),
) (T, error) {
	if !isJiraKey(issueKey) {
		var none T

		return none, notAJiraIssue(issueKey)
	}

	return ask(jiraClient, question)
}

// tellJiraIssue has Jira change the issue issueKey names, refusing a key with
// no project part as askJiraIssue does.
func tellJiraIssue(jiraClient func() (jira.Client, error), issueKey jira.Key, change func(jira.Client) error) error {
	if !isJiraKey(issueKey) {
		return notAJiraIssue(issueKey)
	}

	return tell(jiraClient, change)
}

// notAJiraIssue is the refusal of a key with no project part: missing, as an
// issue Jira lacks is.
func notAJiraIssue(issueKey jira.Key) error {
	return fmt.Errorf("%w: %w: %q", jira.ErrNotFound, errNotAJiraKey, issueKey)
}

// browseJiraIssue links an issue for someone to click, or is empty for a key
// with no project part, which has no page of its own on Jira.
func browseJiraIssue(client jira.Client, issueKey jira.Key) string {
	if !isJiraKey(issueKey) {
		return ""
	}

	return client.BrowseURL(issueKey)
}

// isJiraKey reports a key shaped like Jira's, PROJ-42, rather than the bare
// forge issue number a branch can carry instead, which Jira would refuse or
// read as the id of an unrelated issue.
func isJiraKey(issueKey jira.Key) bool {
	ref, known := convention.RefOf(string(issueKey))

	return known && ref.Tracker == convention.TrackerJira
}
