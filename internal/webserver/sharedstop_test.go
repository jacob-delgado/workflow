// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package webserver_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/jira"
	"github.com/jacob-delgado/workflow/internal/loop"
	"github.com/jacob-delgado/workflow/internal/messaging"
	"github.com/jacob-delgado/workflow/internal/messaging/directory"
	"github.com/jacob-delgado/workflow/internal/taskwarrior"
	"github.com/jacob-delgado/workflow/internal/webserver"
)

// The tests here hold a read every stream shares to the server's lifetime: it
// runs on when the page whose frame began it leaves, but ends when the server
// stops, or the stream that began it, which waits for it, holds the server's
// drain for the whole grace.

// sharedReads holds each read every stream shares, by what it reads: each case
// hands its seam's first call to held, then answers as the seam did.
func sharedReads() map[string]func(deps *webserver.Deps, held func(context.Context) error) {
	return map[string]func(deps *webserver.Deps, held func(context.Context) error){
		"the forge's read of the branch": func(deps *webserver.Deps, held func(context.Context) error) {
			answer := deps.Forge.FindPullRequest
			deps.Forge.FindPullRequest = func(ctx context.Context, branch string) (forge.PullRequest, bool, error) {
				err := held(ctx)
				if err != nil {
					return forge.PullRequest{}, false, err
				}

				return answer(ctx, branch)
			}
		},
		"a view's search": func(deps *webserver.Deps, held func(context.Context) error) {
			answer := deps.Jira.Search
			deps.Jira.Search = func(ctx context.Context, jql string, startAt int) (jira.SearchResult, error) {
				err := held(ctx)
				if err != nil {
					return jira.SearchResult{}, err
				}

				return answer(ctx, jql, startAt)
			}
		},
		"the store's keeping of a view's first page": func(deps *webserver.Deps, held func(context.Context) error) {
			deps.Store.CacheIssues = func(ctx context.Context, _ string, _ []jira.Issue) error { return held(ctx) }
		},
		"the ask of which issues are yours": func(deps *webserver.Deps, held func(context.Context) error) {
			answer := deps.Jira.SearchLenient
			deps.Jira.SearchLenient = func(ctx context.Context, jql string, startAt int) (jira.SearchResult, error) {
				err := held(ctx)
				if err != nil {
					return jira.SearchResult{}, err
				}

				return answer(ctx, jql, startAt)
			}
		},
		"the ask of who a post comes from": func(deps *webserver.Deps, held func(context.Context) error) {
			answer := deps.Forge.Author
			deps.Forge.Author = func(ctx context.Context) (string, error) {
				err := held(ctx)
				if err != nil {
					return "", err
				}

				return answer(ctx)
			}
		},
		"the store's read of what was announced": func(deps *webserver.Deps, held func(context.Context) error) {
			deps.Store.Announced = func(ctx context.Context) []loop.Announced {
				_ = held(ctx)

				return nil
			}
		},
		"the store's read of the last scope": func(deps *webserver.Deps, held func(context.Context) error) {
			deps.Store.LastScope = func(ctx context.Context) (string, bool) {
				_ = held(ctx)

				return "", false
			}
		},
		"the search for Taskwarrior": func(deps *webserver.Deps, held func(context.Context) error) {
			fake := fakeTaskwarrior()
			deps.Tasks = fake.seams()
			deps.Tasks.Install = func(ctx context.Context) (taskwarrior.Install, error) {
				return fake.install, held(ctx)
			}
		},
	}
}

func TestASharedReadEndsWhenTheServerStops(t *testing.T) {
	t.Parallel()

	for name, hold := range sharedReads() {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			var asked atomic.Int32

			held, deps := newHeldUntilEnded(), scopedDeps([]jira.Key{yourIssue}, nil, &asked)
			hold(&deps, held.once())
			handler, lifetime, stop := serveForALifetime(t, deps, scopedConfig())
			stream := openStreamOf(t, handler, httptest.NewRequestWithContext(lifetime, http.MethodGet, "/api/events", nil))

			select {
			case <-held.entered:
			case <-time.After(endsWithin):
				t.Fatal("no frame began the shared read")
			}

			// Act
			stop()

			// Assert
			err := held.endedWith(t)
			if !errors.Is(err, context.Canceled) {
				t.Errorf("the shared read's context ended with %v, want it canceled once the server stopped", err)
			}

			select {
			case <-stream.done:
			case <-time.After(endsWithin):
				t.Error("the stream that began the shared read went on after the server stopped")
			}
		})
	}
}

// Ctrl+C stops the server with a shared read under way: the stream that began
// it waits for it, so Serve would wait out its whole grace for a read that
// went on.
func TestServeReturnsPromptlyWithASharedReadUnderWay(t *testing.T) {
	t.Parallel()

	// Arrange
	held, deps := newHeldUntilEnded(), filledDeps()
	deps.Forge.FindPullRequest = heldFirstFind(held, deps.Forge.FindPullRequest)
	handler, lifetime, stop := serveForALifetime(t, deps, scopedConfig())
	listener := listen(t)
	served := make(chan error, 1)

	go func() { served <- webserver.Serve(lifetime, listener, handler) }()

	go askOnce(t, "http://"+listener.Addr().String()+"/api/events")

	<-held.entered

	// Act
	stop()

	// Assert
	select {
	case <-served:
	case <-time.After(endsWithin):
		t.Errorf("Serve had not returned %v after the server stopped, want it within: the shared read held "+
			"the drain", endsWithin)
	}
}

// holdingSlack is a Slack on this machine that holds every request until it
// ends, and a client that reads from it with a user token. entered closes
// when the first request arrives, and ended once it has ended.
func holdingSlack(t *testing.T) (messaging.Client, <-chan struct{}, <-chan struct{}) {
	t.Helper()

	arrived, finished := make(chan struct{}), make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, request *http.Request) {
		close(arrived)

		select {
		case <-request.Context().Done():
			close(finished)
		case <-time.After(heldPatience):
		}
	}))
	t.Cleanup(server.Close)

	settings := config.Messaging{Kind: config.KindSlack, ClientID: slackApp, Channel: slackChannel}
	client := messaging.New(server.Client().Do, server.URL, settings).WithToken(
		func(context.Context, config.Secret) (config.Secret, error) { return "slack-token-for-tests", nil })

	return client, arrived, finished
}

// A Slack directory read goes on when the page that began it leaves, for any
// other page waiting on it, but it ends when the server stops: the request
// that began it waits for it, and would hold the drain for the whole grace.
func TestASlackDirectoryReadEndsWhenTheServerStops(t *testing.T) {
	t.Parallel()

	// Arrange
	client, entered, ended := holdingSlack(t)
	slack := directory.New(func() (messaging.Client, error) { return client, nil }, time.Now)
	deps := filledDeps()
	deps.Messaging.UserGroups = slack.UserGroups
	handler, lifetime, stop := serveForALifetime(t, deps, slackUserConfig())
	request := httptest.NewRequestWithContext(lifetime, http.MethodGet, groupsPath, nil)
	request.Host = loopbackHost
	answered := make(chan struct{})

	go func() {
		defer close(answered)

		handler.ServeHTTP(httptest.NewRecorder(), request)
	}()

	select {
	case <-entered:
	case <-time.After(endsWithin):
		t.Fatal("no page began the Slack directory read")
	}

	// Act
	stop()

	// Assert
	select {
	case <-ended:
	case <-time.After(endsWithin):
		t.Error("the Slack directory read went on after the server stopped")
	}

	<-answered
}
