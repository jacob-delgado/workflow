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
)

// The tests here hold the context each kind of request hands Taskwarrior: a
// read's ends with its page, a write's with the server, and a search every
// stream shares with neither.

// startPath starts the plain task.
const startPath = tasksPath + "/" + plainUUID + "/start"

func TestATaskReadStopsWhenItsPageLeaves(t *testing.T) {
	t.Parallel()

	// Arrange
	held := newHeldUntilEnded()
	tasks := fakeTaskwarrior().seams()
	tasks.Pending = func(ctx context.Context) (taskwarrior.List, error) { return taskwarrior.List{}, held.wait(ctx) }

	var heard []error

	deps := webserver.Deps{Tasks: tasks, Unexpected: func(_ config.Config, err error) { heard = append(heard, err) }}
	ctx, leave := context.WithCancel(t.Context())
	answered := asking(ctx, serve(t, deps, config.Default()), http.MethodGet, tasksPath)

	<-held.entered

	// Act
	leave()

	// Assert
	err := held.endedWith(t)
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
	held := newHeld()
	tasks := fakeTaskwarrior().seams()
	tasks.Start = func(ctx context.Context, _ string) error { return held.wait(ctx) }
	ctx, leave := context.WithCancel(t.Context())
	answered := asking(ctx, serve(t, webserver.Deps{Tasks: tasks}, config.Default()), http.MethodPost, startPath)

	<-held.entered

	// Act
	leave()
	close(held.release)

	// Assert
	err := held.endedWith(t)
	if err != nil {
		t.Errorf("the write's context ended with %v once its page left, want it to go on", err)
	}

	<-answered
}

func TestATaskWriteEndsWhenTheServerStops(t *testing.T) {
	t.Parallel()

	// Arrange
	held := newHeldUntilEnded()
	tasks := fakeTaskwarrior().seams()
	tasks.Start = func(ctx context.Context, _ string) error { return held.wait(ctx) }
	handler, stop := serveUntilStopped(t, webserver.Deps{Tasks: tasks})
	answered := asking(t.Context(), handler, http.MethodPost, startPath)

	<-held.entered

	// Act
	stop()

	// Assert
	err := held.endedWith(t)
	if !errors.Is(err, context.Canceled) {
		t.Errorf("the write's context ended with %v, want it canceled once the server stopped", err)
	}

	<-answered
}

func TestAStreamsTaskReadStopsWhenItsPageLeaves(t *testing.T) {
	t.Parallel()

	// Arrange
	held := newHeldUntilEnded()
	tasks := fakeTaskwarrior().seams()
	tasks.Pending = func(ctx context.Context) (taskwarrior.List, error) { return taskwarrior.List{}, held.wait(ctx) }
	stream := openStream(t, serve(t, webserver.Deps{Tasks: tasks}, config.Default()), "/api/events")

	<-held.entered

	// Act
	stream.cancel()

	// Assert
	err := held.endedWith(t)
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
	held, fake := newHeld(), fakeTaskwarrior()
	tasks := fake.seams()
	searched := false
	tasks.Install = func(ctx context.Context) (taskwarrior.Install, error) {
		if searched {
			return fake.install, nil
		}

		searched = true

		return fake.install, held.wait(ctx)
	}
	handler := serve(t, webserver.Deps{Tasks: tasks}, config.Default())
	first := openStream(t, handler, "/api/events")

	<-held.entered
	first.cancel()
	close(held.release)
	<-first.done

	// Act
	frame := firstSnapshot(t, streamOnce(t, handler, "/api/events").Body.String())

	// Assert
	if !frame.Tasks.Available {
		t.Errorf("the next page's tasks = %+v, want Taskwarrior found though the first page left mid-search",
			frame.Tasks)
	}
}
