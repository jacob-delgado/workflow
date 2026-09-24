// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package webserver_test

import (
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jacob-delgado/workflow/internal/api"
	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/webserver"
)

func TestGetConfigServesAnEditMadeOnDisk(t *testing.T) {
	t.Parallel()

	// Arrange
	handler, path := servedFile(t, fileAtStart, webserver.Info{})
	before := get(t, handler, "/api/config")
	rewrite(t, path, editedFile)

	// Act
	recorder := get(t, handler, "/api/config")

	// Assert
	if out := decode[api.Config](t, recorder); out.Jira.BaseURL == nil || *out.Jira.BaseURL != editBaseURL {
		t.Errorf("jira.base_url = %v, want the edit made on disk", out.Jira.BaseURL)
	}

	etag := etagOf(recorder)
	if etag == etagOf(before) || !strings.HasPrefix(etag, `"`) || !strings.HasSuffix(etag, `"`) {
		t.Errorf("ETag = %s, before the edit %s; want a new quoted revision", etag, etagOf(before))
	}
}

func TestAnEditReadFromDiskReachesTheViews(t *testing.T) {
	t.Parallel()

	// Arrange
	handler, path := servedFile(t, fileAtStart, webserver.Info{})
	rewrite(t, path, withSprint)
	get(t, handler, "/api/config")

	// Act
	views := decode[api.ViewList](t, get(t, handler, "/api/views"))

	// Assert
	if len(views.Views) != 1 || views.Views[0].Name != sprintView {
		t.Errorf("views = %+v, want the Sprint view the edit added", views.Views)
	}
}

func TestGetConfigRefusesAFileOnDiskThatIsNotValid(t *testing.T) {
	t.Parallel()

	// Arrange
	handler, path := servedFile(t, withSprint, webserver.Info{})
	rewrite(t, path, notValid)

	// Act
	recorder := get(t, handler, "/api/config")

	// Assert
	refusedWith(t, recorder, http.StatusUnprocessableEntity, api.Unprocessable, path)

	views := decode[api.ViewList](t, get(t, handler, "/api/views"))
	if len(views.Views) != 1 || views.Views[0].Name != sprintView {
		t.Errorf("views = %+v, want the configuration in effect kept", views.Views)
	}
}

func TestGetConfigAnswersAFileItCanNoLongerReadAsInternal(t *testing.T) {
	t.Parallel()

	// Arrange
	// The file gives way to a directory, whose revision no read can take.
	handler, path := servedFile(t, fileAtStart, webserver.Info{})

	err := errors.Join(os.Remove(path), os.Mkdir(path, 0o700))
	if err != nil {
		t.Fatalf("putting a directory where the file was: %v", err)
	}

	// Act
	recorder := get(t, handler, "/api/config")

	// Assert
	refusedWith(t, recorder, http.StatusInternalServerError, api.Internal, path)
}

func TestAFileDeletedThenPutBackIsReadAsBefore(t *testing.T) {
	t.Parallel()

	// Arrange
	handler, path := servedFile(t, fileAtStart, webserver.Info{})
	first := get(t, handler, "/api/config")
	deleteFile(t, path)
	get(t, handler, "/api/config")
	rewrite(t, path, fileAtStart)

	// Act: read the file put back
	back := get(t, handler, "/api/config")

	// Assert: the read names what the first one did, not a file gone
	if etagOf(back) != etagOf(first) {
		t.Errorf("ETag = %s, want the first read's %s", etagOf(back), etagOf(first))
	}

	// Act: save over that read
	saved := putConfigOver(t, handler, jiraAt(t, savedBaseURL), etagOf(back))

	// Assert: the save is written over the file put back
	if saved.Code != http.StatusOK || !strings.Contains(onDisk(t, path), savedBaseURL) {
		t.Errorf("status = %d and the file holds %q, want the save written", saved.Code, onDisk(t, path))
	}
}

func TestHandlerRefusesAConfigurationPathItCannotRead(t *testing.T) {
	t.Parallel()

	// Arrange
	cfg := config.Default()
	cfg.Path = t.TempDir()

	// Act
	_, err := webserver.Handler(webserver.Deps{}, cfg, webserver.Info{}, nil)

	// Assert
	if err == nil {
		t.Error("Handler over a directory for a configuration file = nil, want the failure to read it")
	}
}

func TestHandlerTakesUpAnEditMadeBeforeItWasBuilt(t *testing.T) {
	t.Parallel()

	// Arrange
	// The process reads the file, and the file is edited before the server is
	// built over what the process read.
	path := filepath.Join(t.TempDir(), config.FileName)
	rewrite(t, path, fileAtStart)

	cfg, err := config.LoadFile(path)
	if err != nil {
		t.Fatalf("loading the file: %v", err)
	}

	rewrite(t, path, editedFile)
	handler := serveWith(t, webserver.Deps{}, cfg, webserver.Info{})

	// Act: read the configuration
	read := get(t, handler, "/api/config")

	// Assert: the read serves the edit
	if out := decode[api.Config](t, read); out.Jira.BaseURL == nil || *out.Jira.BaseURL != editBaseURL {
		t.Errorf("the read served %s, want the edit made before the server was built", read.Body.String())
	}

	// Act: save what the read served, over the revision it named
	saved := putConfigOver(t, handler, read.Body.String(), etagOf(read))

	// Assert: the edit is saved back, not replaced by what the process read
	if got := onDisk(t, path); saved.Code != http.StatusOK || !strings.Contains(got, editBaseURL) {
		t.Errorf("status = %d and the file holds %q, want the edit kept", saved.Code, got)
	}
}

func TestHandlerStartsFromWhatTheProcessReadWhenTheFileHasTurnedInvalid(t *testing.T) {
	t.Parallel()

	// Arrange
	// The process read a valid configuration, and the file has turned invalid
	// since: the server starts from what the process read rather than refusing
	// to start.
	path := filepath.Join(t.TempDir(), config.FileName)
	rewrite(t, path, notValid)

	cfg := config.Default()
	cfg.Path = path
	cfg.Jira.Views = []config.JiraView{{Name: sprintView, JQL: "sprint = 1"}}

	// Act
	handler, err := webserver.Handler(webserver.Deps{}, cfg, webserver.Info{}, nil)
	// Assert
	if err != nil {
		t.Fatalf("Handler over a file that is not valid = %v, want it served anyway", err)
	}

	views := decode[api.ViewList](t, get(t, handler, "/api/views"))
	if len(views.Views) != 1 || views.Views[0].Name != sprintView {
		t.Errorf("views = %+v, want the configuration the process read", views.Views)
	}
}
