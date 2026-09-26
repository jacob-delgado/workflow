// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

import (
	"strconv"
	"strings"

	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/seams"
)

// requestPeople names the reviewers, assignees and labels a pull request
// carries, or "" when it carries none, so a test can assert on them without
// changing the recorded call for the pull requests that name nobody.
func requestPeople(request forge.NewPullRequest) string {
	var parts []string

	if len(request.Reviewers) > 0 {
		parts = append(parts, "reviewers="+strings.Join(request.Reviewers, ","))
	}

	if len(request.Assignees) > 0 {
		parts = append(parts, "assignees="+strings.Join(request.Assignees, ","))
	}

	if len(request.Labels) > 0 {
		parts = append(parts, "labels="+strings.Join(request.Labels, ","))
	}

	return strings.Join(parts, " ")
}

// nextCI is the next CI answer: each check takes the next, and the last one
// repeats.
func (w *world) nextCI() forge.CI {
	w.mu.Lock()
	defer w.mu.Unlock()

	answer := w.ci[0]
	if len(w.ci) > 1 {
		w.ci = w.ci[1:]
	}

	return answer
}

// forgeDeps fakes the forge.
func (w *world) forgeDeps() seams.Forge {
	return seams.Forge{
		FindPullRequest: func(branch string) (forge.PullRequest, bool, error) {
			w.record("find " + branch)

			return w.pull, w.pullFound, w.pullErr
		},
		CreatePullRequest: func(request forge.NewPullRequest) (forge.PullRequest, error) {
			call := "open " + request.Title + " " + request.Head + ">" + request.Base + " draft=" +
				map[bool]string{false: "no", true: "yes"}[request.Draft]
			if people := requestPeople(request); people != "" {
				call += " " + people
			}

			w.record(call + "\n" + request.Body)

			// A refused create returns no pull, the way the real forge does; a pull
			// whose reviewers could not be added returns the pull with the error.
			if w.openErr != nil {
				return forge.PullRequest{}, w.openErr
			}

			return w.pull, w.reviewerErr
		},
		EditPullRequest: func(pull forge.PullRequest, edit forge.PullRequestEdit) (forge.PullRequest, error) {
			w.record("edit " + strconv.Itoa(pull.Number) + " " + edit.Title + "\n" + edit.Body)

			if w.editPullErr != nil {
				return forge.PullRequest{}, w.editPullErr
			}

			updated := pull
			updated.Title, updated.Body = edit.Title, edit.Body

			return updated, nil
		},
		CheckStatus: func(_ forge.PullRequest, head string) (forge.CI, error) {
			w.record("ci " + head)

			return w.nextCI(), w.ciErr
		},
		Rerun: func(_ forge.PullRequest, head string) (bool, error) {
			w.record("rerun " + head)

			return !w.nothingToRerun, w.rerunErr
		},
		Merge: func(pull forge.PullRequest, method forge.MergeMethod) error {
			w.record("merge " + strconv.Itoa(pull.Number) + " " + string(method))

			return w.mergeErr
		},
		MergeMethods: func() ([]forge.MergeMethod, error) {
			w.record("merge-methods")

			if w.mergeMethods == nil && w.mergeMethodsErr == nil {
				return []forge.MergeMethod{forge.MergeCommit, forge.MergeSquash}, nil
			}

			return w.mergeMethods, w.mergeMethodsErr
		},
		ReviewRequests: func() ([]forge.ReviewRequest, error) {
			w.record("reviews")

			return w.reviews, w.reviewsErr
		},
		Templates: func() []forge.Template { return w.templates },
		Author:    func() (string, error) { return w.author, w.authorErr },
		Kind:      w.forgeKind,
	}
}
