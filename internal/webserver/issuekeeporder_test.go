// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package webserver_test

// The store keeps the newest list a search found, whichever frame's write to
// it ends last.

import (
	"context"
	"net/http"
	"sync"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

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

		held, tracker := webtest.NewHeld(), &movingTracker{status: inProgress}
		holdFirst := heldOnCall(1, held)
		deps := formDeps(&issueWrites{})
		deps.Jira.Transition, deps.Jira.Search = tracker.transition, tracker.search
		deps.Store.CacheIssues = func(ctx context.Context, _ string, issues []jira.Issue, _ time.Time) error {
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

func TestADropDuringAStoreWriteStartsNoSecondWriter(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		// Arrange
		// The first frame's keep is held, and a status change then drops the
		// list, so the next frame searches again while that keep is under way.
		var searches, keeps atomic.Int32

		held, tracker := webtest.NewHeld(), &movingTracker{status: inProgress}
		holdFirst := heldOnCall(1, held)
		deps := formDeps(&issueWrites{})
		deps.Jira.Transition = tracker.transition
		deps.Jira.Search = func(ctx context.Context, jql string, startAt int) (jira.SearchResult, error) {
			searches.Add(1)

			return tracker.search(ctx, jql, startAt)
		}
		deps.Store.CacheIssues = func(ctx context.Context, _ string, _ []jira.Issue, _ time.Time) error {
			keeps.Add(1)

			return holdFirst(ctx)
		}
		handler := webtest.ServeWith(t, deps, config.Default(),
			webserver.Info{Version: webtest.TestVersion, StreamInterval: streamTick})
		webtest.OpenStream(t, handler, "/api/events")

		synctest.Wait()
		webtest.Send(t, handler, http.MethodPost, statusChangesPath, noFieldsChange)
		t.Cleanup(func() { close(held.Release) })

		// Act
		webtest.OpenStream(t, handler, "/api/events")
		synctest.Wait()

		// Assert
		if got := searches.Load(); got != 2 {
			t.Fatalf("the tracker was searched %d times, want twice: once by each frame, around the drop", got)
		}

		if got := keeps.Load(); got != 1 {
			t.Errorf("the store was asked to keep the list %d times while the first keep was under way, want once: "+
				"the frame keeping it keeps the newer page after its own", got)
		}
	})
}
