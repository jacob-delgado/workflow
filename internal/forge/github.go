// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package forge

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
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
	query := url.Values{"head": {owner + ":" + branch}, queryState: {"open"}}.Encode()

	pulls, err := repoCall[[]githubPull](ctx, client, repo, http.MethodGet, githubRepoPath(repo)+"/pulls?"+query, nil)
	if err != nil || len(pulls) == 0 {
		return PullRequest{}, false, err
	}

	pull := pulls[0].pullRequest()
	githubReviewState(ctx, client, repo, &pull)

	return pull, true, nil
}

// githubSearchQuery finds the open pull requests that request the token owner's
// review. @me is GitHub's own name for whoever the token belongs to, so the
// user's own name is never needed.
const githubSearchQuery = "is:pr is:open review-requested:@me"

// githubReviewSearch is the search endpoint's answer: the matching issues, which
// for this query are all pull requests.
type githubReviewSearch struct {
	Items []githubReviewItem `json:"items"`
}

// githubReviewItem is one pull request as GitHub's search sends it.
type githubReviewItem struct {
	Number    int       `json:"number"`
	URL       string    `json:"html_url"`
	Title     string    `json:"title"`
	Draft     bool      `json:"draft"`
	CreatedAt time.Time `json:"created_at"`
	User      struct {
		Login string `json:"login"`
	} `json:"user"`
	RepositoryURL string `json:"repository_url"`
}

// reviewRequest flattens a search item. GitHub's search does not carry CI, so it
// is left unknown rather than fetched with a request per entry.
func (g githubReviewItem) reviewRequest() ReviewRequest {
	_, repository, _ := strings.Cut(g.RepositoryURL, "/repos/")

	return ReviewRequest{
		Number: g.Number, URL: g.URL, Title: g.Title, Draft: g.Draft,
		Author: g.User.Login, Repository: repository, CI: CINone, OpenedAt: g.CreatedAt,
	}
}

// githubIssues lists the open issues in the repository assigned to the token
// owner. It searches with a repo filter, so @me needs no username, and with
// is:issue so the pull requests that search also returns are left out.
func githubIssues(ctx context.Context, client Client, repo Repo) ([]Issue, error) {
	query := url.Values{
		"q":          {"is:issue is:open assignee:@me repo:" + repo.Path},
		perPageParam: {strconv.Itoa(githubPerPage)},
	}.Encode()

	found, err := call[githubIssueSearch](ctx, client, http.MethodGet, "/search/issues?"+query, nil)
	if err != nil {
		return nil, err
	}

	issues := make([]Issue, 0, len(found.Items))
	for _, item := range found.Items {
		issues = append(issues, item.issue())
	}

	return issues, nil
}

// githubIssueSearch is the search endpoint's answer, here all issues.
type githubIssueSearch struct {
	Items []githubIssue `json:"items"`
}

// githubIssue is an issue as GitHub sends it, from the search or a single read.
type githubIssue struct {
	Number int    `json:"number"`
	URL    string `json:"html_url"`
	Title  string `json:"title"`
	Body   string `json:"body"`
	User   struct {
		Login string `json:"login"`
	} `json:"user"`
}

func (g githubIssue) issue() Issue {
	return Issue{Number: g.Number, URL: g.URL, Title: g.Title}
}

func (g githubIssue) detail() IssueDetail {
	return IssueDetail{Issue: g.issue(), Body: g.Body, Author: g.User.Login}
}

// githubReadIssue reads one issue's body and author.
func githubReadIssue(ctx context.Context, client Client, repo Repo, number int) (IssueDetail, error) {
	read, err := repoCall[githubIssue](ctx, client, repo, http.MethodGet,
		githubRepoPath(repo)+issuesSegment+"/"+strconv.Itoa(number), nil)
	if err != nil {
		return IssueDetail{}, err
	}

	return read.detail(), nil
}

// githubIssueState is the PATCH body that closes an issue.
type githubIssueState struct {
	State string `json:"state"`
}

// githubCloseIssue closes an issue by setting its state to closed.
func githubCloseIssue(ctx context.Context, client Client, repo Repo, number int) error {
	_, err := repoCall[githubIssue](ctx, client, repo, http.MethodPatch,
		githubRepoPath(repo)+issuesSegment+"/"+strconv.Itoa(number), githubIssueState{State: "closed"})

	return err
}

