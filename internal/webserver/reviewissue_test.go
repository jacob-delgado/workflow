// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package webserver_test

import (
	"fmt"
	"net/http"
	"reflect"
	"testing"

	"github.com/jacob-delgado/workflow/internal/api"
	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/gitrepo"
	"github.com/jacob-delgado/workflow/internal/jira"
	"github.com/jacob-delgado/workflow/internal/webserver"
)

// unnamedBranch is a branch begun outside workflow, named for no issue.
const unnamedBranch = "my-thing"

// reviewOn is filledDeps on a branch begun outside workflow, linked to link,
// with a pull request whose description is body.
func reviewOn(link, body string) webserver.Deps {
	deps := filledDeps()
	deps.Git.Branch = func() (gitrepo.Branch, error) {
		return gitrepo.Branch{Name: unnamedBranch, IssueLink: link}, nil
	}
	deps.Forge.FindPullRequest = func(string) (forge.PullRequest, bool, error) {
		return forge.PullRequest{Number: 9, Title: "Speed up search", Body: body}, true, nil
	}
	deps.Jira.BrowseURL = func(key jira.Key) string { return "https://tracker.example/" + string(key) }

	return deps
}

func TestTheReviewNamesTheIssueTheBranchWasLinkedTo(t *testing.T) {
	t.Parallel()

	// Act
	review := decode[api.Review](t, get(t, serve(t, reviewOn("PROJ-7", ""), config.Default()), "/api/review"))

	// Assert
	issue := review.Issue
	if issue == nil || issue.Key != "PROJ-7" || issue.Tracker != api.IssueTrackerJira ||
		issue.URL != "https://tracker.example/PROJ-7" || issue.Origin != api.LinkedIssueOriginByHand {
		t.Errorf("issue = %+v, want PROJ-7 from the link, with its page", issue)
	}
}

func TestTheReviewFindsTheIssueThePullRequestNames(t *testing.T) {
	t.Parallel()

	// Act
	review := decode[api.Review](t, get(t, serve(t, reviewOn("", "Speeds it up.\n\nCloses #42"), config.Default()),
		"/api/review"))

	// Assert
	issue := review.Issue
	if issue == nil || issue.Key != "42" || issue.Tracker != api.IssueTrackerForge ||
		issue.Origin != api.LinkedIssueOriginPullRequest {
		t.Errorf("issue = %+v, want the forge's 42 from the pull request", issue)
	}
}

func TestAReviewWithNoIssueNamesNone(t *testing.T) {
	t.Parallel()

	// Act
	review := decode[api.Review](t, get(t, serve(t, reviewOn("", "Speeds it up."), config.Default()), "/api/review"))

	// Assert
	if review.Issue != nil {
		t.Errorf("issue = %+v, want none", review.Issue)
	}
}

func TestTheReviewSaysWhyAFailedCheckFailed(t *testing.T) {
	t.Parallel()

	// Arrange
	deps := filledDeps()
	deps.Forge.CheckStatus = func(forge.PullRequest, string) (forge.CI, error) {
		return forge.CI{State: forge.CIFailed, Checks: []forge.Check{{
			ID: "501", Name: "unit-race", Stage: "test", Reason: "script failure", State: forge.CIFailed,
			LogAvailable: true,
		}}}, nil
	}

	// Act
	review := decode[api.Review](t, get(t, serve(t, deps, config.Default()), "/api/review"))

	// Assert
	if review.Ci == nil || len(review.Ci.Checks) != 1 {
		t.Fatalf("ci = %+v, want the one failed check", review.Ci)
	}

	check := review.Ci.Checks[0]
	got := []any{deref(check.ID), deref(check.Stage), deref(check.Reason), deref(check.LogAvailable)}

	if want := []any{"501", "test", "script failure", true}; !reflect.DeepEqual(got, want) {
		t.Errorf("check's id, stage, reason and log = %v, want %v", got, want)
	}
}

// deref is what a pointer the wire may leave out points at, or the zero value.
func deref[T any](value *T) T {
	if value == nil {
		var zero T

		return zero
	}

	return *value
}

// failingDeps is filledDeps with one failed check, 501, whose log the forge
// keeps, recording each log read.
func failingDeps(read *[]string) webserver.Deps {
	deps := filledDeps()
	deps.Forge.CheckStatus = func(forge.PullRequest, string) (forge.CI, error) {
		return forge.CI{State: forge.CIFailed, Checks: []forge.Check{
			{ID: "501", Name: "unit-race", State: forge.CIFailed, LogAvailable: true},
		}}, nil
	}
	deps.Forge.JobLog = func(check forge.Check) (forge.JobLog, error) {
		*read = append(*read, check.ID)

		return forge.JobLog{Text: "--- FAIL: TestRetry", Truncated: true}, nil
	}

	return deps
}

