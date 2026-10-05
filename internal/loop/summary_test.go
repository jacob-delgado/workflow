// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package loop_test

import (
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/jacob-delgado/workflow/internal/activity"
	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/gitrepo"
	"github.com/jacob-delgado/workflow/internal/jira"
	"github.com/jacob-delgado/workflow/internal/loop"
	"github.com/jacob-delgado/workflow/internal/taskwarrior"
)

var errNotRead = errors.New("not read")

// fixSubject and doneStatus are what the cases' commit and issue say.
const (
	fixSubject = "Fix it"
	doneStatus = "Done"
)

// apiRepository and webRepository name the two repositories a read across
// several finds; headHash is the full hash headCommit shortens.
const (
	apiRepository = "acme/api"
	webRepository = "acme/web"
	headHash      = headCommit + "ffff"
	styleSubject  = "Style it"
)

// summaryStart is the start of the one-day period every case reads.
func summaryStart() time.Time { return time.Date(2026, 10, 2, 0, 0, 0, 0, time.UTC) }

func TestYourCommitsReadAsCommitted(t *testing.T) {
	t.Parallel()

	// Arrange
	start := summaryStart()
	read := func(time.Time, time.Time) []loop.RepositoryCommits {
		return []loop.RepositoryCommits{{Commits: []gitrepo.DatedCommit{
			{Hash: headHash, Short: headCommit, Subject: fixSubject, Authored: start},
		}}}
	}

	// Act
	got := loop.CommitsRead(read, start, start.Add(24*time.Hour))

	// Assert
	want := activity.Item{At: start, Kind: activity.Committed, Ref: headCommit, Title: fixSubject}
	if got.Source != activity.SourceGit || len(got.Items) != 1 || got.Items[0] != want {
		t.Errorf("CommitsRead = %+v, want %+v from git", got, want)
	}
}

// twoRepositories answers a read as api's commits and web's, at start.
func twoRepositories(start time.Time, web loop.RepositoryCommits) func(time.Time, time.Time) []loop.RepositoryCommits {
	return func(time.Time, time.Time) []loop.RepositoryCommits {
		return []loop.RepositoryCommits{
			{Repository: apiRepository, Commits: []gitrepo.DatedCommit{
				{Hash: headHash, Short: headCommit, Subject: fixSubject, Authored: start},
			}},
			web,
		}
	}
}

func TestCommitsInSeveralRepositoriesAreNamedByTheirRepository(t *testing.T) {
	t.Parallel()

	// Arrange
	start := summaryStart()
	read := twoRepositories(start, loop.RepositoryCommits{Repository: webRepository, Commits: []gitrepo.DatedCommit{
		{Hash: "def5678ffff", Short: "def5678", Subject: styleSubject, Authored: start},
	}})

	// Act
	got := loop.CommitsRead(read, start, start.Add(24*time.Hour))

	// Assert
	want := []activity.Item{
		{
			At: start, Kind: activity.Committed, Ref: apiRepository + "@" + headCommit, Title: fixSubject,
			Repository: apiRepository,
		},
		{
			At: start, Kind: activity.Committed, Ref: webRepository + "@def5678", Title: styleSubject,
			Repository: webRepository,
		},
	}
	if got.Failed != nil || !slices.Equal(got.Items, want) {
		t.Errorf("CommitsRead = %+v, want %+v", got, want)
	}
}

func TestACommitInTwoRepositoriesIsReadOnceFromTheFirst(t *testing.T) {
	t.Parallel()

	// Arrange
	// A fork, or a second clone, holds the same commit under the same hash.
	start := summaryStart()
	read := twoRepositories(start, loop.RepositoryCommits{Repository: "acme/api-fork", Commits: []gitrepo.DatedCommit{
		{Hash: headHash, Short: headCommit, Subject: fixSubject, Authored: start},
	}})

	// Act
	got := loop.CommitsRead(read, start, start.Add(24*time.Hour))

	// Assert
	if len(got.Items) != 1 || got.Items[0].Repository != apiRepository {
		t.Errorf("CommitsRead = %+v, want the commit once, in acme/api", got.Items)
	}
}

func TestCommitsWithNoHashAreNeverTakenForOneAnother(t *testing.T) {
	t.Parallel()

	// Arrange
	start := summaryStart()
	read := func(time.Time, time.Time) []loop.RepositoryCommits {
		return []loop.RepositoryCommits{{Commits: []gitrepo.DatedCommit{
			{Short: "1111111", Subject: fixSubject, Authored: start},
			{Short: "2222222", Subject: styleSubject, Authored: start},
		}}}
	}

	// Act
	got := loop.CommitsRead(read, start, start.Add(24*time.Hour))

	// Assert
	if len(got.Items) != 2 {
		t.Errorf("CommitsRead = %+v, want both commits", got.Items)
	}
}

