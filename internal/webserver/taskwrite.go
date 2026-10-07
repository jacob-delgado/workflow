// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package webserver

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/jacob-delgado/workflow/internal/api"
	"github.com/jacob-delgado/workflow/internal/taskwarrior"
)

// notAvailable is the detail of a write with no Taskwarrior to make it.
const notAvailable = "Taskwarrior is not available"

// errStartDeclined is a start Taskwarrior declined, which has words of its
// own: a finished task is reopened, not declined.
var errStartDeclined = errors.New("taskwarrior declined to start the task")

// StartTask starts the task the path names.
func (s *server) StartTask(_ context.Context, request api.StartTaskRequestObject) (api.StartTaskResponseObject, error) {
	list, prob, code := s.taskWrite(request.UUID, declinedAsStart(s.deps.Tasks.Start))
	if prob != nil {
		return api.StartTaskdefaultApplicationProblemPlusJSONResponse{Body: *prob, StatusCode: code}, nil
	}

	return api.StartTask200JSONResponse(list), nil
}

// declinedAsStart is start with Taskwarrior's decline told as a start's:
// errStartDeclined. Nil when start is.
func declinedAsStart(start func(uuid string) error) func(uuid string) error {
	if start == nil {
		return nil
	}

	return func(uuid string) error {
		err := start(uuid)
		if errors.Is(err, taskwarrior.ErrNothingChanged) {
			return fmt.Errorf("%w: %w", errStartDeclined, err)
		}

		return err
	}
}

// StopTask stops the task the path names.
func (s *server) StopTask(_ context.Context, request api.StopTaskRequestObject) (api.StopTaskResponseObject, error) {
	list, prob, code := s.taskWrite(request.UUID, s.deps.Tasks.Stop)
	if prob != nil {
		return api.StopTaskdefaultApplicationProblemPlusJSONResponse{Body: *prob, StatusCode: code}, nil
	}

	return api.StopTask200JSONResponse(list), nil
}

// CompleteTask marks the task the path names done.
func (s *server) CompleteTask(
	_ context.Context, request api.CompleteTaskRequestObject,
) (api.CompleteTaskResponseObject, error) {
	list, prob, code := s.taskWrite(request.UUID, s.deps.Tasks.Done)
	if prob != nil {
		return api.CompleteTaskdefaultApplicationProblemPlusJSONResponse{Body: *prob, StatusCode: code}, nil
	}

	return api.CompleteTask200JSONResponse(list), nil
}

// AnnotateTask adds a note to the task the path names, refusing one with no
// text before Taskwarrior is asked.
func (s *server) AnnotateTask(
	_ context.Context, request api.AnnotateTaskRequestObject,
) (api.AnnotateTaskResponseObject, error) {
	if blank(request.Body.Text) {
		return api.AnnotateTask422ApplicationProblemPlusJSONResponse(
			problem(api.ProblemCodeUnprocessable, "text is required")), nil
	}

	list, prob, code := s.taskWrite(request.UUID, withText(s.deps.Tasks.Annotate, request.Body.Text))
	if prob != nil {
		return api.AnnotateTaskdefaultApplicationProblemPlusJSONResponse{Body: *prob, StatusCode: code}, nil
	}

	return api.AnnotateTask200JSONResponse(list), nil
}

// ModifyTask changes the task the path names by a line in Taskwarrior's
// grammar, refusing an empty line before Taskwarrior is asked.
func (s *server) ModifyTask(
	_ context.Context, request api.ModifyTaskRequestObject,
) (api.ModifyTaskResponseObject, error) {
	if blank(request.Body.Line) {
		return api.ModifyTask422ApplicationProblemPlusJSONResponse(problem(api.ProblemCodeUnprocessable, lineRequired)), nil
	}

	list, prob, code := s.taskWrite(request.UUID, withText(s.deps.Tasks.Modify, request.Body.Line))
	if prob != nil {
		return api.ModifyTaskdefaultApplicationProblemPlusJSONResponse{Body: *prob, StatusCode: code}, nil
	}

	return api.ModifyTask200JSONResponse(list), nil
}

// lineRequired is the detail of an add or a modify with no line.
const lineRequired = "a line is required"

// taskWrite makes act's write on the task uuid names — act is nil when there is
// no Taskwarrior to make it — and answers the list after it, or the problem
// either met.
func (s *server) taskWrite(uuid string, act func(uuid string) error) (api.TaskList, *api.Problem, int) {
	if act == nil {
		prob := problem(api.ProblemCodeUnprocessable, notAvailable)

		return api.TaskList{}, &prob, prob.Status
	}

	err := act(uuid)
	if err != nil {
		prob, code := s.taskFault(err)

		return api.TaskList{}, &prob, code
	}

	return s.taskListAfter(taskChange{})
}

// taskListAfter is the list a write answers once it has landed, with what the
// write said and the task it added, or the problem of the read that could not
// make it.
func (s *server) taskListAfter(change taskChange) (api.TaskList, *api.Problem, int) {
	list, err := s.readTaskList(change.said)
	if err != nil {
		prob, code := s.taskFault(err)

		return api.TaskList{}, &prob, code
	}

	if change.added != "" {
		list.Added = &change.added
	}

	return list, nil, http.StatusOK
}

// withText is write with text bound, a write on a task by its uuid alone; nil
// when write is.
func withText(write func(uuid, text string) error, text string) func(uuid string) error {
	if write == nil {
		return nil
	}

	return func(uuid string) error { return write(uuid, text) }
}

// blank reports text with nothing in it but spaces, which Taskwarrior would
// read as no words at all.
func blank(text string) bool {
	return strings.TrimSpace(text) == ""
}