// githubReviews lists the pull requests that request the token owner's review.
func githubReviews(ctx context.Context, client Client) ([]ReviewRequest, error) {
	query := url.Values{"q": {githubSearchQuery}, perPageParam: {strconv.Itoa(githubPerPage)}}.Encode()

	found, err := call[githubReviewSearch](ctx, client, http.MethodGet, "/search/issues?"+query, nil)
	if err != nil {
		return nil, err
	}

	reviews := make([]ReviewRequest, 0, len(found.Items))
	for _, item := range found.Items {
		reviews = append(reviews, item.reviewRequest())
	}

	return reviews, nil
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

// githubCreate opens a pull request on GitHub, then requests its reviewers and
// adds its assignees and labels. Those are separate requests GitHub only takes
// once the pull exists, so their failure is returned alongside the opened pull
// rather than losing it.
func githubCreate(ctx context.Context, client Client, repo Repo, request NewPullRequest) (PullRequest, error) {
	payload := githubNewPull{
		Title: request.Title, Body: request.Body, Head: request.Head, Base: request.Base, Draft: request.Draft,
	}

	created, err := repoCall[githubPull](ctx, client, repo, http.MethodPost, githubRepoPath(repo)+"/pulls", payload)
	if err != nil {
		return PullRequest{}, err
	}

	pull := created.pullRequest()

	return pull, githubAddPeople(ctx, client, repo, pull.Number, request)
}

// githubAddPeople requests reviewers and adds assignees and labels to a pull
// request already opened. GitHub names each list by the same key it reads it
// back under, and takes reviewers on the pull while assignees and labels go on
// its issue side.
func githubAddPeople(ctx context.Context, client Client, repo Repo, number int, request NewPullRequest) error {
	pull := githubRepoPath(repo) + "/pulls/" + strconv.Itoa(number)
	issue := githubRepoPath(repo) + issuesSegment + "/" + strconv.Itoa(number)

	err := githubPostList(ctx, client, repo, pull+"/requested_reviewers", "reviewers", request.Reviewers)
	if err != nil {
		return err
	}

	err = githubPostList(ctx, client, repo, issue+"/assignees", "assignees", request.Assignees)
	if err != nil {
		return err
	}

	return githubPostList(ctx, client, repo, issue+"/labels", "labels", request.Labels)
}

// githubPostList posts a named list to an endpoint, doing nothing when the list
// is empty so no needless request is made.
func githubPostList(ctx context.Context, client Client, repo Repo, path, key string, values []string) error {
	if len(values) == 0 {
		return nil
	}

	_, err := repoCall[json.RawMessage](ctx, client, repo, http.MethodPost, path, map[string][]string{key: values})

	return err
}

// githubCombined is a page of a commit's combined status: the statuses reported
// through the older statuses API, and how many there are in all.
type githubCombined struct {
	TotalCount int `json:"total_count"`
	Statuses   []struct {
		State     string `json:"state"`
		Context   string `json:"context"`
		TargetURL string `json:"target_url"`
	} `json:"statuses"`
}

// githubRuns is a page of a commit's check runs: what GitHub Actions and most
// apps report, and how many there are in all.
type githubRuns struct {
	TotalCount int `json:"total_count"`
	Runs       []struct {
		Status     string `json:"status"`
		Conclusion string `json:"conclusion"`
		Name       string `json:"name"`
		HTMLURL    string `json:"html_url"`
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
			tally.add(Check{Name: status.Context, State: statusState(status.State), URL: status.TargetURL})
		}
	}

	runs, runsComplete, err := githubPages(ctx, client, repo, commit+"/check-runs",
		func(page githubRuns) (int, int) { return len(page.Runs), page.TotalCount })
	if err != nil {
		return CI{}, err
	}

	for _, page := range runs {
		for _, run := range page.Runs {
			tally.add(Check{Name: run.Name, State: runState(run.Status, run.Conclusion), URL: run.HTMLURL})
		}
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
		githubRepoPath(repo), url.QueryEscape(head), githubPerPage)

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

// githubMergeBody is the body that merges a pull request: the method to use.
type githubMergeBody struct {
	MergeMethod string `json:"merge_method"`
}

// githubMergePull merges a pull request by the given method.
func githubMergePull(ctx context.Context, client Client, repo Repo, pull PullRequest, method MergeMethod) error {
	path := fmt.Sprintf("%s/pulls/%d/merge", githubRepoPath(repo), pull.Number)

	return send(ctx, client, http.MethodPut, path, githubMergeBody{MergeMethod: string(method)})
}

// githubRepoSettings is the repository object's merge-method allow flags.
type githubRepoSettings struct {
	AllowMergeCommit bool `json:"allow_merge_commit"`
	AllowSquashMerge bool `json:"allow_squash_merge"`
	AllowRebaseMerge bool `json:"allow_rebase_merge"`
}

// githubMergeMethods reads which merge methods the repository permits, in the
// order a merge commit, a squash, then a rebase.
func githubMergeMethods(ctx context.Context, client Client, repo Repo) ([]MergeMethod, error) {
	settings, err := repoCall[githubRepoSettings](ctx, client, repo, http.MethodGet, githubRepoPath(repo), nil)
	if err != nil {
		return nil, err
	}

	allowed := []struct {
		ok     bool
		method MergeMethod
	}{
		{settings.AllowMergeCommit, MergeCommit},
		{settings.AllowSquashMerge, MergeSquash},
		{settings.AllowRebaseMerge, MergeRebase},
	}

	methods := make([]MergeMethod, 0, len(allowed))

	for _, each := range allowed {
		if each.ok {
			methods = append(methods, each.method)
		}
	}

	return methods, nil
}

// runFailed reports a completed workflow run that did not pass, by the same rule
// runState reads a status by, so every run the Review pane calls failed — a
// timed-out or canceled one included, not only a plain "failure" — is re-run.
func runFailed(conclusion string) bool {
	passing := map[string]bool{succeeded: true, "neutral": true, skipped: true}

	return conclusion != "" && !passing[conclusion]
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
