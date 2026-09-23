// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package webserver_test

import (
	"net/http"
	"strings"
	"testing"

	"github.com/jacob-delgado/workflow/internal/api"
	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/gitrepo"
	"github.com/jacob-delgado/workflow/internal/jira"
	"github.com/jacob-delgado/workflow/internal/webserver"
)

// openFollowable opens a pull request from a branch naming testKey, with the
// tracker's writes wired, and returns what the open offers next.
func openFollowable(t *testing.T, deps webserver.Deps, cfg config.Config) (api.OpenedPullRequest, string) {
	t.Helper()

	recorder := send(t, serve(t, deps, cfg), http.MethodPost, "/api/pull-request", openRequestBody)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d (%s), want the pull request opened", recorder.Code, recorder.Body.String())
	}

	return decode[api.OpenedPullRequest](t, recorder), recorder.Body.String()
}

// followableDeps can open a pull request, and has the tracker's writes wired.
func followableDeps() webserver.Deps {
	deps := openableDeps()
	written := writableDeps(new([]linkCall), new([]jira.Transition))
	deps.LinkPullRequest = written.LinkPullRequest
	deps.Transitions = written.Transitions
	deps.Transition = written.Transition

	return deps
}

// offered names each follow-up as action, key and status, for comparing.
func offered(opened api.OpenedPullRequest) []string {
	names := make([]string, 0, len(opened.FollowUps))
	for _, offer := range opened.FollowUps {
		name := string(offer.Action) + " " + offer.IssueKey
		if offer.Status != nil {
			name += " to " + *offer.Status
		}

		names = append(names, name)
	}

	return names
}

func TestOpenPullRequestOffersTheLinkThenTheMove(t *testing.T) {
	t.Parallel()

	// Arrange
	deps := followableDeps()

	// Act
	opened, _ := openFollowable(t, deps, reviewConfig())

	// Assert
	want := "link " + testKey + ", transition " + testKey + " to " + reviewStatus
	if got := strings.Join(offered(opened), ", "); got != want {
		t.Errorf("follow_ups = %q, want %q", got, want)
	}
}

func TestOpenPullRequestOffersOnlyWhatApplies(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		unwire func(*webserver.Deps)
		cfg    config.Config
		want   string
	}{
		"no review status configured": {
			unwire: func(*webserver.Deps) {}, cfg: config.Default(), want: "link " + testKey,
		},
		// The pull request is open either way; a tracker that cannot be read
		// offers no move rather than failing the open.
		"the moves cannot be read": {
			unwire: func(deps *webserver.Deps) {
				deps.Transitions = func(jira.Key) ([]jira.Transition, error) { return nil, errSeam }
			},
			cfg: reviewConfig(), want: "link " + testKey,
		},
		"no way to move": {
			unwire: func(deps *webserver.Deps) { deps.Transition = nil }, cfg: reviewConfig(), want: "link " + testKey,
		},
		"a tracker that takes no link": {
			unwire: func(deps *webserver.Deps) { deps.LinkPullRequest = nil },
			cfg:    reviewConfig(), want: "transition " + testKey + " to " + reviewStatus,
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			deps := followableDeps()
			tt.unwire(&deps)

			// Act
			opened, _ := openFollowable(t, deps, tt.cfg)

			// Assert
			if got := strings.Join(offered(opened), ", "); got != tt.want {
				t.Errorf("follow_ups = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestOpenPullRequestOffersNothingForABranchNamingNoJiraIssue(t *testing.T) {
	t.Parallel()

	for _, name := range []string{"fix/typo", "42-fix-typo"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			deps := followableDeps()
			deps.Branch = func() (gitrepo.Branch, error) {
				branch := pushedBranch()
				branch.Name = name

				return branch, nil
			}

			// Act
			_, body := openFollowable(t, deps, reviewConfig())

			// Assert
			// An empty list, never null: the page's validator refuses a null.
			if !strings.Contains(body, `"follow_ups":[]`) {
				t.Errorf("body = %s, want follow_ups as an empty list", body)
			}
		})
	}
}
