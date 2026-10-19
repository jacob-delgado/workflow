// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package demo

import (
	"context"
	"slices"
	"time"

	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/gitrepo"
	"github.com/jacob-delgado/workflow/internal/jira"
	"github.com/jacob-delgado/workflow/internal/loop"
)

// yesterday is a time of day on the working day before the demo's, which the
// Summary opens on.
func yesterday(hour, minute int) time.Time {
	day := epoch().Truncate(24*time.Hour).AddDate(0, 0, -1)

	return day.Add(time.Duration(hour)*time.Hour + time.Duration(minute)*time.Minute)
}

// within reports at in the period from start up to end.
func within(at, start, end time.Time) bool {
	return !at.Before(start) && at.Before(end)
}

// during is the items whose time falls in the period from start up to end.
func during[T any](items []T, at func(T) time.Time, start, end time.Time) []T {
	return slices.DeleteFunc(items, func(item T) bool { return !within(at(item), start, end) })
}

// commitsBetween is the commits you wrote in api over a period: yesterday's
// refactor, merged into main.
func (w *World) commitsBetween(_ context.Context, start, end time.Time) []loop.RepositoryCommits {
	written := []gitrepo.DatedCommit{{
		Hash: "c3d4e5f60718293a4b5c6d7e8f90a1b2c3d4e5f6", Short: "c3d4e5f",
		Subject: "refactor(config): route every write through Redact", Authored: yesterday(9, 12),
	}}

	shown := during(written, func(commit gitrepo.DatedCommit) time.Time { return commit.Authored }, start, end)

	return []loop.RepositoryCommits{{Repository: "", Commits: shown}}
}

// jiraActivity is what you did to issues over a period: PROJ-412 moved into
// progress yesterday, and a comment on PROJ-418.
func (w *World) jiraActivity(_ context.Context, start, end time.Time) (jira.Activity, error) {
	events := []jira.Event{
		{
			At: yesterday(11, 30), Kind: jira.EventCommented, Key: "PROJ-418",
			Summary: "Refuse to start when the config names an unknown forge",
		},
		{At: yesterday(14, 5), Kind: jira.EventMoved, Key: workedKey, Summary: workedSummary, Detail: statusInProgress},
	}

	return jira.Activity{Events: during(events, func(event jira.Event) time.Time { return event.At }, start, end)}, nil
}

// forgeActivity is what you did on GitHub over a period: Lee's review-queue
// pull request reviewed yesterday.
func (w *World) forgeActivity(_ context.Context, start, end time.Time) (forge.Activity, error) {
	events := []forge.Event{{
		At: yesterday(14, 20), Kind: forge.EventReviewed, Number: 391, Title: "List the review queue on the web",
		URL: toolURL + "/pull/391", Repository: toolRepository,
	}}

	return forge.Activity{Events: during(events, func(event forge.Event) time.Time { return event.At }, start, end)}, nil
}
