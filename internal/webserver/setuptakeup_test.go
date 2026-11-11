// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package webserver_test

// A first file written is taken up where the server works, and a setup that
// could not be taken up is not left looking like no file was written.

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jacob-delgado/workflow/internal/api"
	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/seams"
	"github.com/jacob-delgado/workflow/internal/setup"
	"github.com/jacob-delgado/workflow/internal/webserver"
	"github.com/jacob-delgado/workflow/internal/webserver/webtest"
)

func TestSetupWhileAWriteIsInFlightIsRefusedWithNothingWritten(t *testing.T) {
	t.Parallel()

	// Arrange
	// A setup takes its file up as a switch does, which no write in flight
	// lets happen, so it is refused as a switch is, before it writes a file
	// the server could not then work with.
	run := newFirstRun(t, http.StatusOK)
	held := webtest.NewHeld()
	deps := run.deps(t)
	deps.Store.Favor = func(ctx context.Context, _ string) error { return held.Wait(ctx) }
	handler := webtest.Serve(t, deps, config.Default())

	var favoring sync.WaitGroup

	favoring.Go(func() {
		webtest.Send(t, handler, http.MethodPut, reposPath+"/favorites", `{"dir":"/srv/web"}`)
	})
	<-held.Entered

	// Act
	recorder := webtest.Send(t, handler, http.MethodPost, setupPath, setupBody(t, false, false))

	close(held.Release)
	favoring.Wait()

	// Assert
	assertProblem(t, recorder, http.StatusConflict, "a write is in flight")

	_, err := os.Lstat(run.where.Path(setup.Repository))
	if err == nil || len(run.noted) != 0 {
		t.Errorf("file written: %t, noted %q; want nothing written and nothing noted", err == nil, run.noted)
	}
}

// reachFunc is how the server wires a directory to switch to: Deps.Reach.
type reachFunc = func(ctx context.Context, dir string) (webserver.World, error)

// notTakenUp is a first run over kept whose setup wrote its file, which the
// server could not take up, with the Reach reach makes of the run's deps, and
// the run, which holds the failures the server noted.
func notTakenUp(t *testing.T, kept seams.Store, reach func(webserver.Deps) reachFunc) (http.Handler, *firstRun) {
	t.Helper()

	run := newFirstRun(t, http.StatusOK)
	wired := run.deps(t)
	wired.Store = kept
	wired.Reach = reach(wired)
	handler := webtest.Serve(t, wired, config.Default())

	recorder := webtest.Send(t, handler, http.MethodPost, setupPath, setupBody(t, false, false))
	if result := webtest.Decode[api.SetupResult](t, recorder); recorder.Code != http.StatusOK || result.Reopened {
		t.Fatalf("setup answered %d %+v, want the file written and not taken up", recorder.Code, result)
	}

	return handler, run
}

// failingOnce is the run's reach, failing the first time it is asked.
func failingOnce(deps webserver.Deps) reachFunc {
	return failingOnceThen(deps.Reach)
}

// failingOnceThen is reach, failing the first time it is asked.
func failingOnceThen(reach reachFunc) reachFunc {
	var failed atomic.Bool

	return func(ctx context.Context, dir string) (webserver.World, error) {
		if !failed.Swap(true) {
			return webserver.World{}, errSetupUnreachable
		}

		return reach(ctx, dir)
	}
}

func TestAFileASetupCouldNotTakeUpIsTakenUpWhenTheConfigurationIsRead(t *testing.T) {
	t.Parallel()

	// Arrange
	handler, _ := notTakenUp(t, seams.Store{}, failingOnce)

	// Act
	recorder := webtest.Get(t, handler, "/api/config")

	// Assert
	if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), setupJira) {
		t.Errorf("GET /api/config = %d %s, want the file the setup wrote served", recorder.Code, recorder.Body.String())
	}
}

