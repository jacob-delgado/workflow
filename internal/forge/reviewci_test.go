// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package forge_test

import (
	"testing"

	"github.com/jacob-delgado/workflow/internal/forge"
)

// The GitHub paths a review request's CI is read from: the pull request, for
// the commit its branch is at, and that commit's statuses and check runs.
const (
	reviewPullPath     = "/repos/ex/repo/pulls/42"
	reviewStatusPath   = "/repos/ex/repo/commits/c0ffee/status"
	reviewCheckRunPath = "/repos/ex/repo/commits/c0ffee/check-runs"
)

// reviewPull is pull request #42, its branch at commit c0ffee.
const reviewPull = `{"number":42,"head":{"sha":"c0ffee"}}`

// notJSON is an answer no forge reading JSON can read.
const notJSON = "not json"

// reviewSearch is GitHub's search finding the one pull request #42 in ex/repo.
const reviewSearch = `{"total_count":1,"items":[{"number":42,"html_url":"https://github.com/ex/repo/pull/42",` +
	`"title":"fix: token","created_at":"2026-09-15T08:00:00Z","user":{"login":"ana"},` +
	`"repository_url":"https://api.github.com/repos/ex/repo"}]}`

// checkRunsConcluding is a commit's one completed check run, concluding as
// conclusion.
func checkRunsConcluding(conclusion string) string {
	return `{"total_count":1,"check_runs":[{"id":901,"status":"completed","conclusion":"` + conclusion +
		`","name":"unit","html_url":"https://ci/901"}]}`
}

func TestGitHubReviewRequestsCarryWhereTheirCIStands(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		routes map[string]string
		want   forge.CIState
	}{
		"a check run failed": {
			routes: map[string]string{
				reviewPullPath: reviewPull, reviewStatusPath: `{"total_count":0}`,
				reviewCheckRunPath: checkRunsConcluding("failure"),
			},
			want: forge.CIFailed,
		},
		"every check passed": {
			routes: map[string]string{
				reviewPullPath: reviewPull, reviewStatusPath: `{"total_count":0}`,
				reviewCheckRunPath: checkRunsConcluding("success"),
			},
			want: forge.CIPassed,
		},
		"the pull request could not be read": {
			routes: map[string]string{reviewPullPath: notJSON},
			want:   forge.CINone,
		},
		"its checks could not be read": {
			routes: map[string]string{reviewPullPath: reviewPull, reviewStatusPath: notJSON},
			want:   forge.CINone,
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			tt.routes[githubSearchPath] = reviewSearch
			client, _ := recordingForge(t, routing(tt.routes))

			// Act
			queue, err := client.ReviewRequests(t.Context(), forge.KindGitHub)

			// Assert
			if err != nil || len(queue.Requests) != 1 || queue.Requests[0].CI != tt.want {
				t.Errorf("ReviewRequests = %+v, %v; want the one request with CI %s", queue, err, tt.want.Word())
			}
		})
	}
}
