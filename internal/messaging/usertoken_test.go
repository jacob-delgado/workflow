// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package messaging_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/messaging"
)

// The user tokens a refresh swaps: the one Slack calls expired, and its
// replacement. Not shaped like real tokens: gitleaks scans this repository.
const (
	staleToken = "stale-user-token"
	freshToken = "fresh-user-token"
)

// rotating is a token source that hands out the stale token until asked past
// it, recording what it was asked past.
type rotating struct {
	lock  sync.Mutex
	asked []config.Secret
}

func (r *rotating) token(_ context.Context, expired config.Secret) (config.Secret, error) {
	r.lock.Lock()
	defer r.lock.Unlock()

	r.asked = append(r.asked, expired)

	if expired == staleToken {
		return freshToken, nil
	}

	return staleToken, nil
}

// slackExpiringOnce answers a request carrying the stale token with
// token_expired and any other with ok, recording the tokens it saw.
func slackExpiringOnce(t *testing.T, okAnswer string) (*httptest.Server, *[]string) {
	t.Helper()

	var (
		lock sync.Mutex
		seen []string
	)

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		auth := request.Header.Get("Authorization")

		lock.Lock()

		seen = append(seen, auth)

		lock.Unlock()

		if auth == "Bearer "+staleToken {
			_, _ = writer.Write([]byte(`{"ok":false,"error":"token_expired"}`))

			return
		}

		_, _ = writer.Write([]byte(okAnswer))
	}))
	t.Cleanup(server.Close)

	return server, &seen
}

// userCredentials is a Slack user token's settings.
func userCredentials() config.Messaging {
	return config.Messaging{Kind: config.KindSlack, ClientID: "1234.5678", Channel: "#dev"}
}

func TestAPostSlackCallsExpiredIsMadeAgainWithANewerToken(t *testing.T) {
	t.Parallel()

	// Arrange
	server, seen := slackExpiringOnce(t, `{"ok":true}`)
	source := &rotating{}
	client := messaging.New(server.Client().Do, server.URL, userCredentials()).WithToken(source.token)

	// Act
	err := client.Post(t.Context(), "", message)
	// Assert
	if err != nil {
		t.Fatalf("Post = %v, want it made again with the newer token", err)
	}

	if len(*seen) != 2 || (*seen)[1] != "Bearer "+freshToken {
		t.Errorf("Slack saw %q, want the stale token and then the fresh one", *seen)
	}

	if len(source.asked) != 2 || source.asked[1] != staleToken {
		t.Errorf("the source was asked past %q, want past the stale token the second time", source.asked)
	}
}

func TestATokenSlackCallsExpiredTwiceIsReportedExpired(t *testing.T) {
	t.Parallel()

	// Arrange
	server, _ := slackReceiving(t, http.StatusOK, `{"ok":false,"error":"token_expired"}`)
	client := messaging.New(server.Client().Do, server.URL, userCredentials()).
		WithToken(func(context.Context, config.Secret) (config.Secret, error) { return staleToken, nil })

	// Act
	err := client.Post(t.Context(), "", message)

	// Assert
	if !errors.Is(err, messaging.ErrTokenExpired) || !errors.Is(err, messaging.ErrRejected) {
		t.Errorf("Post = %v, want an expired token reported, still as a credential refused", err)
	}
}

func TestAuthTestAsksAgainWithANewerTokenAfterExpiry(t *testing.T) {
	t.Parallel()

	// Arrange
	server, _ := slackExpiringOnce(t, `{"ok":true,"user":"jacob","team":"Example"}`)
	source := &rotating{}
	client := messaging.New(server.Client().Do, server.URL, userCredentials()).WithToken(source.token)

	// Act
	identity, err := client.AuthTest(t.Context())

	// Assert
	if err != nil || identity.User != "jacob" {
		t.Errorf("AuthTest = %+v, %v; want the user the newer token belongs to", identity, err)
	}
}

func TestAUserTokenWithNoSourceSendsNothing(t *testing.T) {
	t.Parallel()

	// Arrange
	var sent atomic.Bool

	client := messaging.New(counting(&sent), messaging.APIBase, userCredentials())

	// Act
	err := client.Post(t.Context(), "", message)

	// Assert
	if !errors.Is(err, messaging.ErrNoCredential) || sent.Load() {
		t.Errorf("Post = %v (sent %t), want no credential and nothing sent", err, sent.Load())
	}
}
