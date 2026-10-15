// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package webserver_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/jacob-delgado/workflow/internal/api"
	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/jira"
	"github.com/jacob-delgado/workflow/internal/webserver/webtest"
)

func TestTheContractRefusesWhatTheServerCannotUse(t *testing.T) {
	t.Parallel()

	cases := map[string]struct{ method, target, body string }{
		"a checkout of no branch": {
			method: http.MethodPost, target: webtest.CheckoutPath, body: `{"branch":""}`,
		},
		"a branch for no issue": {
			method: http.MethodPost, target: branchesAt, body: webtest.NoIssueKey,
		},
		"a worktree for no issue": {
			method: http.MethodPost, target: worktreesAt, body: webtest.NoIssueKey,
		},
		"a summary of a period that is no date": {
			method: http.MethodGet, target: activityPath + "?from=yesterday",
		},
		"a summary posted for a day that is no date": {
			method: http.MethodPost, target: activityPostPath, body: `{"from":"2026-13-01","to":"2026-13-01","text":"x"}`,
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			deps := webtest.FilledDeps()
			deps.Messaging.Post = func(context.Context, string, string) error { return nil }
			handler := webtest.Serve(t, deps, config.Default())

			// Act
			recorder := webtest.Send(t, handler, tt.method, tt.target, tt.body)

			// Assert
			if recorder.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400 from the contract: %s", recorder.Code, recorder.Body.String())
			}

			if got := webtest.Decode[api.Problem](t, recorder).Code; got != api.ProblemCodeBadRequest {
				t.Errorf("code = %q, want bad_request", got)
			}
		})
	}
}

// fieldsOf is the fields of the JSON object at path in the recorder's body,
// by name, as they came over the wire.
func fieldsOf(t *testing.T, body []byte, path ...string) map[string]json.RawMessage {
	t.Helper()

	var object map[string]json.RawMessage

	err := json.Unmarshal(body, &object)
	if err != nil {
		t.Fatalf("decoding %s: %v", body, err)
	}

	for _, name := range path {
		var inner []map[string]json.RawMessage

		err = json.Unmarshal(object[name], &inner)
		if err != nil || len(inner) == 0 {
			t.Fatalf("decoding %s's first entry from %s: %v", name, body, err)
		}

		object = inner[0]
	}

	return object
}

func TestACommentWhoseDateCouldNotBeReadCarriesNone(t *testing.T) {
	t.Parallel()

	// Arrange
	deps := webtest.FilledDeps()
	deps.Jira.Issue = func(_ context.Context, key jira.Key) (jira.IssueDetail, error) {
		return jira.IssueDetail{
			Issue:    jira.Issue{Key: key, Summary: webtest.TestSummary},
			Comments: []jira.Comment{{Author: webtest.TestReporter, Body: "Repro'd"}}, CommentTotal: 1,
		}, nil
	}

	// Act
	recorder := webtest.Get(t, webtest.Serve(t, deps, config.Default()), "/api/issues/"+webtest.TestKey)

	// Assert
	if comment := fieldsOf(t, recorder.Body.Bytes(), "comments"); comment["created"] != nil {
		t.Errorf("the comment carries created %s, want none rather than the zero time", comment["created"])
	}
}

func TestAReviewRequestWithNoTimeOpenedCarriesNone(t *testing.T) {
	t.Parallel()

	// Arrange
	deps := webtest.FilledDeps()
	deps.Forge.ReviewRequests = func(context.Context) ([]forge.ReviewRequest, error) {
		return []forge.ReviewRequest{{Number: 7, URL: "https://x/7", Title: "docs", Author: webtest.TestAuthor}}, nil
	}

	// Act
	recorder := webtest.Get(t, webtest.Serve(t, deps, config.Default()), "/api/reviews")

	// Assert
	if request := fieldsOf(t, recorder.Body.Bytes(), "requests"); request["opened_at"] != nil {
		t.Errorf("the request carries opened_at %s, want none rather than the zero time", request["opened_at"])
	}
}

func TestAReviewRequestCarriesTheTimeItWasOpened(t *testing.T) {
	t.Parallel()

	// Arrange
	opened := time.Date(2026, 10, 1, 9, 0, 0, 0, time.UTC)
	deps := webtest.FilledDeps()
	deps.Forge.ReviewRequests = func(context.Context) ([]forge.ReviewRequest, error) {
		return []forge.ReviewRequest{{Number: 7, URL: "https://x/7", Title: "docs", OpenedAt: opened}}, nil
	}

	// Act
	queue := webtest.Decode[api.ReviewQueue](t, webtest.Get(t, webtest.Serve(t, deps, config.Default()), "/api/reviews"))

	// Assert
	if len(queue.Requests) != 1 || queue.Requests[0].OpenedAt == nil || !queue.Requests[0].OpenedAt.Equal(opened) {
		t.Errorf("requests = %+v, want #7 opened at %v", queue.Requests, opened)
	}
}

func TestTheMessagingDestinationNamesItsServiceByKind(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		kind config.MessagingKind
		want api.MessagingDestinationKind
	}{
		"slack, written out":      {kind: config.KindSlack, want: api.MessagingDestinationKindSlack},
		"slack, as an empty kind": {kind: "", want: api.MessagingDestinationKindSlack},
		"teams":                   {kind: config.KindTeams, want: api.MessagingDestinationKindTeams},
		"discord":                 {kind: config.KindDiscord, want: api.MessagingDestinationKindDiscord},
		"a plain webhook":         {kind: config.KindWebhook, want: api.MessagingDestinationKindWebhook},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			cfg := config.Default()
			cfg.Messaging.Kind = tt.kind

			// Act
			destination := webtest.Decode[api.MessagingDestination](t, webtest.Get(t, webtest.Serve(t, webtest.FilledDeps(),
				cfg), "/api/messaging"))

			// Assert
			if destination.Kind != tt.want {
				t.Errorf("kind = %q, want %q", destination.Kind, tt.want)
			}
		})
	}
}
