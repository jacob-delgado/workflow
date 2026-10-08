// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

import (
	"testing"
	"time"

	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/gitrepo"
	"github.com/jacob-delgado/workflow/internal/jira"
	"github.com/jacob-delgado/workflow/internal/loop"
	"github.com/jacob-delgado/workflow/internal/taskwarrior"
	"github.com/jacob-delgado/workflow/internal/tui"
)

// heldLive is the world's interface, sized and loaded as live does, with one
// seam wrapped by hold: it answers until the hold is armed, and never after.
func heldLive(t *testing.T, held *hold, repo *world, wrap func(deps *tui.Deps, held *hold)) tui.Model {
	t.Helper()

	deps := repo.deps()
	wrap(&deps, held)
	model := sized(t, tui.New(repo.cfg, nil, deps), 120, 40)

	return drain(t, model, model.Init())
}

func TestARefreshMarksEveryPanesTitleInFlightUntilItsAnswerArrives(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		world      func() *world
		wrap       func(deps *tui.Deps, held *hold)
		pane, want string
	}{
		"refreshing Branch, pane 2": {
			world: newWorld, pane: "2", want: "Branch ◐",
			wrap: func(deps *tui.Deps, held *hold) {
				read := deps.Git.Branch
				deps.Git.Branch = func() (gitrepo.Branch, error) {
					held.wait()

					return read()
				}
			},
		},
		"refreshing Commits, pane 3": {
			world: newWorld, pane: "3", want: "Commits ◐",
			wrap: func(deps *tui.Deps, held *hold) {
				read := deps.Git.Changes
				deps.Git.Changes = func() ([]gitrepo.Change, error) {
					held.wait()

					return read()
				}
			},
		},
		"refreshing Review, pane 4": {
			world: newWorld, pane: "4", want: "Review ◐",
			wrap: func(deps *tui.Deps, held *hold) {
				check := deps.Forge.CheckStatus
				deps.Forge.CheckStatus = func(pull forge.PullRequest, head string) (forge.CI, error) {
					held.wait()

					return check(pull, head)
				}
			},
		},
		"refreshing Slack, pane 5": {
			world: newWorld, pane: "5", want: "Slack ◐",
			wrap: func(deps *tui.Deps, held *hold) {
				read := deps.Store.Announced
				deps.Store.Announced = func() []loop.Announced {
					held.wait()

					return read()
				}
			},
		},
		"refreshing Reviews, pane 6": {
			world: withReviews, pane: "6", want: "Reviews ◐",
			wrap: func(deps *tui.Deps, held *hold) {
				read := deps.Forge.ReviewRequests
				deps.Forge.ReviewRequests = func() ([]forge.ReviewRequest, error) {
					held.wait()

					return read()
				}
			},
		},
		"refreshing Tasks, pane 7": {
			world: withTasks, pane: "7", want: "Tasks ◐",
			wrap: func(deps *tui.Deps, held *hold) {
				read := deps.Tasks.Pending
				deps.Tasks.Pending = func() (taskwarrior.List, error) {
					held.wait()

					return read()
				}
			},
		},
		"refreshing Summary, pane 8": {
			world: summaryWorld, pane: "8", want: "Summary ◐",
			wrap: func(deps *tui.Deps, held *hold) {
				read := deps.Jira.Activity
				deps.Jira.Activity = func(start, end time.Time) (jira.Activity, error) {
					held.wait()

					return read(start, end)
				}
			},
		},
		"refreshing Repositories, pane 9": {
			world: reposWorld, pane: "9", want: "Repositories ◐",
			wrap: func(deps *tui.Deps, held *hold) {
				read := deps.Store.Favorites
				deps.Store.Favorites = func() ([]string, error) {
					held.wait()

					return read()
				}
			},
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			held := newHold(t)
			started := heldLive(t, held, tt.world(), tt.wrap)
			held.armed.Store(true)

			// Act
			view := holding(t, held, started, tt.pane, "r").View().Content

			// Assert
			requireScreen(t, view, tt.want)
		})
	}
}

func TestARefreshedPanesTitleLosesTheMarkOnceItsAnswerArrives(t *testing.T) {
	t.Parallel()

	// Arrange
	onReview := typing(t, newWorld().live(t, 120, 40), "4")

	// Act
	view := typing(t, onReview, "r").View().Content

	// Assert
	refuseScreen(t, view, "Review ◐")
}

func TestOpeningTheRepositoriesPaneSaysItsFirstReadIsUnderWay(t *testing.T) {
	t.Parallel()

	// Arrange
	held := newHold(t)
	started := heldLive(t, held, reposWorld(), func(deps *tui.Deps, held *hold) {
		read := deps.Store.Favorites
		deps.Store.Favorites = func() ([]string, error) {
			held.wait()

			return read()
		}
	})
	held.armed.Store(true)

	// Act
	view := holding(t, held, started, "9").View().Content

	// Assert
	refuseScreen(t, view, "favorites, read when opened")
	requireScreen(t, view, "◐ reading…")
}

// yesterdaysIssues is an issue list the store cached in an earlier session.
func yesterdaysIssues() []jira.Issue {
	return []jira.Issue{{Key: "OLD-1", Summary: "From the cache", StatusCategory: categoryNew}}
}

func TestASessionOpenedOnTheCacheMarksTheIssuesInFlightUntilTheSearchAnswers(t *testing.T) {
	t.Parallel()

	// Arrange
	starting := newWorld()
	starting.cachedIssues = yesterdaysIssues()

	// Act: open on the cached list, before Init's search has answered
	seeded := sized(t, tui.New(starting.cfg, nil, starting.deps()), 120, 40)

	// Assert: the cached list shows, marked as being read again
	requireScreen(t, seeded.View().Content, "OLD-1", "1 Issues ◐")

	// Act: Init's search answers
	searched := drain(t, seeded, seeded.Init())

	// Assert: the mark is gone
	refuseScreen(t, searched.View().Content, "1 Issues ◐")
}

func TestSwitchingToACachedViewMarksItInFlightUntilItsSearchAnswers(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := twoViewRepo()
	repo.cachedIssues = yesterdaysIssues()
	model := sized(t, tui.New(twoViewConfig(), nil, repo.deps()), 120, 40)
	started := drain(t, model, model.Init())

	// Act: switch to the second view, its search not yet answered
	switched, search := pressed(t, started, "v")

	// Assert: yesterday's list shows, marked as being read again
	requireScreen(t, switched.View().Content, "Sprint board ◐")

	// Act: the search answers
	searched := drain(t, switched, search)

	// Assert: the mark is gone
	refuseScreen(t, searched.View().Content, "Sprint board ◐")
}
