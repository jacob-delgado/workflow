// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

import (
	"time"

	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/gitrepo"
	"github.com/jacob-delgado/workflow/internal/jira"
	"github.com/jacob-delgado/workflow/internal/loop"
	"github.com/jacob-delgado/workflow/internal/taskwarrior"
	"github.com/jacob-delgado/workflow/internal/tui"
)

// activityWorld is what each source answers the Summary: git's commits, the
// tasks touched, what Jira and the forge say you did, or why one could not be
// read.
type activityWorld struct {
	commits    []gitrepo.DatedCommit
	commitsErr error
	touched    []taskwarrior.Task
	jira       jira.Activity
	jiraErr    error
	forge      forge.Activity
}

// summaryRead is how a read of a source over a period is recorded.
func summaryRead(source string, start time.Time) string {
	return "summary " + source + " " + start.UTC().Format(time.RFC3339)
}

// withActivity wires the Summary's seams into deps when the world has
// answers for them, recording each read with the start it was asked for.
func (w *world) withActivity(deps tui.Deps) tui.Deps {
	if w.done == nil {
		return deps
	}

	done := w.done
	deps.Git.CommitsBetween = func(start, _ time.Time) []loop.RepositoryCommits {
		w.record(summaryRead("git", start))

		return []loop.RepositoryCommits{{Repository: "", Commits: done.commits, Failed: done.commitsErr}}
	}
	deps.Tasks.Touched = func(since time.Time) ([]taskwarrior.Task, error) {
		w.record(summaryRead("tasks", since))

		return done.touched, nil
	}
	deps.Jira.Activity = func(start, _ time.Time) (jira.Activity, error) {
		w.record(summaryRead("jira", start))

		return done.jira, done.jiraErr
	}
	deps.Forge.Activity = func(start, _ time.Time) (forge.Activity, error) {
		w.record(summaryRead("forge", start))

		return done.forge, nil
	}

	return deps
}
