// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package webserver

import (
	"context"

	"github.com/jacob-delgado/workflow/internal/api"
	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/gitrepo"
	"github.com/jacob-delgado/workflow/internal/jira"
)

// config returns a copy of the configuration in effect, taken under the read
// lock so a concurrent write endpoint cannot tear it.
func (s *server) config() config.Config {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.cfg
}

// GetHealth reports the build and whether writes are held back.
func (s *server) GetHealth(_ context.Context, _ api.GetHealthRequestObject) (api.GetHealthResponseObject, error) {
	return api.GetHealth200JSONResponse{Version: s.info.Version, DryRun: s.info.DryRun}, nil
}

// ListViews lists the configured issue views, or the one built-in list.
func (s *server) ListViews(_ context.Context, _ api.ListViewsRequestObject) (api.ListViewsResponseObject, error) {
	return api.ListViews200JSONResponse{Views: viewsDTO(s.config())}, nil
}

// ListIssues returns one page of the issues a view matches.
func (s *server) ListIssues(
	_ context.Context, request api.ListIssuesRequestObject,
) (api.ListIssuesResponseObject, error) {
	startAt := 0
	if request.Params.StartAt != nil {
		startAt = *request.Params.StartAt
	}

	if s.deps.Search == nil {
		return api.ListIssues200JSONResponse(issuesPageDTO(jira.SearchResult{}, startAt)), nil
	}

	view := ""
	if request.Params.View != nil {
		view = *request.Params.View
	}

	result, err := s.deps.Search(resolveJQL(s.config(), view), startAt)
	if err != nil {
		body, code := fault(err)

		return api.ListIssuesdefaultJSONResponse{Body: body, StatusCode: code}, nil
	}

	return api.ListIssues200JSONResponse(issuesPageDTO(result, startAt)), nil
}

// GetIssue returns one issue in full.
func (s *server) GetIssue(_ context.Context, request api.GetIssueRequestObject) (api.GetIssueResponseObject, error) {
	if s.deps.Issue == nil {
		return api.GetIssue404JSONResponse{Code: api.NotFound, Message: "no tracker is configured"}, nil
	}

	detail, err := s.deps.Issue(jira.Key(request.Key))
	if err != nil {
		body, code := fault(err)

		return api.GetIssuedefaultJSONResponse{Body: body, StatusCode: code}, nil
	}

	return api.GetIssue200JSONResponse(issueDetailDTO(detail)), nil
}

// GetBranch returns the current branch, or an empty one outside a repository.
func (s *server) GetBranch(_ context.Context, _ api.GetBranchRequestObject) (api.GetBranchResponseObject, error) {
	if s.deps.Branch == nil {
		return api.GetBranch200JSONResponse(branchDTO(gitrepo.Branch{})), nil
	}

	branch, err := s.deps.Branch()
	if err != nil {
		body, code := fault(err)

		return api.GetBranchdefaultJSONResponse{Body: body, StatusCode: code}, nil
	}

	return api.GetBranch200JSONResponse(branchDTO(branch)), nil
}

// ListChanges returns the working tree's changes, or none outside a repository.
func (s *server) ListChanges(_ context.Context, _ api.ListChangesRequestObject) (api.ListChangesResponseObject, error) {
	if s.deps.Changes == nil {
		return api.ListChanges200JSONResponse(changesDTO(nil)), nil
	}

	changes, err := s.deps.Changes()
	if err != nil {
		body, code := fault(err)

		return api.ListChangesdefaultJSONResponse{Body: body, StatusCode: code}, nil
	}

	return api.ListChanges200JSONResponse(changesDTO(changes)), nil
}

// GetReview returns the branch's pull request and its CI, if one is open.
func (s *server) GetReview(_ context.Context, _ api.GetReviewRequestObject) (api.GetReviewResponseObject, error) {
	if s.deps.Branch == nil || s.deps.FindPull == nil {
		return api.GetReview200JSONResponse{Found: false}, nil
	}

	branch, err := s.deps.Branch()
	if err != nil {
		body, code := fault(err)

		return api.GetReviewdefaultJSONResponse{Body: body, StatusCode: code}, nil
	}

	pull, found, err := s.deps.FindPull(branch.Name)
	if err != nil {
		body, code := fault(err)

		return api.GetReviewdefaultJSONResponse{Body: body, StatusCode: code}, nil
	}

	return api.GetReview200JSONResponse(s.review(pull, found, branch.Head)), nil
}

// review assembles the review state, folding in CI when a pull request is found
// and CI can be read. A CI read that fails leaves the pull request without it
// rather than failing the whole answer.
func (s *server) review(pull forge.PullRequest, found bool, head string) api.Review {
	result := api.Review{Found: found}
	if !found {
		return result
	}

	dto := pullDTO(pull)
	result.Pull = &dto

	if s.deps.CheckCI == nil {
		return result
	}

	status, err := s.deps.CheckCI(pull, head)
	if err != nil {
		return result
	}

	ci := ciDTO(status)
	result.Ci = &ci

	return result
}

// GetMessaging returns the service, and where and as whom an announcement would
// post.
func (s *server) GetMessaging(
	_ context.Context, _ api.GetMessagingRequestObject,
) (api.GetMessagingResponseObject, error) {
	return api.GetMessaging200JSONResponse(messagingDTO(s.config(), s.author())), nil
}

// author resolves who a post would come from, or an empty string when the forge
// is not configured or cannot say.
func (s *server) author() string {
	if s.deps.Author == nil {
		return ""
	}

	name, err := s.deps.Author()
	if err != nil {
		return ""
	}

	return name
}

// viewsDTO is the views a configuration offers, or the one built-in list — open
// issues assigned to you — when it names none.
func viewsDTO(cfg config.Config) []api.JiraView {
	if len(cfg.Jira.Views) == 0 {
		return []api.JiraView{{Name: "Assigned to me", Jql: jira.AssignedToMe}}
	}

	out := make([]api.JiraView, 0, len(cfg.Jira.Views))
	for _, view := range cfg.Jira.Views {
		out = append(out, api.JiraView{Name: view.Name, Jql: view.JQL})
	}

	return out
}

// resolveJQL is the JQL for the named view, or the first view's when the name is
// empty or unknown.
func resolveJQL(cfg config.Config, name string) string {
	views := viewsDTO(cfg)
	for _, view := range views {
		if view.Name == name {
			return view.Jql
		}
	}

	return views[0].Jql
}
