// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package directory_test

// A channel's members are labeled from users.list, or in a workspace too large
// to list one by one, waiting as Slack asks, a minute in all at most, but never
// past the read's own end.

import (
	"context"
	"errors"
	"slices"
	"testing"
	"testing/synctest"
	"time"

	"github.com/jacob-delgado/workflow/internal/httpx"
)

func TestAChannelInAWorkspaceTooLargeToListIsLabeledOneByOne(t *testing.T) {
	t.Parallel()

	// Arrange
	slack, client := startSlack(t, tooLargeToList())
	slackDirectory, _ := directoryOver(client)

	// Act
	members, err := slackDirectory.ChannelMembers(t.Context(), "#dev")

	// Assert
	want := channelMembers()
	if err != nil || !slices.Equal(members, want) {
		t.Errorf("ChannelMembers = %v, %v; want %v", members, err, want)
	}

	if asked := slack.count("/users.info"); asked != 3 {
		t.Errorf("users.info was asked %d times, want once per member", asked)
	}
}

// tooLargeToList are the answers of a workspace past users.list's page cap,
// whose channel's members users.info labels one by one.
func tooLargeToList() map[string]string {
	bodies := directoryBodies()
	bodies["/users.list"] = `{"ok":true,"members":[],"response_metadata":{"next_cursor":"more"}}`
	bodies["/users.info?user=U0ADA"] = `{"ok":true,"user":{"id":"U0ADA","name":"ada","profile":{"display_name":"Ada"}}}`
	bodies["/users.info?user=U0BOB"] = `{"ok":true,"user":{"id":"U0BOB","name":"bob","real_name":"Bob B","profile":{}}}`
	bodies["/users.info?user=U0BOT"] = `{"ok":true,"user":{"id":"U0BOT","name":"robot","is_bot":true,"profile":{}}}`

	return bodies
}

func TestAMemberSlackAsksToWaitForIsLabeledOnceTheWaitIsOver(t *testing.T) {
	t.Parallel()

	// Arrange
	slack, client := startSlack(t, tooLargeToList())
	slack.limit("/users.info?user=U0BOB", "1")

	slackDirectory, _ := directoryOver(client)

	// Act
	members, err := slackDirectory.ChannelMembers(t.Context(), "#dev")

	// Assert
	want := channelMembers()
	if err != nil || !slices.Equal(members, want) {
		t.Errorf("ChannelMembers = %v, %v; want %v after waiting as Slack asked", members, err, want)
	}
}

func TestAWaitTooLongFailsTheReadAndKeepsWhoWasLabeled(t *testing.T) {
	t.Parallel()

	// Arrange
	slack, client := startSlack(t, tooLargeToList())
	slack.limit("/users.info?user=U0BOB", "3600")

	slackDirectory, _ := directoryOver(client)
	started := time.Now()

	// Act
	_, failed := slackDirectory.ChannelMembers(t.Context(), "#dev")
	members, err := slackDirectory.ChannelMembers(t.Context(), "#dev")

	// Assert
	if !errors.Is(failed, httpx.ErrRateLimited) || time.Since(started) > time.Minute {
		t.Errorf("the first read = %v after %s, want Slack's rate limit at once", failed, time.Since(started))
	}

	want := channelMembers()
	if err != nil || !slices.Equal(members, want) {
		t.Errorf("ChannelMembers = %v, %v; want %v", members, err, want)
	}

	if asked := slack.count("/users.info"); asked != 4 {
		t.Errorf("users.info was asked %d times, want Ada's label kept and Bob's asked again", asked)
	}
}

func TestAWaitForSlackEndsWithTheRead(t *testing.T) {
	t.Parallel()

	// Arrange
	slack, client := startSlack(t, tooLargeToList())
	slack.limit("/users.info?user=U0BOB", "30")

	slackDirectory, _ := directoryOver(client)
	ctx, cancel := context.WithTimeout(t.Context(), 100*time.Millisecond)
	t.Cleanup(cancel)

	// Act
	_, err := slackDirectory.ChannelMembers(ctx, "#dev")

	// Assert
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("ChannelMembers = %v, want the read's own deadline rather than the whole wait", err)
	}
}

func TestAChannelIsLabeledOnceSlacksWaitOnTheUserListIsOver(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		// Arrange
		slack := newFakeSlack(directoryBodies())
		slack.limit("/users.list", "2")

		slackDirectory, _ := directoryOver(slackClient(inProcess(slack.answer), slackBase))
		started := time.Now()

		// Act
		members, err := slackDirectory.ChannelMembers(t.Context(), "#dev")

		// Assert
		want := channelMembers()
		if err != nil || !slices.Equal(members, want) {
			t.Errorf("ChannelMembers = %v, %v; want %v after waiting as Slack asked", members, err, want)
		}

		if waited := time.Since(started); waited < 2*time.Second {
			t.Errorf("the read waited %s, want the two seconds Slack asked for", waited)
		}

		if asked := slack.count("/users.list"); asked != 2 {
			t.Errorf("users.list was asked %d times, want once more after the wait", asked)
		}
	})
}

func TestAWaitOnTheUserListTooLongFailsTheReadAtOnce(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		// Arrange
		slack := newFakeSlack(directoryBodies())
		slack.limit("/users.list", "3600")

		slackDirectory, _ := directoryOver(slackClient(inProcess(slack.answer), slackBase))
		started := time.Now()

		// Act
		_, err := slackDirectory.ChannelMembers(t.Context(), "#dev")

		// Assert
		if !errors.Is(err, httpx.ErrRateLimited) || time.Since(started) > 0 {
			t.Errorf("ChannelMembers = %v after %s, want Slack's rate limit at once", err, time.Since(started))
		}

		if asked := slack.count("/users.list"); asked != 1 {
			t.Errorf("users.list was asked %d times, want once", asked)
		}
	})
}

func TestTheWaitsOfOneLabelingAddUpToAMinuteAtMost(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		// Arrange
		// Forty seconds on users.list leaves twenty of the minute, too little
		// for the thirty users.info then asks.
		slack := newFakeSlack(tooLargeToList())
		slack.limit("/users.list", "40")
		slack.limit("/users.info?user=U0BOB", "30")

		slackDirectory, _ := directoryOver(slackClient(inProcess(slack.answer), slackBase))
		started := time.Now()

		// Act
		_, err := slackDirectory.ChannelMembers(t.Context(), "#dev")

		// Assert
		if !errors.Is(err, httpx.ErrRateLimited) {
			t.Errorf("ChannelMembers = %v, want Slack's rate limit once the minute is spent", err)
		}

		if waited := time.Since(started); waited != 40*time.Second {
			t.Errorf("the read waited %s, want the forty seconds on users.list alone", waited)
		}
	})
}

// slackBase is where an in-process Slack is asked: any address, since no
// request leaves the process.
const slackBase = "http://slack.invalid"
