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

// queryState is the query parameter both forges name the open/closed filter.
const (
	queryState = "state"
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
	// Base is the branch it merges into, as the forge says; a find reads it,
	// and it is empty where the forge did not.
	Base string
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
// one" — the status line, the link offer, the offer to open a new one — asks
// this rather than trusting found alone. (Opened, below, is the different
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
	// request already opened. Reviewers, and on GitLab assignees, are added
	// best effort: one the forge cannot add leaves the rest added and
	// ErrSomePeopleNotAdded returned.
	Reviewers []string
	// TeamReviewers are teams named "org/team", asked to review as a team on
	// GitHub; on GitLab a group stands for its active direct members.
	TeamReviewers []string
	Assignees     []string
	Labels        []string
}

// ErrSomePeopleNotAdded reports a pull request opened without some of the
// reviewers or assignees named for it, which the message names; it wraps why
// they could not be added, such as ErrNoUser. They are added best effort, so a
// mistyped name never costs the pull request.
var ErrSomePeopleNotAdded = errors.New("some reviewers or assignees could not be added")

// ErrTeamOfAnotherOrg reports a team reviewer of an organization other than
// the repository's, which GitHub cannot be asked for: it names a team by its
// slug alone, within the repository's own organization.
var ErrTeamOfAnotherOrg = errors.New("the team belongs to another organization")

// missedPeople is the reviewers and assignees a pull request could not be
// given, and the first reason each kind of failure gave.
type missedPeople struct {
	names []string
	cause error
}

// miss records a reviewer or assignee that could not be added, and why.
func (m *missedPeople) miss(name string, cause error) {
	m.names = append(m.names, name)

	if !errors.Is(m.cause, cause) {
		m.cause = errors.Join(m.cause, cause)
	}
}

// err is ErrSomePeopleNotAdded naming everyone missed, or nil when nobody
// was.
func (m *missedPeople) err() error {
	if len(m.names) == 0 {
		return nil
	}

	return peopleNotAdded(m.cause, m.names)
}

// peopleNotAdded is ErrSomePeopleNotAdded naming missed, for cause.
func peopleNotAdded(cause error, missed []string) error {
	return fmt.Errorf("%w (%s): %w", ErrSomePeopleNotAdded, strings.Join(missed, ", "), cause)
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

// NewestFirst is the requests ordered with the one opened last first, for a
// look at what just arrived. Requests opened at the same moment keep the
// forge's order.
func NewestFirst(requests []ReviewRequest) []ReviewRequest {
	queue := slices.Clone(requests)
	slices.SortStableFunc(queue, func(left, right ReviewRequest) int {
		return right.OpenedAt.Compare(left.OpenedAt)
	})

	return queue
}

// ByRepository is the requests grouped by repository, in its name's order,
// and the longest-waiting first within each.
func ByRepository(requests []ReviewRequest) []ReviewRequest {
	queue := OldestFirst(requests)
	slices.SortStableFunc(queue, func(left, right ReviewRequest) int {
		return strings.Compare(left.Repository, right.Repository)
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
	assign     func(ctx context.Context, c Client, repo Repo, number int, username string) error

	issueComments  func(ctx context.Context, c Client, repo Repo, number, total int) ([]IssueComment, error)
	commentOnIssue func(ctx context.Context, c Client, repo Repo, number int, text string) (IssueComment, error)

	activity func(ctx context.Context, c Client, start, end time.Time) (Activity, error)
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
			assign: githubAssignIssue, issueComments: githubIssueComments, commentOnIssue: githubCommentOnIssue,
			activity: githubActivity,
		},
		KindGitLab: {
			find: gitlabFind, create: gitlabCreate, update: gitlabUpdate, status: gitlabStatus, rerun: gitlabRerun,
			merge: gitlabMergePull, methods: gitlabMergeMethods,
			reviews: gitlabReviews, issues: gitlabIssues, readIssue: gitlabReadIssue, closeIssue: gitlabCloseIssue,
			assign: gitlabAssignIssue, issueComments: gitlabIssueComments, commentOnIssue: gitlabCommentOnIssue,
			activity: gitlabActivity,
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
// an open one checks IsOpen rather than trusting found alone.
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
