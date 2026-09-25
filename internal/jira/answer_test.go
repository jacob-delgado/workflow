// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package jira_test

import (
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/jacob-delgado/workflow/internal/httpx"
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

func TestARejectionCarriesJirasReasons(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		body string
		want string
	}{
		// The reason is the whole point: "400" tells nobody what to change.
		"a reason for the request": {
			body: `{"errorMessages":["Error in the JQL Query: Expecting a value but got '='."],"errors":{}}`,
			want: "Expecting a value but got '='.",
		},
		// Fields in name order, so the same rejection always reads the same way.
		"the request's reasons, then each field's in order": {
			body: `{"errorMessages":["The issue could not be moved."],` +
				`"errors":{"resolution":"Resolution is required.","assignee":"Assignee is required."}}`,
			want: "The issue could not be moved.; Assignee is required.; Resolution is required.",
		},
		// Nothing in Jira echoes a credential, but the reason is text a server
		// chose, and it is about to be printed: the token is masked, and the rest
		// of the reason kept.
		"a reason quoting the token": {
			body: `{"errorMessages":["Rejected Authorization: Bearer ` + token + `"],"errors":{}}`,
			want: "Rejected Authorization",
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			client := serve(t, failWith(http.StatusBadRequest, tt.body, servedUser))

			// Act
			_, err := client.Search(t.Context(), jira.AssignedToMe, 0)

			// Assert
			if !errors.Is(err, jira.ErrRejected) {
				t.Fatalf("Search returned %v, want ErrRejected", err)
			}

			if !strings.Contains(err.Error(), tt.want) || strings.Contains(err.Error(), token) {
				t.Errorf("Search returned %v, want Jira's reasons %q and never the token", err, tt.want)
			}
		})
	}
}

func TestAnExplainedStatusKeepsWhatItsStatusSays(t *testing.T) {
	t.Parallel()

	// Jira's reason leads the message, but a caller telling failures apart by
	// the status — a missing issue, a token refused — still finds it.
	cases := map[string]struct {
		status int
		reason string
		also   error
	}{
		"a missing issue": {
			status: http.StatusNotFound, reason: "Issue Does Not Exist", also: jira.ErrNotFound,
		},
		"a token refused for this": {
			status: http.StatusForbidden, reason: "You do not have permission to assign issues.", also: jira.ErrForbidden,
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			body := `{"errorMessages":["` + tt.reason + `"],"errors":{}}`
			client := serve(t, failWith(tt.status, body, servedUser))

			// Act
			_, err := client.Search(t.Context(), jira.AssignedToMe, 0)

			// Assert
			if !errors.Is(err, jira.ErrRejected) || !errors.Is(err, tt.also) {
				t.Fatalf("Search returned %v, want both ErrRejected and %v", err, tt.also)
			}

			if !strings.Contains(err.Error(), tt.reason) || strings.Contains(err.Error(), tt.also.Error()) {
				t.Errorf("Search returned %q, want Jira's reason %q in place of %q", err, tt.reason, tt.also)
			}
		})
	}
}

func TestAStatusWithoutAReasonKeepsItsOwnError(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		status int
		body   string
		user   string
		want   error
	}{
		// A proxy's page in front of a missing context path is not Jira talking.
		"html from a proxy": {
			status: http.StatusNotFound, body: "<html>Not Found</html>", user: servedUser, want: jira.ErrNoAPI,
		},
		"json with no reason": {
			status: http.StatusInternalServerError, body: `{"errorMessages":[],"errors":{}}`, user: servedUser,
			want: jira.ErrUnexpectedStatus,
		},
		// Jira gives "Login Required" here, but the status says it better, and
		// callers test for this error by identity.
		"unauthorized": {
			status: http.StatusUnauthorized, body: `{"errorMessages":["Login Required"]}`, user: servedUser,
			want: jira.ErrUnauthorized,
		},
		// A 429 is told apart from a refusal, so the caller is told to wait
		// rather than that its credential is wrong.
		"rate limited": {
			status: http.StatusTooManyRequests, body: `{}`, user: servedUser,
			want: httpx.ErrRateLimited,
		},
		// Served anonymously, Jira's reason describes what an anonymous user may
		// not do — true, and useless: the configured token is what failed.
		"an anonymous answer, blamed on the credential": {
			status: http.StatusNotFound,
			body:   `{"errorMessages":["You do not have the permission to see the specified issue."],"errors":{}}`,
			user:   "anonymous",
			want:   jira.ErrUnauthorized,
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			client := serve(t, failWith(tt.status, tt.body, tt.user))

			// Act
			_, err := client.Search(t.Context(), jira.AssignedToMe, 0)

			// Assert
			if !errors.Is(err, tt.want) {
				t.Errorf("Search returned %v, want %v", err, tt.want)
			}

			if errors.Is(err, jira.ErrRejected) {
				t.Errorf("Search returned %v, which claims a reason Jira did not give", err)
			}

			if errors.Is(err, jira.ErrNotFound) {
				t.Errorf("Search returned %v, marked not-found with no resource reason", err)
			}
		})
	}
}
