// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package webserver_test

// A first file written is taken up where the server works, and a setup that
// could not be taken up is not left looking like no file was written.

import (
	"context"
	"net/http"
	"os"
	"sync"
	"testing"

	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/setup"
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
