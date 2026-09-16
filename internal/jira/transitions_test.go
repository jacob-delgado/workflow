// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package jira_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"slices"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/jira"
)

// transitionsBody is shaped like Data Center's answer: each transition names the
// status it leads to, which is rarely the transition's own name.
const transitionsBody = `{"expand":"transitions","transitions":[` +
	`{"id":"21","name":"Start Progress","to":{"name":"In Progress","statusCategory":{"key":"indeterminate"}}},` +
	`{"id":"31","name":"Done","to":{"name":"Done","statusCategory":{"key":"done"}}}]}`

// startProgress is the first transition in transitionsBody.
func startProgress() jira.Transition {
	return jira.Transition{
		ID: "21", Name: "Start Progress", ToStatus: inProgress, ToStatusCategory: indeterminate, Fields: nil,
	}
}

// sameTransition compares two transitions, fields included.
func sameTransition(got, want jira.Transition) bool {
	return got.ID == want.ID && got.Name == want.Name && got.ToStatus == want.ToStatus &&
		got.ToStatusCategory == want.ToStatusCategory && slices.EqualFunc(got.Fields, want.Fields, equalFields)
}

// offline is a transport that fails every request, and records whether one was
// attempted at all.
func offline(sent *atomic.Bool) jira.Doer {
	return func(*http.Request) (*http.Response, error) {
		sent.Store(true)

		return nil, errFixtureTimeout
	}
}

func TestTransitionsListsWhatJiraOffers(t *testing.T) {
	t.Parallel()

	var requested atomic.Value

	client := serve(t, func(writer http.ResponseWriter, request *http.Request) {
		requested.Store(request.Method + " " + request.URL.Path)
		answer(transitionsBody, "fred")(writer, request)
	})

	found, err := client.Transitions(t.Context(), "OPS-1")
	if err != nil {
		t.Fatalf("Transitions returned %v, want nil", err)
	}

	want := []jira.Transition{
		startProgress(),
		{ID: "31", Name: "Done", ToStatus: "Done", ToStatusCategory: "done", Fields: nil},
	}

	if len(found) != len(want) {
		t.Fatalf("got %d transitions, want %d", len(found), len(want))
	}

	for index, transition := range want {
		if !sameTransition(found[index], transition) {
			t.Errorf("transition %d = %+v, want %+v", index, found[index], transition)
		}
	}

	if got := requested.Load(); got != "GET /rest/api/2/issue/OPS-1/transitions" {
		t.Errorf("requested %v, want GET of the issue's transitions", got)
	}
}

func TestTransitionsEscapesTheIssueKey(t *testing.T) {
	t.Parallel()

	var requested atomic.Value

	client := serve(t, func(writer http.ResponseWriter, request *http.Request) {
		requested.Store(request.RequestURI)
		answer(transitionsBody, "fred")(writer, request)
	})

	// A key is text a server supplied. It must stay one path segment, never
	// climb out of the issue it names.
	_, err := client.Transitions(t.Context(), "../../myself?x=")
	if err != nil {
		t.Fatalf("Transitions returned %v, want nil", err)
	}

	got, _ := requested.Load().(string)
	if !strings.HasPrefix(got, "/rest/api/2/issue/..%2F..%2Fmyself%3Fx=/transitions") {
		t.Errorf("requested %q, want the key escaped into a single segment", got)
	}
}

func TestTransitionsTreatsAnAnonymousAnswerAsARejectedCredential(t *testing.T) {
	t.Parallel()

	// Checked live: an issue's transitions, served anonymously, are an empty
	// list with 200 — which would read as "Jira offers nothing to move to".
	_, err := serve(t, answer(`{"expand":"transitions","transitions":[]}`, "anonymous")).
		Transitions(t.Context(), "OPS-1")
	if !errors.Is(err, jira.ErrUnauthorized) {
		t.Errorf("Transitions returned %v, want ErrUnauthorized", err)
	}
}

func TestTransitionsReportsAnIssueJiraWillNotShow(t *testing.T) {
	t.Parallel()

	body := `{"errorMessages":["Issue Does Not Exist"],"errors":{}}`

	_, err := serve(t, failWith(http.StatusNotFound, body, "fred")).Transitions(t.Context(), "OPS-404")
	if !errors.Is(err, jira.ErrRejected) || !strings.Contains(err.Error(), "Issue Does Not Exist") {
		t.Errorf("Transitions returned %v, want Jira's reason rather than a missing API", err)
	}
}

