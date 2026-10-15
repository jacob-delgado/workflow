// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package webserver_test

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/jacob-delgado/workflow/internal/api"
	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/gitrepo"
	"github.com/jacob-delgado/workflow/internal/jira"
	"github.com/jacob-delgado/workflow/internal/webserver"
	"github.com/jacob-delgado/workflow/internal/webserver/webtest"
)

func TestLinkRecordsThePullOnTheIssue(t *testing.T) {
	t.Parallel()

	// Arrange
	var links []linkCall

	deps := writableDeps(&links, new([]jira.Transition))
	openPull, _, _ := deps.Forge.FindPullRequest(t.Context(), webtest.TestBranchName)

	// Act
	recorder := post(t, deps, config.Default(), linkPath)

	// Assert
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d (%s), want 200", recorder.Code, recorder.Body.String())
	}

	want := linkCall{issueKey: webtest.TestKey, url: openPull.URL, title: openPull.Title}
	if len(links) != 1 || links[0] != want {
		t.Errorf("links = %+v, want the branch's pull request 42 recorded on %s", links, webtest.TestKey)
	}

	if linked := webtest.Decode[api.PullRequest](t, recorder); linked.Number != 42 {
		t.Errorf("answer = %+v, want the linked pull request 42", linked)
	}
}

func TestLinkRefusesAKeyTheBranchDoesNotName(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		branch, key string
	}{
		"another issue":           {branch: webtest.TestBranchName, key: "PROJ-9"},
		"a branch naming nothing": {branch: "fix/typo", key: webtest.TestKey},
		// A forge issue number is not a Jira key: Jira would refuse it, or read
		// it as an unrelated issue's id.
		"a forge issue number": {branch: "42-fix-typo", key: "42"},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			var links []linkCall

			deps := writableDeps(&links, new([]jira.Transition))
			deps.Git.Branch = func(context.Context) (gitrepo.Branch, error) { return gitrepo.Branch{Name: tt.branch}, nil }

			// Act
			recorder := post(t, deps, config.Default(), "/api/issues/"+tt.key+"/link")

			// Assert
			if recorder.Code != http.StatusConflict || len(links) != 0 {
				t.Errorf("status = %d, links = %+v; want 409 and nothing linked", recorder.Code, links)
			}
		})
	}
}

func TestLinkRefusesABranchWithNoPullRequest(t *testing.T) {
	t.Parallel()

	// Arrange
	var links []linkCall

	deps := writableDeps(&links, new([]jira.Transition))
	deps.Forge.FindPullRequest = func(context.Context, string) (forge.PullRequest, bool, error) {
		return forge.PullRequest{}, false, nil
	}

	// Act
	recorder := post(t, deps, config.Default(), linkPath)

	// Assert
	if recorder.Code != http.StatusConflict || len(links) != 0 {
		t.Errorf("status = %d, links = %+v; want 409 and nothing linked", recorder.Code, links)
	}
}

func TestLinkIsUnavailableWithoutItsSeams(t *testing.T) {
	t.Parallel()

	cases := map[string]func(*webserver.Deps){
		"no link seam":       func(deps *webserver.Deps) { deps.Jira.LinkPullRequest = nil },
		webtest.NoBranchSeam: func(deps *webserver.Deps) { deps.Git.Branch = nil },
		webtest.NoForgeSeam:  func(deps *webserver.Deps) { deps.Forge.FindPullRequest = nil },
	}

	for name, unwire := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			deps := writableDeps(new([]linkCall), new([]jira.Transition))
			unwire(&deps)

			// Act
			recorder := post(t, deps, config.Default(), linkPath)

			// Assert
			if recorder.Code != http.StatusUnprocessableEntity {
				t.Errorf("status = %d, want 422 when linking is not available", recorder.Code)
			}
		})
	}
}

