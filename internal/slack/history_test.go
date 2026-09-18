// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package slack_test

import (
	"errors"
	"net/http"
	"sync/atomic"
	"testing"

	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/slack"
)

// historyURL is the pull request address these tests look for in a channel.
const historyURL = "https://github.com/example/repo/pull/42"

// The Slack Web API paths the history search calls.
const (
	listPath    = "/conversations.list"
	historyPath = "/conversations.history"
)

// errUnexpectedRequest is a doer's complaint that it was called when it should
// not have been.
var errUnexpectedRequest = errors.New("the client made a request it should not have")

func TestAlreadyPostedFindsTheURLInTheResolvedChannel(t *testing.T) {
	t.Parallel()

	// Arrange
	var historyChannel atomic.Value

	client := serve(t, func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case listPath:
			_, _ = writer.Write([]byte(`{"ok":true,"channels":[{"id":"C0DEV","name":"dev"}]}`))
		case historyPath:
			historyChannel.Store(request.URL.Query().Get("channel"))

			_, _ = writer.Write([]byte(`{"ok":true,"messages":[{"text":"Ana opened <` + historyURL + `|PR>"}]}`))
		default:
			writer.WriteHeader(http.StatusNotFound)
		}
	})

	// Act
	found, err := client.AlreadyPosted(t.Context(), "#dev", historyURL)

	// Assert
	if err != nil || !found || historyChannel.Load() != "C0DEV" {
		t.Errorf("AlreadyPosted = %t, %v; searched channel %v; want it found in C0DEV", found, err, historyChannel.Load())
	}
}

func TestAlreadyPostedIsFalseWhenTheURLIsAbsent(t *testing.T) {
	t.Parallel()

	// Arrange
	client := serve(t, func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path == listPath {
			_, _ = writer.Write([]byte(`{"ok":true,"channels":[{"id":"C0DEV","name":"dev"}]}`))

			return
		}

		_, _ = writer.Write([]byte(`{"ok":true,"messages":[{"text":"unrelated chatter"}]}`))
	})

	// Act
	found, err := client.AlreadyPosted(t.Context(), "#dev", historyURL)

	// Assert
	if err != nil || found {
		t.Errorf("AlreadyPosted = %t, %v; want it not found", found, err)
	}
}

func TestAlreadyPostedFollowsPagesToFindTheChannel(t *testing.T) {
	t.Parallel()

	// Arrange
	// The channel is only on the second page of the list.
	client := serve(t, func(writer http.ResponseWriter, request *http.Request) {
		switch {
		case request.URL.Path == listPath && request.URL.Query().Get("cursor") == "":
			_, _ = writer.Write([]byte(`{"ok":true,"channels":[{"id":"C0OPS","name":"ops"}],` +
				`"response_metadata":{"next_cursor":"page2"}}`))
		case request.URL.Path == listPath:
			_, _ = writer.Write([]byte(`{"ok":true,"channels":[{"id":"C0DEV","name":"dev"}]}`))
		default:
			_, _ = writer.Write([]byte(`{"ok":true,"messages":[{"text":"<` + historyURL + `|PR>"}]}`))
		}
	})

	// Act
	found, err := client.AlreadyPosted(t.Context(), "#dev", historyURL)

	// Assert
	if err != nil || !found {
		t.Errorf("AlreadyPosted = %t, %v; want it found on the second page", found, err)
	}
}

func TestAlreadyPostedReportsAChannelItCannotSee(t *testing.T) {
	t.Parallel()

	// Arrange
	client := serve(t, func(writer http.ResponseWriter, _ *http.Request) {
		_, _ = writer.Write([]byte(`{"ok":true,"channels":[{"id":"C0OPS","name":"ops"}]}`))
	})

	// Act
	_, err := client.AlreadyPosted(t.Context(), "#dev", historyURL)

	// Assert
	if !errors.Is(err, slack.ErrChannelNotFound) {
		t.Errorf("AlreadyPosted error = %v, want ErrChannelNotFound", err)
	}
}

func TestAlreadyPostedDefaultsToTheConfiguredChannel(t *testing.T) {
	t.Parallel()

	// Arrange
	client := serve(t, func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path == listPath {
			_, _ = writer.Write([]byte(`{"ok":true,"channels":[{"id":"C0DEV","name":"dev"}]}`))

			return
		}

		_, _ = writer.Write([]byte(`{"ok":true,"messages":[{"text":"<` + historyURL + `|PR>"}]}`))
	})

	// Act
	found, err := client.AlreadyPosted(t.Context(), "", historyURL)

	// Assert
	if err != nil || !found {
		t.Errorf("AlreadyPosted with the default channel = %t, %v; want it found", found, err)
	}
}

func TestAlreadyPostedRelaysAChannelListRejection(t *testing.T) {
	t.Parallel()

	// Arrange
	client := serve(t, func(writer http.ResponseWriter, _ *http.Request) {
		_, _ = writer.Write([]byte(`{"ok":false,"error":"invalid_auth"}`))
	})

	// Act
	_, err := client.AlreadyPosted(t.Context(), "#dev", historyURL)

	// Assert
	if !errors.Is(err, slack.ErrRejected) {
		t.Errorf("AlreadyPosted error = %v, want ErrRejected", err)
	}
}

func TestAlreadyPostedRelaysAHistoryRejection(t *testing.T) {
	t.Parallel()

	// Arrange
	client := serve(t, func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path == listPath {
			_, _ = writer.Write([]byte(`{"ok":true,"channels":[{"id":"C0DEV","name":"dev"}]}`))

			return
		}

		_, _ = writer.Write([]byte(`{"ok":false,"error":"not_in_channel"}`))
	})

	// Act
	_, err := client.AlreadyPosted(t.Context(), "#dev", historyURL)

	// Assert
	if !errors.Is(err, slack.ErrRejected) {
		t.Errorf("AlreadyPosted error = %v, want ErrRejected", err)
	}
}

func TestAlreadyPostedReportsATransportFailure(t *testing.T) {
	t.Parallel()

	// Arrange
	client := slack.New(func(*http.Request) (*http.Response, error) {
		return nil, errBrokeOff
	}, slack.APIBase, botCredentials())

	// Act
	_, err := client.AlreadyPosted(t.Context(), "#dev", historyURL)

	// Assert
	if !errors.Is(err, slack.ErrUnreachable) {
		t.Errorf("AlreadyPosted error = %v, want ErrUnreachable", err)
	}
}

func TestAlreadyPostedCannotCheckAWebhook(t *testing.T) {
	t.Parallel()

	// Arrange
	client := slack.New(func(*http.Request) (*http.Response, error) {
		return nil, errUnexpectedRequest
	}, slack.APIBase, config.Slack{WebhookURL: "https://hooks.slack.example/x"})

	// Act
	_, err := client.AlreadyPosted(t.Context(), "#dev", historyURL)

	// Assert
	if !errors.Is(err, slack.ErrWebhookUncheckable) {
		t.Errorf("AlreadyPosted error = %v, want ErrWebhookUncheckable", err)
	}
}
