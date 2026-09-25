// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package wiring

import (
	"context"
	"errors"
	"fmt"

	"github.com/jacob-delgado/workflow/internal/jira"
	"github.com/jacob-delgado/workflow/internal/loop"
)

// errNotAJiraKey is a tracker key with no project part, such as the forge issue
// number 42 a branch can name. Jira would read that number as the id of an
// unrelated issue, so it is never asked.
var errNotAJiraKey = errors.New("not a Jira issue key")

// readJiraIssue reads one issue in full. A key with no project part is refused
// as missing without asking, as the forge's issues refuse a key that is not a
// number, so every surface falls back as it does for an issue Jira lacks.
func readJiraIssue(ctx context.Context, client jira.Client, issueKey jira.Key) (jira.IssueDetail, error) {
	if !loop.IsJiraKey(issueKey) {
		return jira.IssueDetail{}, fmt.Errorf("%w: %w: %q", jira.ErrNotFound, errNotAJiraKey, issueKey)
	}

	return client.Issue(ctx, issueKey)
}

// browseJiraIssue links an issue for someone to click, or is empty for a key
// with no project part, which has no page of its own on Jira.
func browseJiraIssue(client jira.Client, issueKey jira.Key) string {
	if !loop.IsJiraKey(issueKey) {
		return ""
	}

	return client.BrowseURL(issueKey)
}
