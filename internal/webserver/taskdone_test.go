// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package webserver_test

import (
	"errors"
	"net/http"
	"slices"
	"testing"
	"time"

	"github.com/jacob-delgado/workflow/internal/api"
	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/taskwarrior"
)

// What a done answers of the task it marked: the list after the write no
// longer holds it, so the answer describes it apart, as the stream's linked
// tasks will once they catch up, for the issue's Tasks card to show at once.

// errLinkedUnread is a read of the linked tasks that failed.
var errLinkedUnread = errors.New("taskwarrior did not answer in time")

// finishedTask is the started task as Taskwarrior holds it once it is done:
// stopped, completed and out of the working set.
func finishedTask() taskwarrior.Task {
	task := startedTask()
	task.ID, task.Status, task.Start, task.End = 0, taskwarrior.Completed, time.Time{}, taskDay().Add(3*time.Hour)

	return task
}

func TestDoneAnswersTheTaskAsTheLinkedTasksDescribeItAfterTheWrite(t *testing.T) {
	t.Parallel()

	// Arrange
	fake := fakeTaskwarrior()
	fake.linked = []taskwarrior.Task{finishedTask(), doneTask()}

	// Act
	recorder := send(t, serve(t, tasksDeps(fake), config.Default()), http.MethodPost, taskPath(startedUUID, doneVerb), "")

	// Assert
	done := decode[api.TaskList](t, recorder).Done
	if done == nil {
		t.Fatalf("answer = %d %s, want the task done described", recorder.Code, recorder.Body)
	}

	stood := api.TaskFacet{Kind: api.TaskFacetKindState, Value: "completed", Label: "completed"}
	if done.UUID != startedUUID || done.State != api.TaskStateCompleted || !slices.Contains(done.Facets, stood) {
		t.Errorf("done = %+v, want %s completed, holding %+v", *done, startedUUID, stood)
	}

	if calls := fake.asked(); slices.Index(calls, "linked") < slices.Index(calls, "done "+startedUUID) {
		t.Errorf("calls = %q, want the linked tasks read after the done", calls)
	}
}

func TestDoneOfATaskLinkedToNoIssueAnswersNoTaskDone(t *testing.T) {
	t.Parallel()

	// Arrange
	fake := fakeTaskwarrior()

	// Act
	recorder := send(t, serve(t, tasksDeps(fake), config.Default()), http.MethodPost, taskPath(plainUUID, doneVerb), "")

	// Assert
	if list := decode[api.TaskList](t, recorder); recorder.Code != http.StatusOK || list.Done != nil {
		t.Errorf("answer = %d %+v, want 200 with no task done described", recorder.Code, list)
	}
}

func TestDoneWhoseLinkedTasksCannotBeReadAgainStillAnswersTheList(t *testing.T) {
	t.Parallel()

	// Arrange
	// The done landed; only the read of the linked tasks after it failed.
	fake := fakeTaskwarrior()
	fake.fail["linked"] = errLinkedUnread

	// Act
	recorder := send(t, serve(t, tasksDeps(fake), config.Default()), http.MethodPost, taskPath(startedUUID, doneVerb), "")

	// Assert
	list := decode[api.TaskList](t, recorder)
	if recorder.Code != http.StatusOK || !list.Available || list.Done != nil {
		t.Errorf("answer = %d %+v, want 200 with the list and no task done described", recorder.Code, list)
	}
}
