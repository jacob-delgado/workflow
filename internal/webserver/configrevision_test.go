// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package webserver_test

import (
	"errors"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jacob-delgado/workflow/internal/api"
	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/webserver"
)

// The configuration file these tests serve, and the edits made to it outside
// the browser. fileToken is a secret the file holds, which no answer may carry.
const (
	fileToken    = "file-token-5678"
	fileAtStart  = `{"jira": {"base_url": "https://file.example.com", "token": "` + fileToken + `"}}`
	editBaseURL  = "https://edited.example.com"
	editedFile   = `{"jira": {"base_url": "` + editBaseURL + `", "token": "` + fileToken + `"}}`
	savedBaseURL = "https://saved.example.com"
	sprintView   = "Sprint"
	withSprint   = `{"jira": {"token": "` + fileToken + `", "views": [{"name": "Sprint", "jql": "sprint = 1"}]}}`
	notValid     = `{"jira": {"token": "` + fileToken + `", "base_url": `
	// otherToken is a file whose token another edit put in place of fileToken.
	otherToken = `{"jira": {"base_url": "https://file.example.com", "token": "other-token-9999"}}`
)

// servedFile serves a configuration file holding contents, read as the process
// reads it at startup, and returns the handler with the file's path.
func servedFile(t *testing.T, contents string, info webserver.Info) (http.Handler, string) {
	t.Helper()

	path := filepath.Join(t.TempDir(), config.FileName)
	rewrite(t, path, contents)

	cfg, err := config.LoadFile(path)
	if err != nil {
		t.Fatalf("loading the served file: %v", err)
	}

	return serveWith(t, webserver.Deps{}, cfg, info), path
}

// rewrite leaves the file at path holding contents, as an edit made outside
// the browser does.
func rewrite(t *testing.T, path, contents string) {
	t.Helper()

	err := os.WriteFile(path, []byte(contents), config.FileMode)
	if err != nil {
		t.Fatalf("writing %s: %v", path, err)
	}
}

// deleteFile deletes the file at path, as a deletion made outside the browser
// does.
func deleteFile(t *testing.T, path string) {
	t.Helper()

	err := os.Remove(path)
	if err != nil {
		t.Fatalf("deleting %s: %v", path, err)
	}
}

// onDisk is what the file at path holds, or "" when there is none.
func onDisk(t *testing.T, path string) string {
	t.Helper()

	contents, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return ""
	}

	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}

	return string(contents)
}

// jiraAt is a save's body: the default configuration with Jira at baseURL.
func jiraAt(t *testing.T, baseURL string) string {
	t.Helper()

	cfg := config.Default()
	cfg.Jira.BaseURL = baseURL

	return marshal(t, cfg)
}

// etagOf is the ETag a response carries.
func etagOf(recorder *httptest.ResponseRecorder) string {
	return recorder.Header().Get("ETag")
}

// refusedWith checks that a refusal carries status and code, and that its
// detail names neither the file nor anything the file holds.
func refusedWith(t *testing.T, recorder *httptest.ResponseRecorder, status int, code api.ProblemCode, path string) {
	t.Helper()

	if recorder.Code != status {
		t.Fatalf("status = %d, want %d: %s", recorder.Code, status, recorder.Body.String())
	}

	if failure := decode[api.Problem](t, recorder); failure.Code != code {
		t.Errorf("code = %q, want %q", failure.Code, code)
	}

	if body := recorder.Body.String(); strings.Contains(body, path) || strings.Contains(body, fileToken) {
		t.Errorf("the refusal %q names the file or what it holds", body)
	}
}

func TestASaveOverAnEditMadeOnDiskIsRefused(t *testing.T) {
	t.Parallel()

	// Arrange
	handler, path := servedFile(t, withSprint, webserver.Info{})
	read := get(t, handler, "/api/config")
	rewrite(t, path, editedFile)

	// Act
	recorder := putConfigOver(t, handler, jiraAt(t, savedBaseURL), etagOf(read))

	// Assert
	refusedWith(t, recorder, http.StatusConflict, api.Conflict, path)

	if got := onDisk(t, path); got != editedFile {
		t.Errorf("the file holds %q, want the edit left as it was", got)
	}

	// The views are read from the configuration in effect, without reading the
	// file again, so they show whether the refused save was taken up.
	views := decode[api.ViewList](t, get(t, handler, "/api/views"))
	if len(views.Views) != 1 || views.Views[0].Name != sprintView {
		t.Errorf("views = %+v, want the configuration in effect kept", views.Views)
	}
}

