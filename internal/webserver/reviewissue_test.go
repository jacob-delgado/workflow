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
	deps.Branch = func() (gitrepo.Branch, error) {
		return gitrepo.Branch{Name: unnamedBranch, IssueLink: link}, nil
	}
	deps.FindPull = func(string) (forge.PullRequest, bool, error) {
		return forge.PullRequest{Number: 9, Title: "Speed up search", Body: body}, true, nil
	}
	deps.BrowseURL = func(key jira.Key) string { return "https://tracker.example/" + string(key) }

	return deps
}

func TestTheReviewNamesTheIssueTheBranchWasLinkedTo(t *testing.T) {
	t.Parallel()

	// Act
	review := decode[api.Review](t, get(t, serve(t, reviewOn("PROJ-7", ""), config.Default()), "/api/review"))

	// Assert
	issue := review.Issue
	if issue == nil || issue.Key != "PROJ-7" || issue.Tracker != api.Jira ||
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
	if issue == nil || issue.Key != "42" || issue.Tracker != api.Forge ||
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
	deps.CheckCI = func(forge.PullRequest, string) (forge.CI, error) {
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
	deps.CheckCI = func(forge.PullRequest, string) (forge.CI, error) {
		return forge.CI{State: forge.CIFailed, Checks: []forge.Check{
			{ID: "501", Name: "unit-race", State: forge.CIFailed, LogAvailable: true},
		}}, nil
	}
	deps.JobLog = func(check forge.Check) (forge.JobLog, error) {
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
	deps.FindPull = func(string) (forge.PullRequest, bool, error) {
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
			deps.JobLog = func(forge.Check) (forge.JobLog, error) { return forge.JobLog{}, cause }

			// Act
			answer := get(t, serve(t, deps, config.Default()), "/api/review/checks/501/log")

			// Assert
			if answer.Code != http.StatusBadGateway {
				t.Errorf("status %d, want 502 for %v", answer.Code, cause)
			}
		})
	}
}
