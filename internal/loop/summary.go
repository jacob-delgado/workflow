// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package loop

import (
	"errors"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/jacob-delgado/workflow/internal/activity"
	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/gitrepo"
	"github.com/jacob-delgado/workflow/internal/jira"
	"github.com/jacob-delgado/workflow/internal/messaging"
	"github.com/jacob-delgado/workflow/internal/taskwarrior"
)

// ErrEmptySummary refuses a Summary edited down to nothing: there is nothing
// to post.
var ErrEmptySummary = errors.New("nothing to post: the summary was empty")

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

	return unanswered(activity.Read{Source: activity.SourceGit, Items: items}, repositoryErrors(failures))
}

// NotSetUp reports an error that says a service was never set up to ask, as
// against one that was asked and refused: Jira or the forge has no
// credential, origin names no forge workflow reads, no Taskwarrior is
// installed or it has never run, or git has no user.email. Every surface tells
// it as guidance — what to set up — rather than as a failure.
func NotSetUp(err error) bool {
	for _, missing := range []error{
		jira.ErrNoCredential, forge.ErrNoToken, forge.ErrNotARemote, forge.ErrUnknownForge, gitrepo.ErrNoIdentity,
		taskwarrior.ErrNotInstalled, taskwarrior.ErrNotTaskwarrior, taskwarrior.ErrNotConfigured,
	} {
		if errors.Is(err, missing) {
			return true
		}
	}

	return false
}

// unanswered is read with why its source did not answer: NotSetUp when every
// failure says nothing was set up to ask, Failed when any is a refusal.
func unanswered(read activity.Read, err error) activity.Read {
	if err == nil {
		return read
	}

	if slices.ContainsFunc(Failures(err), func(failure error) bool { return !NotSetUp(failure) }) {
		read.Failed = err
	} else {
		read.NotSetUp = err
	}

	return read
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

	return RepositoryError{Repository: r.Repository, Err: r.Failed}
}

// RepositoryError is why one repository of several could not be read, with
// the name it is listed by, so a surface that words the failure its own way
// can still say which repository it was.
type RepositoryError struct {
	Repository string
	Err        error
}

// Error is the failure prefixed by the repository's name.
func (f RepositoryError) Error() string { return f.Repository + ": " + f.Err.Error() }

// Unwrap is the failure itself, so errors.Is still classifies it.
func (f RepositoryError) Unwrap() error { return f.Err }

// RepositoryErrors is why each repository of a read could not be read, one
// failure apiece. It is its own type, rather than errors.Join's, so Failures
// splits it and never an error that merely wraps two causes.
type RepositoryErrors []error

// Error is every failure, one to a line.
func (e RepositoryErrors) Error() string { return errors.Join(e...).Error() }

// Unwrap is the failures, so errors.Is and errors.As still classify each.
func (e RepositoryErrors) Unwrap() []error { return e }

// repositoryErrors is failures as one error, or nil when there are none.
func repositoryErrors(failures []error) error {
	if len(failures) == 0 {
		return nil
	}

	return RepositoryErrors(failures)
}

// Failures is each failure a read's error holds: each repository's when
// CommitsRead read several, the error itself otherwise, none when there is
// none.
func Failures(err error) []error {
	if err == nil {
		return nil
	}

	if each, several := errors.AsType[RepositoryErrors](err); several {
		return each
	}

	return []error{err}
}

// TasksRead is what you did to tasks from start up to end: each task touched
// since start reads as its adding, starting, notes and completing that fall
// in the period.
func TasksRead(touched func(since time.Time) ([]taskwarrior.Task, error), start, end time.Time) activity.Read {
	tasks, err := touched(start)
	if err != nil {
		return unanswered(activity.Read{Source: activity.SourceTasks}, err)
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
		return unanswered(activity.Read{Source: activity.SourceJira}, err)
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
		return unanswered(activity.Read{Source: activity.SourceForge}, err)
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

// ActivitySeams are what the Summary asks each source: the commits you wrote,
// the tasks touched since a time, what you did to Jira issues — linked as
// BrowseURL links an issue — and on the forge, whose changes ForgeKind names.
// A nil read is a source the Summary does not ask.
type ActivitySeams struct {
	Commits   func(start, end time.Time) []RepositoryCommits
	Touched   func(since time.Time) ([]taskwarrior.Task, error)
	Jira      func(start, end time.Time) (jira.Activity, error)
	BrowseURL func(jira.Key) string
	Forge     func(start, end time.Time) (forge.Activity, error)
	ForgeKind forge.Kind
}

// SourceRead is one source's read for the Summary, not yet made, so a surface
// can make each on its own and show each as it answers.
type SourceRead struct {
	Source activity.Source
	Read   func() activity.Read
}

// SummaryReads are the reads of every source the seams reach, from start up
// to end, in the order the sources are listed: the one way every surface asks
// them, so a period reads the same in each.
func SummaryReads(seams ActivitySeams, start, end time.Time) []SourceRead {
	var reads []SourceRead

	if seams.Commits != nil {
		reads = append(reads, SourceRead{activity.SourceGit, func() activity.Read {
			return CommitsRead(seams.Commits, start, end)
		}})
	}

	if seams.Touched != nil {
		reads = append(reads, SourceRead{activity.SourceTasks, func() activity.Read {
			return TasksRead(seams.Touched, start, end)
		}})
	}

	if seams.Jira != nil {
		reads = append(reads, SourceRead{activity.SourceJira, func() activity.Read {
			return JiraRead(seams.Jira, seams.browse, start, end)
		}})
	}

	if seams.Forge != nil {
		reads = append(reads, SourceRead{activity.SourceForge, func() activity.Read {
			return ForgeRead(seams.Forge, seams.ForgeKind, start, end)
		}})
	}

	return reads
}

// browse links an issue as BrowseURL does, or not at all without one.
func (s ActivitySeams) browse(issueKey jira.Key) string {
	if s.BrowseURL == nil {
		return ""
	}

	return s.BrowseURL(issueKey)
}

// ReadAll makes each read in turn, for a surface that shows the Summary once
// every source has answered.
func ReadAll(reads []SourceRead) []activity.Read {
	made := make([]activity.Read, 0, len(reads))
	for _, read := range reads {
		made = append(made, read.Read())
	}

	return made
}

// Trade-off TRADE-32: a posted Summary is not recorded, as an announcement
// is; what was done is read back from the sources each time.

// PostSummary posts text — the Summary's Markdown, as it was previewed or
// edited — to channel, rendered for the service kind names.
func PostSummary(post func(channel, text string) error, kind config.MessagingKind, channel, text string) error {
	if strings.TrimSpace(text) == "" {
		return ErrEmptySummary
	}

	return post(channel, messaging.RenderMarkdown(kind, text))
}