func TestARepositoryThatCannotBeReadIsNamedAndTheOthersStillRead(t *testing.T) {
	t.Parallel()

	// Arrange
	start := summaryStart()
	read := twoRepositories(start, loop.RepositoryCommits{Repository: webRepository, Failed: errNotRead})

	// Act
	got := loop.CommitsRead(read, start, start.Add(24*time.Hour))

	// Assert
	if !errors.Is(got.Failed, errNotRead) || !strings.Contains(got.Failed.Error(), webRepository) || len(got.Items) != 1 {
		t.Errorf("CommitsRead = %+v, want acme/web named as failed beside acme/api's commit", got)
	}
}

func TestEachRepositoryThatFailedIsNamedInItsOwnFailure(t *testing.T) {
	t.Parallel()

	// Arrange
	start := summaryStart()
	read := func(time.Time, time.Time) []loop.RepositoryCommits {
		return []loop.RepositoryCommits{
			{Repository: apiRepository, Failed: errNotRead},
			{Repository: webRepository, Failed: errNotRead},
		}
	}

	// Act
	got := loop.CommitsRead(read, start, start.Add(24*time.Hour))

	// Assert
	var named []string

	for _, failure := range loop.Failures(got.Failed) {
		repository, isNamed := errors.AsType[loop.RepositoryError](failure)
		if isNamed && errors.Is(repository.Err, errNotRead) {
			named = append(named, repository.Repository)
		}
	}

	if !slices.Equal(named, []string{apiRepository, webRepository}) {
		t.Errorf("failures named %q, want %q", named, []string{apiRepository, webRepository})
	}
}

func TestFailuresOfOneErrorIsThatError(t *testing.T) {
	t.Parallel()

	// Act
	got := loop.Failures(errNotRead)

	// Assert
	if len(got) != 1 || !errors.Is(got[0], errNotRead) {
		t.Errorf("Failures(errNotRead) = %v, want it alone", got)
	}
}

func TestFailuresOfNoErrorIsNone(t *testing.T) {
	t.Parallel()

	// Act & Assert
	if got := loop.Failures(nil); got != nil {
		t.Errorf("Failures(nil) = %v, want none", got)
	}
}

func TestATaskReadsAsEachThingDoneToItInThePeriod(t *testing.T) {
	t.Parallel()

	// Arrange
	start := summaryStart()
	task := taskwarrior.Task{
		UUID: "8f1c2d3e-0000", ID: 0, Status: "completed", Description: "Ship it",
		Entry: start.Add(-48 * time.Hour), Start: start.Add(time.Hour), End: start.Add(3 * time.Hour),
		Annotations: []taskwarrior.Annotation{{Entry: start.Add(2 * time.Hour), Description: "half way"}},
	}
	read := func(time.Time) ([]taskwarrior.Task, error) { return []taskwarrior.Task{task}, nil }

	// Act
	got := loop.TasksRead(read, start, start.Add(24*time.Hour))

	// Assert
	// Added before the period, so not counted; a completed task has no id,
	// so it is named by the start of its UUID.
	kinds := []activity.Kind{activity.TaskStarted, activity.TaskAnnotated, activity.TaskCompleted}
	if len(got.Items) != len(kinds) {
		t.Fatalf("TasksRead = %+v, want %d items", got.Items, len(kinds))
	}

	for index, item := range activity.Merge(got.Items) {
		if item.Kind != kinds[index] || item.Ref != "8f1c2d3e" || item.Title != "Ship it" {
			t.Errorf("item %d = %+v, want %v of 8f1c2d3e Ship it", index, item, kinds[index])
		}
	}
}

func TestJirasEventsReadAsIssueItemsWithTheirLinks(t *testing.T) {
	t.Parallel()

	// Arrange
	start := summaryStart()
	read := func(time.Time, time.Time) (jira.Activity, error) {
		return jira.Activity{Truncated: true, Events: []jira.Event{
			{At: start, Kind: jira.EventMoved, Key: issueKey, Summary: fixSubject, Detail: doneStatus},
		}}, nil
	}
	browse := func(key jira.Key) string { return "https://jira.example.com/browse/" + string(key) }

	// Act
	got := loop.JiraRead(read, browse, start, start.Add(24*time.Hour))

	// Assert
	want := activity.Item{
		At: start, Kind: activity.IssueMoved, Ref: issueKey, Title: fixSubject + ", to " + doneStatus,
		URL: "https://jira.example.com/browse/" + issueKey,
	}
	if !got.Truncated || len(got.Items) != 1 || got.Items[0] != want {
		t.Errorf("JiraRead = %+v, want %+v, truncated", got, want)
	}
}