func TestACheckLogIsReadForACheckOfThePullRequest(t *testing.T) {
	t.Parallel()

	// Arrange
	var read []string

	// Act
	log := decode[api.JobLog](t, get(t, serve(t, failingDeps(&read), config.Default()), "/api/review/checks/501/log"))

	// Assert
	if log.Text != "--- FAIL: TestRetry" || !log.Truncated || len(read) != 1 {
		t.Errorf("log = %+v after reading %v, want 501's log, read once", log, read)
	}
}

func TestACheckLogIsNotReadForAnIDThePullRequestDoesNotList(t *testing.T) {
	t.Parallel()

	// Arrange
	var read []string

	// Act
	answer := get(t, serve(t, failingDeps(&read), config.Default()), "/api/review/checks/999/log")

	// Assert
	if answer.Code != http.StatusNotFound || len(read) != 0 {
		t.Errorf("status %d after reading %v, want 404 and nothing read", answer.Code, read)
	}
}

func TestACheckLogWhoseForgeCannotBeReachedIsNotANotFound(t *testing.T) {
	t.Parallel()

	// Arrange
	var read []string

	deps := failingDeps(&read)
	deps.Forge.FindPullRequest = func(string) (forge.PullRequest, bool, error) {
		return forge.PullRequest{}, false, fmt.Errorf("finding the pull request: %w", forge.ErrUnreachable)
	}

	// Act
	answer := get(t, serve(t, deps, config.Default()), "/api/review/checks/501/log")

	// Assert
	if answer.Code != http.StatusBadGateway || len(read) != 0 {
		t.Errorf("status %d after reading %v, want 502 and nothing read", answer.Code, read)
	}
}

func TestACheckLogTheForgeCouldNotHandOverIsAnUpstreamFault(t *testing.T) {
	t.Parallel()

	cases := map[string]error{
		"sent to plain http":     forge.ErrInsecureLog,
		"redirected nowhere":     forge.ErrLogNotRedirected,
		"its storage refused it": fmt.Errorf("%w: status 403", forge.ErrLogStorage),
	}

	for name, cause := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			var read []string

			deps := failingDeps(&read)
			deps.Forge.JobLog = func(forge.Check) (forge.JobLog, error) { return forge.JobLog{}, cause }

			// Act
			answer := get(t, serve(t, deps, config.Default()), "/api/review/checks/501/log")

			// Assert
			if answer.Code != http.StatusBadGateway {
				t.Errorf("status %d, want 502 for %v", answer.Code, cause)
			}
		})
	}
}

func TestACheckLogTheForgeHasNoneOfIsUnprocessable(t *testing.T) {
	t.Parallel()

	cases := map[string]func(deps *webserver.Deps){
		"the check keeps no log": func(deps *webserver.Deps) {
			listed := deps.Forge.CheckStatus
			deps.Forge.CheckStatus = func(pull forge.PullRequest, head string) (forge.CI, error) {
				ci, err := listed(pull, head)
				for index := range ci.Checks {
					ci.Checks[index].LogAvailable = false
				}

				return ci, err
			}
		},
		"no way to read a log": func(deps *webserver.Deps) { deps.Forge.JobLog = nil },
	}

	for name, change := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			var read []string

			deps := failingDeps(&read)
			change(&deps)

			// Act
			answer := get(t, serve(t, deps, config.Default()), "/api/review/checks/501/log")

			// Assert
			if answer.Code != http.StatusUnprocessableEntity || len(read) != 0 {
				t.Errorf("status %d after reading %v, want 422 and nothing read", answer.Code, read)
			}
		})
	}
}

func TestACheckLogWhoseBranchOrCICannotBeReadIsNotANotFound(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		change func(deps *webserver.Deps)
		want   int
	}{
		"the branch checked out": {want: http.StatusInternalServerError, change: func(deps *webserver.Deps) {
			deps.Git.Branch = func() (gitrepo.Branch, error) { return gitrepo.Branch{}, errSeam }
		}},
		"the CI": {want: http.StatusBadGateway, change: func(deps *webserver.Deps) {
			deps.Forge.CheckStatus = func(forge.PullRequest, string) (forge.CI, error) {
				return forge.CI{}, fmt.Errorf("reading the checks: %w", forge.ErrUnreachable)
			}
		}},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			var read []string

			deps := failingDeps(&read)
			tt.change(&deps)

			// Act
			answer := get(t, serve(t, deps, config.Default()), "/api/review/checks/501/log")

			// Assert
			if answer.Code != tt.want || len(read) != 0 {
				t.Errorf("status %d after reading %v, want %d and nothing read", answer.Code, read, tt.want)
			}
		})
	}
}
