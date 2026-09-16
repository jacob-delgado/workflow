// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package forge

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
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
	HeadPipeline *struct {
		Status string `json:"status"`
	} `json:"head_pipeline"`
}

// pullRequest flattens a GitLab merge request.
func (g gitlabMerge) pullRequest() PullRequest {
	return PullRequest{Number: g.IID, URL: g.URL, Title: g.Title, Draft: g.Draft}
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
	query := url.Values{"source_branch": {branch}, "state": {"opened"}}.Encode()

	merges, err := repoCall[[]gitlabMerge](ctx, client, repo, http.MethodGet,
		gitlabProjectPath(repo)+"/merge_requests?"+query, nil)
	if err != nil || len(merges) == 0 {
		return PullRequest{}, false, err
	}

	return merges[0].pullRequest(), true, nil
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
		return CI{State: CINone, Total: 0, Done: 0, Failed: 0}, nil
	}

	return CI{State: pipelineState(merge.HeadPipeline.Status), Total: 0, Done: 0, Failed: 0}, nil
}

// pipelineState reads a GitLab pipeline status. A status this does not know is
// running rather than passed: announcing green on a guess is the worse mistake.
func pipelineState(status string) CIState {
	states := map[string]CIState{
		"success": CIPassed, "skipped": CIPassed,
		"failed": CIFailed, "canceled": CIFailed,
	}

	state, known := states[status]
	if !known {
		return CIRunning
	}

	return state
}
