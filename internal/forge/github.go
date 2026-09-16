// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package forge

import (
	"context"
	"net/http"
	"net/url"
	"strings"
)

// githubPull is a pull request as GitHub sends one.
type githubPull struct {
	Number int    `json:"number"`
	URL    string `json:"html_url"`
	Title  string `json:"title"`
	Draft  bool   `json:"draft"`
}

// pullRequest flattens a GitHub pull request.
func (g githubPull) pullRequest() PullRequest {
	return PullRequest(g)
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

	return pulls[0].pullRequest(), true, nil
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

// githubCombined is a commit's combined status: the statuses reported through
// the older statuses API.
type githubCombined struct {
	Statuses []struct {
		State string `json:"state"`
	} `json:"statuses"`
}

// githubRuns is a commit's check runs: what GitHub Actions and most apps report.
type githubRuns struct {
	Runs []struct {
		Status     string `json:"status"`
		Conclusion string `json:"conclusion"`
	} `json:"check_runs"`
}

// githubStatus reads both of GitHub's CI reports for a commit. A repository can
// use either or both, and neither alone is the whole picture.
func githubStatus(ctx context.Context, client Client, repo Repo, _ PullRequest, head string) (CI, error) {
	commit := githubRepoPath(repo) + "/commits/" + url.PathEscape(head)

	combined, err := repoCall[githubCombined](ctx, client, repo, http.MethodGet, commit+"/status", nil)
	if err != nil {
		return CI{}, err
	}

	runs, err := repoCall[githubRuns](ctx, client, repo, http.MethodGet, commit+"/check-runs?per_page=100", nil)
	if err != nil {
		return CI{}, err
	}

	var tally ciTally

	// The combined status's own "state" is deliberately ignored: with no
	// statuses at all it says "pending", which would wait forever for CI that
	// is never coming. Only the statuses themselves count.
	for _, status := range combined.Statuses {
		tally.status(status.State)
	}

	for _, run := range runs.Runs {
		tally.run(run.Status, run.Conclusion)
	}

	return tally.ci(), nil
}
