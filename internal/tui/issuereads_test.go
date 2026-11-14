// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

// The Issues list is searched at start, on a refresh, and when its view is
// switched to, and a view switched to shows the list the store cached for
// it while its search is out. The list shows, and the store caches, what the
// search begun last found; the cache stands in for that search alone.

import (
	"context"
	"net/http"
	"slices"
	"sync/atomic"
	"testing"

	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/jira"
	"github.com/jacob-delgado/workflow/internal/tui"
)

// issueReads is the world's seams with each read of the cached list
// recorded as "cached <jql>", and every search of jql failing, unreachable,
// while failing holds.
func issueReads(repo *world, jql string, failing *atomic.Bool) tui.Deps {
	deps := repo.deps()
	search, cached := deps.Jira.Search, deps.Store.CachedIssues

	deps.Jira.Search = func(ctx context.Context, asked string, startAt int) (jira.SearchResult, error) {
		if failing.Load() && asked == jql {
			repo.record("search " + asked)

			return jira.SearchResult{Issues: nil, Total: 0}, jira.ErrUnreachable
		}

		return search(ctx, asked, startAt)
	}
	deps.Store.CachedIssues = func(ctx context.Context, asked string) ([]jira.Issue, bool) {
		repo.record("cached " + asked)

		return cached(ctx, asked)
	}

	return deps
}

// freshlyCached is an issue list the store cached since yesterday's.
func freshlyCached() []jira.Issue {
	return []jira.Issue{{Key: "NEW-1", Summary: "From the fresh cache", StatusCategory: categoryNew}}
}

func TestASearchASecondRefreshSupersededIsNotCachedLast(t *testing.T) {
	t.Parallel()

	// Arrange
	// The list of two is cached at start. A refresh searches, slowly, while
	// four issues are listed; a second refresh, with five listed, answers
	// first.
	repo := newWorld()
	started := repo.live(t, 120, 40)
	repo.issues = manyIssues(4)
	slow, late := withheldKeys(t, started, repo.counting("search "), "r")
	repo.issues = manyIssues(5)
	refreshed := typing(t, slow, "r")

	// Act
	handedOver(t, refreshed, late)

	// Assert
	if calls := repo.asked("cache"); !slices.Equal(calls, []string{"cache 2", "cache 5"}) {
		t.Errorf("cache calls = %q, want the two at start, then the five, and the four never", calls)
	}
}

func TestAViewSwitchedToShowsTheCacheItsLatestSwitchRead(t *testing.T) {
	t.Parallel()

	// Arrange
	// The sprint board is switched to, and its cache, yesterday's, is read
	// slowly; the view is left and switched to again, its search slow, by
	// which time the store holds a fresher cache, and yesterday's read
	// answers before the fresh one.
	repo := twoViewRepo()
	repo.cachedIssues = yesterdaysIssues()
	model := sized(t, tui.New(twoViewConfig(), nil, issueReads(repo, "", &atomic.Bool{})), 120, 40)
	switched, stale := withheldKeys(t, drain(t, model, model.Init()), repo.counting("cached "), "v")
	repo.cachedIssues = freshlyCached()
	again, reads := pressed(t, typing(t, switched, "v"), "v")
	stalled := handedOver(t, again, stale)

	// Act
	shown, _ := withholding(t, stalled, reads, repo.counting("search "))

	// Assert
	view := shown.View().Content
	requireScreen(t, view, "NEW-1", "Sprint board ◐")
	refuseScreen(t, view, "OLD-1")
}

func TestTheCacheAnsweringAfterTheSearchFailedIsListedBesideTheFailure(t *testing.T) {
	t.Parallel()

	// Arrange
	// The sprint board's search fails before its cache is read.
	repo := twoViewRepo()
	repo.cachedIssues = yesterdaysIssues()
	failing := &atomic.Bool{}

	failing.Store(true)

	model := sized(t, tui.New(twoViewConfig(), nil, issueReads(repo, scopedSprintJQL, failing)), 120, 40)
	switched, cached := withheldKeys(t, drain(t, model, model.Init()), repo.counting("cached "), "v")

	// Act
	view := handedOver(t, switched, cached).View().Content

	// Assert
	requireScreen(t, view, "OLD-1", "failed")
}

func TestAFailedRefreshKeepsTheSelectedIssueShown(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := newWorld()
	failing := &atomic.Bool{}
	model := sized(t, tui.New(repo.cfg, nil, issueReads(repo, jira.Views(repo.cfg.Jira.Views)[0].JQL, failing)), 120, 40)
	listed := drain(t, model, model.Init())

	failing.Store(true)

	// Act
	view := typing(t, listed, "r").View().Content

	// Assert
	requireScreen(t, view, "PROJ-412", "Tokens reach the log.", "failed")
}

func TestWithNoFileButIssuesListedTheSelectedIssueIsShown(t *testing.T) {
	t.Parallel()

	// Arrange
	// No .workflow.json is found and the setup is wired, yet the tracker
	// answers with issues.
	deps := newFirstRun(t, http.StatusOK).deps(t)
	deps.Jira = newWorld().deps().Jira
	model := sized(t, tui.New(config.Default(), config.ErrNotFound, deps), 120, 40)

	// Act
	view := drain(t, model, model.Init()).View().Content

	// Assert
	requireScreen(t, view, "Tokens reach the log.")
	refuseScreen(t, view, "sets one up here")
}
