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
	"sync/atomic"
	"testing"
	"time"

	"github.com/jacob-delgado/workflow/internal/api"
	"github.com/jacob-delgado/workflow/internal/loop"
	"github.com/jacob-delgado/workflow/internal/messaging"
	"github.com/jacob-delgado/workflow/internal/store"
	"github.com/jacob-delgado/workflow/internal/webserver"
)

// keptWriteHeld is how long a fake kept write takes: long enough that a
// removal asked once it has begun arrives while it is under way.
const keptWriteHeld = 50 * time.Millisecond

func TestSetRepoGroupsIsRefusedWithoutASlackWorkspace(t *testing.T) {
	t.Parallel()

	// Arrange
	// A repository's groups are kept per Slack workspace, and a webhook names
	// none, so there is no workspace whose groups to change.
	var posted string

	fake := newFakeKept()
	fake.groupsErr = messaging.ErrNoCredential
	handler := keptOver(t, fake, slackWebhookConfig(), &posted)

	// Act
	recorder := send(t, handler, http.MethodPut, repoGroupsPath, `{"ids":[]}`)

	// Assert
	if recorder.Code != http.StatusUnprocessableEntity || len(fake.repoGroups) != 1 {
		t.Errorf("status %d (%s), groups %+v; want 422 and the group kept",
			recorder.Code, recorder.Body.String(), fake.repoGroups)
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

func TestRemoveLocalDataWaitsForAKeptWriteUnderWay(t *testing.T) {
	t.Parallel()

	// Arrange
	// The removal is asked once the kept write has begun, and notes whether
	// that write was still under way when it ran.
	var (
		writing    atomic.Bool
		overlapped atomic.Bool
	)

	linking := make(chan struct{})
	deps := filledDeps()
	newFakeKept().wire(&deps)
	deps.LinkOwner = func(string, loop.OwnerLink) error {
		writing.Store(true)
		close(linking)
		time.Sleep(keptWriteHeld)
		writing.Store(false)

		return nil
	}
	deps.LocalData = func(context.Context) (string, []store.DataFile, error) { return storeDir, nil, nil }
	deps.RemoveLocalData = func(store.CleanScope) error {
		overlapped.Store(writing.Load())

		return nil
	}
	handler := serveWith(t, deps, slackUserConfig(), webserver.Info{Version: testVersion})

	var write sync.WaitGroup

	write.Go(func() { send(t, handler, http.MethodPut, peoplePath, `{"owner":"ben","not_on_slack":true}`) })
	<-linking

	// Act
	removed := send(t, handler, http.MethodDelete, "/api/local-data?scope=all", "")

	write.Wait()

	// Assert
	if removed.Code != http.StatusOK || overlapped.Load() {
		t.Errorf("removal = %d, ran while a kept write was under way: %v; want 200, after it",
			removed.Code, overlapped.Load())
	}
}

func TestRemoveLocalDataSaysAFileNotRemovedMayLeaveOthersGone(t *testing.T) {
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