func TestTransitionsReportsAnUnreachableServer(t *testing.T) {
	t.Parallel()

	var sent atomic.Bool

	_, err := jira.New(offline(&sent), bearerConfig(exampleBaseURL)).Transitions(t.Context(), "OPS-1")
	if !errors.Is(err, jira.ErrUnreachable) || !errors.Is(err, errFixtureTimeout) {
		t.Errorf("Transitions returned %v, want ErrUnreachable wrapping the cause", err)
	}
}

func TestTransitionsWithoutACredentialSendsNothing(t *testing.T) {
	t.Parallel()

	var sent atomic.Bool

	client := jira.New(offline(&sent), config.Jira{BaseURL: exampleBaseURL, Token: "", User: ""})

	_, err := client.Transitions(t.Context(), "OPS-1")
	if !errors.Is(err, jira.ErrNoCredential) {
		t.Errorf("Transitions returned %v, want ErrNoCredential", err)
	}

	if sent.Load() {
		t.Error("a request went out with no credential to send")
	}
}

func TestTransitionsReportsAnUnreadableBody(t *testing.T) {
	t.Parallel()

	_, err := serve(t, answer("{not json", "fred")).Transitions(t.Context(), "OPS-1")
	if err == nil {
		t.Error("Transitions accepted a malformed body, want an error")
	}
}

func TestApplyTransitionPostsTheChosenTransition(t *testing.T) {
	t.Parallel()

	var (
		requested   atomic.Value
		contentType atomic.Value
		chosen      atomic.Value
	)

	client := serve(t, func(writer http.ResponseWriter, request *http.Request) {
		requested.Store(request.Method + " " + request.URL.Path)
		contentType.Store(request.Header.Get("Content-Type"))

		var body struct {
			Transition struct {
				ID string `json:"id"`
			} `json:"transition"`
		}

		_ = json.NewDecoder(request.Body).Decode(&body)
		chosen.Store(body.Transition.ID)

		// Data Center answers a transition it made with no content at all.
		writer.Header().Set("X-Ausername", "fred")
		writer.WriteHeader(http.StatusNoContent)
	})

	err := client.ApplyTransition(t.Context(), "OPS-1", startProgress(), nil)
	if err != nil {
		t.Fatalf("ApplyTransition returned %v, want nil", err)
	}

	if got := requested.Load(); got != "POST /rest/api/2/issue/OPS-1/transitions" {
		t.Errorf("requested %v, want a POST to the issue's transitions", got)
	}

	if got := contentType.Load(); got != jsonMediaType {
		t.Errorf("Content-Type = %v, want application/json", got)
	}

	if got := chosen.Load(); got != "21" {
		t.Errorf("sent transition %v, want 21", got)
	}
}

func TestApplyTransitionReportsWhyJiraRefused(t *testing.T) {
	t.Parallel()

	// The common refusal: the transition has a screen, and a field on it is
	// required.
	body := `{"errorMessages":[],"errors":{"resolution":"Resolution is required."}}`

	client := serve(t, failWith(http.StatusBadRequest, body, "fred"))

	err := client.ApplyTransition(t.Context(), "OPS-1", startProgress(), nil)
	if !errors.Is(err, jira.ErrRejected) || !strings.Contains(err.Error(), "Resolution is required.") {
		t.Errorf("ApplyTransition returned %v, want Jira's reason", err)
	}
}

func TestApplyTransitionReportsAnUnreachableServer(t *testing.T) {
	t.Parallel()

	var sent atomic.Bool

	err := jira.New(offline(&sent), bearerConfig(exampleBaseURL)).
		ApplyTransition(t.Context(), "OPS-1", startProgress(), nil)
	if !errors.Is(err, jira.ErrUnreachable) || !errors.Is(err, errFixtureTimeout) {
		t.Errorf("ApplyTransition returned %v, want ErrUnreachable wrapping the cause", err)
	}
}

func TestApplyTransitionWithoutACredentialSendsNothing(t *testing.T) {
	t.Parallel()

	var sent atomic.Bool

	client := jira.New(offline(&sent), config.Jira{BaseURL: exampleBaseURL, Token: "", User: ""})

	err := client.ApplyTransition(t.Context(), "OPS-1", startProgress(), nil)
	if !errors.Is(err, jira.ErrNoCredential) {
		t.Errorf("ApplyTransition returned %v, want ErrNoCredential", err)
	}

	if sent.Load() {
		t.Error("a transition went out with no credential to send")
	}
}
