// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package webserver_test

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/taskwarrior"
	"github.com/jacob-delgado/workflow/internal/webserver"
	"github.com/jacob-delgado/workflow/internal/webserver/webtest"
)

// The tests here hold the context each kind of request hands Taskwarrior: a
// read's ends with its page, a write's with the server, and a search every
// stream shares with neither.

// startPath starts the plain task.
const startPath = webtest.TasksPath + "/" + webtest.PlainUUID + "/start"

func TestATaskReadStopsWhenItsPageLeaves(t *testing.T) {
	t.Parallel()

	// Arrange
	held := webtest.NewHeldUntilEnded()
	tasks := webtest.FakeTaskwarrior().Seams()
	tasks.Pending = func(ctx context.Context) (taskwarrior.List, error) { return taskwarrior.List{}, held.Wait(ctx) }

	var heard []error

	deps := webserver.Deps{Tasks: tasks, Unexpected: func(_ config.Config, err error) { heard = append(heard, err) }}
	ctx, leave := context.WithCancel(t.Context())
	answered := webtest.Asking(ctx, webtest.Serve(t, deps, config.Default()), http.MethodGet, webtest.TasksPath)

	<-held.Entered

	// Act
	leave()

	// Assert
	err := held.EndedWith(t)
	if !errors.Is(err, context.Canceled) {
		t.Errorf("the read's context ended with %v, want it canceled once its page left", err)
	}

	<-answered

	if len(heard) != 0 {
		t.Errorf("Unexpected heard %v, want a read its page left kept quiet", heard)
	}
}

func TestATaskWriteGoesOnWhenItsPageLeaves(t *testing.T) {
	t.Parallel()

	// Arrange
	held := webtest.NewHeld()
	tasks := webtest.FakeTaskwarrior().Seams()
	tasks.Start = func(ctx context.Context, _ string) error { return held.Wait(ctx) }
	ctx, leave := context.WithCancel(t.Context())
	answered := webtest.Asking(ctx, webtest.Serve(t, webserver.Deps{Tasks: tasks}, config.Default()), http.MethodPost,
		startPath)

	<-held.Entered

	// Act
	leave()
	close(held.Release)

	// Assert
	err := held.EndedWith(t)
	if err != nil {
		t.Errorf("the write's context ended with %v once its page left, want it to go on", err)
	}

	<-answered
}

func TestATaskWriteEndsWhenTheServerStops(t *testing.T) {
	t.Parallel()

	// Arrange
	held := webtest.NewHeldUntilEnded()
	tasks := webtest.FakeTaskwarrior().Seams()
	tasks.Start = func(ctx context.Context, _ string) error { return held.Wait(ctx) }
	handler, stop := webtest.ServeUntilStopped(t, webserver.Deps{Tasks: tasks})
	answered := webtest.Asking(t.Context(), handler, http.MethodPost, startPath)

	<-held.Entered

	// Act
	stop()

	// Assert
	err := held.EndedWith(t)
	if !errors.Is(err, context.Canceled) {
		t.Errorf("the write's context ended with %v, want it canceled once the server stopped", err)
	}

	<-answered
}

func TestAStreamsTaskReadStopsWhenItsPageLeaves(t *testing.T) {
	t.Parallel()

	// Arrange
	held := webtest.NewHeldUntilEnded()
	tasks := webtest.FakeTaskwarrior().Seams()
	tasks.Pending = func(ctx context.Context) (taskwarrior.List, error) { return taskwarrior.List{}, held.Wait(ctx) }
	stream := webtest.OpenStream(t, webtest.Serve(t, webserver.Deps{Tasks: tasks}, config.Default()), "/api/events")

	<-held.Entered

	// Act
	stream.Cancel()

	// Assert
	err := held.EndedWith(t)
	if !errors.Is(err, context.Canceled) {
		t.Errorf("the frame's read ended with %v, want it canceled once its page left", err)
	}
}

// Every stream shares the search for Taskwarrior, and holds a failed one for a
// minute: a search canceled because the page that began it left would be held
// as no Taskwarrior for every other page.
func TestASearchForTaskwarriorGoesOnWhenThePageThatBeganItLeaves(t *testing.T) {
	t.Parallel()

	// Arrange
	held, fake := webtest.NewHeld(), webtest.FakeTaskwarrior()
	tasks := fake.Seams()
	searched := false
	tasks.Install = func(ctx context.Context) (taskwarrior.Install, error) {
		if searched {
			return fake.Install, nil
		}

		searched = true

		return fake.Install, held.Wait(ctx)
	}
	handler := webtest.Serve(t, webserver.Deps{Tasks: tasks}, config.Default())
	first := webtest.OpenStream(t, handler, "/api/events")

	<-held.Entered
	first.Cancel()
	close(held.Release)
	<-first.Done

	// Act
	frame := webtest.FirstSnapshot(t, webtest.StreamOnce(t, handler, "/api/events").Body.String())

	// Assert
	if !frame.Tasks.Available {
		t.Errorf("the next page's tasks = %+v, want Taskwarrior found though the first page left mid-search",
			frame.Tasks)
	}
}
