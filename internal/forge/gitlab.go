// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package forge

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// draftPrefix marks a merge request as a draft by its title, which every
// version of GitLab reads; the draft parameter on create is newer.
const draftPrefix = "Draft: "

// gitlabMerge is a merge request as GitLab sends one.
type gitlabMerge struct {
	IID          int    `json:"iid"`
	URL          string `json:"web_url"`
	Title        string `json:"title"`
	Draft        bool   `json:"draft"`
	MergeStatus  string `json:"merge_status"`
	HeadPipeline *struct {
		Status string `json:"status"`
		URL    string `json:"web_url"`
	} `json:"head_pipeline"`
}

// pullRequest flattens a GitLab merge request. Approvals are filled in
// separately; GitLab has no "changes requested" state, so it stays false.
func (g gitlabMerge) pullRequest() PullRequest {
	return PullRequest{
		Number: g.IID, URL: g.URL, Title: g.Title, Draft: g.Draft, Mergeable: gitlabMergeable(g.MergeStatus),
	}
}

// gitlabMergeable reads GitLab's merge_status. Anything but the two settled
// answers — including a status still being computed — is unknown.
func gitlabMergeable(status string) Mergeability {
	switch status {
	case "can_be_merged":
		return MergeClean
	case "cannot_be_merged":
		return MergeConflicts
	default:
		return MergeUnknown
	}
}

// gitlabApprovals is the approvals endpoint's answer: who has approved.
type gitlabApprovals struct {
	ApprovedBy []struct{} `json:"approved_by"`
}

// gitlabReviewState fills in a merge request's approvals. It is best effort: an
// approvals endpoint the token cannot reach leaves the count at zero rather than
// failing the whole find.
func gitlabReviewState(ctx context.Context, client Client, repo Repo, pull *PullRequest) {
	approvals, err := repoCall[gitlabApprovals](ctx, client, repo, http.MethodGet,
		gitlabProjectPath(repo)+"/merge_requests/"+strconv.Itoa(pull.Number)+"/approvals", nil)
	if err == nil {
		pull.Approvals = len(approvals.ApprovedBy)
	}
}

// gitlabNewMerge is the body that opens a merge request.
type gitlabNewMerge struct {
	Title        string `json:"title"`
	Description  string `json:"description"`
	SourceBranch string `json:"source_branch"`
	TargetBranch string `json:"target_branch"`
}

// gitlabProjectPath is where a project lives in GitLab's API: its whole path as
// ONE escaped segment, because projects nest and the slashes are part of the
// name.
func gitlabProjectPath(repo Repo) string {
	return "/projects/" + url.PathEscape(repo.Path)
}

// gitlabFind finds the open merge request from a branch.
func gitlabFind(ctx context.Context, client Client, repo Repo, branch string) (PullRequest, bool, error) {
	query := url.Values{"source_branch": {branch}, queryState: {stateOpened}}.Encode()

	merges, err := repoCall[[]gitlabMerge](ctx, client, repo, http.MethodGet,
		gitlabProjectPath(repo)+"/merge_requests?"+query, nil)
	if err != nil || len(merges) == 0 {
		return PullRequest{}, false, err
	}

	pull := merges[0].pullRequest()
	gitlabReviewState(ctx, client, repo, &pull)

	return pull, true, nil
}

// gitlabPerPage bounds one page of a listing; stateOpened is GitLab's name
// for the open state.
const (
	gitlabPerPage = 100
	stateOpened   = "opened"
)

// gitlabReviewMerge is a merge request as GitLab's listing sends it, with the
// fields a review queue shows: who opened it, since when, and its head pipeline.
type gitlabReviewMerge struct {
	IID       int       `json:"iid"`
	URL       string    `json:"web_url"`
	Title     string    `json:"title"`
	Draft     bool      `json:"draft"`
	CreatedAt time.Time `json:"created_at"`
	Author    struct {
		Username string `json:"username"`
	} `json:"author"`
	References struct {
		Full string `json:"full"`
	} `json:"references"`
	HeadPipeline *struct {
		Status string `json:"status"`
	} `json:"head_pipeline"`
}

// reviewRequest flattens a listed merge request. references.full is
// "group/project!iid", so the project is what precedes the bang.
func (g gitlabReviewMerge) reviewRequest() ReviewRequest {
	project, _, _ := strings.Cut(g.References.Full, "!")

	ciState := CINone
	if g.HeadPipeline != nil {
		ciState = pipelineState(g.HeadPipeline.Status)
	}

	return ReviewRequest{
		Number: g.IID, URL: g.URL, Title: g.Title, Draft: g.Draft,
		Author: g.Author.Username, Repository: project, CI: ciState, OpenedAt: g.CreatedAt,
	}
}

