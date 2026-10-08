// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package forge_test

import (
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/jacob-delgado/workflow/internal/forge"
)

// gitlabScopeRefusal is what gitlab.com answers a write made with a token that
// has read_api but not api.
const gitlabScopeRefusal = `{"error":"insufficient_scope",` +
	`"error_description":"The request requires higher privileges than provided by the access token.",` +
	`"scope":"api"}`

// githubTokenRefusal is GitHub refusing what a fine-grained token may do.
const githubTokenRefusal = `{"message":"Resource not accessible by personal access token"}`

// clientOn is a client of kind whose every request is answered with status and
// body.
func clientOn(t *testing.T, kind forge.Kind, status int, body string) forge.Client {
	t.Helper()

	client, _ := recordingForge(t, answering(status, body))

	return client.On(kind)
}

func TestAdviceSaysWhyTheForgeTurnedTheTokenDown(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		kind   forge.Kind
		status int
		body   string
		want   string
	}{
		"a GitLab token without the api scope": {
			kind: forge.KindGitLab, status: http.StatusForbidden, body: gitlabScopeRefusal,
			want: "GitLab refused this: the token may lack the api scope, or your role may not allow it. " +
				"GitLab said: insufficient_scope: The request requires higher privileges than provided by " +
				"the access token. (needs the api scope)",
		},
		"a GitLab token it did not accept": {
			kind: forge.KindGitLab, status: http.StatusUnauthorized, body: `{"message":"401 Unauthorized"}`,
			want: "GitLab did not accept the token; it may have expired or been revoked. " +
				"`workflow doctor --online` tests it. GitLab said: 401 Unauthorized",
		},
		"a GitHub refusal": {
			kind: forge.KindGitHub, status: http.StatusForbidden,
			body: githubTokenRefusal,
			want: "GitHub refused this: the token may lack the repo scope or the fine-grained permission this " +
				"needs, or your role may not allow it. GitHub said: Resource not accessible by personal access token",
		},
		"a forge that is not named, saying nothing": {
			kind: forge.KindUnknown, status: http.StatusForbidden, body: `{}`,
			want: "The forge refused this: the token may lack a scope this needs, or your role may not allow it.",
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			client := clientOn(t, tt.kind, tt.status, tt.body)
			_, err := client.CreatePullRequest(t.Context(), gitlabRepo(), forge.NewPullRequest{})

			// Act
			advice, ok := forge.Advice(err)

			// Assert
			if !ok || advice != tt.want {
				t.Errorf("Advice(%v) = %q, %t\nwant %q", err, advice, ok, tt.want)
			}
		})
	}
}

func TestAdviceLeavesAnyOtherFailureToItsOwnWords(t *testing.T) {
	t.Parallel()

	// Arrange
	client := clientOn(t, forge.KindGitLab, http.StatusBadGateway, `{}`)
	_, err := client.CreatePullRequest(t.Context(), gitlabRepo(), forge.NewPullRequest{})

	// Act
	_, ok := forge.Advice(err)

	// Assert
	if ok {
		t.Errorf("Advice(%v) gave advice, want none for a failure that is not a refusal", err)
	}
}

func TestARefusalStillAnswersToItsStatus(t *testing.T) {
	t.Parallel()

	// Arrange
	client := clientOn(t, forge.KindGitLab, http.StatusForbidden, gitlabScopeRefusal)

	// Act
	_, err := client.CreatePullRequest(t.Context(), gitlabRepo(), forge.NewPullRequest{})

	// Assert
	if !errors.Is(err, forge.ErrRefused) || errors.Is(err, forge.ErrUnauthorized) {
		t.Errorf("CreatePullRequest = %v, want ErrRefused alone", err)
	}
}

func TestAGitLabValidationMessageByFieldIsKept(t *testing.T) {
	t.Parallel()

	// Arrange
	client := clientOn(t, forge.KindGitLab, http.StatusUnprocessableEntity,
		`{"message":{"base":["Another open merge request already exists for this source branch"]}}`)

	// Act
	_, err := client.CreatePullRequest(t.Context(), gitlabRepo(), forge.NewPullRequest{})

	// Assert
	if !errors.Is(err, forge.ErrRejected) ||
		!strings.Contains(err.Error(), "base: Another open merge request already exists for this source branch") {
		t.Errorf("CreatePullRequest = %v, want GitLab's reason by field", err)
	}
}

func TestARefusalsReasonNamesNoAddress(t *testing.T) {
	t.Parallel()

	// Arrange
	// A gateway in front of a self-managed forge can answer for it, and point at
	// its own sign-in page on an internal host.
	client := clientOn(t, forge.KindGitLab, http.StatusUnauthorized,
		`{"error":"access_denied","error_description":"Sign in at https://sso.corp.internal/login?next=%2F first."}`)
	_, err := client.CreatePullRequest(t.Context(), gitlabRepo(), forge.NewPullRequest{})

	// Act
	advice, _ := forge.Advice(err)

	// Assert
	if strings.Contains(advice, "sso.corp.internal") || !strings.Contains(advice, "Sign in at (an address) first.") {
		t.Errorf("Advice = %q, want the reason kept with its address left out", advice)
	}
}
