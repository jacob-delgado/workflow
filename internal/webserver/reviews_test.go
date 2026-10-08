// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package webserver_test

import (
	"fmt"
	"net/http"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/jacob-delgado/workflow/internal/api"
	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/httpx"
	"github.com/jacob-delgado/workflow/internal/webserver"
)

// reviewsPath is where the review queue is read.
const reviewsPath = "/api/reviews"

// queueDeps is the filled Deps with a review queue that answers requests, or
// fails with err.
func queueDeps(requests []forge.ReviewRequest, err error) webserver.Deps {
	deps := filledDeps()
	deps.ReviewRequests = func() ([]forge.ReviewRequest, error) { return requests, err }

	return deps
}

func TestListReviewsAnswersTheForgeQueue(t *testing.T) {
	t.Parallel()

	// Arrange
	// The forge answers newest first; the queue is worked oldest first, as
	// `workflow reviews` prints it and the terminal's Reviews pane lists it.
	opened := time.Date(2026, 9, 14, 9, 30, 0, 0, time.UTC)
	older := forge.ReviewRequest{
		Number: 42, URL: "https://github.com/ex/api/pull/42", Title: "fix: redact the token",
		Author: "ana", Repository: "ex/api", Draft: true, CI: forge.CIFailed, OpenedAt: opened,
	}
	newer := forge.ReviewRequest{
		Number: 7, URL: "https://github.com/ex/web/pull/7", Title: "feat: a reviews section",
		Author: "sam", CI: forge.CIPassed, OpenedAt: opened.Add(26 * time.Hour),
	}

	// Act
	recorder := get(t, serve(t, queueDeps([]forge.ReviewRequest{newer, older}, nil), config.Default()), reviewsPath)

	// Assert
	queue := decode[api.ReviewQueue](t, recorder)
	want := []api.ReviewRequest{
		{
			Number: 42, URL: older.URL, Title: older.Title, Author: "ana", Repository: "ex/api",
			Draft: true, Ci: api.CIStateFailed, OpenedAt: &opened,
		},
		{
			Number: 7, URL: newer.URL, Title: newer.Title, Author: "sam", Repository: "",
			Draft: false, Ci: api.CIStatePassed, OpenedAt: &newer.OpenedAt,
		},
	}

	if !queue.Available || !slices.EqualFunc(queue.Requests, want, sameRequest) {
		t.Errorf("queue = %+v, want available with %+v", queue, want)
	}
}

// sameRequest is whether two review requests on the wire match, but for their
// facets, comparing the times with Equal, since a decoded time carries no
// monotonic reading.
func sameRequest(got, want api.ReviewRequest) bool {
	sameTime := got.OpenedAt != nil && want.OpenedAt != nil && got.OpenedAt.Equal(*want.OpenedAt)
	got.OpenedAt, got.Facets, want.Facets = want.OpenedAt, nil, nil

	return sameTime && reflect.DeepEqual(got, want)
}

// facetLines is each facet as kind|value|label, in order.
func facetLines(facets []api.ReviewFacet) []string {
	lines := make([]string, 0, len(facets))
	for _, facet := range facets {
		lines = append(lines, string(facet.Kind)+"|"+facet.Value+"|"+facet.Label)
	}

	return lines
}

func TestListReviewsDescribesEachRequestsFacets(t *testing.T) {
	t.Parallel()

	// Arrange
	requests := []forge.ReviewRequest{
		{Number: 1, Author: "lee", Repository: "ex/tools", Draft: true, CI: forge.CIRunning},
		{Number: 2, Author: "kim", CI: forge.CINone},
	}

	// Act
	recorder := get(t, serve(t, queueDeps(requests, nil), config.Default()), reviewsPath)

	// Assert
	queue := decode[api.ReviewQueue](t, recorder)
	want := [][]string{
		{"repository|ex/tools|ex/tools", "ci|running|CI running", "draft|draft|draft", "author|lee|by lee"},
		{"repository||no repository", "ci|none|CI none", "draft|ready|ready", "author|kim|by kim"},
	}

	if len(queue.Requests) != len(want) {
		t.Fatalf("queue = %+v, want %d requests", queue.Requests, len(want))
	}

	for index, request := range queue.Requests {
		if got := facetLines(request.Facets); !slices.Equal(got, want[index]) {
			t.Errorf("request #%d facets = %q, want %q", request.Number, got, want[index])
		}
	}
}