func TestAFileTheServerCannotTakeUpIsNotReadAsNoFile(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		reach func(webserver.Deps) reachFunc
		// noted is how many failures the server notes, the setup's own
		// take-up's included.
		noted int
	}{
		"the directory cannot be wired": {
			reach: func(webserver.Deps) reachFunc {
				return func(context.Context, string) (webserver.World, error) {
					return webserver.World{}, errSetupUnreachable
				}
			},
			noted: 2,
		},
		"the server cannot switch": {
			reach: func(webserver.Deps) reachFunc { return nil },
			noted: 0,
		},
		"the directory holds no file to serve": {
			// A dangling link at the path is something there, which the
			// configuration's load reads as no file.
			reach: func(deps webserver.Deps) reachFunc {
				return failingOnceThen(func(context.Context, string) (webserver.World, error) {
					return webserver.World{Deps: deps, Config: config.Default()}, nil
				})
			},
			noted: 1,
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			handler, run := notTakenUp(t, seams.Store{}, tt.reach)

			// Act
			recorder := webtest.Get(t, handler, "/api/config")

			// Assert
			assertProblem(t, recorder, http.StatusConflict, "restart workflow --web")

			if len(run.noted) != tt.noted {
				t.Errorf("noted %q, want %d", run.noted, tt.noted)
			}
		})
	}
}

func TestAReadWhileAWriteIsInFlightLeavesAFileMadeNotTakenUp(t *testing.T) {
	t.Parallel()

	// Arrange
	held := webtest.NewHeld()

	kept := seams.Store{Favor: func(ctx context.Context, _ string) error { return held.Wait(ctx) }}
	handler, run := notTakenUp(t, kept, failingOnce)

	var favoring sync.WaitGroup

	favoring.Go(func() {
		webtest.Send(t, handler, http.MethodPut, reposPath+"/favorites", `{"dir":"/srv/web"}`)
	})
	<-held.Entered

	// Act
	recorder := webtest.Get(t, handler, "/api/config")

	close(held.Release)
	favoring.Wait()

	// Assert
	assertProblem(t, recorder, http.StatusConflict, "read the configuration again")

	if len(run.noted) != 1 {
		t.Errorf("noted %q, want only the setup's own take-up", run.noted)
	}
}

func TestAFileASetupCouldNotTakeUpIsNotWrittenAsNoFile(t *testing.T) {
	t.Parallel()

	// Arrange
	handler, _ := notTakenUp(t, seams.Store{}, failingOnce)

	// Act
	recorder := webtest.PutConfigOver(t, handler, webtest.Marshal(t, config.Default()), `"none"`)

	// Assert
	assertProblem(t, recorder, http.StatusConflict, "read the configuration again")
}

// secondReadArrives is how long a test waits, once a first read is taking a
// file up, for a second read to arrive while it is under way.
const secondReadArrives = 50 * time.Millisecond

func TestASecondReadWhileAFileMadeIsTakenUpReadsIt(t *testing.T) {
	t.Parallel()

	// Arrange
	// Of two reads that find a file made, the second waits for the first's
	// take-up and reads the file it took up, rather than being refused as if
	// a write were in flight, or wiring the directory again.
	held := webtest.NewHeld()

	var wirings atomic.Int32

	handler, _ := notTakenUp(t, seams.Store{}, func(deps webserver.Deps) reachFunc {
		return failingOnceThen(func(ctx context.Context, dir string) (webserver.World, error) {
			if wirings.Add(1) == 1 {
				err := held.Wait(ctx)
				if err != nil {
					return webserver.World{}, err
				}
			}

			return deps.Reach(ctx, dir)
		})
	})

	var first sync.WaitGroup

	first.Go(func() { webtest.Get(t, handler, "/api/config") })
	<-held.Entered

	var (
		second   sync.WaitGroup
		recorder *httptest.ResponseRecorder
	)

	// Act
	second.Go(func() { recorder = webtest.Get(t, handler, "/api/config") })
	time.Sleep(secondReadArrives)
	close(held.Release)
	first.Wait()
	second.Wait()

	// Assert
	if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), setupJira) || wirings.Load() != 1 {
		t.Errorf("second GET /api/config = %d %s after %d wirings; want the file the first took up, wired once",
			recorder.Code, recorder.Body.String(), wirings.Load())
	}
}
