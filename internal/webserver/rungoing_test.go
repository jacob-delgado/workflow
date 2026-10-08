// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package webserver_test

// One run goes at a time: the stream carries it while it goes, and a stop
// ends it, even one asked for before its program has started.

import (
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jacob-delgado/workflow/internal/api"
	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/proc"
)

// heldRun is a run's output that writes a line, then waits until it is
// stopped, as a hook that hangs does.
type heldRun struct {
	lines   chan string
	once    sync.Once
	stopped chan struct{}
}

// newHeldRun is a run that has written its first line.
func newHeldRun(first string) *heldRun {
	held := &heldRun{lines: make(chan string, 1), stopped: make(chan struct{})}
	held.lines <- first

	return held
}

// output is the run as a seam answers it.
func (h *heldRun) output() proc.Output {
	return proc.Output{
		Lines: h.lines,
		Wait: func() error {
			<-h.stopped

			return errExit
		},
		Stop: func() {
			h.once.Do(func() {
				close(h.lines)
				close(h.stopped)
			})
		},
	}
}

// startHeld starts a run that hangs after its first line on handler, and
// returns the recorder its stream is written to, once that line is written,
// and a channel closed when the stream ends.
func startHeld(t *testing.T, handler http.Handler) (*syncRecorder, chan struct{}) {
	t.Helper()

	recorder := newSyncRecorder()
	done := make(chan struct{})

	go func() {
		defer close(done)

		handler.ServeHTTP(recorder, runRequest(t, preCommitRun))
	}()

	recorder.waitFor(t, "hanging")

	return recorder, done
}

func TestOneRunGoesAtATimeAndAStopEndsIt(t *testing.T) {
	t.Parallel()

	// Arrange
	held := newHeldRun("hanging")
	handler := serve(t, runDeps(&runCalls{}, held.output), config.Default())
	first, done := startHeld(t, handler)

	// Act: start a second while the first is going
	second := startRun(t, handler, preCommitRun)

	// Assert: it is refused
	assertProblem(t, second, http.StatusConflict, "already going")

	// Act: stop the first
	stopped := send(t, handler, http.MethodDelete, runningAt, "")

	<-done

	// Assert: it ends stopped
	if stopped.Code != http.StatusNoContent {
		t.Fatalf("stop status = %d, want 204: %s", stopped.Code, stopped.Body)
	}

	if !strings.Contains(first.body(), `"state":"stopped"`) {
		t.Errorf("the stream %s does not end with the run stopped", first.body())
	}
}

func TestARunStoppedBeforeItsProgramStartsEndsStopped(t *testing.T) {
	t.Parallel()

	// Arrange
	// The run is claimed, and its program is held starting, when the stop
	// comes: the server has no way to stop it yet.
	starting, release := make(chan struct{}), make(chan struct{})
	deps := runDeps(&runCalls{}, passing)
	deps.RunHook = func(string) (proc.Output, error) {
		close(starting)
		<-release

		return passing(), nil
	}
	handler := serve(t, deps, config.Default())
	recorder := newSyncRecorder()
	ran := make(chan struct{})

	go func() {
		defer close(ran)

		handler.ServeHTTP(recorder, runRequest(t, preCommitRun))
	}()

	<-starting

	// Act
	stopped := send(t, handler, http.MethodDelete, runningAt, "")

	close(release)
	<-ran

	// Assert
	if stopped.Code != http.StatusNoContent || !strings.Contains(recorder.body(), `"state":"stopped"`) {
		t.Errorf("stop = %d, stream %s; want 204 and the run ended stopped", stopped.Code, recorder.body())
	}
}

func TestStoppingWithNoRunGoingIsNotFound(t *testing.T) {
	t.Parallel()

	// Act
	recorder := send(t, serve(t, runDeps(&runCalls{}, passing), config.Default()), http.MethodDelete, runningAt, "")

	// Assert
	assertProblem(t, recorder, http.StatusNotFound, "no run is going")
}

func TestTheStreamCarriesTheRunGoingNow(t *testing.T) {
	t.Parallel()

	// Arrange
	held := newHeldRun("hanging")
	handler := serve(t, runDeps(&runCalls{}, held.output), config.Default())
	_, done := startHeld(t, handler)

	// Act
	snap := firstSnapshot(t, streamOnce(t, handler, "/api/events").Body.String())

	// Assert
	held.output().Stop()
	<-done

	if snap.Run == nil || snap.Run.State != api.RunStateInProgress || !slices.Equal(snap.Run.Lines, []string{"hanging"}) {
		t.Errorf("the frame's run = %+v, want pre-commit in progress with its line so far", snap.Run)
	}
}

func TestTheStreamCarriesNoRunOnceItEnded(t *testing.T) {
	t.Parallel()

	// Arrange
	handler := serve(t, runDeps(&runCalls{}, passing), config.Default())
	startRun(t, handler, preCommitRun)

	// Act
	snap := firstSnapshot(t, streamOnce(t, handler, "/api/events").Body.String())

	// Assert
	if snap.Run != nil {
		t.Errorf("the frame's run = %+v, want none once it ended", snap.Run)
	}
}

// writeWait is how long a test waits for a run's stream to write what it is
// waiting for before failing: a failsafe, never the pace of the test.
const writeWait = 5 * time.Second

// syncRecorder is a ResponseRecorder another goroutine may read while the
// handler writes to it, and wait on for what it writes.
type syncRecorder struct {
	mu       sync.Mutex
	recorder *httptest.ResponseRecorder
	// wrote is closed, and replaced, on each write.
	wrote chan struct{}
}

// newSyncRecorder is an empty syncRecorder.
func newSyncRecorder() *syncRecorder {
	return &syncRecorder{recorder: httptest.NewRecorder(), wrote: make(chan struct{})}
}

// Header is the response's header.
func (r *syncRecorder) Header() http.Header {
	r.mu.Lock()
	defer r.mu.Unlock()

	return r.recorder.Header()
}

// Write records a part of the body, and wakes anyone waiting on what it
// writes.
func (r *syncRecorder) Write(part []byte) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	close(r.wrote)
	r.wrote = make(chan struct{})

	return r.recorder.Write(part) //nolint:wrapcheck // a recorder's write is the write the handler made
}

// WriteHeader records the status.
func (r *syncRecorder) WriteHeader(status int) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.recorder.WriteHeader(status)
}

// Flush does nothing: the body is recorded as it is written.
func (r *syncRecorder) Flush() {}

// body is what has been written so far.
func (r *syncRecorder) body() string {
	body, _ := r.written()

	return body
}

// written is what has been written so far, and a channel closed on the next
// write.
func (r *syncRecorder) written() (string, <-chan struct{}) {
	r.mu.Lock()
	defer r.mu.Unlock()

	return r.recorder.Body.String(), r.wrote
}

// waitFor waits until the body holds text, failing the test if writeWait
// passes first.
func (r *syncRecorder) waitFor(t *testing.T, text string) {
	t.Helper()

	failsafe := time.After(writeWait)

	for {
		body, next := r.written()
		if strings.Contains(body, text) {
			return
		}

		select {
		case <-next:
		case <-failsafe:
			t.Fatalf("the stream never wrote %q; body %q", text, body)
		}
	}
}