func TestListReviewsShipsTheOrderTheFilterOffersItsValuesIn(t *testing.T) {
	t.Parallel()

	// Arrange
	requests := []forge.ReviewRequest{
		{Number: 1, Author: "ivy", Repository: "ex/web", CI: forge.CIRunning},
		{Number: 2, Author: "lee", Repository: "ex/lib", Draft: true, CI: forge.CIFailed},
	}

	// Act
	recorder := get(t, serve(t, queueDeps(requests, nil), config.Default()), reviewsPath)

	// Assert
	queue := decode[api.ReviewQueue](t, recorder)
	want := []string{
		"repository|ex/lib|ex/lib", "repository|ex/web|ex/web", "ci|failed|CI failed", "ci|passed|CI passed",
		"ci|running|CI running", "ci|none|CI none", "draft|draft|draft", "draft|ready|ready",
		"author|ivy|by ivy", "author|lee|by lee",
	}

	if got := facetLines(queue.FacetOrder); !slices.Equal(got, want) {
		t.Errorf("facet_order = %q, want %q", got, want)
	}
}

func TestListReviewsAnswersAnEmptyQueueAsAnEmptyList(t *testing.T) {
	t.Parallel()

	// Act
	recorder := get(t, serve(t, queueDeps(nil, nil), config.Default()), reviewsPath)

	// Assert
	body := recorder.Body.String()
	if recorder.Code != http.StatusOK || !strings.Contains(body, `"available":true`) ||
		!strings.Contains(body, `"requests":[]`) {
		t.Errorf("answer = %d %s, want an available queue with requests [], not null", recorder.Code, body)
	}
}

func TestListReviewsSaysThereIsNoForgeToAsk(t *testing.T) {
	t.Parallel()

	// No forge to ask is an answer about where the server runs, not a missing
	// resource: the queue is not available, and empty, rather than a 404.
	cases := map[string]func() ([]forge.ReviewRequest, error){
		noForgeSeam: nil,
		"outside a repository": func() ([]forge.ReviewRequest, error) {
			return nil, fmt.Errorf("reading origin: %w", forge.ErrNotARemote)
		},
		"an origin on no forge it can read": func() ([]forge.ReviewRequest, error) {
			return nil, fmt.Errorf("%s — set forge.kind and forge.host: %w", forgeHost, forge.ErrUnknownForge)
		},
	}

	for name, seam := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			deps := filledDeps()
			deps.ReviewRequests = seam

			// Act
			recorder := get(t, serve(t, deps, config.Default()), reviewsPath)

			// Assert
			body := recorder.Body.String()
			if recorder.Code != http.StatusOK || !strings.Contains(body, `"available":false`) ||
				!strings.Contains(body, `"requests":[]`) || !strings.Contains(body, `"facet_order":[]`) {
				t.Errorf("answer = %d %s, want an unavailable, empty queue with nothing to offer", recorder.Code, body)
			}

			if strings.Contains(body, forgeHost) {
				t.Errorf("body = %q, leaks the forge host", body)
			}
		})
	}
}

func TestListReviewsDetailOmitsTheForgeHost(t *testing.T) {
	t.Parallel()

	// Each way the forge's search can fail, carrying the host as a wrapped
	// error can: the answer is classified — a setting to fix, or a wait — and
	// never names the host.
	cases := map[string]struct {
		cause      error
		wantStatus int
		want       string
	}{
		"an unreachable forge": {
			cause:      fmt.Errorf("%w: https://%s/search/issues", forge.ErrUnreachable, forgeHost),
			wantStatus: http.StatusBadGateway, want: "could not be reached",
		},
		"a forge asking to wait": {
			cause:      fmt.Errorf("searching https://%s: %w", forgeHost, httpx.RateLimited(http.Header{})),
			wantStatus: http.StatusServiceUnavailable, want: waitAndTryAgain,
		},
		"a refusal of the token": {
			cause:      fmt.Errorf("searching https://%s: %w", forgeHost, forge.ErrRefused),
			wantStatus: http.StatusUnprocessableEntity, want: lacksAScope,
		},
		"a status the forge explained": {
			cause: fmt.Errorf("searching https://%s: %w", forgeHost,
				fmt.Errorf("%w: 500 Internal Server Error", forge.ErrRejected)),
			wantStatus: http.StatusBadGateway, want: undocumentedStatus,
		},
		"no token found": {
			cause:      fmt.Errorf("token for %s: %w", forgeHost, forge.ErrNoToken),
			wantStatus: http.StatusUnprocessableEntity, want: "no forge token was found",
		},
		"a token not accepted": {
			cause:      fmt.Errorf("searching https://%s: %w", forgeHost, forge.ErrUnauthorized),
			wantStatus: http.StatusUnprocessableEntity, want: "did not accept the token",
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Act
			recorder := get(t, serve(t, queueDeps(nil, tt.cause), config.Default()), reviewsPath)

			// Assert
			failure := decode[api.Problem](t, recorder)
			if recorder.Code != tt.wantStatus || !strings.Contains(failure.Detail, tt.want) {
				t.Errorf("status/detail = %d/%q, want %d saying %q", recorder.Code, failure.Detail, tt.wantStatus, tt.want)
			}

			if strings.Contains(recorder.Body.String(), forgeHost) {
				t.Errorf("body = %q, leaks the forge host", recorder.Body.String())
			}
		})
	}
}
