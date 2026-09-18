// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package forge

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"
)

// Mergeability is whether the forge thinks a pull request can merge as it
// stands: unknown until the forge has worked it out, then clean or conflicting.
type Mergeability int

const (
	// MergeUnknown means the forge has not said, or is still computing it.
	MergeUnknown Mergeability = iota
	// MergeClean means the branch merges without conflict.
	MergeClean
	// MergeConflicts means the branch conflicts with its base.
	MergeConflicts
)

// queryState is the query parameter both forges name the open/closed filter,
// and perPageParam bounds one page of a listing.
const (
	queryState   = "state"
	perPageParam = "per_page"
)

// PullRequest is a pull request on GitHub, or a merge request on GitLab.
type PullRequest struct {
	// Number is GitHub's number or GitLab's iid: the one people write as #42 or
	// !42.
	Number int
	URL    string
	Title  string
	Draft  bool
	// Approvals is how many reviewers have approved; ChangesRequested is whether
	// any reviewer is still asking for changes; Mergeable is whether it can merge.
	// These are best effort — a forge that will not say leaves them at zero and
	// unknown rather than failing the whole read.
	Approvals        int
	ChangesRequested bool
	Mergeable        Mergeability
}

// NewPullRequest is a pull request to open.
type NewPullRequest struct {
	Title string
	Body  string
	// Head is the branch with the work; Base is the branch it merges into.
	Head  string
	Base  string
	Draft bool
}

// ReviewRequest is an open pull or merge request that asks for your review. It
// carries what a review queue is read for — who wants it, since when, and where
// CI stands — across whichever repositories on the forge requested you.
type ReviewRequest struct {
	Number int
	URL    string
	Title  string
	Author string
	// Repository is the owner/name (GitHub) or group/project (GitLab) the request
	// is in, since a review queue spans repositories.
	Repository string
	Draft      bool
	// CI is best effort: the forge's own summary where the listing carries one,
	// and CINone where it does not, since the queue is read without a follow-up
	// request per entry.
	CI       CIState
	OpenedAt time.Time
}

// dialect is how one forge answers the questions a review needs. The two
// forges agree on what a pull request is and on almost nothing about how to ask
// for one.
type dialect struct {
	find       func(ctx context.Context, c Client, repo Repo, branch string) (PullRequest, bool, error)
	create     func(ctx context.Context, c Client, repo Repo, request NewPullRequest) (PullRequest, error)
	status     func(ctx context.Context, c Client, repo Repo, pull PullRequest, head string) (CI, error)
	reviews    func(ctx context.Context, c Client) ([]ReviewRequest, error)
	issues     func(ctx context.Context, c Client, repo Repo) ([]Issue, error)
	readIssue  func(ctx context.Context, c Client, repo Repo, number int) (IssueDetail, error)
	closeIssue func(ctx context.Context, c Client, repo Repo, number int) error
}

// dialectFor is the dialect of a forge, or ErrUnknownForge for a host whose
// forge could not be told.
func dialectFor(kind Kind) (dialect, error) {
	//nolint:exhaustive // KindUnknown has no dialect on purpose; its lookup miss is the ErrUnknownForge below.
	dialects := map[Kind]dialect{
		KindGitHub: {
			find: githubFind, create: githubCreate, status: githubStatus, reviews: githubReviews,
			issues: githubIssues, readIssue: githubReadIssue, closeIssue: githubCloseIssue,
		},
		KindGitLab: {
			find: gitlabFind, create: gitlabCreate, status: gitlabStatus, reviews: gitlabReviews,
			issues: gitlabIssues, readIssue: gitlabReadIssue, closeIssue: gitlabCloseIssue,
		},
	}

	found, ok := dialects[kind]
	if !ok {
		return dialect{}, ErrUnknownForge
	}

	return found, nil
}

// FindPullRequest finds the open pull request from a branch, if there is one.
func (c Client) FindPullRequest(ctx context.Context, repo Repo, branch string) (PullRequest, bool, error) {
	speaks, err := dialectFor(repo.Kind)
	if err != nil {
		return PullRequest{}, false, err
	}

	return speaks.find(ctx, c, repo, branch)
}

// CreatePullRequest opens a pull request.
func (c Client) CreatePullRequest(ctx context.Context, repo Repo, request NewPullRequest) (PullRequest, error) {
	speaks, err := dialectFor(repo.Kind)
	if err != nil {
		return PullRequest{}, err
	}

	return speaks.create(ctx, c, repo, request)
}

// CheckStatus reports how CI stands on a pull request whose head is the given
// commit.
func (c Client) CheckStatus(ctx context.Context, repo Repo, pull PullRequest, head string) (CI, error) {
	speaks, err := dialectFor(repo.Kind)
	if err != nil {
		return CI{}, err
	}

	return speaks.status(ctx, c, repo, pull, head)
}

// ReviewRequests lists the open pull or merge requests on the forge that ask
// the token's owner for a review. The kind chooses how to ask; the client can
// only reach the one forge its base URL points at.
func (c Client) ReviewRequests(ctx context.Context, kind Kind) ([]ReviewRequest, error) {
	speaks, err := dialectFor(kind)
	if err != nil {
		return nil, err
	}

	return speaks.reviews(ctx, c)
}

// repoCall is call for a request about one repository, where 404 means the
// token cannot see the repository rather than that no API is there.
func repoCall[T any](ctx context.Context, client Client, repo Repo, method, path string, payload any) (T, error) {
	answer, err := call[T](ctx, client, method, path, payload)
	if errors.Is(err, ErrNoAPI) {
		return answer, fmt.Errorf("%w: %s", ErrNoRepository, repo.Path)
	}

	return answer, err
}

// escapedPath escapes each segment of a repository path, keeping the slashes.
func escapedPath(path string) string {
	segments := strings.Split(path, "/")
	for index, segment := range segments {
		segments[index] = url.PathEscape(segment)
	}

	return strings.Join(segments, "/")
}
