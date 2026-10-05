// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package loop

import (
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/jacob-delgado/workflow/internal/activity"
	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/gitrepo"
	"github.com/jacob-delgado/workflow/internal/jira"
	"github.com/jacob-delgado/workflow/internal/taskwarrior"
)

// shortUUID is how much of a UUID names a task that has no id, as Taskwarrior
// itself shortens one.
const shortUUID = 8

// RepositoryCommits is one repository's commits for the Summary, or why they
// could not be read. Repository names it when the Summary reads more than one,
// and is "" when it reads only the repository workflow is in.
type RepositoryCommits struct {
	Repository string
	Commits    []gitrepo.DatedCommit
	Failed     error
}

// CommitsRead is your commits from start up to end as the Summary reads them,
// each named by its repository when there is more than one. A commit two
// repositories share — a fork, a second clone — is read once, from the first;
// a repository that cannot be read is named in the failure, and the others'
// commits are still read.
func CommitsRead(read func(start, end time.Time) []RepositoryCommits, start, end time.Time) activity.Read {
	var (
		items    []activity.Item
		failures []error
	)

	seen := map[string]bool{}

	for _, repository := range read(start, end) {
		if repository.Failed != nil {
			failures = append(failures, repository.failure())
		}

		for _, commit := range repository.Commits {
			if commit.Hash != "" && seen[commit.Hash] {
				continue
			}

			seen[commit.Hash] = true

			items = append(items, repository.item(commit))
		}
	}

	return activity.Read{Source: activity.SourceGit, Items: items, Failed: errors.Join(failures...)}
}

// item is one commit as the Summary lists it, its hash written as GitHub
// writes a commit in another repository when the repository is named.
func (r RepositoryCommits) item(commit gitrepo.DatedCommit) activity.Item {
	ref := commit.Short
	if r.Repository != "" {
		ref = r.Repository + "@" + commit.Short
	}

	return activity.Item{
		At: commit.Authored, Kind: activity.Committed, Ref: ref, Title: commit.Subject, Repository: r.Repository,
	}
}

// failure is why the repository could not be read, naming it when it has a
// name.
func (r RepositoryCommits) failure() error {
	if r.Repository == "" {
		return r.Failed
	}

	return fmt.Errorf("%s: %w", r.Repository, r.Failed)
}

// TasksRead is what you did to tasks from start up to end: each task touched
// since start reads as its adding, starting, notes and completing that fall
// in the period.
func TasksRead(touched func(since time.Time) ([]taskwarrior.Task, error), start, end time.Time) activity.Read {
	tasks, err := touched(start)
	if err != nil {
		return activity.Read{Source: activity.SourceTasks, Failed: err}
	}

	var items []activity.Item

	for _, task := range tasks {
		add := func(at time.Time, kind activity.Kind) {
			if !at.IsZero() && !at.Before(start) && at.Before(end) {
				items = append(items, activity.Item{At: at, Kind: kind, Ref: taskRef(task), Title: task.Description})
			}
		}

		add(task.Entry, activity.TaskAdded)
		add(task.Start, activity.TaskStarted)

		for _, note := range task.Annotations {
			add(note.Entry, activity.TaskAnnotated)
		}

		if task.Status == taskwarrior.Completed {
			add(task.End, activity.TaskCompleted)
		}
	}

	return activity.Read{Source: activity.SourceTasks, Items: items}
}

// taskRef names a task by its id, or by the start of its UUID once it has
// none, as a completed task does not.
func taskRef(task taskwarrior.Task) string {
	if task.ID > 0 {
		return strconv.Itoa(task.ID)
	}

	return task.UUID[:min(len(task.UUID), shortUUID)]
}

// JiraRead is what you did to Jira issues from start up to end, each linked
// to its page as browse names it.
func JiraRead(
	read func(start, end time.Time) (jira.Activity, error), browse func(jira.Key) string, start, end time.Time,
) activity.Read {
	done, err := read(start, end)
	if err != nil {
		return activity.Read{Source: activity.SourceJira, Failed: err}
	}

	kinds := map[jira.EventKind]activity.Kind{
		jira.EventCreated: activity.IssueCreated, jira.EventMoved: activity.IssueMoved,
		jira.EventWorked: activity.IssueWorked, jira.EventCommented: activity.IssueCommented,
	}

	items := make([]activity.Item, 0, len(done.Events))
	for _, event := range done.Events {
		items = append(items, activity.Item{
			At: event.At, Kind: kinds[event.Kind], Ref: string(event.Key), Title: eventTitle(event), URL: browse(event.Key),
		})
	}

	return activity.Read{Source: activity.SourceJira, Items: items, Truncated: done.Truncated}
}

// eventTitle is an issue's summary, with the status it was moved to or the
// time logged on it.
func eventTitle(event jira.Event) string {
	switch event.Kind {
	case jira.EventMoved:
		return event.Summary + ", to " + event.Detail
	case jira.EventWorked:
		return event.Summary + ", " + event.Detail
	case jira.EventCreated, jira.EventCommented:
		return event.Summary
	}

	return event.Summary
}

// ForgeRead is what you did on the forge from start up to end, each pull or
// merge request named by its repository and number as kind writes it.
func ForgeRead(
	read func(start, end time.Time) (forge.Activity, error), kind forge.Kind, start, end time.Time,
) activity.Read {
	done, err := read(start, end)
	if err != nil {
		return activity.Read{Source: activity.SourceForge, Failed: err}
	}

	kinds := map[forge.EventKind]activity.Kind{
		forge.EventOpened: activity.PullOpened, forge.EventMerged: activity.PullMerged,
		forge.EventReviewed: activity.PullReviewed,
	}

	items := make([]activity.Item, 0, len(done.Events))
	for _, event := range done.Events {
		items = append(items, activity.Item{
			At: event.At, Kind: kinds[event.Kind], Ref: event.Repository + kind.Sigil() + strconv.Itoa(event.Number),
			Title: event.Title, URL: event.URL, Repository: event.Repository,
		})
	}

	return activity.Read{Source: activity.SourceForge, Items: items, Truncated: done.Truncated}
}
