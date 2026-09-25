// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package forge

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
)

// githubCommitStatus is one status reported through the older statuses API.
type githubCommitStatus struct {
	State     string `json:"state"`
	Context   string `json:"context"`
	TargetURL string `json:"target_url"`
}

// githubCombined is a page of a commit's combined status: its statuses, and how
// many there are in all.
type githubCombined struct {
	TotalCount int                  `json:"total_count"`
	Statuses   []githubCommitStatus `json:"statuses"`
}

// githubCheckRun is one check run: what GitHub Actions and most apps report.
type githubCheckRun struct {
	Status     string `json:"status"`
	Conclusion string `json:"conclusion"`
	Name       string `json:"name"`
	HTMLURL    string `json:"html_url"`
}

// githubRuns is a page of a commit's check runs, and how many there are in all.
type githubRuns struct {
	TotalCount int              `json:"total_count"`
	Runs       []githubCheckRun `json:"check_runs"`
}

// githubStatus reads both of GitHub's CI reports for a commit. A repository can
// use either or both, and neither alone is the whole picture. Both are paged, so
// a failing status or run past the first page is not missed and reported as a
// pass; a listing too long to read in full is reported as still running rather
// than as passed.
func githubStatus(ctx context.Context, client Client, repo Repo, _ PullRequest, head string) (CI, error) {
	commit := githubRepoPath(repo) + "/commits/" + url.PathEscape(head)

	var tally ciTally

	statuses, statusesComplete, err := githubPages(ctx, client, repo, commit+"/status",
		func(page githubCombined) ([]githubCommitStatus, int) { return page.Statuses, page.TotalCount })
	if err != nil {
		return CI{}, err
	}

	// The combined status's own "state" is deliberately ignored: with no
	// statuses at all it says "pending", which would wait forever for CI that
	// is never coming. Only the statuses themselves count.
	for _, status := range statuses {
		tally.add(Check{Name: status.Context, State: statusState(status.State), URL: status.TargetURL})
	}

	runs, runsComplete, err := githubPages(ctx, client, repo, commit+"/check-runs",
		func(page githubRuns) ([]githubCheckRun, int) { return page.Runs, page.TotalCount })
	if err != nil {
		return CI{}, err
	}

	for _, run := range runs {
		tally.add(Check{Name: run.Name, State: runState(run.Status, run.Conclusion), URL: run.HTMLURL})
	}

	if !statusesComplete || !runsComplete {
		tally.running = true
	}

	return tally.ci(), nil
}

// githubRunList is a commit's workflow runs: each run's id and how it concluded,
// so the failed ones can be re-run.
type githubRunList struct {
	Runs []struct {
		ID         int64  `json:"id"`
		Conclusion string `json:"conclusion"`
	} `json:"workflow_runs"`
}

// githubRerun re-runs the failed jobs of every failed workflow run on the head
// commit, and reports whether any run was re-run. A failure that is a status
// reported by something other than Actions has no run to re-run, so nothing is,
// which is why the caller is told rather than left to assume one started.
func githubRerun(ctx context.Context, client Client, repo Repo, _ PullRequest, head string) (bool, error) {
	runs := fmt.Sprintf("%s/actions/runs?head_sha=%s&per_page=%d",
		githubRepoPath(repo), url.QueryEscape(head), perPage)

	list, err := repoCall[githubRunList](ctx, client, repo, http.MethodGet, runs, nil)
	if err != nil {
		return false, err
	}

	reran := false

	for _, run := range list.Runs {
		if !runFailed(run.Conclusion) {
			continue
		}

		rerun := fmt.Sprintf("%s/actions/runs/%d/rerun-failed-jobs", githubRepoPath(repo), run.ID)

		err = send(ctx, client, http.MethodPost, rerun, nil)
		if err != nil {
			return reran, err
		}

		reran = true
	}

	return reran, nil
}

// runFailed reports a completed workflow run that did not pass, by the same rule
// runState reads a status by, so every run the Review pane calls failed — a
// timed-out or canceled one included, not only a plain "failure" — is re-run.
func runFailed(conclusion string) bool {
	passing := map[string]bool{succeeded: true, "neutral": true, skipped: true}

	return conclusion != "" && !passing[conclusion]
}

// githubPages reads a GitHub listing to its end, and reports whether it read as
// many items as GitHub counts: a listing cut short by the bound, or by a page
// short of the count, is not the whole of it. items takes a page apart into its
// items and total_count.
func githubPages[P, T any](
	ctx context.Context, client Client, repo Repo, path string, items func(P) ([]T, int),
) ([]T, bool, error) {
	counted := 0

	listed, err := readPages(func(page int) ([]T, int, error) {
		answer, err := repoCall[P](ctx, client, repo, http.MethodGet, path+"?"+pageQuery(nil, page), nil)
		found, total := items(answer)
		counted = total

		return found, total, err
	})
	if err != nil {
		return nil, false, err
	}

	return listed, len(listed) >= counted, nil
}
