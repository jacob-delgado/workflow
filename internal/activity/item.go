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

// Outcome is how a source answered a read.
type Outcome int

const (
	// OutcomeRead is a source that answered; its items are what it said.
	OutcomeRead Outcome = iota
	// OutcomeFailed is a source that is set up and refused.
	OutcomeFailed
	// OutcomeNotSetUp is a source with nothing set up to ask, which a summary
	// leaves out rather than calls unread.
	OutcomeNotSetUp
)

// Read is what one source said for a period: its items, whether it had more
// than it gave, and its outcome, with why it could not be read when it could
// not. A read starts read; Failing and LeftOut move it on.
type Read struct {
	Source    Source
	Items     []Item
	Truncated bool
	outcome   Outcome
	why       error
}

// Outcome is how the source answered.
func (r Read) Outcome() Outcome { return r.outcome }

// Why is why the source could not be read, or nil when it was.
func (r Read) Why() error { return r.why }

// Failing is the read refused for err, whatever it was before: a refusal
// outranks a source not set up. A nil err leaves the read as it was.
func (r Read) Failing(err error) Read {
	if err == nil {
		return r
	}

	r.outcome, r.why = OutcomeFailed, err

	return r
}

// LeftOut is the read left out as not set up, for err, unless it already
// failed, which a failed read stays. A nil err leaves the read as it was.
func (r Read) LeftOut(err error) Read {
	if err == nil || r.outcome == OutcomeFailed {
		return r
	}

	r.outcome, r.why = OutcomeNotSetUp, err

	return r
}

// Merge is the items of every list, oldest first, items at the same moment in
// the order they were listed. It keeps every item: a commit two repositories
// share is read once by the read that lists them (loop.CommitsRead), by its
// full hash, where an item's abbreviated Ref could not tell it apart.
func Merge(lists ...[]Item) []Item {
	merged := slices.Concat(lists...)

	slices.SortStableFunc(merged, func(a, b Item) int { return a.At.Compare(b.At) })

	return merged
}
