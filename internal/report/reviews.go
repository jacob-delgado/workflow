// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package report

import (
	"time"

	"github.com/jacob-delgado/workflow/internal/api"
	"github.com/jacob-delgado/workflow/internal/forge"
)

// ReviewRequests maps the requests waiting on your review onto the wire, an
// empty list rather than null when none waits: the requests GET /api/reviews
// answers, and what `workflow reviews --json` prints.
func ReviewRequests(requests []forge.ReviewRequest) []api.ReviewRequest {
	queue := make([]api.ReviewRequest, 0, len(requests))
	for _, request := range requests {
		queue = append(queue, api.ReviewRequest{
			Number: request.Number, URL: request.URL, Title: request.Title, Author: request.Author,
			Repository: request.Repository, Draft: request.Draft, Ci: api.CIState(request.CI.Word()),
			OpenedAt: openedAt(request.OpenedAt),
		})
	}

	return queue
}

// openedAt is when a request was opened, or nil where the forge did not say:
// the wire leaves such a time out, never sending the zero time.
func openedAt(moment time.Time) *time.Time {
	if moment.IsZero() {
		return nil
	}

	return &moment
}
