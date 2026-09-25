// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package wiring

import (
	"context"
	"errors"
	"fmt"

	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/httpx"
	"github.com/jacob-delgado/workflow/internal/jira"
	"github.com/jacob-delgado/workflow/internal/loop"
	"github.com/jacob-delgado/workflow/internal/tui"
)

// errNotAJiraKey is a tracker key with no project part, such as the forge issue
// number 42 a branch can name. Jira would read that number as the id of an
// unrelated issue, so it is never asked.
var errNotAJiraKey = errors.New("not a Jira issue key")

// jiraDeps is what the interface asks of Jira, each seam reaching it through
// jiraClient, which finds the token the first time Jira is asked. It needs no
// guard for a missing or malformed configuration: the client refuses before
// sending anything, and the pane shows why.
func jiraDeps(ctx context.Context, settings config.Jira, jiraClient func() (jira.Client, error)) tui.JiraDeps {
	// A link reads only the address, so it comes from a client that never sends
	// and so never needs the token.
	addresses := jira.New(nil, settings)

	return tui.JiraDeps{
		Search: func(jql string, startAt int) (jira.SearchResult, error) {
			return askJira(jiraClient, func(client jira.Client) (jira.SearchResult, error) {
				return client.Search(ctx, jql, startAt)
			})
		},
		Issue: func(issueKey jira.Key) (jira.IssueDetail, error) { return readJiraIssue(ctx, jiraClient, issueKey) },
		Transitions: func(issueKey jira.Key) ([]jira.Transition, error) {
			return askJira(jiraClient, func(client jira.Client) ([]jira.Transition, error) {
				return client.Transitions(ctx, issueKey)
			})
		},
		Transition: func(issueKey jira.Key, to jira.Transition, values []jira.FieldValue) error {
			return tellJira(jiraClient, func(client jira.Client) error {
				return client.ApplyTransition(ctx, issueKey, to, values)
			})
		},
		Comment: func(issueKey jira.Key, text string) (jira.Comment, error) {
			return askJira(jiraClient, func(client jira.Client) (jira.Comment, error) {
				return client.AddComment(ctx, issueKey, text)
			})
		},
		Assign: func(issueKey jira.Key, assignee string) error {
			return tellJira(jiraClient, func(client jira.Client) error { return client.Assign(ctx, issueKey, assignee) })
		},
		AddWorklog: func(issueKey jira.Key, timeSpent, comment string) (jira.Worklog, error) {
			return askJira(jiraClient, func(client jira.Client) (jira.Worklog, error) {
				return client.AddWorklog(ctx, issueKey, timeSpent, comment)
			})
		},
		LinkPullRequest: func(issueKey jira.Key, pullURL, title string) error {
			return tellJira(jiraClient, func(client jira.Client) error {
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

// askJira connects to Jira, then asks it what ask does.
func askJira[T any](jiraClient func() (jira.Client, error), ask func(jira.Client) (T, error)) (T, error) {
	client, err := jiraClient()
	if err != nil {
		var none T

		return none, err
	}

	return ask(client)
}

// tellJira connects to Jira, then has it make the change tell does.
func tellJira(jiraClient func() (jira.Client, error), tell func(jira.Client) error) error {
	client, err := jiraClient()
	if err != nil {
		return err
	}

	return tell(client)
}

// readJiraIssue reads one issue in full. A key with no project part is refused
// as missing without asking, or finding the token to ask with, as the forge's
// issues refuse a key that is not a number, so every surface falls back as it
// does for an issue Jira lacks.
func readJiraIssue(
	ctx context.Context, jiraClient func() (jira.Client, error), issueKey jira.Key,
) (jira.IssueDetail, error) {
	if !loop.IsJiraKey(issueKey) {
		return jira.IssueDetail{}, fmt.Errorf("%w: %w: %q", jira.ErrNotFound, errNotAJiraKey, issueKey)
	}

	return askJira(jiraClient, func(client jira.Client) (jira.IssueDetail, error) { return client.Issue(ctx, issueKey) })
}

// browseJiraIssue links an issue for someone to click, or is empty for a key
// with no project part, which has no page of its own on Jira.
func browseJiraIssue(client jira.Client, issueKey jira.Key) string {
	if !loop.IsJiraKey(issueKey) {
		return ""
	}

	return client.BrowseURL(issueKey)
}
