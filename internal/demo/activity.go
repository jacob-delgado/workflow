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
// refactor, merged into main, and every commit the demo has made since.
func (w *World) commitsBetween(_ context.Context, start, end time.Time) []loop.RepositoryCommits {
	refactor := gitrepo.DatedCommit{
		Hash: "e5d2b87f05edefb5b1d3e891e49886a82447e515", Short: "e5d2b87",
		Subject: "refactor(config): route every write through Redact", Authored: yesterday(9, 12),
	}
	written := append([]gitrepo.DatedCommit{refactor}, locked(w, func() []gitrepo.DatedCommit {
		return slices.Clone(w.repo.written)
	})...)

	shown := during(written, func(commit gitrepo.DatedCommit) time.Time { return commit.Authored }, start, end)

	return []loop.RepositoryCommits{{Repository: "", Commits: shown}}
}

// jiraActivity is what you did to issues over a period: PROJ-412 moved into
// progress yesterday, a comment on PROJ-418, and every move the demo made.
func (w *World) jiraActivity(_ context.Context, start, end time.Time) (jira.Activity, error) {
	commented := jira.Event{
		At: yesterday(11, 30), Kind: jira.EventCommented, Key: "PROJ-418",
		Summary: "Refuse to start when the config names an unknown forge",
	}
	moved := jira.Event{
		At: yesterday(14, 5), Kind: jira.EventMoved, Key: workedKey, Summary: workedSummary, Detail: statusInProgress,
	}
	events := append([]jira.Event{commented, moved}, locked(w, func() []jira.Event { return slices.Clone(w.moves) })...)

	return jira.Activity{Events: during(events, func(event jira.Event) time.Time { return event.At }, start, end)}, nil
}

// forgeActivity is what you did on GitHub over a period: Sam's review-queue
// pull request reviewed yesterday, and every pull request the demo opened.
func (w *World) forgeActivity(_ context.Context, start, end time.Time) (forge.Activity, error) {
	reviewed := forge.Event{
		At: yesterday(14, 20), Kind: forge.EventReviewed, Number: 377,
		Title: "Retry the forge search once on a gateway timeout", URL: toolURL + "/pull/377", Repository: toolRepository,
	}
	events := append([]forge.Event{reviewed}, w.pullsOpened()...)

	return forge.Activity{Events: during(events, func(event forge.Event) time.Time { return event.At }, start, end)}, nil
}
