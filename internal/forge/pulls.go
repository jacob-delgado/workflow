// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package forge

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"slices"
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
	// queryAll asks a forge for every state, so a find sees a merged pull and
	// not only an open one; wireClosed is what both forges call a closed one.
	queryAll   = "all"
	wireClosed = "closed"
)

// MergeMethod is how a pull request is merged: a merge commit, a squashed
// commit, or a rebase. Which of these a repository permits is the repository's
// to say, so a merge is only ever offered one the repository allows.
type MergeMethod string

const (
	// MergeCommit merges with a merge commit.
	MergeCommit MergeMethod = "merge"
	// MergeSquash squashes the branch into one commit.
	MergeSquash MergeMethod = "squash"
	// MergeRebase rebases the branch onto the base.
	MergeRebase MergeMethod = "rebase"
)

// PullState is where a pull request stands in its life: open, merged, or
// closed without merging.
type PullState int

const (
	// StateOpen means the pull request is open. It is the zero value.
	StateOpen PullState = iota
	// StateMerged means it has merged, so its branch's work is done.
	StateMerged
	// StateClosed means it was closed without merging.
	StateClosed
)

// PullRequest is a pull request on GitHub, or a merge request on GitLab.
type PullRequest struct {
	// Number is GitHub's number or GitLab's iid: the one people write as #42 or
	// !42.
	Number int
	URL    string
	Title  string
	// Body is the pull request's description, read so an edit can open on it.
	Body  string
	Draft bool
	// State is whether it is open, merged or closed, so a merged branch can be
	// finished. A find reads it; the zero value is open.
	State PullState
	// Approvals is how many reviewers have approved; ChangesRequested is whether
	// any reviewer is still asking for changes; Mergeable is whether it can merge.
	// These are best effort — a forge that will not say leaves them at zero and
	// unknown rather than failing the whole read.
	Approvals        int
	ChangesRequested bool
	Mergeable        Mergeability
}

// IsOpen reports whether this pull request is in the open state. FindPullRequest
// also returns a merged pull request, so a caller that means "is there an open
// one" — the standup list, the status line, the offer to open a new one — asks
// this rather than trusting found alone. (Opened, above, is the different
// question of whether the forge created it at all.)
func (p PullRequest) IsOpen() bool {
	return p.State == StateOpen
}

// NewPullRequest is a pull request to open.
type NewPullRequest struct {
	Title string
	Body  string
	// Head is the branch with the work; Base is the branch it merges into.
	Head  string
	Base  string
	Draft bool
	// Reviewers and Assignees are usernames; Labels are label names. Each forge
	// asks for them its own way — GitHub in a request after the pull is opened,
	// GitLab at creation — but a failure to add them never discards a pull
	// request already opened.
	Reviewers []string
	Assignees []string
	Labels    []string
}

// Opened reports whether this is a pull request the forge created, told from
// the zero value a failed create returns by its number, which the forge always
// assigns.
func (p PullRequest) Opened() bool {
	return p.Number != 0
}

