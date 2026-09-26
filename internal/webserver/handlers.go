// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package webserver

import (
	"context"
	"errors"
	"fmt"
	"net/http"

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

// GetHealth reports the build, whether writes are held back, and the forge's own
// words for a proposed change, so the browser names it as the terminal does.
func (s *server) GetHealth(_ context.Context, _ api.GetHealthRequestObject) (api.GetHealthResponseObject, error) {
	return api.GetHealth200JSONResponse{
		Version:    s.info.Version,
		DryRun:     s.info.DryRun,
		ForgeNoun:  s.info.ForgeKind.Noun(),
		ForgeSigil: s.info.ForgeKind.Sigil(),
	}, nil
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

	view := ""
	if request.Params.View != nil {
		view = *request.Params.View
	}

	jql, ok := resolveJQL(s.config(), view)
	if !ok {
		return api.ListIssues404ApplicationProblemPlusJSONResponse(problem(api.NotFound, unknownView(view))), nil
	}

	if s.deps.Search == nil {
		return api.ListIssues200JSONResponse(issuesPageDTO(jira.SearchResult{}, startAt)), nil
	}

	result, err := s.deps.Search(jql, startAt)
	if err != nil {
		body, code := fault(err)

		return api.ListIssuesdefaultApplicationProblemPlusJSONResponse{Body: body, StatusCode: code}, nil
	}

	return api.ListIssues200JSONResponse(issuesPageDTO(result, startAt)), nil
}

// GetIssue returns one issue in full.
func (s *server) GetIssue(_ context.Context, request api.GetIssueRequestObject) (api.GetIssueResponseObject, error) {
	if s.deps.Issue == nil {
		return api.GetIssuedefaultApplicationProblemPlusJSONResponse{
			Body:       problem(api.Unprocessable, "no issue tracker is configured"),
			StatusCode: http.StatusUnprocessableEntity,
		}, nil
	}

	detail, err := s.deps.Issue(jira.Key(request.Key))
	if err != nil {
		if errors.Is(err, jira.ErrNotFound) || errors.Is(err, forge.ErrNoRepository) {
			return api.GetIssue404ApplicationProblemPlusJSONResponse(
				problem(api.NotFound, "issue "+request.Key+" was not found")), nil
		}

		body, code := fault(err)

		return api.GetIssuedefaultApplicationProblemPlusJSONResponse{Body: body, StatusCode: code}, nil
	}

	return api.GetIssue200JSONResponse(issueDetailDTO(detail, s.browseURL(detail.Issue.Key))), nil
}

// browseURL is the issue's page in the tracker, or "" when no tracker link is
// wired.
func (s *server) browseURL(key jira.Key) string {
	if s.deps.BrowseURL == nil {
		return ""
	}

	return s.deps.BrowseURL(key)
}

// GetBranch returns the current branch, or an empty one when no repository is
// configured.
func (s *server) GetBranch(_ context.Context, _ api.GetBranchRequestObject) (api.GetBranchResponseObject, error) {
	branch, err := s.readBranch()
	if err != nil {
		body, code := fault(err)

		return api.GetBranchdefaultApplicationProblemPlusJSONResponse{Body: body, StatusCode: code}, nil
	}

	return api.GetBranch200JSONResponse(branchDTO(branch)), nil
}

// readBranch is the checked-out branch, or an empty one when no repository is
// configured: the one read behind GET /api/branch and the event stream's frame
// (frameBranch), which reads it once for every panel that describes it.
func (s *server) readBranch() (gitrepo.Branch, error) {
	if s.deps.Branch == nil {
		return gitrepo.Branch{}, nil
	}

	return s.deps.Branch()
}

// branchAfter re-reads the branch once a write to it has landed. A re-read
// that fails does not undo the write, so fallback — the branch as the caller
// knows it — is answered rather than a completed write reported as failed; the
// event stream brings the rest.
func (s *server) branchAfter(fallback gitrepo.Branch) gitrepo.Branch {
	after, err := s.deps.Branch()
	if err != nil {
		return fallback
	}

	return after
}

// ListChanges returns the working tree's changes, or none when no repository is
// configured.
func (s *server) ListChanges(_ context.Context, _ api.ListChangesRequestObject) (api.ListChangesResponseObject, error) {
	changes, err := s.readChanges()
	if err != nil {
		body, code := fault(err)

		return api.ListChangesdefaultApplicationProblemPlusJSONResponse{Body: body, StatusCode: code}, nil
	}

	return api.ListChanges200JSONResponse(changesDTO(changes)), nil
}

// readChanges is the working tree's changes, or none when no repository is
// configured: the one read behind GET /api/changes and the event stream's
// changes panel.
func (s *server) readChanges() ([]gitrepo.Change, error) {
	if s.deps.Changes == nil {
		return nil, nil
	}

	return s.deps.Changes()
}

// GetReview returns the branch's pull request and its CI, if one is found.
func (s *server) GetReview(_ context.Context, _ api.GetReviewRequestObject) (api.GetReviewResponseObject, error) {
	review, err := s.readReview()
	if err != nil {
		body, code := fault(err)

		return api.GetReviewdefaultApplicationProblemPlusJSONResponse{Body: body, StatusCode: code}, nil
	}

	return api.GetReview200JSONResponse(review), nil
}

// readReview is the checked-out branch's pull request and its CI, or none found
// when no repository or forge is configured: the read behind GET /api/review.
// Without a forge the branch is not read, since nothing would ask about it.
func (s *server) readReview() (api.Review, error) {
	if s.deps.Branch == nil || s.deps.FindPull == nil {
		return api.Review{Found: false}, nil
	}

	branch, err := s.deps.Branch()
	if err != nil {
		return api.Review{}, err
	}

	return s.reviewFor(branch)
}

// reviewFor is the branch's pull request and its CI, or none found when no
// forge is configured: the one read behind GET /api/review and the event
// stream's review panel, which hands it the branch its frame read.
func (s *server) reviewFor(branch gitrepo.Branch) (api.Review, error) {
	if s.deps.FindPull == nil {
		return api.Review{Found: false}, nil
	}

	pull, found, err := s.deps.FindPull(branch.Name)
	if err != nil {
		return api.Review{}, err
	}

	return s.review(pull, found, branch.Head), nil
}

// review assembles the review state, folding in CI when an open pull request is
// found and CI can be read. A merged pull request has no live CI, so it is not
// asked about, as the terminal does not ask. A CI read that fails leaves the pull
// request without it rather than failing the whole answer.
func (s *server) review(pull forge.PullRequest, found bool, head string) api.Review {
	result := api.Review{Found: found}
	if !found {
		return result
	}

	dto := pullDTO(pull)
	result.Pull = &dto

	if s.deps.CheckCI == nil || !pull.IsOpen() {
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

// ListReviews returns the pull requests on the forge that ask for your review,
// the longest-waiting first — the queue `workflow reviews` prints and the
// interface's Reviews pane lists. With no forge to ask, it answers that the
// queue is not available rather than failing: that is where the server runs,
// not something the page asked for wrongly.
func (s *server) ListReviews(
	_ context.Context, _ api.ListReviewsRequestObject,
) (api.ListReviewsResponseObject, error) {
	if s.deps.ReviewRequests == nil {
		return api.ListReviews200JSONResponse(noReviewQueue()), nil
	}

	requests, err := s.deps.ReviewRequests()
	if noForgeToAsk(err) {
		return api.ListReviews200JSONResponse(noReviewQueue()), nil
	}

	if err != nil {
		body, code := fault(err)

		return api.ListReviewsdefaultApplicationProblemPlusJSONResponse{Body: body, StatusCode: code}, nil
	}

	return api.ListReviews200JSONResponse(reviewQueueDTO(forge.OldestFirst(requests))), nil
}

// noForgeToAsk reports a review read that found no forge to ask: no remote —
// outside a repository, or with no origin — or an origin on a host that is not
// a forge workflow can read.
func noForgeToAsk(err error) bool {
	return errors.Is(err, forge.ErrNotARemote) || errors.Is(err, forge.ErrUnknownForge)
}

// GetMessaging returns the service, and where and as whom an announcement would
// post.
func (s *server) GetMessaging(
	_ context.Context, _ api.GetMessagingRequestObject,
) (api.GetMessagingResponseObject, error) {
	return api.GetMessaging200JSONResponse(s.readMessaging()), nil
}

// readMessaging is the messaging destination: the one read behind GET
// /api/messaging and the event stream's messaging panel.
func (s *server) readMessaging() api.MessagingDestination {
	return messagingDTO(s.config(), s.readAuthor())
}

// readAuthor resolves who a post would come from, or an empty string when the
// forge is not configured or cannot say.
func (s *server) readAuthor() string {
	if s.deps.Author == nil {
		return ""
	}

	name, err := s.cachedAuthor()
	if err != nil {
		return ""
	}

	return name
}

// cachedAuthor is who a post would come from: the forge's kept answer once it
// has given one, else a fresh ask whose answer is kept for the server's life.
func (s *server) cachedAuthor() (string, error) {
	s.author.mu.Lock()
	defer s.author.mu.Unlock()

	if s.author.known {
		return s.author.name, nil
	}

	name, err := s.deps.Author()
	if err != nil {
		return "", fmt.Errorf("reading the author: %w", err)
	}

	s.author.name, s.author.known = name, true

	return name, nil
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
// empty. It reports false for a name no view carries, so a typo is refused
// rather than quietly answered with the default view's issues.
func resolveJQL(cfg config.Config, name string) (string, bool) {
	views := viewsDTO(cfg)
	if name == "" {
		return views[0].Jql, true
	}

	for _, view := range views {
		if view.Name == name {
			return view.Jql, true
		}
	}

	return "", false
}

// unknownView is the detail for a view name the configuration does not carry.
func unknownView(name string) string {
	return fmt.Sprintf("no view is named %q", name)
}
