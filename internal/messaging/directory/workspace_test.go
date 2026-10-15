// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package directory_test

// The workspace's own name is read with the directory and held as it is.

import (
	"errors"
	"testing"
	"time"

	"github.com/jacob-delgado/workflow/internal/messaging"
	"github.com/jacob-delgado/workflow/internal/messaging/directory"
)

func TestTheWorkspaceIsReadOnceWithTheDirectory(t *testing.T) {
	t.Parallel()

	// Arrange
	slack, client := startSlack(t, directoryBodies())
	slackDirectory, _ := directoryOver(client)
	_, _ = slackDirectory.Workspace(t.Context())

	// Act
	workspace, err := slackDirectory.Workspace(t.Context())

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
	slackDirectory, _ := directoryOver(client)
	_, _ = slackDirectory.Workspace(t.Context())

	slackDirectory.Refresh()

	// Act
	_, _ = slackDirectory.Workspace(t.Context())

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
	slackDirectory, _ := directoryOver(client)
	_, _ = slackDirectory.Workspace(t.Context())

	// Act
	workspace, err := slackDirectory.Workspace(t.Context())

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
	slackDirectory := directory.New(settings.current, time.Now)

	// Act
	_, err := slackDirectory.Workspace(t.Context())

	// Assert
	if !errors.Is(err, messaging.ErrNoCredential) {
		t.Errorf("Workspace with no user token = %v, want ErrNoCredential", err)
	}
}