// PullRequestEdit is what changes about an open pull request: its title and its
// description. Everything else about it — its branches, its reviewers — is left
// as it is.
type PullRequestEdit struct {
	Title string
	Body  string
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

// OldestFirst is the review queue in the order it is worked through, the
// longest-waiting request first — the order every surface lists it in. Requests
// opened at the same moment keep the forge's order, and the forge's answer is
// left as it came.
func OldestFirst(requests []ReviewRequest) []ReviewRequest {
	queue := slices.Clone(requests)
	slices.SortStableFunc(queue, func(left, right ReviewRequest) int {
		return left.OpenedAt.Compare(right.OpenedAt)
	})

	return queue
}

// dialect is how one forge answers the questions a review needs. The two
// forges agree on what a pull request is and on almost nothing about how to ask
// for one.
type dialect struct {
	find       func(ctx context.Context, c Client, repo Repo, branch string) (PullRequest, bool, error)
	create     func(ctx context.Context, c Client, repo Repo, request NewPullRequest) (PullRequest, error)
	update     func(ctx context.Context, c Client, repo Repo, pull PullRequest, edit PullRequestEdit) (PullRequest, error)
	status     func(ctx context.Context, c Client, repo Repo, pull PullRequest, head string) (CI, error)
	rerun      func(ctx context.Context, c Client, repo Repo, pull PullRequest, head string) (bool, error)
	merge      func(ctx context.Context, c Client, repo Repo, pull PullRequest, method MergeMethod) error
	methods    func(ctx context.Context, c Client, repo Repo) ([]MergeMethod, error)
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
			find: githubFind, create: githubCreate, update: githubUpdate, status: githubStatus, rerun: githubRerun,
			merge: githubMergePull, methods: githubMergeMethods,
			reviews: githubReviews, issues: githubIssues, readIssue: githubReadIssue, closeIssue: githubCloseIssue,
		},
		KindGitLab: {
			find: gitlabFind, create: gitlabCreate, update: gitlabUpdate, status: gitlabStatus, rerun: gitlabRerun,
			merge: gitlabMergePull, methods: gitlabMergeMethods,
			reviews: gitlabReviews, issues: gitlabIssues, readIssue: gitlabReadIssue, closeIssue: gitlabCloseIssue,
		},
	}

	found, ok := dialects[kind]
	if !ok {
		return dialect{}, ErrUnknownForge
	}

	return found, nil
}

// stated is a wire pull request that knows its own state, so one helper can
// choose the branch's pull request the same way for either forge.
type stated interface {
	state() PullState
}

// pickPull chooses the branch's pull request to show: an open one if there is
// one, otherwise the merged one that means the branch is done. A pull closed
// without merging is passed over, so the branch still offers to open a new one.
func pickPull[T stated](pulls []T) (T, bool) {
	var (
		merged      T
		foundMerged bool
	)

	for index := range pulls {
		switch pulls[index].state() {
		case StateOpen:
			return pulls[index], true
		case StateMerged:
			if !foundMerged {
				merged, foundMerged = pulls[index], true
			}
		case StateClosed:
		}
	}

	return merged, foundMerged
}

// FindPullRequest finds the branch's pull request: an open one, or the merged
// one that means the branch is finished. A pull closed without merging is passed
// over. The returned PullRequest carries its State, so a caller that wants only
// an open one checks Opened rather than trusting found alone.
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

// EditPullRequest changes an open pull request's title and description, leaving
// the pull request otherwise as it is.
func (c Client) EditPullRequest(
	ctx context.Context, repo Repo, pull PullRequest, edit PullRequestEdit,
) (PullRequest, error) {
	speaks, err := dialectFor(repo.Kind)
	if err != nil {
		return PullRequest{}, err
	}

	return speaks.update(ctx, c, repo, pull, edit)
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

// RerunChecks re-runs the failed CI on a pull request whose head is the given
// commit, and reports whether anything was re-run — a failure with no re-runnable
// job restarts nothing. It needs a write scope the read path does not, so it can
// fail with ErrRefused where reading the status did not.
func (c Client) RerunChecks(ctx context.Context, repo Repo, pull PullRequest, head string) (bool, error) {
	speaks, err := dialectFor(repo.Kind)
	if err != nil {
		return false, err
	}

	return speaks.rerun(ctx, c, repo, pull, head)
}

// Merge merges a pull request by the given method, which must be one the
// repository permits. Like re-running checks it needs a write scope the read
// path does not, so it can fail with ErrRefused where reading the pull did not.
func (c Client) Merge(ctx context.Context, repo Repo, pull PullRequest, method MergeMethod) error {
	speaks, err := dialectFor(repo.Kind)
	if err != nil {
		return err
	}

	return speaks.merge(ctx, c, repo, pull, method)
}

// MergeMethods lists the merge methods the repository permits, so a merge is
// only ever offered one it allows.
func (c Client) MergeMethods(ctx context.Context, repo Repo) ([]MergeMethod, error) {
	speaks, err := dialectFor(repo.Kind)
	if err != nil {
		return nil, err
	}

	return speaks.methods(ctx, c, repo)
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
