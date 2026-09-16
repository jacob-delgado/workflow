// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package forge

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
)

// PullRequest is a pull request on GitHub, or a merge request on GitLab.
type PullRequest struct {
	// Number is GitHub's number or GitLab's iid: the one people write as #42 or
	// !42.
	Number int
	URL    string
	Title  string
	Draft  bool
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

// dialect is how one forge answers the questions a review needs. The two
// forges agree on what a pull request is and on almost nothing about how to ask
// for one.
type dialect struct {
	find   func(ctx context.Context, c Client, repo Repo, branch string) (PullRequest, bool, error)
	create func(ctx context.Context, c Client, repo Repo, request NewPullRequest) (PullRequest, error)
	status func(ctx context.Context, c Client, repo Repo, pull PullRequest, head string) (CI, error)
}

// dialectFor is the dialect of a forge, or ErrUnknownForge for a host whose
// forge could not be told.
func dialectFor(kind Kind) (dialect, error) {
	dialects := map[Kind]dialect{
		KindGitHub: {find: githubFind, create: githubCreate, status: githubStatus},
		KindGitLab: {find: gitlabFind, create: gitlabCreate, status: gitlabStatus},
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
