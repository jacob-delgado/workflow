// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package webserver_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jacob-delgado/workflow/internal/api"
	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/gitrepo"
	"github.com/jacob-delgado/workflow/internal/jira"
	"github.com/jacob-delgado/workflow/internal/webserver"
)

// Fixtures the issue-write tests share.
const (
	reviewStatus = "In Review"
	// jiraHost is the tracker's address, which a Jira error can carry and no
	// answer may repeat.
	jiraHost = "jira.internal.example"
	// forgeHost is the forge's address, which a forge error can carry and no
	// answer may repeat either.
	forgeHost = "git.internal.example"
	linkPath  = "/api/issues/" + testKey + "/link"
	movePath  = "/api/issues/" + testKey + "/transition"
)

// linkCall is what a link seam was asked to record.
type linkCall struct {
	issueKey jira.Key
	url      string
	title    string
}

// writableDeps is filledDeps with the tracker's writes wired: the checked-out
// branch names testKey and has the pull request 42 open, and Jira offers a
// fields-less move to the review status. Each write records what it was asked.
func writableDeps(links *[]linkCall, moves *[]jira.Transition) webserver.Deps {
	deps := filledDeps()
	deps.LinkPullRequest = func(issueKey jira.Key, pullURL, title string) error {
		*links = append(*links, linkCall{issueKey: issueKey, url: pullURL, title: title})

		return nil
	}
	deps.Transitions = func(jira.Key) ([]jira.Transition, error) {
		return []jira.Transition{
			{ID: "11", ToStatus: "Blocked"},
			{ID: "21", ToStatus: reviewStatus},
		}, nil
	}
	deps.Transition = func(_ jira.Key, to jira.Transition, _ []jira.FieldValue) error {
		*moves = append(*moves, to)

		return nil
	}

	return deps
}

// reviewConfig is the default configuration with the review status set.
func reviewConfig() config.Config {
	cfg := config.Default()
	cfg.Jira.ReviewStatus = reviewStatus

	return cfg
}

// post sends a bodyless POST to path on a server over deps and cfg.
func post(t *testing.T, deps webserver.Deps, cfg config.Config, path string) *httptest.ResponseRecorder {
	t.Helper()

	return send(t, serve(t, deps, cfg), http.MethodPost, path, "")
}

func TestLinkRecordsThePullOnTheIssue(t *testing.T) {
	t.Parallel()

	// Arrange
	var links []linkCall

	deps := writableDeps(&links, new([]jira.Transition))
	openPull, _, _ := deps.FindPull(testBranchName)

	// Act
	recorder := post(t, deps, config.Default(), linkPath)

	// Assert
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d (%s), want 200", recorder.Code, recorder.Body.String())
	}

	want := linkCall{issueKey: testKey, url: openPull.URL, title: openPull.Title}
	if len(links) != 1 || links[0] != want {
		t.Errorf("links = %+v, want the branch's pull request 42 recorded on %s", links, testKey)
	}

	if linked := decode[api.PullRequest](t, recorder); linked.Number != 42 {
		t.Errorf("answer = %+v, want the linked pull request 42", linked)
	}
}

func TestLinkRefusesAKeyTheBranchDoesNotName(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		branch, key string
	}{
		"another issue":           {branch: testBranchName, key: "PROJ-9"},
		"a branch naming nothing": {branch: "fix/typo", key: testKey},
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
			deps.Branch = func() (gitrepo.Branch, error) { return gitrepo.Branch{Name: tt.branch}, nil }

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
	deps.FindPull = func(string) (forge.PullRequest, bool, error) { return forge.PullRequest{}, false, nil }

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
		"no link seam": func(deps *webserver.Deps) { deps.LinkPullRequest = nil },
		noBranchSeam:   func(deps *webserver.Deps) { deps.Branch = nil },
		noForgeSeam:    func(deps *webserver.Deps) { deps.FindPull = nil },
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
				deps.Branch = func() (gitrepo.Branch, error) { return gitrepo.Branch{}, errSeam }
			},
			wantCode: http.StatusInternalServerError,
		},
		"the forge cannot be reached": {
			unwire: func(deps *webserver.Deps) {
				deps.FindPull = func(string) (forge.PullRequest, bool, error) {
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

	if moved := decode[api.MovedIssue](t, recorder); moved.Key != testKey || moved.Status != reviewStatus {
		t.Errorf("answer = %+v, want %s now in %s", moved, testKey, reviewStatus)
	}
}

func TestTransitionRefusesAFormTransition(t *testing.T) {
	t.Parallel()

	// Arrange
	// Jira wants a resolution filled to move there, which a fields-less move
	// cannot give; the move belongs to the terminal's status picker.
	var moves []jira.Transition

	deps := writableDeps(new([]linkCall), &moves)
	deps.Transitions = func(jira.Key) ([]jira.Transition, error) {
		return []jira.Transition{{ID: "21", ToStatus: reviewStatus, Fields: []jira.Field{{ID: "resolution"}}}}, nil
	}

	// Act
	recorder := post(t, deps, reviewConfig(), movePath)

	// Assert
	failure := decode[api.Problem](t, recorder)
	if recorder.Code != http.StatusConflict || failure.Code != api.Conflict || len(moves) != 0 {
		t.Errorf("status/code = %d/%s, moves = %+v; want 409/conflict and nothing moved", recorder.Code, failure.Code, moves)
	}
}

func TestTransitionRefusesAMoveJiraDoesNotOffer(t *testing.T) {
	t.Parallel()

	// Arrange
	var moves []jira.Transition

	deps := writableDeps(new([]linkCall), &moves)
	deps.Transitions = func(jira.Key) ([]jira.Transition, error) {
		return []jira.Transition{{ID: "31", ToStatus: "Done"}}, nil
	}

	// Act
	recorder := post(t, deps, reviewConfig(), movePath)

	// Assert
	if recorder.Code != http.StatusConflict || len(moves) != 0 {
		t.Errorf("status = %d, moves = %+v; want 409 and nothing moved", recorder.Code, moves)
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
			unwire: func(deps *webserver.Deps) { deps.Transitions = nil }, cfg: reviewConfig(),
		},
		"no transition seam": {unwire: func(deps *webserver.Deps) { deps.Transition = nil }, cfg: reviewConfig()},
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

			dryRun := webserver.Info{Version: testVersion, DryRun: true}
			handler := serveWith(t, writableDeps(&links, &moves), reviewConfig(), dryRun)

			// Act
			recorder := send(t, handler, http.MethodPost, path, "")

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
				deps.LinkPullRequest = func(jira.Key, string, string) error { return unreachable }
			},
			path: linkPath,
		},
		"reading the moves": {
			unwire: func(deps *webserver.Deps) {
				deps.Transitions = func(jira.Key) ([]jira.Transition, error) { return nil, unreachable }
			},
			path: movePath,
		},
		"the move": {
			unwire: func(deps *webserver.Deps) {
				deps.Transition = func(jira.Key, jira.Transition, []jira.FieldValue) error { return unreachable }
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
			failure := decode[api.Problem](t, recorder)
			if recorder.Code != http.StatusBadGateway || failure.Code != api.Unreachable {
				t.Errorf("status/code = %d/%s, want 502/unreachable", recorder.Code, failure.Code)
			}

			if strings.Contains(recorder.Body.String(), jiraHost) {
				t.Errorf("body = %q, leaks the Jira host", recorder.Body.String())
			}
		})
	}
}
