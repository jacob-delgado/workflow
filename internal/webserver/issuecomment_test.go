// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package webserver_test

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/jacob-delgado/workflow/internal/api"
	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/jira"
	"github.com/jacob-delgado/workflow/internal/webserver"
	"github.com/jacob-delgado/workflow/internal/webserver/webtest"
)

func TestACommentIsPostedAndAnsweredAsJiraStoredIt(t *testing.T) {
	t.Parallel()

	// Arrange
	var comments []commentCall

	deps := commentingDeps(&comments)

	// Act
	recorder := postComment(t, deps, commentPath, `{"text":"Looks good to me"}`)

	// Assert
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d (%s), want 200", recorder.Code, recorder.Body.String())
	}

	want := commentCall{issueKey: webtest.TestKey, text: "Looks good to me"}
	if len(comments) != 1 || comments[0] != want {
		t.Errorf("comments = %+v, want %+v posted once", comments, want)
	}

	if posted := webtest.Decode[api.Comment](t, recorder); posted.Author != "Ana Lima" || posted.Body != want.text {
		t.Errorf("answer = %+v, want the comment as Jira stored it", posted)
	}
}

func TestACommentIsRefusedWhereItCannotBePosted(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		unwire func(*webserver.Deps)
		path   string
		body   string
	}{
		"blank text": {path: commentPath, body: `{"text":"  \n\t "}`},
		"no Jira to reach": {
			unwire: func(deps *webserver.Deps) { deps.Jira.Comment = nil },
			path:   commentPath,
			body:   `{"text":"hi"}`,
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			var comments []commentCall

			deps := commentingDeps(&comments)
			if tt.unwire != nil {
				tt.unwire(&deps)
			}

			// Act
			recorder := postComment(t, deps, tt.path, tt.body)

			// Assert
			failure := webtest.Decode[api.Problem](t, recorder)
			if recorder.Code != http.StatusUnprocessableEntity || failure.Code != api.ProblemCodeUnprocessable ||
				len(comments) != 0 {
				t.Errorf("status/code = %d/%s, comments = %+v; want 422/unprocessable and nothing posted",
					recorder.Code, failure.Code, comments)
			}
		})
	}
}

func TestACommentJiraCannotTakeSaysWhyWithoutItsHost(t *testing.T) {
	t.Parallel()

	// failing is a Jira error that names the tracker's address, as Jira's do.
	failing := func(cause error) error { return fmt.Errorf("%w: https://%s/rest/api/2/issue", cause, jiraHost) }
	cases := map[string]struct {
		err    error
		status int
		code   api.ProblemCode
	}{
		"no such issue": {err: failing(jira.ErrNotFound), status: http.StatusNotFound, code: api.ProblemCodeNotFound},
		"refused": {
			err: failing(jira.ErrRejected), status: http.StatusUnprocessableEntity, code: api.ProblemCodeUnprocessable,
		},
		"unreachable": {err: failing(jira.ErrUnreachable), status: http.StatusBadGateway, code: api.ProblemCodeUnreachable},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			deps := webtest.FilledDeps()
			deps.Jira.Comment = func(context.Context, jira.Key, string) (jira.Comment, error) { return jira.Comment{}, tt.err }

			// Act
			recorder := postComment(t, deps, commentPath, `{"text":"hi"}`)

			// Assert
			failure := webtest.Decode[api.Problem](t, recorder)
			if recorder.Code != tt.status || failure.Code != tt.code {
				t.Errorf("status/code = %d/%s, want %d/%s", recorder.Code, failure.Code, tt.status, tt.code)
			}

			if strings.Contains(recorder.Body.String(), jiraHost) {
				t.Errorf("body = %q, leaks the Jira host", recorder.Body.String())
			}
		})
	}
}

func TestDryRunRefusesAComment(t *testing.T) {
	t.Parallel()

	// Arrange
	var comments []commentCall

	dryRun := webserver.Info{Version: webtest.TestVersion, DryRun: true}
	handler := webtest.ServeWith(t, commentingDeps(&comments), config.Default(), dryRun)

	// Act
	recorder := webtest.Send(t, handler, http.MethodPost, commentPath, `{"text":"hi"}`)

	// Assert
	if recorder.Code != http.StatusForbidden || len(comments) != 0 {
		t.Errorf("status = %d, comments = %+v; want 403 and nothing posted", recorder.Code, comments)
	}
}

// forgeCommentPath is where a comment on forge issue 42 is posted.
const forgeCommentPath = "/api/issues/42/comment"

func TestAForgeIssuesCommentIsPostedAsWrittenWithMarkdownOn(t *testing.T) {
	t.Parallel()

	// Arrange
	// The forge renders Markdown itself; Jira's conversion would post its
	// wiki markup there.
	var comments []commentCall

	cfg := config.Default()
	cfg.Jira.MarkdownComments = true
	handler := webtest.Serve(t, commentingDeps(&comments), cfg)

	// Act
	recorder := webtest.Send(t, handler, http.MethodPost, forgeCommentPath, `{"text":"Ship **it**"}`)

	// Assert
	want := commentCall{issueKey: "42", text: "Ship **it**"}
	if recorder.Code != http.StatusOK || len(comments) != 1 || comments[0] != want {
		t.Errorf("status %d, comments = %+v; want %+v posted once", recorder.Code, comments, want)
	}
}

func TestACommentTheForgeCannotTakeSaysWhyWithoutItsHost(t *testing.T) {
	t.Parallel()

	const forgeHost = "git.internal.example"

	cases := map[string]struct {
		err    error
		status int
		detail string
	}{
		"turned down with a reason": {
			err:    fmt.Errorf("%w: Body is too long", forge.ErrRejected),
			status: http.StatusUnprocessableEntity, detail: "Body is too long",
		},
		"a forge that cannot be told": {
			err:    fmt.Errorf("%w: %s", forge.ErrUnknownForge, forgeHost),
			status: http.StatusUnprocessableEntity, detail: "set forge.kind and forge.host",
		},
		"a remote naming no repository": {
			err:    fmt.Errorf("%w: https://%s/", forge.ErrNotARemote, forgeHost),
			status: http.StatusUnprocessableEntity, detail: "origin",
		},
		"a repository the forge does not have": {
			err:    fmt.Errorf("%w: %s/42", forge.ErrNoRepository, forgeHost),
			status: http.StatusNotFound, detail: "not found",
		},
		"unreachable": {
			err:    fmt.Errorf("%w: https://%s", forge.ErrUnreachable, forgeHost),
			status: http.StatusBadGateway, detail: "",
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			deps := webtest.FilledDeps()
			deps.Jira.Comment = func(context.Context, jira.Key, string) (jira.Comment, error) { return jira.Comment{}, tt.err }

			// Act
			recorder := postComment(t, deps, forgeCommentPath, `{"text":"hi"}`)

			// Assert
			failure := webtest.Decode[api.Problem](t, recorder)
			if recorder.Code != tt.status || !strings.Contains(failure.Detail, tt.detail) {
				t.Errorf("status %d, detail %q; want %d naming %q", recorder.Code, failure.Detail, tt.status, tt.detail)
			}

			if strings.Contains(recorder.Body.String(), forgeHost) {
				t.Errorf("body = %q, leaks the forge's host", recorder.Body.String())
			}
		})
	}
}
