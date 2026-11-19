// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package webserver_test

// A save reopens the server with the file it wrote, as a switch wires a
// directory, so every setting applies at once; one made while another write
// is in flight is saved all the same, and the server goes on with what it had.

import (
	"context"
	"net/http"
	"path/filepath"
	"sync"
	"testing"

	"github.com/jacob-delgado/workflow/internal/api"
	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/seams"
	"github.com/jacob-delgado/workflow/internal/webserver"
	"github.com/jacob-delgado/workflow/internal/webserver/webtest"
)

// reopenedHeader says whether a save reopened the server with its file.
const reopenedHeader = "Workflow-Reopened"

// reopening is a server over a home file of its own that wires its directory
// again from that file on disk, recording each directory it wires.
type reopening struct {
	mu      sync.Mutex
	path    string
	reached []string
}

// newReopening is a reopening over a fresh home file.
func newReopening(t *testing.T) *reopening {
	t.Helper()

	path := filepath.Join(t.TempDir(), config.FileName)
	webtest.Rewrite(t, path, `{"jira": {"project": "OLD"}}`)

	return &reopening{path: path}
}

// deps are deps working in the test's directory whose Reach reads the file
// again, and builds a server whose health names the jira.project it was wired
// with, so a test sees which wiring is served.
func (r *reopening) deps(deps webserver.Deps) webserver.Deps {
	deps.Repositories.Here = seams.Place{Dir: webtest.RepoPath}
	deps.Reach = func(_ context.Context, dir string) (webserver.World, error) {
		r.mu.Lock()
		r.reached = append(r.reached, dir)
		r.mu.Unlock()

		cfg, _, err := config.LoadLayersAt(config.Files{Home: r.path})

		return webserver.World{Deps: deps, Config: cfg, Info: webserver.Info{Version: wiredWith(cfg.Jira.Project)}}, err
	}

	return deps
}

// wiredWith is the version a server wired again with project reports.
func wiredWith(project string) string {
	return "wired with " + project
}

// servedVersion is the version the server now serving reports.
func servedVersion(t *testing.T, handler http.Handler) string {
	t.Helper()

	return webtest.Decode[api.Health](t, webtest.Get(t, handler, "/api/health")).Version
}

// config is the file as the server was first given it.
func (r *reopening) config(t *testing.T) config.Config {
	t.Helper()

	cfg, _, err := config.LoadLayersAt(config.Files{Home: r.path})
	if err != nil {
		t.Fatalf("reading %s: %v", r.path, err)
	}

	return cfg
}

// wired is every directory wired since the server started.
func (r *reopening) wired() []string {
	r.mu.Lock()
	defer r.mu.Unlock()

	return append([]string(nil), r.reached...)
}

// projectSaved is the file's jira.project as the file now holds it.
func (r *reopening) projectSaved(t *testing.T) string {
	t.Helper()

	return r.config(t).Jira.Project
}

// withProject is the file's configuration with jira.project set to project.
func (r *reopening) withProject(t *testing.T, project string) string {
	t.Helper()

	next := r.config(t)
	next.Jira.Project = project

	return webtest.Marshal(t, next)
}

func TestASaveReopensTheServerWithTheFileWritten(t *testing.T) {
	t.Parallel()

	// Arrange
	run := newReopening(t)
	handler := webtest.Serve(t, run.deps(webserver.Deps{}), run.config(t))

	// Act
	recorder := webtest.PutConfig(t, handler, run.withProject(t, "NEW"))

	// Assert
	if recorder.Code != http.StatusOK || recorder.Header().Get(reopenedHeader) != "true" {
		t.Fatalf("save = %d, %s %q; want 200 and the server reopened: %s",
			recorder.Code, reopenedHeader, recorder.Header().Get(reopenedHeader), recorder.Body)
	}

	if wired := run.wired(); len(wired) != 1 || wired[0] != webtest.RepoPath {
		t.Errorf("directories wired = %q, want %s wired again once", wired, webtest.RepoPath)
	}

	if served := servedVersion(t, handler); served != wiredWith("NEW") {
		t.Errorf("the server serves %q, want the one %q, wired from the file saved", served, wiredWith("NEW"))
	}
}

func TestASaveWhileAWriteIsInFlightIsSavedButNotReopened(t *testing.T) {
	t.Parallel()

	// Arrange
	run := newReopening(t)
	held := webtest.NewHeld()
	deps := run.deps(webtest.FilledDeps())
	deps.Store.Favor = func(ctx context.Context, _ string) error { return held.Wait(ctx) }
	handler := webtest.Serve(t, deps, run.config(t))

	var favoring sync.WaitGroup

	favoring.Go(func() {
		webtest.Send(t, handler, http.MethodPut, reposPath+"/favorites", `{"dir":"/srv/web"}`)
	})
	<-held.Entered

	// Act
	recorder := webtest.PutConfig(t, handler, run.withProject(t, "NEW"))

	close(held.Release)
	favoring.Wait()

	// Assert
	if recorder.Code != http.StatusOK || recorder.Header().Get(reopenedHeader) != "false" {
		t.Fatalf("save = %d, %s %q; want 200 and the server not reopened: %s",
			recorder.Code, reopenedHeader, recorder.Header().Get(reopenedHeader), recorder.Body)
	}

	if saved, wired := run.projectSaved(t), run.wired(); saved != "NEW" || len(wired) != 0 {
		t.Errorf("jira.project saved %q, directories wired %q; want NEW saved and nothing wired again",
			saved, wired)
	}

	if served := servedVersion(t, handler); served != webtest.TestVersion {
		t.Errorf("the server serves %q, want the one it started with, %q", served, webtest.TestVersion)
	}
}
