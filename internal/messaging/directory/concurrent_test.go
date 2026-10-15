// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package directory_test

// Readers who ask while a read is under way share it, and a refresh neither
// waits for it nor keeps what it read.

import (
	"sync"
	"testing"
	"time"

	"github.com/jacob-delgado/workflow/internal/messaging"
	"github.com/jacob-delgado/workflow/internal/messaging/directory"
)

func TestConcurrentReadsOfAChannelShareOneSetOfRequests(t *testing.T) {
	t.Parallel()

	// Arrange
	// The second reader begins once the first is waiting on Slack, and the
	// read is let go once the second has found it in flight.
	slack, client := startSlack(t, directoryBodies())
	arrived, release := slack.hold(t, "/users.conversations")
	clock := newWatchedClock()
	slackDirectory := directory.New(func() (messaging.Client, error) { return client, nil }, clock.now)

	var readers sync.WaitGroup

	readers.Go(func() { _, _ = slackDirectory.ChannelMembers(t.Context(), "dev") })
	<-arrived
	readers.Go(func() { _, _ = slackDirectory.ChannelMembers(t.Context(), "dev") })

	select {
	case <-clock.read:
	case <-time.After(joinWait):
		t.Fatal("the second reader never found the read in flight")
	}

	// Act
	release()
	readers.Wait()

	// Assert
	if asked := slack.requests(); asked != 3 {
		t.Errorf("Slack was asked %d times, want one channel, one members and one users.list read", asked)
	}
}

func TestRefreshDoesNotWaitForASlowReadNorKeepItsResult(t *testing.T) {
	t.Parallel()

	// Arrange
	slack, client := startSlack(t, directoryBodies())
	arrived, release := slack.hold(t, "/users.list")
	slackDirectory, _ := directoryOver(client)

	var reader sync.WaitGroup

	reader.Go(func() { _, _ = slackDirectory.ChannelMembers(t.Context(), "dev") })

	<-arrived

	refreshed := make(chan struct{})

	// Act
	go func() { slackDirectory.Refresh(); close(refreshed) }()

	// Assert
	select {
	case <-refreshed:
	case <-time.After(5 * time.Second):
		t.Fatal("Refresh waited for the read in flight")
	}

	release()
	reader.Wait()

	_, _ = slackDirectory.ChannelMembers(t.Context(), "dev")

	if asked := slack.count("/users.list"); asked != 2 {
		t.Errorf("users.list was asked %d times, want the read begun before Refresh not kept", asked)
	}
}
