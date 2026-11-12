// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package webserver_test

// The store keeps the newest list a search found, whichever frame's write to
// it ends last.

import (
	"context"
	"net/http"
	"sync"
	"testing"
	"testing/synctest"

	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/jira"
	"github.com/jacob-delgado/workflow/internal/webserver"
	"github.com/jacob-delgado/workflow/internal/webserver/webtest"
)

// lastKept is the issues the store holds for a view: those of the last list
// it was asked to keep.
type lastKept struct {
	mu     sync.Mutex
	issues []jira.Issue
}

func (l *lastKept) keep(issues []jira.Issue) {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.issues = issues
}

func (l *lastKept) held() []jira.Issue {
	l.mu.Lock()
	defer l.mu.Unlock()

	return l.issues
}

func TestTheStoreKeepsTheNewerSearchWhenTheOlderOnesKeepEndsLast(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		// Arrange
		// The first frame's search finds the issue in progress, and its keep is
		// held; a status change then drops the list, and a second page's frame
		// searches again and finds it blocked.
		var kept lastKept

		held, tracker := webtest.NewHeld(), &movingTracker{status: "In Progress"}
		holdFirst := heldOnCall(1, held)
		deps := formDeps(&issueWrites{})
		deps.Jira.Transition, deps.Jira.Search = tracker.transition, tracker.search
		deps.Store.CacheIssues = func(ctx context.Context, _ string, issues []jira.Issue) error {
			err := holdFirst(ctx)
			if err == nil {
				kept.keep(issues)
			}

			return err
		}
		handler := webtest.ServeWith(t, deps, config.Default(),
			webserver.Info{Version: webtest.TestVersion, StreamInterval: streamTick})
		first := webtest.OpenStream(t, handler, "/api/events")

		synctest.Wait()
		webtest.Send(t, handler, http.MethodPost, statusChangesPath, noFieldsChange)

		second := webtest.OpenStream(t, handler, "/api/events")

		synctest.Wait()

		// Act
		close(held.Release)
		synctest.Wait()

		// Assert
		first.Close()
		second.Close()

		if issues := kept.held(); len(issues) != 1 || issues[0].Status != blocked {
			t.Errorf("the store keeps %+v, want %s %s, as the newer search found it", issues, webtest.TestKey, blocked)
		}
	})
}