// gitlabIssue is an issue as GitLab sends it, from the listing or a single read.
type gitlabIssue struct {
	IID         int    `json:"iid"`
	URL         string `json:"web_url"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Author      struct {
		Username string `json:"username"`
	} `json:"author"`
}

func (g gitlabIssue) issue() Issue {
	return Issue{Number: g.IID, URL: g.URL, Title: g.Title}
}

func (g gitlabIssue) detail() IssueDetail {
	return IssueDetail{Issue: g.issue(), Body: g.Description, Author: g.Author.Username}
}

// gitlabIssues lists the open issues in the project assigned to the token owner.
// GitLab filters by assignee username, not a "me" token, so who the token
// belongs to is asked first.
func gitlabIssues(ctx context.Context, client Client, repo Repo) ([]Issue, error) {
	viewer, err := client.Whoami(ctx)
	if err != nil {
		return nil, err
	}

	query := url.Values{
		queryState:          {stateOpened},
		"assignee_username": {viewer.Name()},
		perPageParam:        {strconv.Itoa(gitlabPerPage)},
	}.Encode()

	listed, err := repoCall[[]gitlabIssue](ctx, client, repo, http.MethodGet,
		gitlabProjectPath(repo)+issuesSegment+"?"+query, nil)
	if err != nil {
		return nil, err
	}

	issues := make([]Issue, 0, len(listed))
	for _, one := range listed {
		issues = append(issues, one.issue())
	}

	return issues, nil
}

// gitlabReadIssue reads one issue's body and author.
func gitlabReadIssue(ctx context.Context, client Client, repo Repo, number int) (IssueDetail, error) {
	read, err := repoCall[gitlabIssue](ctx, client, repo, http.MethodGet,
		gitlabProjectPath(repo)+issuesSegment+"/"+strconv.Itoa(number), nil)
	if err != nil {
		return IssueDetail{}, err
	}

	return read.detail(), nil
}

// gitlabIssueState is the PUT body that closes an issue. GitLab closes by an
// event verb, not by a state.
type gitlabIssueState struct {
	StateEvent string `json:"state_event"`
}

// gitlabCloseIssue closes an issue.
func gitlabCloseIssue(ctx context.Context, client Client, repo Repo, number int) error {
	_, err := repoCall[gitlabIssue](ctx, client, repo, http.MethodPut,
		gitlabProjectPath(repo)+issuesSegment+"/"+strconv.Itoa(number), gitlabIssueState{StateEvent: "close"})

	return err
}

// gitlabReviews lists the merge requests that request the token owner's review.
// GitLab filters by reviewer username, not a "me" token, so who the token
// belongs to is asked first.
func gitlabReviews(ctx context.Context, client Client) ([]ReviewRequest, error) {
	viewer, err := client.Whoami(ctx)
	if err != nil {
		return nil, err
	}

	query := url.Values{
		"scope":             {"all"},
		queryState:          {stateOpened},
		"reviewer_username": {viewer.Name()},
		perPageParam:        {strconv.Itoa(gitlabPerPage)},
	}.Encode()

	merges, err := call[[]gitlabReviewMerge](ctx, client, http.MethodGet, "/merge_requests?"+query, nil)
	if err != nil {
		return nil, err
	}

	reviews := make([]ReviewRequest, 0, len(merges))
	for _, merge := range merges {
		reviews = append(reviews, merge.reviewRequest())
	}

	return reviews, nil
}

// gitlabCreate opens a merge request.
func gitlabCreate(ctx context.Context, client Client, repo Repo, request NewPullRequest) (PullRequest, error) {
	title := request.Title
	if request.Draft {
		title = draftPrefix + title
	}

	payload := gitlabNewMerge{
		Title: title, Description: request.Body, SourceBranch: request.Head, TargetBranch: request.Base,
	}

	created, err := repoCall[gitlabMerge](ctx, client, repo, http.MethodPost,
		gitlabProjectPath(repo)+"/merge_requests", payload)
	if err != nil {
		return PullRequest{}, err
	}

	return created.pullRequest(), nil
}

// gitlabStatus reads the merge request's own head pipeline. Pipelines found by
// commit are not the same thing: checked on a public project, a commit carried
// pipelines from unrelated workloads alongside the review's.
func gitlabStatus(ctx context.Context, client Client, repo Repo, pull PullRequest, _ string) (CI, error) {
	merge, err := repoCall[gitlabMerge](ctx, client, repo, http.MethodGet,
		gitlabProjectPath(repo)+"/merge_requests/"+strconv.Itoa(pull.Number), nil)
	if err != nil {
		return CI{}, err
	}

	if merge.HeadPipeline == nil {
		return CI{State: CINone, Total: 0, Done: 0, Failed: 0, Checks: nil}, nil
	}

	// GitLab reports the review's head pipeline as a whole, so it is the one
	// check there is to list — its own page opens the jobs within it.
	state := pipelineState(merge.HeadPipeline.Status)
	pipeline := Check{Name: "pipeline", State: state, URL: merge.HeadPipeline.URL}

	return CI{State: state, Total: 0, Done: 0, Failed: 0, Checks: []Check{pipeline}}, nil
}

// pipelineState reads a GitLab pipeline status. A status this does not know is
// running rather than passed: announcing green on a guess is the worse mistake.
func pipelineState(status string) CIState {
	states := map[string]CIState{
		succeeded: CIPassed, "skipped": CIPassed,
		"failed": CIFailed, "canceled": CIFailed,
	}

	state, known := states[status]
	if !known {
		return CIRunning
	}

	return state
}
