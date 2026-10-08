// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package forge

import (
	"slices"
	"strings"
	"time"
)

// ReviewRequest is an open pull or merge request that asks for your review. It
// carries what a review queue is read for — who wants it, since when, and where
// CI stands — across whichever repositories on the forge requested you.
type ReviewRequest struct {
	Number int
	URL    string
	Title  string
	Author string
	// Repository is the owner/name (GitHub) or group/project (GitLab) the request
	// is in, since a review queue spans repositories.
	Repository string
	Draft      bool
	// CI is best effort: the forge's own summary where the listing carries one,
	// and CINone where it does not, since the queue is read without a follow-up
	// request per entry.
	CI       CIState
	OpenedAt time.Time
}

// OldestFirst is the review queue in the order it is worked through, the
// longest-waiting request first — the order every surface lists it in. Requests
// opened at the same moment keep the forge's order, and the forge's answer is
// left as it came.
func OldestFirst(requests []ReviewRequest) []ReviewRequest {
	queue := slices.Clone(requests)
	slices.SortStableFunc(queue, func(left, right ReviewRequest) int {
		return left.OpenedAt.Compare(right.OpenedAt)
	})

	return queue
}

// NewestFirst is the requests ordered with the one opened last first, for a
// look at what just arrived. Requests opened at the same moment keep the
// forge's order.
func NewestFirst(requests []ReviewRequest) []ReviewRequest {
	queue := slices.Clone(requests)
	slices.SortStableFunc(queue, func(left, right ReviewRequest) int {
		return right.OpenedAt.Compare(left.OpenedAt)
	})

	return queue
}

// ByRepository is the requests grouped by repository, in its name's order,
// and the longest-waiting first within each.
func ByRepository(requests []ReviewRequest) []ReviewRequest {
	queue := OldestFirst(requests)
	slices.SortStableFunc(queue, func(left, right ReviewRequest) int {
		return strings.Compare(left.Repository, right.Repository)
	})

	return queue
}
