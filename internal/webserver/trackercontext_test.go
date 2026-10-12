// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package webserver_test

import (
	"context"
	"errors"
	"net/http"
	"slices"
	"sync/atomic"
	"testing"

	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/jira"
	"github.com/jacob-delgado/workflow/internal/seams"
	"github.com/jacob-delgado/workflow/internal/webserver"
)

// The tests here hold the context each kind of request hands the tracker: a
// read's ends with its page, and a search every stream shares runs on when the
// page whose frame began it leaves.

// heldFirst is a search whose first call is held by held, under the context
// it is asked under, before it answers as answer does; every later call
// answers at once.
func heldFirst(
	held *heldCall, answer func(context.Context, string, int) (jira.SearchResult, error),
) func(context.Context, string, int) (jira.SearchResult, error) {
	var began atomic.Bool

	return func(ctx context.Context, jql string, startAt int) (jira.SearchResult, error) {
		if began.CompareAndSwap(false, true) {
			err := held.wait(ctx)
			if err != nil {
				return jira.SearchResult{}, err
			}
		}

		return answer(ctx, jql, startAt)
	}
}

// leftMidRead opens a stream on handler, and closes it once held is entered,
// letting held go only after: the frame's page has left while its read was
// under way.
func leftMidRead(t *testing.T, handler http.Handler, held *heldCall) {
	t.Helper()

	stream := openStream(t, handler, "/api/events")

	<-held.entered
	stream.cancel()
	close(held.release)
	<-stream.done
}

func TestAnIssueReadStopsWhenItsPageLeaves(t *testing.T) {
	t.Parallel()

	// Arrange
	held := newHeldUntilEnded()
	tracker := seams.Jira{Issue: func(ctx context.Context, _ jira.Key) (jira.IssueDetail, error) {
		return jira.IssueDetail{}, held.wait(ctx)
	}}
	ctx, leave := context.WithCancel(t.Context())
	answered := asking(ctx, serve(t, webserver.Deps{Jira: tracker}, config.Default()), http.MethodGet,
		"/api/issues/"+testKey)

	<-held.entered

	// Act
	leave()

	// Assert
	err := held.endedWith(t)
	if !errors.Is(err, context.Canceled) {
		t.Errorf("the read's context ended with %v, want it canceled once its page left", err)
	}

	<-answered
}

// Every stream shares the search of a view, and holds one that failed for the
// forge's interval: a search canceled because the page that began it left
// would be held as failed for every other page.
func TestASharedIssueSearchGoesOnWhenThePageThatBeganItLeaves(t *testing.T) {
	t.Parallel()

	// Arrange
	held, deps := newHeld(), filledDeps()
	deps.Jira.Search = heldFirst(held, deps.Jira.Search)
	handler := serve(t, deps, config.Default())
	leftMidRead(t, handler, held)

	// Act
	frame := firstSnapshot(t, streamOnce(t, handler, "/api/events").Body.String())

	// Assert
	if frame.Issues.Total != 1 || frame.Problems != nil {
		t.Errorf("the next page's issues = %+v, problems %+v; want the search's answer, though the page "+
			"that began it left mid-search", frame.Issues, frame.Problems)
	}
}

// Every stream shares the ask of which issues are yours, and keeps the last
// answer for a minute when one fails: an ask canceled because the page that
// began it left would count every issue as yours on every other page.
func TestASharedAskForYourIssuesGoesOnWhenThePageThatBeganItLeaves(t *testing.T) {
	t.Parallel()

	// Arrange
	var asked atomic.Int32

	held, deps := newHeld(), scopedDeps([]jira.Key{yourIssue}, nil, &asked)
	deps.Jira.SearchLenient = heldFirst(held, deps.Jira.SearchLenient)
	handler := serve(t, deps, scopedConfig())
	leftMidRead(t, handler, held)

	// Act
	frame := firstSnapshot(t, streamOnce(t, handler, "/api/events").Body.String())

	// Assert
	want := []string{yourLocalBranch}
	if got := listed(frame.Branches); !slices.Equal(got, want) {
		t.Errorf("the next page's branches = %v, want only yours, %v, though the page that asked left "+
			"mid-ask", got, want)
	}
}
