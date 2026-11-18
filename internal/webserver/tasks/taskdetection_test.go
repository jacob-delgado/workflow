// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tasks_test

// The stream's summary and the Tasks list say alike whether Taskwarrior can
// be asked, and the list never waits on the stream's search for it.

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/jacob-delgado/workflow/internal/api"
	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/taskwarrior"
	"github.com/jacob-delgado/workflow/internal/webserver"
	"github.com/jacob-delgado/workflow/internal/webserver/webtest"
)

// eventsPath is where the event stream is read.
const eventsPath = "/api/events"

// firstSearchHeld is tasksDeps over fake whose first search for Taskwarrior
// waits on held and then answers err, or asks the fake when err is nil; every
// later search asks the fake.
func firstSearchHeld(fake *webtest.TaskFake, held *webtest.HeldCall, err error) webserver.Deps {
	deps := tasksDeps(fake)
	install := deps.Tasks.Install

	var first sync.Once

	deps.Tasks.Install = func(ctx context.Context) (taskwarrior.Install, error) {
		isFirst := false

		first.Do(func() { isFirst = true })

		if !isFirst {
			return install(ctx)
		}

		waitErr := held.Wait(ctx)
		if waitErr != nil {
			return taskwarrior.Install{}, waitErr
		}

		if err != nil {
			return taskwarrior.Install{}, err
		}

		return install(ctx)
	}

	return deps
}

// searchAside runs a stream's frame on a goroutine of the test once its
// search for Taskwarrior is held, and answers the function that lets the
// search go and waits for the frame; the test's cleanup calls it too.
func searchAside(t *testing.T, handler http.Handler, held *webtest.HeldCall) func() {
	t.Helper()

	streamed := make(chan struct{})

	go func() {
		defer close(streamed)

		webtest.StreamOnce(t, handler, eventsPath)
	}()

	<-held.Entered

	letGo := sync.OnceFunc(func() {
		close(held.Release)
		<-streamed
	})
	t.Cleanup(letGo)

	return letGo
}

func TestTheStreamFindsTaskwarriorOnceTheListHasFoundIt(t *testing.T) {
	t.Parallel()

	// Arrange
	// The stream's search found no Taskwarrior, which it holds for a minute;
	// Taskwarrior is then installed, and the Tasks list finds it.
	fake := webtest.FakeTaskwarrior()
	fake.Fail["install"] = taskwarrior.ErrNotInstalled
	handler := webtest.Serve(t, tasksDeps(fake), config.Default())
	webtest.StreamOnce(t, handler, eventsPath)

	delete(fake.Fail, "install")

	if listed := webtest.Decode[api.TaskList](t, webtest.Get(t, handler, webtest.TasksPath)); !listed.Available {
		t.Fatalf("the list says %q, want Taskwarrior found", listed.Reason)
	}

	// Act
	frame := webtest.FirstSnapshot(t, webtest.StreamOnce(t, handler, eventsPath).Body.String())

	// Assert
	if !frame.Tasks.Available {
		t.Errorf("the frame after the list found Taskwarrior says %q, want it available as the list said",
			frame.Tasks.Reason)
	}
}

func TestTheTasksListAnswersWhileAStreamSearchesForTaskwarrior(t *testing.T) {
	t.Parallel()

	// Arrange
	// A stream's search for Taskwarrior runs every task program on PATH, so
	// it can take long; it is held here while the list is read.
	var asking sync.WaitGroup

	t.Cleanup(asking.Wait)

	fake, held := webtest.FakeTaskwarrior(), webtest.NewHeld()
	handler := webtest.Serve(t, firstSearchHeld(fake, held, nil), config.Default())
	searchAside(t, handler, held)

	listed := make(chan *httptest.ResponseRecorder, 1)

	// Act
	asking.Go(func() { listed <- webtest.Get(t, handler, webtest.TasksPath) })

	// Assert
	select {
	case recorder := <-listed:
		if recorder.Code != http.StatusOK {
			t.Errorf("GET %s = %d while a stream's search was held, want 200", webtest.TasksPath, recorder.Code)
		}
	case <-time.After(webtest.HeldPatience):
		t.Fatalf("GET %s did not answer while a stream's search was held, want it answered at once",
			webtest.TasksPath)
	}
}

func TestASearchTheListOutranIsNotHeld(t *testing.T) {
	t.Parallel()

	// Arrange
	// A stream's search is held; the list finds Taskwarrior meanwhile; the
	// held search then finds none, an answer older than the list's.
	var asking sync.WaitGroup

	t.Cleanup(asking.Wait)

	fake, held := webtest.FakeTaskwarrior(), webtest.NewHeld()
	handler := webtest.Serve(t, firstSearchHeld(fake, held, taskwarrior.ErrNotInstalled), config.Default())
	letGo := searchAside(t, handler, held)

	if listed := listedWithin(t, handler, &asking); !listed.Available {
		t.Fatalf("the list says %q, want Taskwarrior found", listed.Reason)
	}

	letGo()

	searched := fake.Count("install")

	// Act
	frame := webtest.FirstSnapshot(t, webtest.StreamOnce(t, handler, eventsPath).Body.String())

	// Assert
	if fake.Count("install") != searched+1 || !frame.Tasks.Available {
		t.Errorf("the frame after a late search that found none searched %d times and says %q, "+
			"want it to search again and find Taskwarrior", fake.Count("install")-searched, frame.Tasks.Reason)
	}
}

// listedWithin is the Tasks list as GET /api/tasks answers it, read on a
// goroutine of the test that asking counts, failing the test when no answer
// comes within webtest.HeldPatience: a list waiting behind a held search
// fails rather than hanging. asking is waited on in a cleanup registered
// before the search was held, so the hold is let go first.
func listedWithin(t *testing.T, handler http.Handler, asking *sync.WaitGroup) api.TaskList {
	t.Helper()

	listed := make(chan *httptest.ResponseRecorder, 1)

	asking.Go(func() { listed <- webtest.Get(t, handler, webtest.TasksPath) })

	select {
	case recorder := <-listed:
		return webtest.Decode[api.TaskList](t, recorder)
	case <-time.After(webtest.HeldPatience):
		t.Fatalf("GET %s did not answer while a stream's search was held, want it answered at once",
			webtest.TasksPath)

		return api.TaskList{}
	}
}
