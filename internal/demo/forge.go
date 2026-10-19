// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package demo

import (
	"context"
	"time"

	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/seams"
)

// The repository on GitHub that api's origin names.
const (
	forgeRepository = "acme/api"
	// toolRepository is the workflow tool's own repository, where the review
	// queue mostly waits.
	toolRepository = "acme/workflow"
	toolURL        = "https://" + githubHost + "/" + toolRepository
)

// forgeDeps is the demo's GitHub.
func (w *World) forgeDeps() seams.Forge {
	return seams.Forge{
		FindPullRequest: w.findPullRequest,
		ReviewRequests:  reviewRequests,
		Author:          func(context.Context) (string, error) { return author, nil },
		Kind:            forge.KindGitHub,
		Activity:        w.forgeActivity,
	}
}

// findPullRequest is the pull request opened for a branch, if one was.
func (w *World) findPullRequest(_ context.Context, branch string) (forge.PullRequest, bool, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	opened, found := w.pulls[branch]

	return opened.pull, found, nil
}

// reviewRequests are the pull requests that ask for your review, oldest
// first, as the web mock lists them (mockReviews.ts).
func reviewRequests(context.Context) ([]forge.ReviewRequest, error) {
	const day = 24 * time.Hour

	return []forge.ReviewRequest{
		{
			Number: 377, URL: toolURL + "/pull/377", Author: "sam.ortiz",
			Title: "Retry the forge search once on a gateway timeout", Repository: toolRepository,
			CI: forge.CIFailed, OpenedAt: epoch().Add(-9 * day),
		},
		{
			Number: 88, URL: "https://github.com/acme/build-images/pull/88", Author: author,
			Title: "Pin the Go toolchain in the build image", Repository: "acme/build-images",
			CI: forge.CIPassed, OpenedAt: epoch().Add(-50 * time.Hour),
		},
		{
			Number: 391, URL: toolURL + "/pull/391", Author: "lee.chen",
			Title: "List the review queue on the web", Repository: toolRepository, Draft: true,
			CI: forge.CIRunning, OpenedAt: epoch().Add(-3 * time.Hour),
		},
		{
			Number: 12, URL: "https://github.com/acme/handbook/pull/12", Author: "mia.okafor",
			Title: "Document the release checklist", Repository: "acme/handbook",
			CI: forge.CINone, OpenedAt: epoch().Add(-30 * time.Minute),
		},
	}, nil
}
