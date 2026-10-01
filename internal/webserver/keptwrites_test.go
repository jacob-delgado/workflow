// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package webserver_test

// A repository's groups are kept or cleared without Slack's directory, and a
// kept write and a clean never run at once.

import (
	"context"
	"fmt"
	"net/http"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jacob-delgado/workflow/internal/api"
	"github.com/jacob-delgado/workflow/internal/loop"
	"github.com/jacob-delgado/workflow/internal/messaging"
	"github.com/jacob-delgado/workflow/internal/store"
	"github.com/jacob-delgado/workflow/internal/webserver"
)

// overlapWait is how long a test lets a write that should wait show it does
// not.
const overlapWait = 50 * time.Millisecond

func TestSetRepoGroupsClearsWithoutASlackDirectory(t *testing.T) {
	t.Parallel()

	// Arrange
	var posted string

	fake := newFakeKept()
	fake.groupsErr = messaging.ErrNoCredential
	handler := keptOver(t, fake, slackWebhookConfig(), &posted)

	// Act
	recorder := send(t, handler, http.MethodPut, repoGroupsPath, `{"ids":[]}`)

	// Assert
	if recorder.Code != http.StatusOK || len(fake.repoGroups) != 0 {
		t.Errorf("status %d (%s), groups %+v; want 200 and none kept", recorder.Code, recorder.Body.String(), fake.repoGroups)
	}
}

func TestSetRepoGroupsKeepsASavedGroupTheDirectoryCannotList(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		listed []loop.SlackTarget
		failed error
	}{
		"no longer listed":  {listed: []loop.SlackTarget{podGroup()}, failed: nil},
		"scope missing":     {listed: nil, failed: &messaging.MissingScopeError{Needed: groupsRead}},
		"no user token now": {listed: nil, failed: messaging.ErrNoCredential},
	}

	for name, test := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			var posted string

			fake := newFakeKept()
			fake.groups, fake.groupsErr = test.listed, test.failed
			handler := keptOver(t, fake, slackUserConfig(), &posted)

			// Act
			recorder := send(t, handler, http.MethodPut, repoGroupsPath, `{"ids":["S0API"]}`)

			// Assert
			want := []loop.SlackTarget{apiReviews()}
			if recorder.Code != http.StatusOK || !reflect.DeepEqual(fake.repoGroups, want) {
				t.Errorf("status %d (%s), groups %+v; want 200 keeping %+v",
					recorder.Code, recorder.Body.String(), fake.repoGroups, want)
			}
		})
	}
}

func TestCleanLocalDataWaitsForAKeptWriteUnderWay(t *testing.T) {
	t.Parallel()

	// Arrange
	linking, release, cleaned := make(chan struct{}), make(chan struct{}), make(chan struct{})
	deps := filledDeps()
	newFakeKept().wire(&deps)
	deps.LinkOwner = func(string, *loop.SlackTarget) error {
		close(linking)
		<-release

		return nil
	}
	deps.LocalData = func(context.Context) (string, []store.DataFile, error) { return storeDir, nil, nil }
	deps.CleanLocalData = func(store.CleanScope) error {
		close(cleaned)

		return nil
	}
	handler := serveWith(t, deps, slackUserConfig(), webserver.Info{Version: testVersion})

	var requests sync.WaitGroup

	requests.Go(func() { send(t, handler, http.MethodPut, peoplePath, `{"owner":"ben","not_on_slack":true}`) })
	<-linking
	requests.Go(func() { send(t, handler, http.MethodDelete, "/api/local-data?scope=all", "") })

	// Act
	var overlapped bool

	select {
	case <-cleaned:
		overlapped = true
	case <-time.After(overlapWait):
	}

	close(release)

	// Assert
	if overlapped {
		t.Error("the clean ran while a kept write was under way")
	}

	requests.Wait()
}

func TestCleanLocalDataSaysAFileNotRemovedMayLeaveOthersGone(t *testing.T) {
	t.Parallel()

	// Arrange
	fake := newFakeStore()
	fake.failure = fmt.Errorf("%w: busy", store.ErrNotCleaned)
	handler := storeServer(t, fake, webserver.Info{Version: testVersion})

	// Act
	answer := send(t, handler, http.MethodDelete, "/api/local-data?scope=all", "")

	// Assert
	detail := decode[api.Problem](t, answer).Detail
	if strings.Contains(detail, "nothing was removed") || !strings.Contains(detail, "listing") {
		t.Errorf("detail %q, want it pointing at the listing, not promising nothing was removed", detail)
	}
}
