// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package activity

import (
	"cmp"
	"slices"
	"time"
	"unicode"
	"unicode/utf8"
)

// Source is a place work leaves a trace.
type Source int

const (
	// SourceGit is the commits in the repository.
	SourceGit Source = iota + 1
	// SourceTasks is Taskwarrior.
	SourceTasks
	// SourceJira is Jira.
	SourceJira
	// SourceForge is GitHub or GitLab.
	SourceForge
)

// Name is the source as a sentence names it.
func (s Source) Name() string {
	switch s {
	case SourceGit:
		return "git"
	case SourceTasks:
		return "Taskwarrior"
	case SourceJira:
		return "Jira"
	case SourceForge:
		return "the forge"
	}

	return "a source"
}

// Title is the source as a sentence starts with it.
func (s Source) Title() string {
	name := s.Name()
	first, size := utf8.DecodeRuneInString(name)

	return string(unicode.ToUpper(first)) + name[size:]
}

// Kind is what was done.
type Kind int

const (
	// Committed is a commit you wrote, at its author date.
	Committed Kind = iota + 1
	// TaskAdded is a task added.
	TaskAdded
	// TaskStarted is a task started.
	TaskStarted
	// TaskAnnotated is a note added to a task.
	TaskAnnotated
	// TaskCompleted is a task done.
	TaskCompleted
	// IssueCreated is an issue you reported.
	IssueCreated
	// IssueMoved is an issue you moved to another status.
	IssueMoved
	// IssueWorked is work you logged on an issue.
	IssueWorked
	// IssueCommented is a comment you wrote on an issue.
	IssueCommented
	// PullOpened is a pull or merge request you opened.
	PullOpened
	// PullMerged is a pull or merge request of yours that was merged.
	PullMerged
	// PullReviewed is a pull or merge request you reviewed.
	PullReviewed
)

// Verb is what was done, as a line of the summary says it.
func (k Kind) Verb() string {
	verbs := map[Kind]string{
		Committed: "committed", TaskAdded: "added task", TaskStarted: "started task",
		TaskAnnotated: "annotated task", TaskCompleted: "completed task",
		IssueCreated: "created", IssueMoved: "moved", IssueWorked: "logged work on",
		IssueCommented: "commented on", PullOpened: "opened", PullMerged: "merged", PullReviewed: "reviewed",
	}

	return cmp.Or(verbs[k], "did")
}

// Source is where a kind of thing done leaves its trace.
func (k Kind) Source() Source {
	switch k {
	case Committed:
		return SourceGit
	case TaskAdded, TaskStarted, TaskAnnotated, TaskCompleted:
		return SourceTasks
	case IssueCreated, IssueMoved, IssueWorked, IssueCommented:
		return SourceJira
	case PullOpened, PullMerged, PullReviewed:
		return SourceForge
	}

	return SourceGit
}

// Item is one thing done: when, what, and what it was done to — a commit's
// hash, a task's id, an issue's key or a pull request's number — with a link
// to it where there is one, and the repository it was in when the summary
// reads more than one.
type Item struct {
	At         time.Time
	Kind       Kind
	Ref        string
	Title      string
	URL        string
	Repository string
}

// Read is what one source said for a period: its items, whether it had more
// than it gave, and why it could not be read when it could not — Failed, a
// source that is set up and refused, or NotSetUp, one with nothing set up to
// ask, which a summary leaves out rather than calls unread.
type Read struct {
	Source    Source
	Items     []Item
	Truncated bool
	Failed    error
	NotSetUp  error
}

// Merge is the items of every list, oldest first, with a commit seen more
// than once — on two branches, or in two repositories that share it — kept
// once.
func Merge(lists ...[]Item) []Item {
	var merged []Item

	seen := map[string]bool{}

	for _, item := range slices.Concat(lists...) {
		if item.Kind == Committed && item.Ref != "" {
			if seen[item.Ref] {
				continue
			}

			seen[item.Ref] = true
		}

		merged = append(merged, item)
	}

	slices.SortStableFunc(merged, func(a, b Item) int { return a.At.Compare(b.At) })

	return merged
}
