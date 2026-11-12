// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package forge_test

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/httpx"
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

// noStatuses is a commit's statuses when nothing reported one.
const noStatuses = `{"total_count":0}`

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
				reviewPullPath: reviewPull, reviewStatusPath: noStatuses,
				reviewCheckRunPath: checkRunsConcluding("failure"),
			},
			want: forge.CIFailed,
		},
		"every check passed": {
			routes: map[string]string{
				reviewPullPath: reviewPull, reviewStatusPath: noStatuses,
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

// reviewSearchOf is GitHub's search finding count pull requests in ex/repo,
// numbered from one, each opened an hour after the next, so #1 is the newest.
func reviewSearchOf(count int) string {
	newest := time.Date(2026, 9, 15, 8, 0, 0, 0, time.UTC)
	items := make([]string, 0, count)

	for number := 1; number <= count; number++ {
		opened := newest.Add(-time.Duration(number) * time.Hour).Format(time.RFC3339)
		items = append(items, fmt.Sprintf(`{"number":%d,"html_url":"https://github.com/ex/repo/pull/%d",`+
			`"created_at":%q,"user":{"login":"ana"},"repository_url":"https://api.github.com/repos/ex/repo"}`,
			number, number, opened))
	}

	return fmt.Sprintf(`{"total_count":%d,"items":[%s]}`, count, strings.Join(items, ","))
}

// pullsRead counts the pull request reads among requests.
func pullsRead(requests []recorded) int {
	read := 0

	for _, request := range requests {
		if strings.HasPrefix(request.path, "/repos/ex/repo/pulls/") {
			read++
		}
	}

	return read
}

func TestGitHubReviewCIIsReadForTheHundredLongestWaiting(t *testing.T) {
	t.Parallel()

	// Arrange
	client, requests := recordingForge(t, func(asked recorded) (int, string) {
		switch {
		case asked.path == githubSearchPath:
			return http.StatusOK, reviewSearchOf(101)
		case strings.HasPrefix(asked.path, "/repos/ex/repo/pulls/"):
			return http.StatusOK, reviewPull
		case asked.path == reviewCheckRunPath:
			return http.StatusOK, checkRunsConcluding("success")
		default:
			return http.StatusOK, noStatuses
		}
	})

	// Act
	queue, err := client.ReviewRequests(t.Context(), forge.KindGitHub)

	// Assert
	if err != nil || len(queue.Requests) != 101 {
		t.Fatalf("ReviewRequests = %d requests, %v; want all 101", len(queue.Requests), err)
	}

	if read := pullsRead(*requests); read != 100 || queue.Requests[0].CI != forge.CINone ||
		queue.Requests[100].CI != forge.CIPassed {
		t.Errorf("read %d pull requests, #1 CI %s, #101 CI %s; want 100, the newest none, the oldest passed",
			read, queue.Requests[0].CI.Word(), queue.Requests[100].CI.Word())
	}
}

func TestGitHubReviewCIStopsAtTheFirstReadEveryReadWouldMeet(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		status int
		want   error
	}{
		"a rate limit":            {status: http.StatusTooManyRequests, want: httpx.ErrRateLimited},
		"a token no longer valid": {status: http.StatusUnauthorized, want: forge.ErrUnauthorized},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			client, requests := recordingForge(t, func(asked recorded) (int, string) {
				if asked.path == githubSearchPath {
					return http.StatusOK, reviewSearchOf(3)
				}

				return tt.status, `{"message":"no"}`
			})

			// Act
			_, err := client.ReviewRequests(t.Context(), forge.KindGitHub)

			// Assert
			if !errors.Is(err, tt.want) || pullsRead(*requests) != 1 {
				t.Errorf("ReviewRequests = %v after %d pull reads; want %v after one", err, pullsRead(*requests), tt.want)
			}
		})
	}
}
