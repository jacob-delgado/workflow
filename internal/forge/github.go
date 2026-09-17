// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package forge

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

// githubPull is a pull request as GitHub sends one.
type githubPull struct {
	Number int    `json:"number"`
	URL    string `json:"html_url"`
	Title  string `json:"title"`
	Draft  bool   `json:"draft"`
}

// pullRequest flattens a GitHub pull request. Review state is filled in
// separately, so it stays zero here.
func (g githubPull) pullRequest() PullRequest {
	return PullRequest{Number: g.Number, URL: g.URL, Title: g.Title, Draft: g.Draft}
}

// githubNewPull is the body that opens a pull request on GitHub.
type githubNewPull struct {
	Title string `json:"title"`
	Body  string `json:"body"`
	Head  string `json:"head"`
	Base  string `json:"base"`
	Draft bool   `json:"draft"`
}

// githubRepoPath is where a repository lives in GitHub's API.
func githubRepoPath(repo Repo) string {
	return "/repos/" + escapedPath(repo.Path)
}

// githubFind finds the open pull request from a branch. GitHub filters by head
// as owner:branch; the branch alone matches nothing.
func githubFind(ctx context.Context, client Client, repo Repo, branch string) (PullRequest, bool, error) {
	owner, _, _ := strings.Cut(repo.Path, "/")
	query := url.Values{"head": {owner + ":" + branch}, "state": {"open"}}.Encode()

	pulls, err := repoCall[[]githubPull](ctx, client, repo, http.MethodGet, githubRepoPath(repo)+"/pulls?"+query, nil)
	if err != nil || len(pulls) == 0 {
		return PullRequest{}, false, err
	}

	pull := pulls[0].pullRequest()
	githubReviewState(ctx, client, repo, &pull)

	return pull, true, nil
}

// githubDetail is the single-pull-request read, for the mergeable flag the list
// does not carry. GitHub returns null while it is still working the merge out.
type githubDetail struct {
	Mergeable *bool `json:"mergeable"`
}

// githubReview is one review on a pull request: whose it is and where it stands.
type githubReview struct {
	State string                 `json:"state"`
	User  struct{ Login string } `json:"user"`
}

// githubReviewState fills in a pull request's approvals, requested changes and
// mergeability. It is best effort: a call the token cannot make leaves the
// fields as they are rather than failing the whole find.
func githubReviewState(ctx context.Context, client Client, repo Repo, pull *PullRequest) {
	base := githubRepoPath(repo) + "/pulls/" + strconv.Itoa(pull.Number)

	detail, err := repoCall[githubDetail](ctx, client, repo, http.MethodGet, base, nil)
	if err == nil {
		pull.Mergeable = mergeability(detail.Mergeable)
	}

	reviews, err := repoCall[[]githubReview](ctx, client, repo, http.MethodGet, base+"/reviews", nil)
	if err == nil {
		pull.Approvals, pull.ChangesRequested = tallyReviews(reviews)
	}
}

// mergeability reads GitHub's tri-state mergeable flag: null means still being
// worked out, not that it cannot merge.
func mergeability(mergeable *bool) Mergeability {
	switch {
	case mergeable == nil:
		return MergeUnknown
	case *mergeable:
		return MergeClean
	default:
		return MergeConflicts
	}
}

// tallyReviews counts approvals and whether changes are requested from each
// reviewer's latest stance. A comment does not change a stance; a dismissal
// clears it.
func tallyReviews(reviews []githubReview) (int, bool) {
	latest := make(map[string]string, len(reviews))

	for _, review := range reviews {
		switch review.State {
		case "APPROVED", "CHANGES_REQUESTED":
			latest[review.User.Login] = review.State
		case "DISMISSED":
			delete(latest, review.User.Login)
		}
	}

	approvals, changes := 0, false

	for _, state := range latest {
		if state == "APPROVED" {
			approvals++
		} else {
			changes = true
		}
	}

	return approvals, changes
}

// githubCreate opens a pull request on GitHub.
func githubCreate(ctx context.Context, client Client, repo Repo, request NewPullRequest) (PullRequest, error) {
	payload := githubNewPull(request)

	created, err := repoCall[githubPull](ctx, client, repo, http.MethodPost, githubRepoPath(repo)+"/pulls", payload)
	if err != nil {
		return PullRequest{}, err
	}

	return created.pullRequest(), nil
}

// githubCombined is a page of a commit's combined status: the statuses reported
// through the older statuses API, and how many there are in all.
type githubCombined struct {
	TotalCount int `json:"total_count"`
	Statuses   []struct {
		State string `json:"state"`
	} `json:"statuses"`
}

// githubRuns is a page of a commit's check runs: what GitHub Actions and most
// apps report, and how many there are in all.
type githubRuns struct {
	TotalCount int `json:"total_count"`
	Runs       []struct {
		Status     string `json:"status"`
		Conclusion string `json:"conclusion"`
	} `json:"check_runs"`
}

// GitHub paging: a full page, and a bound far above any real commit's check
// count, so a listing is read to the end without an unbounded loop.
const (
	githubPerPage  = 100
	githubMaxPages = 20
)

// githubStatus reads both of GitHub's CI reports for a commit. A repository can
// use either or both, and neither alone is the whole picture. Both are paged, so
// a failing status or run past the first page is not missed and reported as a
// pass; a listing too long to read in full is reported as still running rather
// than as passed.
func githubStatus(ctx context.Context, client Client, repo Repo, _ PullRequest, head string) (CI, error) {
	commit := githubRepoPath(repo) + "/commits/" + url.PathEscape(head)

	var tally ciTally

	statuses, statusesComplete, err := githubPages(ctx, client, repo, commit+"/status",
		func(page githubCombined) (int, int) { return len(page.Statuses), page.TotalCount })
	if err != nil {
		return CI{}, err
	}

	// The combined status's own "state" is deliberately ignored: with no
	// statuses at all it says "pending", which would wait forever for CI that
	// is never coming. Only the statuses themselves count.
	for _, page := range statuses {
		for _, status := range page.Statuses {
			tally.status(status.State)
		}
	}

	runs, runsComplete, err := githubPages(ctx, client, repo, commit+"/check-runs",
		func(page githubRuns) (int, int) { return len(page.Runs), page.TotalCount })
	if err != nil {
		return CI{}, err
	}

	for _, page := range runs {
		for _, run := range page.Runs {
			tally.run(run.Status, run.Conclusion)
		}
	}

	if !statusesComplete || !runsComplete {
		tally.running = true
	}

	return tally.ci(), nil
}

// githubPages reads a paged listing to its end, reporting whether every page was
// read within the bound. counts returns a page's size and GitHub's total_count.
func githubPages[T any](
	ctx context.Context, client Client, repo Repo, base string, counts func(T) (int, int),
) ([]T, bool, error) {
	var pages []T

	read := 0

	for page := 1; page <= githubMaxPages; page++ {
		path := fmt.Sprintf("%s?per_page=%d&page=%d", base, githubPerPage, page)

		one, err := repoCall[T](ctx, client, repo, http.MethodGet, path, nil)
		if err != nil {
			return nil, false, err
		}

		pages = append(pages, one)

		size, total := counts(one)
		read += size

		if size == 0 || read >= total {
			return pages, read >= total, nil
		}
	}

	return pages, false, nil
}
