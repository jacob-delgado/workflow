// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package webserver_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/jira"
	"github.com/jacob-delgado/workflow/internal/webserver"
	"github.com/jacob-delgado/workflow/internal/webserver/webtest"
)

// Fixtures the issue-write tests share.
const (
	reviewStatus = "In Review"
	// jiraHost is the tracker's address, which a Jira error can carry and no
	// answer may repeat.
	jiraHost = "jira.internal.example"
	// forgeHost is the forge's address, which a forge error can carry and no
	// answer may repeat either.
	forgeHost = "git.internal.example"
	linkPath  = "/api/issues/" + webtest.TestKey + "/link"
	movePath  = "/api/issues/" + webtest.TestKey + "/transition"
)

// linkCall is what a link seam was asked to record.
type linkCall struct {
	issueKey jira.Key
	url      string
	title    string
}

// writableDeps is webtest.FilledDeps with the tracker's writes wired: the
// checked-out branch names webtest.TestKey and has the pull request 42 open,
// and Jira offers a fields-less move to the review status. Each write records
// what it was asked.
func writableDeps(links *[]linkCall, moves *[]jira.Transition) webserver.Deps {
	deps := webtest.FilledDeps()
	deps.Jira.LinkPullRequest = func(_ context.Context, issueKey jira.Key, pullURL, title string) error {
		*links = append(*links, linkCall{issueKey: issueKey, url: pullURL, title: title})

		return nil
	}
	deps.Jira.Transitions = func(context.Context, jira.Key) ([]jira.Transition, error) {
		return []jira.Transition{
			{ID: "11", ToStatus: "Blocked"},
			{ID: "21", ToStatus: reviewStatus},
		}, nil
	}
	deps.Jira.Transition = func(_ context.Context, _ jira.Key, to jira.Transition, _ []jira.FieldValue) error {
		*moves = append(*moves, to)

		return nil
	}

	return deps
}

// reviewConfig is the default configuration with the review status set.
func reviewConfig() config.Config {
	cfg := config.Default()
	cfg.Jira.ReviewStatus = reviewStatus

	return cfg
}

// post sends a bodyless POST to path on a server over deps and cfg.
func post(t *testing.T, deps webserver.Deps, cfg config.Config, path string) *httptest.ResponseRecorder {
	t.Helper()

	return webtest.Send(t, webtest.Serve(t, deps, cfg), http.MethodPost, path, "")
}

// commentPath is where a comment on webtest.TestKey is posted.
const commentPath = "/api/issues/" + webtest.TestKey + "/comment"

// commentCall is what a comment seam was asked to post.
type commentCall struct {
	issueKey jira.Key
	text     string
}

// commentingDeps is webtest.FilledDeps with a comment seam that records what it
// was asked and answers it as Jira would store it.
func commentingDeps(comments *[]commentCall) webserver.Deps {
	deps := webtest.FilledDeps()
	deps.Jira.Comment = func(_ context.Context, issueKey jira.Key, text string) (jira.Comment, error) {
		*comments = append(*comments, commentCall{issueKey: issueKey, text: text})

		return jira.Comment{Author: "Ana Lima", Body: text}, nil
	}

	return deps
}

// postComment posts a comment request body to path on a server over deps.
func postComment(t *testing.T, deps webserver.Deps, path, body string) *httptest.ResponseRecorder {
	t.Helper()

	return webtest.Send(t, webtest.Serve(t, deps, config.Default()), http.MethodPost, path, body)
}