func TestASaveOverARevisionAnotherSaveReplacedIsRefused(t *testing.T) {
	t.Parallel()

	// Arrange
	// Two tabs read the same file; the second saves first.
	handler, path := servedFile(t, fileAtStart, webserver.Info{})
	first := get(t, handler, "/api/config")
	second := get(t, handler, "/api/config")

	if saved := putConfigOver(t, handler, jiraAt(t, savedBaseURL), etagOf(second)); saved.Code != http.StatusOK {
		t.Fatalf("the second tab's save: status = %d: %s", saved.Code, saved.Body.String())
	}

	want := onDisk(t, path)

	// Act
	recorder := putConfigOver(t, handler, jiraAt(t, "https://stale.example.com"), etagOf(first))

	// Assert
	refusedWith(t, recorder, http.StatusConflict, api.Conflict, path)

	if got := onDisk(t, path); got != want {
		t.Errorf("the file holds %q, want the second tab's save left as it was", got)
	}

	if out := decode[api.Config](t, get(t, handler, "/api/config")); out.Jira.BaseURL == nil ||
		*out.Jira.BaseURL != savedBaseURL {
		t.Errorf("jira.base_url = %v, want the second tab's save still in effect", out.Jira.BaseURL)
	}
}

func TestASaveOverARevisionTheFileCameBackToIsRefused(t *testing.T) {
	t.Parallel()

	// Arrange
	// The file is edited, a read elsewhere takes the edit up, and the edit is
	// undone: the file is back at the revision this read named, but the
	// configuration in effect, whose secrets a save keeps, is the edit's.
	handler, path := servedFile(t, fileAtStart, webserver.Info{})
	read := get(t, handler, "/api/config")
	rewrite(t, path, otherToken)
	get(t, handler, "/api/config")
	rewrite(t, path, fileAtStart)

	// Act
	recorder := putConfigOver(t, handler, read.Body.String(), etagOf(read))

	// Assert
	refusedWith(t, recorder, http.StatusConflict, api.Conflict, path)

	if got := onDisk(t, path); got != fileAtStart {
		t.Errorf("the file holds %q, want it left as it was, token and all", got)
	}
}

func TestASaveThatNamesNoRevisionIsRefused(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		ifMatch string
		status  int
		code    api.ProblemCode
	}{
		"no If-Match":                {ifMatch: "", status: http.StatusPreconditionRequired, code: api.PreconditionRequired},
		"text in no revision's form": {ifMatch: `"stale"`, status: http.StatusBadRequest, code: api.BadRequest},
		"a revision without quotes":  {ifMatch: "none", status: http.StatusBadRequest, code: api.BadRequest},
		"a revision left unfinished": {ifMatch: `"none`, status: http.StatusBadRequest, code: api.BadRequest},
		"a revision never opened":    {ifMatch: `none"`, status: http.StatusBadRequest, code: api.BadRequest},
		"a gone mark over nothing":   {ifMatch: `"none-"`, status: http.StatusBadRequest, code: api.BadRequest},
		"a gone mark over no revision's form": {
			ifMatch: `"none-stale"`, status: http.StatusBadRequest, code: api.BadRequest,
		},
		// A read marks the file gone only after reading one, so no read's tag
		// is a gone mark over the no-file revision.
		"a gone mark over no file": {ifMatch: `"none-none"`, status: http.StatusBadRequest, code: api.BadRequest},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			handler, path := servedFile(t, fileAtStart, webserver.Info{})

			// Act
			recorder := putConfigOver(t, handler, jiraAt(t, savedBaseURL), tt.ifMatch)

			// Assert
			refusedWith(t, recorder, tt.status, tt.code, path)

			if got := onDisk(t, path); got != fileAtStart {
				t.Errorf("the file holds %q, want it untouched", got)
			}
		})
	}
}

