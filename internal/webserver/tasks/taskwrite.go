// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tasks

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/jacob-delgado/workflow/internal/api"
	"github.com/jacob-delgado/workflow/internal/taskwarrior"
	"github.com/jacob-delgado/workflow/internal/webserver/problem"
)

// notAvailable is the detail of a write with no Taskwarrior to make it.
const notAvailable = "Taskwarrior is not available"

// ErrStartDeclined is a start Taskwarrior declined, which has words of its
// own: a finished task is reopened, not declined.
var ErrStartDeclined = errors.New("taskwarrior declined to start the task")

// StartTask starts the task the path names.
func (h *Handlers) StartTask(
	ctx context.Context, request api.StartTaskRequestObject,
) (api.StartTaskResponseObject, error) {
	list, prob := h.taskWrite(ctx, request.UUID, declinedAsStart(h.seams.Tasks.Start))
	if prob != nil {
		return problem.Answer[api.StartTaskdefaultApplicationProblemPlusJSONResponse](*prob), nil
	}

	return api.StartTask200JSONResponse(list), nil
}

// declinedAsStart is start with Taskwarrior's decline told as a start's:
// ErrStartDeclined. Nil when start is.
func declinedAsStart(start func(ctx context.Context, uuid string) error) func(context.Context, string) error {
	if start == nil {
		return nil
	}

	return func(ctx context.Context, uuid string) error {
		err := start(ctx, uuid)
		if errors.Is(err, taskwarrior.ErrNothingChanged) {
			return fmt.Errorf("%w: %w", ErrStartDeclined, err)
		}

		return err
	}
}

// StopTask stops the task the path names.
func (h *Handlers) StopTask(
	ctx context.Context, request api.StopTaskRequestObject,
) (api.StopTaskResponseObject, error) {
	list, prob := h.taskWrite(ctx, request.UUID, h.seams.Tasks.Stop)
	if prob != nil {
		return problem.Answer[api.StopTaskdefaultApplicationProblemPlusJSONResponse](*prob), nil
	}

	return api.StopTask200JSONResponse(list), nil
}

// CompleteTask marks the task the path names done, and answers the list after
// it with the task beside it as the linked tasks describe it now, since the
// list no longer holds it.
func (h *Handlers) CompleteTask(
	ctx context.Context, request api.CompleteTaskRequestObject,
) (api.CompleteTaskResponseObject, error) {
	list, prob := h.taskWrite(ctx, request.UUID, h.seams.Tasks.Done)
	if prob != nil {
		return problem.Answer[api.CompleteTaskdefaultApplicationProblemPlusJSONResponse](*prob), nil
	}

	list.Done = h.linkedTask(ctx, request.UUID)

	return api.CompleteTask200JSONResponse(list), nil
}

// linkedTask is the task uuid names as the stream's linked tasks describe it,
// read now: nil when no task linked to an issue is that one, or when the
// linked tasks could not be read, which leaves the write as it landed.
func (h *Handlers) linkedTask(ctx context.Context, uuid string) *api.Task {
	linked, err := h.seams.Tasks.Linked(ctx)
	if err != nil {
		return nil
	}

	known := knownTasks(linked)

	index := slices.IndexFunc(known, func(task taskwarrior.Task) bool { return task.UUID == uuid })
	if index < 0 {
		return nil
	}

	return &h.tasksDTO(known, h.kit.Now())[index]
}

// AnnotateTask adds a note to the task the path names, refusing one with no
// text before Taskwarrior is asked.
func (h *Handlers) AnnotateTask(
	ctx context.Context, request api.AnnotateTaskRequestObject,
) (api.AnnotateTaskResponseObject, error) {
	if blank(request.Body.Text) {
		return api.AnnotateTask422ApplicationProblemPlusJSONResponse(
			problem.New(api.ProblemCodeUnprocessable, "text is required")), nil
	}

	list, prob := h.taskWrite(ctx, request.UUID, withText(h.seams.Tasks.Annotate, request.Body.Text))
	if prob != nil {
		return problem.Answer[api.AnnotateTaskdefaultApplicationProblemPlusJSONResponse](*prob), nil
	}

	return api.AnnotateTask200JSONResponse(list), nil
}

// ModifyTask changes the task the path names by a line in Taskwarrior's
// grammar, refusing an empty line before Taskwarrior is asked.
func (h *Handlers) ModifyTask(
	ctx context.Context, request api.ModifyTaskRequestObject,
) (api.ModifyTaskResponseObject, error) {
	if blank(request.Body.Line) {
		return api.ModifyTask422ApplicationProblemPlusJSONResponse(
			problem.New(api.ProblemCodeUnprocessable, lineRequired)), nil
	}

	list, prob := h.taskWrite(ctx, request.UUID, withText(h.seams.Tasks.Modify, request.Body.Line))
	if prob != nil {
		return problem.Answer[api.ModifyTaskdefaultApplicationProblemPlusJSONResponse](*prob), nil
	}

	return api.ModifyTask200JSONResponse(list), nil
}

// lineRequired is the detail of an add or a modify with no line.
const lineRequired = "a line is required"

// taskWrite makes act's write on the task uuid names under ctx — act is nil
// when there is no Taskwarrior to make it — and answers the list after it, or
// the problem either met.
func (h *Handlers) taskWrite(
	ctx context.Context, uuid string, act func(ctx context.Context, uuid string) error,
) (api.TaskList, *api.Problem) {
	if act == nil {
		prob := problem.New(api.ProblemCodeUnprocessable, notAvailable)

		return api.TaskList{}, &prob
	}

	err := act(ctx, uuid)
	if err != nil {
		prob := h.taskFault(ctx, err)

		return api.TaskList{}, &prob
	}

	return h.taskListAfter(ctx, taskChange{}), nil
}

// taskListAfter is the list a write answers once it has landed, with what the
// write said and the task it added. The write stands whether or not the list
// after it can be read, so a read that fails is answered as a list marked
// unavailable, saying why, rather than as a failure a retry would repeat the
// write over.
func (h *Handlers) taskListAfter(ctx context.Context, change taskChange) api.TaskList {
	list, err := h.readTaskList(ctx, change.said)
	if err != nil {
		list = h.listNotReadAgain(ctx, change.said, err)
	}

	if change.added != "" {
		list.Added = &change.added
	}

	return list
}

// listNotReadAgain is the list a write that landed answers when the list
// after it could not be read: unavailable, with why in taskFault's words.
func (h *Handlers) listNotReadAgain(ctx context.Context, said string, err error) api.TaskList {
	code := api.TaskListReasonCodeUnavailable

	return api.TaskList{
		Available: false, ReasonCode: &code, Said: said, Tasks: []api.Task{}, FacetOrder: []api.TaskFacet{},
		Reason: "The change was made, but your tasks could not be read again: " + h.taskFault(ctx, err).Detail,
	}
}

// withText is write with text bound, a write on a task by its uuid alone; nil
// when write is.
func withText(
	write func(ctx context.Context, uuid, text string) error, text string,
) func(context.Context, string) error {
	if write == nil {
		return nil
	}

	return func(ctx context.Context, uuid string) error { return write(ctx, uuid, text) }
}

// blank reports text with nothing in it but spaces, which Taskwarrior would
// read as no words at all.
func blank(text string) bool {
	return strings.TrimSpace(text) == ""
}
