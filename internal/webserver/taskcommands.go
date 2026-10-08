// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package webserver

import (
	"context"
	"errors"
	"strings"

	"github.com/jacob-delgado/workflow/internal/api"
	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/jira"
	"github.com/jacob-delgado/workflow/internal/taskwarrior"
)

// AddTask adds a task from a line in Taskwarrior's grammar, refusing an empty
// line before Taskwarrior is asked.
func (s *server) AddTask(_ context.Context, request api.AddTaskRequestObject) (api.AddTaskResponseObject, error) {
	if blank(request.Body.Line) {
		return api.AddTask422ApplicationProblemPlusJSONResponse(problem(api.ProblemCodeUnprocessable, lineRequired)), nil
	}

	list, prob := s.taskCommand(addLine(s.deps.Tasks.Add, request.Body.Line), s.taskFault)
	if prob != nil {
		return problemAnswer[api.AddTaskdefaultApplicationProblemPlusJSONResponse](*prob), nil
	}

	return api.AddTask200JSONResponse(list), nil
}

// TrackIssue adds a task for an issue as the tracker has it — its key and page,
// the jira tag, its priority in Taskwarrior's words, and "KEY: summary" — then
// annotates the task with the issue's page, when there is one. The issue is
// read before Taskwarrior is asked anything, so an issue the tracker does not
// have adds no task, and neither does a key the tracker gives that is not one
// word: the rest would reach Taskwarrior as words of their own, ahead of the
// line's --.
func (s *server) TrackIssue(
	_ context.Context, request api.TrackIssueRequestObject,
) (api.TrackIssueResponseObject, error) {
	key := jira.Key(request.Body.IssueKey)

	switch {
	case key == "":
		return trackRefused("an issue key is required"), nil
	case s.deps.Tasks.Add == nil:
		return trackRefused(notAvailable), nil
	case s.deps.Issue == nil:
		return trackRefused("no issue tracker is configured"), nil
	}

	detail, err := s.deps.Issue(key)
	if err != nil {
		return s.issueNotRead(key, err), nil
	}

	if len(strings.Fields(string(detail.Issue.Key))) != 1 {
		return trackRefused("the tracker's key for the issue is not one word, so Taskwarrior cannot take it"), nil
	}

	list, prob := s.taskCommand(s.track(detail.Issue), s.taskFault)
	if prob != nil {
		return problemAnswer[api.TrackIssuedefaultApplicationProblemPlusJSONResponse](*prob), nil
	}

	return api.TrackIssue200JSONResponse(list), nil
}

// trackRefused is the 422 for a track the server will not attempt.
func trackRefused(detail string) api.TrackIssue422ApplicationProblemPlusJSONResponse {
	return api.TrackIssue422ApplicationProblemPlusJSONResponse(problem(api.ProblemCodeUnprocessable, detail))
}

// issueNotRead answers an issue the tracker could not read for a track: one it
// does not have is not found, as GET /api/issues/{key} answers it, and any
// other failure is the tracker's, classified.
func (s *server) issueNotRead(key jira.Key, err error) api.TrackIssueResponseObject {
	if errors.Is(err, jira.ErrNotFound) || errors.Is(err, forge.ErrNoRepository) {
		return api.TrackIssue404ApplicationProblemPlusJSONResponse(
			problem(api.ProblemCodeNotFound, "issue "+string(key)+" was not found"))
	}

	return problemAnswer[api.TrackIssuedefaultApplicationProblemPlusJSONResponse](s.fault(err))
}

// track is the write that tracks issue: the add of its task, then the
// annotation with its page, skipped when the tracker gives none, since
// Taskwarrior refuses an empty one. An annotation that fails leaves the task
// added, so the write says so rather than failing.
func (s *server) track(issue jira.Issue) func() (taskChange, error) {
	page := s.browseURL(issue.Key)
	line := taskwarrior.TrackLine(taskwarrior.IssueLink{
		Key: string(issue.Key), Summary: issue.Summary, URL: page, Priority: issue.Priority,
	})

	return func() (taskChange, error) {
		uuid, err := s.deps.Tasks.Add(line)
		if err != nil || page == "" {
			return taskChange{added: uuid}, err
		}

		err = s.deps.Tasks.Annotate(uuid, page)
		if err != nil {
			prob := s.taskFault(err)

			return taskChange{said: "created but not annotated: " + prob.Detail, added: uuid}, nil
		}

		return taskChange{added: uuid}, nil
	}
}

// UndoTasks reverts Taskwarrior's last change, and says how much it reverted.
func (s *server) UndoTasks(_ context.Context, _ api.UndoTasksRequestObject) (api.UndoTasksResponseObject, error) {
	list, prob := s.taskCommand(saying(s.deps.Tasks.Undo), s.undoFault)
	if prob != nil {
		return problemAnswer[api.UndoTasksdefaultApplicationProblemPlusJSONResponse](*prob), nil
	}

	return api.UndoTasks200JSONResponse(list), nil
}

// SyncTasks syncs Taskwarrior with the backend its taskrc names, and says what
// it printed.
func (s *server) SyncTasks(_ context.Context, _ api.SyncTasksRequestObject) (api.SyncTasksResponseObject, error) {
	list, prob := s.taskCommand(saying(s.deps.Tasks.Sync), s.syncFault)
	if prob != nil {
		return problemAnswer[api.SyncTasksdefaultApplicationProblemPlusJSONResponse](*prob), nil
	}

	return api.SyncTasks200JSONResponse(list), nil
}

// taskChange is what a write that names no task has to add to the list after
// it: what Taskwarrior said, and the uuid of the task it added, if it added
// one.
type taskChange struct {
	said  string
	added string
}

// taskCommand runs a write that names no task — run is nil when there is no
// Taskwarrior to make it — and answers the list after it with what the write
// changed, or the problem either met, the write's as fault words it.
func (s *server) taskCommand(
	run func() (taskChange, error), fault func(err error) api.Problem,
) (api.TaskList, *api.Problem) {
	if run == nil {
		prob := problem(api.ProblemCodeUnprocessable, notAvailable)

		return api.TaskList{}, &prob
	}

	change, err := run()
	if err != nil {
		prob := fault(err)

		return api.TaskList{}, &prob
	}

	return s.taskListAfter(change), nil
}

// addLine is the add of line, which adds the task it names, or nil when add is.
func addLine(add func(line string) (string, error), line string) func() (taskChange, error) {
	if add == nil {
		return nil
	}

	return func() (taskChange, error) {
		uuid, err := add(line)

		return taskChange{added: uuid}, err
	}
}

// saying is run, a write whose change is what it said, or nil when run is.
func saying(run func() (string, error)) func() (taskChange, error) {
	if run == nil {
		return nil
	}

	return func() (taskChange, error) {
		said, err := run()

		return taskChange{said: said}, err
	}
}

// undoFault is taskFault for an undo, which Taskwarrior declines only when it
// has nothing to undo.
func (s *server) undoFault(err error) api.Problem {
	if errors.Is(err, taskwarrior.ErrNothingChanged) {
		return problem(api.ProblemCodeConflict, "Taskwarrior has nothing to undo")
	}

	return s.taskFault(err)
}

// syncFault is taskFault for a sync, whose refusal names the sync server — an
// internal host no answer may carry — so its detail says where to see it.
func (s *server) syncFault(err error) api.Problem {
	if errors.Is(err, taskwarrior.ErrRefused) {
		return problem(api.ProblemCodeUnprocessable, "Taskwarrior could not sync; run task sync in a terminal to see why")
	}

	return s.taskFault(err)
}
