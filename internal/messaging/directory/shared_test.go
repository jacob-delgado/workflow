// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package directory_test

// A directory read is shared by everyone who asks while it runs, so it runs to
// its end for those still waiting even when the one who began it leaves.

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/jacob-delgado/workflow/internal/messaging"
	"github.com/jacob-delgado/workflow/internal/messaging/directory"
)

// startSlackHere is startSlack answering in this process instead of over a
// socket. A request whose context has ended by the time Slack answers fails
// with it, as a real client's does, but with no connection between the two
// to race that end against the answer.
func startSlackHere(bodies map[string]string) (*fakeSlack, messaging.Client) {
	slack := newFakeSlack(bodies)

	answerHere := func(request *http.Request) (*http.Response, error) {
		answer := httptest.NewRecorder()
		slack.answer(answer, request)

		err := request.Context().Err()
		if err != nil {
			return nil, fmt.Errorf("asking Slack: %w", err)
		}

		return answer.Result(), nil
	}

	return slack, slackClient(answerHere, "http://slack.test")
}

func TestASharedReadAnswersThoseStillWaitingWhenTheOneWhoBeganItLeaves(t *testing.T) {
	t.Parallel()

	// Arrange
	// The second asker finds the first's read in flight, and the first leaves
	// before Slack answers.
	slack, client := startSlackHere(directoryBodies())
	arrived, release := slack.hold(t, "/usergroups.list")
	clock := newWatchedClock()
	slackDirectory := directory.New(func() (messaging.Client, error) { return client, nil }, clock.now)

	first, leave := context.WithCancel(t.Context())
	t.Cleanup(leave)

	var (
		askers sync.WaitGroup
		groups []messaging.SlackTarget
		err    error
	)

	askers.Go(func() { _, _ = slackDirectory.UserGroups(first) })
	<-arrived
	askers.Go(func() { groups, err = slackDirectory.UserGroups(t.Context()) })

	select {
	case <-clock.read:
	case <-time.After(joinWait):
		t.Fatal("the second asker never found the read in flight")
	}

	// Act
	leave()
	release()
	askers.Wait()

	// Assert
	want := []messaging.SlackTarget{{ID: "S0CP", Label: "control-plane-pod"}}
	if err != nil || !slices.Equal(groups, want) {
		t.Errorf("UserGroups = %v, %v; want %v, not the first asker's leaving", groups, err, want)
	}

	if asked := slack.count("/usergroups.list"); asked != 1 {
		t.Errorf("usergroups.list was asked %d times, want the one read shared", asked)
	}
}
