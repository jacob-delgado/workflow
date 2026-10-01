// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package wiring_test

// The Slack workspace a user token is for keys every link the store keeps, so
// it is read with the directory and held as long as the directory's reads:
// asked of Slack once, and again once those are dropped, as a switch of
// settings drops them.

import (
	"errors"
	"testing"
	"time"

	"github.com/jacob-delgado/workflow/internal/messaging"
	"github.com/jacob-delgado/workflow/internal/wiring"
)

func TestTheWorkspaceIsReadOnceWithTheDirectory(t *testing.T) {
	t.Parallel()

	// Arrange
	slack, client := startSlack(t, directoryBodies())
	directory, _ := directoryOver(client)
	_, _ = directory.Workspace(t.Context())

	// Act
	workspace, err := directory.Workspace(t.Context())

	// Assert
	if err != nil || workspace != "T0EXAMPLE" {
		t.Errorf("Workspace = %q, %v; want T0EXAMPLE", workspace, err)
	}

	if asked := slack.count("/auth.test"); asked != 1 {
		t.Errorf("auth.test was asked %d times, want once", asked)
	}
}

func TestRefreshReadsTheWorkspaceAgain(t *testing.T) {
	t.Parallel()

	// Arrange
	slack, client := startSlack(t, directoryBodies())
	directory, _ := directoryOver(client)
	_, _ = directory.Workspace(t.Context())

	directory.Refresh()

	// Act
	_, _ = directory.Workspace(t.Context())

	// Assert
	if asked := slack.count("/auth.test"); asked != 2 {
		t.Errorf("auth.test was asked %d times, want it asked again after a refresh", asked)
	}
}

func TestAWorkspaceSlackWouldNotNameIsAnErrorAskedAgain(t *testing.T) {
	t.Parallel()

	// Arrange
	bodies := directoryBodies()
	bodies["/auth.test"] = `{"ok":false,"error":"invalid_auth"}`
	slack, client := startSlack(t, bodies)
	directory, _ := directoryOver(client)
	_, _ = directory.Workspace(t.Context())

	// Act
	workspace, err := directory.Workspace(t.Context())

	// Assert
	if !errors.Is(err, messaging.ErrRejected) || workspace != "" {
		t.Errorf("Workspace = %q, %v; want Slack's refusal", workspace, err)
	}

	if asked := slack.count("/auth.test"); asked != 2 {
		t.Errorf("auth.test was asked %d times, want a failure asked again", asked)
	}
}

func TestNoUserTokenHasNoWorkspace(t *testing.T) {
	t.Parallel()

	// Arrange
	_, client := startSlack(t, directoryBodies())
	settings := &switchable{client: client, available: false}
	directory := wiring.NewSlackDirectory(settings.current, time.Now)

	// Act
	_, err := directory.Workspace(t.Context())

	// Assert
	if !errors.Is(err, messaging.ErrNoCredential) {
		t.Errorf("Workspace with no user token = %v, want ErrNoCredential", err)
	}
}
