// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package messaging_test

// users.info is the large-workspace fallback for tagging one person, so who it
// lets through decides who can be tagged; auth.test's scopes say which reads a
// token can make before one is refused.

import (
	"errors"
	"net/http"
	"sync/atomic"
	"testing"

	"github.com/jacob-delgado/workflow/internal/messaging"
)

// The Slack user every users.info test asks about, and the scope a test grants
// or leaves out.
const (
	adaID      = "U01"
	groupsRead = "usergroups:read"
)

// ada is the Slack user every users.info test asks about.
func ada(t *testing.T) messaging.SlackUserID {
	t.Helper()

	user, err := messaging.ParseSlackUser(adaID)
	if err != nil {
		t.Fatalf("ParseSlackUser: %v", err)
	}

	return user
}

func TestUserLabelsThePersonAskedAbout(t *testing.T) {
	t.Parallel()

	// Arrange
	var asked atomic.Value

	client := serve(t, func(writer http.ResponseWriter, request *http.Request) {
		asked.Store(request.URL.Path + "?" + request.URL.RawQuery)

		_, _ = writer.Write([]byte(`{"ok":true,"user":{"id":"U01","name":"ada","real_name":"Ada L",` +
			`"profile":{"display_name":"ada.l"}}}`))
	})

	// Act
	target, err := client.User(t.Context(), ada(t))

	// Assert
	if err != nil || target != (messaging.SlackTarget{ID: adaID, Label: "ada.l"}) {
		t.Errorf("User = %+v, %v; want ada labeled by her display name", target, err)
	}

	if got := asked.Load(); got != "/users.info?user=U01" {
		t.Errorf("asked %v, want users.info for U01", got)
	}
}

func TestUserRefusesSomeoneWhoCannotBeTagged(t *testing.T) {
	t.Parallel()

	cases := map[string]string{
		"a bot":            `{"id":"U01","name":"robot","is_bot":true,"profile":{}}`,
		"a deleted user":   `{"id":"U01","name":"gone","deleted":true,"profile":{}}`,
		"someone else":     `{"id":"U02","name":"bob","profile":{"display_name":"bob"}}`,
		"slackbot":         `{"id":"USLACKBOT","name":"slackbot","profile":{}}`,
		"an id of no user": `{"id":"bad id","name":"mallory","profile":{}}`,
	}

	for name, user := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			client := slackAnswering(t, `{"ok":true,"user":`+user+`}`)

			// Act
			target, err := client.User(t.Context(), ada(t))

			// Assert
			if !errors.Is(err, messaging.ErrNotTaggable) || target != (messaging.SlackTarget{}) {
				t.Errorf("User = %+v, %v; want nobody, and ErrNotTaggable", target, err)
			}
		})
	}
}

func TestAGrantLacksOnlyAScopeSlackListedTheTokenWithout(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		scopes string
		scope  string
		lacks  bool
	}{
		"a scope listed":                 {scopes: "chat:write, users:read", scope: "users:read", lacks: false},
		"a scope left off the list":      {scopes: "chat:write, users:read", scope: groupsRead, lacks: true},
		"any scope, with no list at all": {scopes: "", scope: groupsRead, lacks: false},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			client := serve(t, func(writer http.ResponseWriter, _ *http.Request) {
				if tt.scopes != "" {
					writer.Header().Set("X-Oauth-Scopes", tt.scopes)
				}

				_, _ = writer.Write([]byte(okBody))
			})

			identity, err := client.AuthTest(t.Context())
			if err != nil {
				t.Fatalf("AuthTest = %v, want the token's identity", err)
			}

			// Act
			lacks := identity.Granted.Lacks(tt.scope)

			// Assert
			if lacks != tt.lacks {
				t.Errorf("Lacks(%q) with %q listed = %v, want %v", tt.scope, tt.scopes, lacks, tt.lacks)
			}
		})
	}
}

func TestUserThatSlackDoesNotKnowIsReported(t *testing.T) {
	t.Parallel()

	// Arrange
	client := slackAnswering(t, `{"ok":false,"error":"user_not_found"}`)

	// Act
	target, err := client.User(t.Context(), ada(t))

	// Assert
	if err == nil || target != (messaging.SlackTarget{}) {
		t.Errorf("User = %+v, %v; want nobody, and Slack's refusal", target, err)
	}
}
