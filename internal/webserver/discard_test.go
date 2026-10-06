// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package webserver_test

import (
	"fmt"
	"net/http"
	"slices"
	"strings"
	"testing"

	"github.com/jacob-delgado/workflow/internal/api"
	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/gitrepo"
	"github.com/jacob-delgado/workflow/internal/webserver"
)

// discardPath is the endpoint that drops a file's changes.
const discardPath = "/api/discard"

// discarding is the staging tree with a discard seam that records each change
// it is handed.
func discarding(discarded *[]gitrepo.Change) webserver.Deps {
	var tree staging

	deps := tree.deps()
	deps.Discard = func(change gitrepo.Change) error {
		*discarded = append(*discarded, change)

		return nil
	}

	return deps
}

func TestDiscardDropsTheChangeTheTreeListsAtThePath(t *testing.T) {
	t.Parallel()

	// Arrange
	var discarded []gitrepo.Change

	// Act
	recorder := sendStaging(t, discarding(&discarded), discardPath, `{"path":"`+renamedPath+`"}`)

	// Assert
	// The change git is handed is the one the server read, so a rename carries
	// the path it came from.
	want := []gitrepo.Change{workTree()[1]}
	if recorder.Code != http.StatusOK || !slices.Equal(discarded, want) {
		t.Errorf("status = %d, discarded %+v; want 200 and exactly %+v", recorder.Code, discarded, want)
	}

	if listed := decode[api.ChangeList](t, recorder); len(listed.Changes) != len(workTree()) {
		t.Errorf("answered %d changes, want the working tree as read again", len(listed.Changes))
	}
}

func TestDiscardRefusesWhatItCannotDrop(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		body   string
		unwire func(*webserver.Deps)
		status int
		code   api.ProblemCode
		detail string
	}{
		"a path that is not a change": {
			body: `{"path":"../../.ssh/id_ed25519"}`, unwire: func(*webserver.Deps) {},
			status: http.StatusNotFound, code: api.NotFound,
		},
		"no discard seam": {
			body: `{"path":"` + editedPath + `"}`, unwire: func(deps *webserver.Deps) { deps.Discard = nil },
			status: http.StatusUnprocessableEntity, code: api.Unprocessable, detail: "discarding is not available",
		},
		"no path": {
			body: `{}`, unwire: func(*webserver.Deps) {},
			status: http.StatusBadRequest, code: api.BadRequest,
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			var discarded []gitrepo.Change

			deps := discarding(&discarded)
			tt.unwire(&deps)

			// Act
			recorder := sendStaging(t, deps, discardPath, tt.body)

			// Assert
			failure := decode[api.Problem](t, recorder)
			if recorder.Code != tt.status || failure.Code != tt.code || len(discarded) != 0 ||
				!strings.Contains(failure.Detail, tt.detail) {
				t.Errorf("status/code = %d/%s, detail %q, discarded %+v; want %d/%s, %q and nothing discarded",
					recorder.Code, failure.Code, failure.Detail, discarded, tt.status, tt.code, tt.detail)
			}
		})
	}
}

func TestADiscardGitRefusesNamesTheFileAndNeverTheHost(t *testing.T) {
	t.Parallel()

	// Arrange
	var discarded []gitrepo.Change

	deps := discarding(&discarded)
	deps.Discard = func(gitrepo.Change) error {
		return fmt.Errorf("discarding %s: %w: fatal: unable to access 'https://%s/acme/repo.git/'",
			editedPath, errSeam, gitHost)
	}

	// Act
	recorder := sendStaging(t, deps, discardPath, `{"path":"`+editedPath+`"}`)

	// Assert
	failure := decode[api.Problem](t, recorder)
	if recorder.Code != http.StatusUnprocessableEntity || !strings.Contains(failure.Detail, "discard "+editedPath) {
		t.Errorf("status = %d, detail %q; want 422 naming the discard of %s", recorder.Code, failure.Detail, editedPath)
	}

	if strings.Contains(recorder.Body.String(), gitHost) {
		t.Errorf("body = %q, leaks the remote's host", recorder.Body.String())
	}
}

func TestADiscardIsRefusedInDryRun(t *testing.T) {
	t.Parallel()

	// Arrange
	var discarded []gitrepo.Change

	dryRun := webserver.Info{Version: testVersion, DryRun: true}
	handler := serveWith(t, discarding(&discarded), config.Default(), dryRun)

	// Act
	recorder := send(t, handler, http.MethodPost, discardPath, `{"path":"`+editedPath+`"}`)

	// Assert
	if recorder.Code != http.StatusForbidden || len(discarded) != 0 {
		t.Errorf("status = %d, discarded %+v; want 403 and nothing discarded", recorder.Code, discarded)
	}
}
