// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package jira_test

import (
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/jacob-delgado/workflow/internal/jira"
)

// failWith answers with a status and a body, optionally naming the user Jira
// thinks it served.
func failWith(status int, body, user string) http.HandlerFunc {
	return func(writer http.ResponseWriter, _ *http.Request) {
		if user != "" {
			writer.Header().Set("X-Ausername", user)
		}

		writer.Header().Set("Content-Type", jsonMediaType)
		writer.WriteHeader(status)

		_, _ = writer.Write([]byte(body))
	}
}

func TestARejectionCarriesJirasReason(t *testing.T) {
	t.Parallel()

	body := `{"errorMessages":["Error in the JQL Query: Expecting a value but got '='."],"errors":{}}`

	_, err := serve(t, failWith(http.StatusBadRequest, body, "fred")).Search(t.Context(), "assignee = = currentUser()")
	if !errors.Is(err, jira.ErrRejected) {
		t.Fatalf("Search returned %v, want ErrRejected", err)
	}

	// The reason is the whole point: "400" tells nobody what to change.
	if !strings.Contains(err.Error(), "Expecting a value but got '='.") {
		t.Errorf("the error does not give Jira's reason: %v", err)
	}
}

func TestARejectionListsTheRequestReasonsThenEachFieldInOrder(t *testing.T) {
	t.Parallel()

	body := `{"errorMessages":["The issue could not be moved."],` +
		`"errors":{"resolution":"Resolution is required.","assignee":"Assignee is required."}}`

	_, err := serve(t, failWith(http.StatusBadRequest, body, "fred")).Search(t.Context(), jira.AssignedToMe)

	// Fields in name order, so the same rejection always reads the same way.
	want := "The issue could not be moved.; Assignee is required.; Resolution is required."
	if err == nil || !strings.Contains(err.Error(), want) {
		t.Errorf("Search returned %v, want the reasons %q", err, want)
	}
}

func TestAStatusWithoutAReasonKeepsItsOwnError(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		status int
		body   string
		want   error
	}{
		// A proxy's page in front of a missing context path is not Jira talking.
		"html from a proxy": {status: http.StatusNotFound, body: "<html>Not Found</html>", want: jira.ErrNoAPI},
		"json with no reason": {
			status: http.StatusInternalServerError, body: `{"errorMessages":[],"errors":{}}`,
			want: jira.ErrUnexpectedStatus,
		},
		// Jira gives "Login Required" here, but the status says it better, and
		// callers test for this error by identity.
		"unauthorized": {
			status: http.StatusUnauthorized, body: `{"errorMessages":["Login Required"]}`,
			want: jira.ErrUnauthorized,
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			_, err := serve(t, failWith(tt.status, tt.body, "fred")).Search(t.Context(), jira.AssignedToMe)
			if !errors.Is(err, tt.want) {
				t.Errorf("Search returned %v, want %v", err, tt.want)
			}

			if errors.Is(err, jira.ErrRejected) {
				t.Errorf("Search returned %v, which claims a reason Jira did not give", err)
			}
		})
	}
}

func TestAnAnonymousAnswerIsBlamedOnTheCredentialBeforeJirasReason(t *testing.T) {
	t.Parallel()

	// Served anonymously, Jira's reason describes what an anonymous user may
	// not do — true, and useless: the configured token is what failed.
	body := `{"errorMessages":["You do not have the permission to see the specified issue."],"errors":{}}`

	_, err := serve(t, failWith(http.StatusNotFound, body, "anonymous")).Search(t.Context(), jira.AssignedToMe)
	if !errors.Is(err, jira.ErrUnauthorized) {
		t.Errorf("Search returned %v, want ErrUnauthorized", err)
	}
}

func TestJirasReasonNeverCarriesTheToken(t *testing.T) {
	t.Parallel()

	// Nothing in Jira echoes a credential, but the reason is text a server
	// chose, and it is about to be printed.
	body := `{"errorMessages":["Rejected Authorization: Bearer ` + token + `"],"errors":{}}`

	_, err := serve(t, failWith(http.StatusBadRequest, body, "fred")).Search(t.Context(), jira.AssignedToMe)
	if err == nil {
		t.Fatal("Search returned nil, want the rejection")
	}

	if strings.Contains(err.Error(), token) {
		t.Errorf("the error carried the token: %v", err)
	}

	if !strings.Contains(err.Error(), "Rejected Authorization") {
		t.Errorf("masking the token dropped the rest of the reason: %v", err)
	}
}