func TestASaveWritesOnceSettingsHasReadTheEdit(t *testing.T) {
	t.Parallel()

	// Arrange
	handler, path := servedFile(t, fileAtStart, webserver.Info{})
	get(t, handler, "/api/config")
	rewrite(t, path, editedFile)
	reread := get(t, handler, "/api/config")

	// Act
	recorder := putConfigOver(t, handler, jiraAt(t, savedBaseURL), etagOf(reread))

	// Assert
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", recorder.Code, recorder.Body.String())
	}

	if got := onDisk(t, path); !strings.Contains(got, savedBaseURL) {
		t.Errorf("the file holds %q, want the save made over the edit it read", got)
	}
}

func TestASaveCanFollowASave(t *testing.T) {
	t.Parallel()

	// Arrange
	handler, path := servedFile(t, fileAtStart, webserver.Info{})
	first := putConfig(t, handler, jiraAt(t, "https://first.example.com"))

	// Act
	second := putConfigOver(t, handler, jiraAt(t, savedBaseURL), etagOf(first))

	// Assert
	if first.Code != http.StatusOK || second.Code != http.StatusOK {
		t.Fatalf("statuses = %d then %d, want both 200: %s", first.Code, second.Code, second.Body.String())
	}

	if got := onDisk(t, path); !strings.Contains(got, savedBaseURL) {
		t.Errorf("the file holds %q, want the second save", got)
	}
}

func TestSettingsWritesBackAFileDeletedOnDisk(t *testing.T) {
	t.Parallel()

	// Arrange
	handler, path := servedFile(t, fileAtStart, webserver.Info{})
	deleteFile(t, path)

	// Act: read with the file gone
	read := get(t, handler, "/api/config")

	// Assert: the configuration in effect stands
	if out := decode[api.Config](t, read); out.Jira.BaseURL == nil || *out.Jira.BaseURL != "https://file.example.com" {
		t.Errorf("jira.base_url = %v, want the configuration in effect kept", out.Jira.BaseURL)
	}

	// Act: save over what was read
	saved := putConfigOver(t, handler, jiraAt(t, savedBaseURL), etagOf(read))

	// Assert: the file is written back
	if saved.Code != http.StatusOK || !strings.Contains(onDisk(t, path), savedBaseURL) {
		t.Errorf("status = %d and the file holds %q, want the save written back", saved.Code, onDisk(t, path))
	}

	// Act: save again over what the save wrote, with no read between
	again := putConfigOver(t, handler, jiraAt(t, "https://again.example.com"), etagOf(saved))

	// Assert: the file written back is no longer gone
	if again.Code != http.StatusOK || !strings.Contains(onDisk(t, path), "https://again.example.com") {
		t.Errorf("status = %d and the file holds %q, want the second save written", again.Code, onDisk(t, path))
	}
}

func TestASaveOverAnEarlierDeletionOfTheFileIsRefused(t *testing.T) {
	t.Parallel()

	// Arrange
	// A read finds the file gone and serves the configuration it held. Another
	// file takes its place, a read takes it up, and it is deleted too: both
	// reads found no file, but the configuration in effect, whose secrets a
	// save keeps, is now the second file's.
	handler, path := servedFile(t, fileAtStart, webserver.Info{})
	deleteFile(t, path)
	firstGone := get(t, handler, "/api/config")
	rewrite(t, path, otherToken)
	get(t, handler, "/api/config")
	deleteFile(t, path)
	get(t, handler, "/api/config")

	// Act
	recorder := putConfigOver(t, handler, firstGone.Body.String(), etagOf(firstGone))

	// Assert
	refusedWith(t, recorder, http.StatusConflict, api.Conflict, path)

	if got := onDisk(t, path); got != "" {
		t.Errorf("the file holds %q, want it still gone", got)
	}
}

func TestDryRunRefusesASettingsSave(t *testing.T) {
	t.Parallel()

	// Arrange
	handler, path := servedFile(t, fileAtStart, webserver.Info{DryRun: true})
	read := get(t, handler, "/api/config")

	// Act
	recorder := putConfigOver(t, handler, jiraAt(t, savedBaseURL), etagOf(read))

	// Assert
	if recorder.Code != http.StatusForbidden || onDisk(t, path) != fileAtStart {
		t.Errorf("status = %d and the file holds %q, want 403 and the file untouched", recorder.Code, onDisk(t, path))
	}
}