func TestLinkReportsWhatItCouldNotRead(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		unwire   func(*webserver.Deps)
		wantCode int
	}{
		"the branch cannot be read": {
			unwire: func(deps *webserver.Deps) {
				deps.Git.Branch = func(context.Context) (gitrepo.Branch, error) { return gitrepo.Branch{}, webtest.ErrSeam }
			},
			wantCode: http.StatusInternalServerError,
		},
		"the forge cannot be reached": {
			unwire: func(deps *webserver.Deps) {
				deps.Forge.FindPullRequest = func(context.Context, string) (forge.PullRequest, bool, error) {
					return forge.PullRequest{}, false, fmt.Errorf("%w: https://%s", forge.ErrUnreachable, forgeHost)
				}
			},
			wantCode: http.StatusBadGateway,
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			var links []linkCall

			deps := writableDeps(&links, new([]jira.Transition))
			tt.unwire(&deps)

			// Act
			recorder := post(t, deps, config.Default(), linkPath)

			// Assert
			if recorder.Code != tt.wantCode || len(links) != 0 {
				t.Errorf("status = %d, links = %+v; want %d and nothing linked", recorder.Code, links, tt.wantCode)
			}

			if strings.Contains(recorder.Body.String(), forgeHost) {
				t.Errorf("body = %q, leaks the forge host", recorder.Body.String())
			}
		})
	}
}

func TestTransitionMovesTheIssueToTheReviewStatus(t *testing.T) {
	t.Parallel()

	// Arrange
	var moves []jira.Transition

	deps := writableDeps(new([]linkCall), &moves)

	// Act
	recorder := post(t, deps, reviewConfig(), movePath)

	// Assert
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d (%s), want 200", recorder.Code, recorder.Body.String())
	}

	if len(moves) != 1 || moves[0].ID != "21" {
		t.Errorf("moves = %+v, want the one move to %s", moves, reviewStatus)
	}

	if moved := webtest.Decode[api.MovedIssue](t, recorder); moved.Key != webtest.TestKey || moved.Status != reviewStatus {
		t.Errorf("answer = %+v, want %s now in %s", moved, webtest.TestKey, reviewStatus)
	}
}

func TestTransitionRefusesAFormTransition(t *testing.T) {
	t.Parallel()

	// Arrange
	// Jira wants a resolution filled to move there, which a fields-less move
	// cannot give; the move belongs to the terminal's status picker.
	var moves []jira.Transition

	deps := writableDeps(new([]linkCall), &moves)
	deps.Jira.Transitions = func(context.Context, jira.Key) ([]jira.Transition, error) {
		return []jira.Transition{{
			ID: "21", ToStatus: reviewStatus, Fields: []jira.Field{{ID: "resolution", Name: "Resolution"}},
		}}, nil
	}

	// Act
	recorder := post(t, deps, reviewConfig(), movePath)

	// Assert
	failure := webtest.Decode[api.Problem](t, recorder)
	if recorder.Code != http.StatusConflict || failure.Code != api.ProblemCodeConflict || len(moves) != 0 {
		t.Errorf("status/code = %d/%s, moves = %+v; want 409/conflict and nothing moved", recorder.Code, failure.Code, moves)
	}

	if !strings.Contains(failure.Detail, "Resolution") {
		t.Errorf("detail = %q, want it to name the field the move asks for", failure.Detail)
	}
}

func TestTransitionRefusesAMoveJiraDoesNotOffer(t *testing.T) {
	t.Parallel()

	// Arrange
	var moves []jira.Transition

	deps := writableDeps(new([]linkCall), &moves)
	deps.Jira.Transitions = func(context.Context, jira.Key) ([]jira.Transition, error) {
		return []jira.Transition{{ID: "31", ToStatus: "Done"}}, nil
	}

	// Act
	recorder := post(t, deps, reviewConfig(), movePath)

	// Assert
	if recorder.Code != http.StatusConflict || len(moves) != 0 {
		t.Errorf("status = %d, moves = %+v; want 409 and nothing moved", recorder.Code, moves)
	}

	if failure := webtest.Decode[api.Problem](t, recorder); !strings.Contains(failure.Detail, "Done") {
		t.Errorf("detail = %q, want it to name the status Jira does offer", failure.Detail)
	}
}