func TestForgeEventsReadByTheirRepositoryAndNumber(t *testing.T) {
	t.Parallel()

	// Arrange
	start := summaryStart()
	read := func(time.Time, time.Time) (forge.Activity, error) {
		return forge.Activity{Events: []forge.Event{
			{At: start, Kind: forge.EventMerged, Number: 42, Title: "Redact", URL: "https://x/42", Repository: "o/r"},
			{At: start, Kind: forge.EventOpened, Number: 5, Title: "Lint"},
		}}, nil
	}

	// Act
	got := loop.ForgeRead(read, forge.KindGitLab, start, start.Add(24*time.Hour))

	// Assert
	if len(got.Items) != 2 || got.Items[0].Ref != "o/r!42" || got.Items[0].Kind != activity.PullMerged ||
		got.Items[1].Ref != "!5" || got.Items[1].Kind != activity.PullOpened {
		t.Errorf("ForgeRead = %+v, want o/r!42 merged and !5 opened", got.Items)
	}
}

func TestEveryJiraEventReadsAsItsOwnKind(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		event jira.Event
		kind  activity.Kind
		title string
	}{
		"created": {jira.Event{Kind: jira.EventCreated, Summary: fixSubject}, activity.IssueCreated, fixSubject},
		"worked": {
			jira.Event{Kind: jira.EventWorked, Summary: fixSubject, Detail: "2h"}, activity.IssueWorked, fixSubject + ", 2h",
		},
		"commented": {jira.Event{Kind: jira.EventCommented, Summary: fixSubject}, activity.IssueCommented, fixSubject},
	}

	for name, happened := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			start := summaryStart()
			event := happened.event
			event.At, event.Key = start, issueKey
			read := func(time.Time, time.Time) (jira.Activity, error) {
				return jira.Activity{Events: []jira.Event{event}}, nil
			}

			// Act
			got := loop.JiraRead(read, func(jira.Key) string { return "" }, start, start.Add(24*time.Hour))

			// Assert
			if len(got.Items) != 1 || got.Items[0].Kind != happened.kind || got.Items[0].Title != happened.title {
				t.Errorf("JiraRead = %+v, want one %v titled %q", got.Items, happened.kind, happened.title)
			}
		})
	}
}

func TestAPendingTaskIsNamedByItsIDAndNotCompleted(t *testing.T) {
	t.Parallel()

	// Arrange
	start := summaryStart()
	task := taskwarrior.Task{
		UUID: "8f1c2d3e-0000", ID: 12, Status: "pending", Description: "Write it",
		Entry: start.Add(time.Hour), End: start.Add(2 * time.Hour),
	}
	read := func(time.Time) ([]taskwarrior.Task, error) { return []taskwarrior.Task{task}, nil }

	// Act
	got := loop.TasksRead(read, start, start.Add(24*time.Hour))

	// Assert
	// A pending task's end is when it stopped waiting, not when it was done.
	if len(got.Items) != 1 || got.Items[0].Kind != activity.TaskAdded || got.Items[0].Ref != "12" {
		t.Errorf("TasksRead = %+v, want task 12 added and nothing else", got.Items)
	}
}

func TestEverySourceThatCannotBeReadIsNamedAsFailed(t *testing.T) {
	t.Parallel()

	start := summaryStart()
	end := start.Add(24 * time.Hour)
	cases := map[activity.Source]func() activity.Read{
		activity.SourceGit: func() activity.Read {
			return loop.CommitsRead(func(time.Time, time.Time) []loop.RepositoryCommits {
				return []loop.RepositoryCommits{{Failed: errNotRead}}
			}, start, end)
		},
		activity.SourceTasks: func() activity.Read {
			return loop.TasksRead(func(time.Time) ([]taskwarrior.Task, error) { return nil, errNotRead }, start, end)
		},
		activity.SourceJira: func() activity.Read {
			return loop.JiraRead(func(time.Time, time.Time) (jira.Activity, error) { return jira.Activity{}, errNotRead },
				func(jira.Key) string { return "" }, start, end)
		},
		activity.SourceForge: func() activity.Read {
			return loop.ForgeRead(func(time.Time, time.Time) (forge.Activity, error) { return forge.Activity{}, errNotRead },
				forge.KindGitHub, start, end)
		},
	}

	for source, read := range cases {
		t.Run(source.Name(), func(t *testing.T) {
			t.Parallel()

			// Act
			got := read()

			// Assert
			if !errors.Is(got.Failed, errNotRead) || got.Source != source {
				t.Errorf("read = %+v, want %s named as failed", got, source.Name())
			}
		})
	}
}
