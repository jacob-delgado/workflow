// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package forge

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// githubPull is a pull request as GitHub sends one. A merged pull is closed
// with a merged_at time, which is how the two are told apart.
type githubPull struct {
	Number   int        `json:"number"`
	URL      string     `json:"html_url"`
	Title    string     `json:"title"`
	Body     string     `json:"body"`
	Draft    bool       `json:"draft"`
	State    string     `json:"state"`
	MergedAt *time.Time `json:"merged_at"`
}

// pullRequest flattens a GitHub pull request. Review state is filled in
// separately, so it stays zero here.
func (g githubPull) pullRequest() PullRequest {
	return PullRequest{Number: g.Number, URL: g.URL, Title: g.Title, Body: g.Body, Draft: g.Draft, State: g.state()}
}

// state reads whether the pull is open, merged or closed.
func (g githubPull) state() PullState {
	switch {
	case g.MergedAt != nil:
		return StateMerged
	case g.State == wireClosed:
		return StateClosed
	default:
		return StateOpen
	}
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

// githubPullPath is where one pull request lives in GitHub's API.
func githubPullPath(repo Repo, number int) string {
	return githubRepoPath(repo) + "/pulls/" + strconv.Itoa(number)
}

// githubEditPull is the PATCH body that changes a pull request's title and body.
type githubEditPull struct {
	Title string `json:"title"`
	Body  string `json:"body"`
}

// githubUpdate edits an open pull request's title and description.
func githubUpdate(
	ctx context.Context, client Client, repo Repo, pull PullRequest, edit PullRequestEdit,
) (PullRequest, error) {
	path := githubPullPath(repo, pull.Number)

	updated, err := repoCall[githubPull](ctx, client, repo, http.MethodPatch, path, githubEditPull(edit))
	if err != nil {
		return PullRequest{}, err
	}

	return updated.pullRequest(), nil
}

// githubFind finds the open pull request from a branch. GitHub filters by head
// as owner:branch; the branch alone matches nothing.
func githubFind(ctx context.Context, client Client, repo Repo, branch string) (PullRequest, bool, error) {
	owner, _, _ := strings.Cut(repo.Path, "/")
	query := url.Values{"head": {owner + ":" + branch}, queryState: {queryAll}}.Encode()

	pulls, err := repoCall[[]githubPull](ctx, client, repo, http.MethodGet, githubRepoPath(repo)+"/pulls?"+query, nil)
	if err != nil {
		return PullRequest{}, false, err
	}

	chosen, ok := pickPull(pulls)
	if !ok {
		return PullRequest{}, false, nil
	}

	pull := chosen.pullRequest()

	// Approvals and mergeability only matter while it is open.
	if pull.State == StateOpen {
		githubReviewState(ctx, client, repo, &pull)
	}

	return pull, true, nil
}

// githubSearchQuery finds the open pull requests that request the token owner's
// review. @me is GitHub's own name for whoever the token belongs to, so the
// user's own name is never needed.
const githubSearchQuery = "is:pr is:open review-requested:@me"

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
	found, err := githubSearch[githubIssue](ctx, client, "is:issue is:open assignee:@me repo:"+repo.Path)
	if err != nil {
		return nil, err
	}

	issues := make([]Issue, 0, len(found))
	for _, item := range found {
		issues = append(issues, item.issue())
	}

	return issues, nil
}

// githubSearchServes is how many results GitHub's search serves, however many
// it finds: a page past them is refused rather than answered empty.
const githubSearchServes = 1000

// githubSearchPage is one page of the search endpoint's answer: the items it
// found, and how many it found in all.
type githubSearchPage[T any] struct {
	TotalCount int `json:"total_count"`
	Items      []T `json:"items"`
}

// githubSearch reads every item an issue search finds, as far as the search
// serves them.
func githubSearch[T any](ctx context.Context, client Client, query string) ([]T, error) {
	return readPages(func(page int) ([]T, int, error) {
		found, err := call[githubSearchPage[T]](ctx, client, http.MethodGet,
			"/search/issues?"+pageQuery(url.Values{"q": {query}}, page), nil)

		return found.Items, min(found.TotalCount, githubSearchServes), err
	})
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
		githubRepoPath(repo)+issuesSegment+"/"+strconv.Itoa(number), githubIssueState{State: wireClosed})

	return err
}

// githubReviews lists the pull requests that request the token owner's review.
func githubReviews(ctx context.Context, client Client) ([]ReviewRequest, error) {
	found, err := githubSearch[githubReviewItem](ctx, client, githubSearchQuery)
	if err != nil {
		return nil, err
	}

	reviews := make([]ReviewRequest, 0, len(found))
	for _, item := range found {
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
	base := githubPullPath(repo, pull.Number)

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
	pull := githubPullPath(repo, number)
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

// githubMergeBody is the body that merges a pull request: the method to use.
type githubMergeBody struct {
	MergeMethod string `json:"merge_method"`
}

// githubMergePull merges a pull request by the given method.
func githubMergePull(ctx context.Context, client Client, repo Repo, pull PullRequest, method MergeMethod) error {
	path := githubPullPath(repo, pull.Number) + "/merge"

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