func TestTransitionNeedsAReviewStatusAndATracker(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		unwire func(*webserver.Deps)
		cfg    config.Config
	}{
		"no review status configured": {unwire: func(*webserver.Deps) {}, cfg: config.Default()},
		"no transitions seam": {
			unwire: func(deps *webserver.Deps) { deps.Jira.Transitions = nil }, cfg: reviewConfig(),
		},
		"no transition seam": {unwire: func(deps *webserver.Deps) { deps.Jira.Transition = nil }, cfg: reviewConfig()},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			var moves []jira.Transition

			deps := writableDeps(new([]linkCall), &moves)
			tt.unwire(&deps)

			// Act
			recorder := post(t, deps, tt.cfg, movePath)

			// Assert
			if recorder.Code != http.StatusUnprocessableEntity || len(moves) != 0 {
				t.Errorf("status = %d, moves = %+v; want 422 and nothing moved", recorder.Code, moves)
			}
		})
	}
}

func TestDryRunRefusesTheIssueWrites(t *testing.T) {
	t.Parallel()

	// The guard refuses by method, so it covers both new writes without either
	// handler knowing about dry run; neither seam is reached.
	for _, path := range []string{linkPath, movePath} {
		t.Run(path, func(t *testing.T) {
			t.Parallel()

			// Arrange
			var (
				links []linkCall
				moves []jira.Transition
			)

			dryRun := webserver.Info{Version: webtest.TestVersion, DryRun: true}
			handler := webtest.ServeWith(t, writableDeps(&links, &moves), reviewConfig(), dryRun)

			// Act
			recorder := webtest.Send(t, handler, http.MethodPost, path, "")

			// Assert
			if recorder.Code != http.StatusForbidden || len(links) != 0 || len(moves) != 0 {
				t.Errorf("status = %d, links = %v, moves = %v; want 403 and nothing written", recorder.Code, links, moves)
			}
		})
	}
}

func TestUnreachableJiraDetailOmitsItsHost(t *testing.T) {
	t.Parallel()

	unreachable := fmt.Errorf("%w: https://%s/rest/api/2/issue", jira.ErrUnreachable, jiraHost)
	cases := map[string]struct {
		unwire func(*webserver.Deps)
		path   string
	}{
		"the link": {
			unwire: func(deps *webserver.Deps) {
				deps.Jira.LinkPullRequest = func(context.Context, jira.Key, string, string) error { return unreachable }
			},
			path: linkPath,
		},
		"reading the moves": {
			unwire: func(deps *webserver.Deps) {
				deps.Jira.Transitions = func(context.Context, jira.Key) ([]jira.Transition, error) { return nil, unreachable }
			},
			path: movePath,
		},
		"the move": {
			unwire: func(deps *webserver.Deps) {
				deps.Jira.Transition = func(context.Context, jira.Key, jira.Transition, []jira.FieldValue) error {
					return unreachable
				}
			},
			path: movePath,
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			deps := writableDeps(new([]linkCall), new([]jira.Transition))
			tt.unwire(&deps)

			// Act
			recorder := post(t, deps, reviewConfig(), tt.path)

			// Assert
			failure := webtest.Decode[api.Problem](t, recorder)
			if recorder.Code != http.StatusBadGateway || failure.Code != api.ProblemCodeUnreachable {
				t.Errorf("status/code = %d/%s, want 502/unreachable", recorder.Code, failure.Code)
			}

			if strings.Contains(recorder.Body.String(), jiraHost) {
				t.Errorf("body = %q, leaks the Jira host", recorder.Body.String())
			}
		})
	}
}
