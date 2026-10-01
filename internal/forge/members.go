// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package forge

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"strings"
)

// ErrNotSupported reports a question the forge has no answer for, as GitHub
// has no group whose members could be listed: a team reviews there as a team.
var ErrNotSupported = errors.New("the forge does not offer this")

// gitlabActive is the state GitLab gives a member who can act, as against one
// blocked, deactivated, or invited and not yet accepted.
const gitlabActive = "active"

// gitlabDeveloper is GitLab's Developer access level, the least that may
// approve a merge request: a Guest, Planner or Reporter cannot, so asking one
// to review only adds a name that can never approve.
const gitlabDeveloper = 30

// gitlabMember is a group member as GitLab lists one: the user's id and name
// and state, where the membership itself stands, and the member's role.
type gitlabMember struct {
	ID              int64  `json:"id"`
	AccessLevel     int    `json:"access_level"`
	Username        string `json:"username"`
	State           string `json:"state"`
	MembershipState string `json:"membership_state"`
}

// canReview reports whether the member can review: an active user whose
// membership, when GitLab says, is active too, in a role that may approve.
func (m gitlabMember) canReview() bool {
	return m.State == gitlabActive && (m.MembershipState == "" || m.MembershipState == gitlabActive) &&
		m.AccessLevel >= gitlabDeveloper
}

// GroupMembers lists the usernames of a GitLab group's direct members who can
// review — active, and a Developer or above — the group named by its full
// path, such as "acme/control-plane". Inherited members are left out: a
// CODEOWNERS group means the people put in it. GitHub has no such list, so
// there it is ErrNotSupported.
func (c Client) GroupMembers(ctx context.Context, group string) ([]string, error) {
	if c.kind != KindGitLab {
		return nil, ErrNotSupported
	}

	members, err := gitlabGroupMembers(ctx, c, group)
	if err != nil {
		return nil, err
	}

	usernames := make([]string, 0, len(members))
	for _, member := range members {
		usernames = append(usernames, member.Username)
	}

	return usernames, nil
}

// IsGroup reports whether a bare CODEOWNERS name — @acme, which CODEOWNERS
// spells for a top-level GitLab group just as it does for a user — is a
// group: GitLab knows no user by it, and knows a group by it. A name it knows
// neither by is no group. GitHub spells every team org/team, so there it is
// ErrNotSupported.
func (c Client) IsGroup(ctx context.Context, name string) (bool, error) {
	if c.kind != KindGitLab {
		return false, ErrNotSupported
	}

	users, err := gitlabUsersNamed(ctx, c, name)
	if err != nil || len(users) > 0 {
		return false, err
	}

	_, err = gitlabGroupMembers(ctx, c, name)
	if errors.Is(err, ErrNoAPI) {
		return false, nil
	}

	return err == nil, err
}

// gitlabGroupMembers reads every page of a group's direct members, as far as
// the page cap, and keeps those who can review.
func gitlabGroupMembers(ctx context.Context, client Client, group string) ([]gitlabMember, error) {
	path := "/groups/" + url.PathEscape(group) + "/members?"

	members, err := readPages(func(page int) ([]gitlabMember, int, error) {
		one, err := call[[]gitlabMember](ctx, client, http.MethodGet, path+pageQuery(nil, page), nil)

		return one, uncounted, err
	})
	if err != nil {
		return nil, err
	}

	return slices.DeleteFunc(members, func(member gitlabMember) bool { return !member.canReview() }), nil
}

// gitlabReviewers is a new merge request's reviewers as GitLab sets them, by
// id, each once, and those that could not be resolved, with why. author is the
// id a group's members are added without, zero for none, once authorAsked.
type gitlabReviewers struct {
	missedPeople

	ids         []int64
	author      int64
	authorAsked bool
}

// gitlabResolveReviewers resolves each named reviewer by looking the user up,
// and each team reviewer to its active members by the ids the listing gives.
// A name no user has is tried as a group, since CODEOWNERS spells a top-level
// group @group just as it spells a user. It is best effort: a name that is
// neither, a lookup that fails, or a team whose members cannot be read is
// missed, and the rest still review. The author is left out of every group
// they are in, as nobody reviews their own merge request.
func gitlabResolveReviewers(ctx context.Context, client Client, request NewPullRequest) gitlabReviewers {
	var resolved gitlabReviewers

	for _, username := range request.Reviewers {
		resolved.addUser(ctx, client, username)
	}

	for _, team := range request.TeamReviewers {
		resolved.addTeam(ctx, client, team)
	}

	return resolved
}

// addUser looks a reviewer up by username and adds their id.
func (r *gitlabReviewers) addUser(ctx context.Context, client Client, username string) {
	found, err := gitlabUsersNamed(ctx, client, username)

	switch {
	case err != nil:
		r.miss(username, err)
	case len(found) == 0:
		r.addGroupNamedAsUser(ctx, client, username)
	default:
		r.add(found[0].ID)
	}
}

// addGroupNamedAsUser adds the members of the group a name no user has may
// be, or misses it as no such user when it is no group either.
func (r *gitlabReviewers) addGroupNamedAsUser(ctx context.Context, client Client, name string) {
	members, err := gitlabGroupMembers(ctx, client, name)
	if err != nil {
		r.miss(name, ErrNoUser)

		return
	}

	r.addMembers(ctx, client, members)
}

// addTeam adds the ids of a team's members who can review.
func (r *gitlabReviewers) addTeam(ctx context.Context, client Client, team string) {
	members, err := gitlabGroupMembers(ctx, client, team)
	if err != nil {
		r.miss(team, err)

		return
	}

	r.addMembers(ctx, client, members)
}

// addMembers adds each member's id but the author's.
func (r *gitlabReviewers) addMembers(ctx context.Context, client Client, members []gitlabMember) {
	if !r.authorAsked {
		r.author, r.authorAsked = gitlabAuthorID(ctx, client), true
	}

	for _, member := range members {
		if member.ID != r.author {
			r.add(member.ID)
		}
	}
}

// gitlabAuthorID is the id of the user the token is, or zero when GitLab does
// not say: the author is then left in a team rather than the merge request
// held back for it.
func gitlabAuthorID(ctx context.Context, client Client) int64 {
	self, err := call[gitlabUser](ctx, client, http.MethodGet, userPath, nil)
	if err != nil {
		return 0
	}

	return self.ID
}

// add adds a reviewer's id, unless it is already there.
func (r *gitlabReviewers) add(id int64) {
	if !slices.Contains(r.ids, id) {
		r.ids = append(r.ids, id)
	}
}

// gitlabUser is a user as GitLab's user lookup sends one; only the id is read,
// which is what a reviewer or assignee is set by.
type gitlabUser struct {
	ID int64 `json:"id"`
}

// ErrNoUser reports a username GitLab does not know, so a reviewer or assignee
// named for a pull request cannot be set rather than being dropped in silence.
var ErrNoUser = errors.New("no such user")

// gitlabUserIDs resolves usernames to the ids GitLab wants for an issue's
// assignees. An unknown name is an error, not a silently missing assignee.
func gitlabUserIDs(ctx context.Context, client Client, usernames []string) ([]int64, error) {
	ids, unknown, err := gitlabKnownIDs(ctx, client, usernames)
	if err != nil {
		return nil, err
	}

	if len(unknown) > 0 {
		return nil, fmt.Errorf("%w: %s", ErrNoUser, strings.Join(unknown, ", "))
	}

	return ids, nil
}

// gitlabKnownIDs looks each username up by name, returning the ids of those
// GitLab knows and the names it does not. A lookup that fails is an error.
func gitlabKnownIDs(ctx context.Context, client Client, usernames []string) ([]int64, []string, error) {
	var (
		ids     []int64
		unknown []string
	)

	for _, username := range usernames {
		found, err := gitlabUsersNamed(ctx, client, username)
		if err != nil {
			return nil, nil, err
		}

		if len(found) == 0 {
			unknown = append(unknown, username)

			continue
		}

		ids = append(ids, found[0].ID)
	}

	return ids, unknown, nil
}

// gitlabResolveAssignees resolves each assignee to the id GitLab sets one by.
// It is best effort, as reviewers are: a name GitLab knows no user by, or one
// whose lookup fails, is recorded in missed and the rest are still assigned.
func gitlabResolveAssignees(ctx context.Context, client Client, usernames []string, missed *missedPeople) []int64 {
	var ids []int64

	for _, username := range usernames {
		found, err := gitlabUsersNamed(ctx, client, username)

		switch {
		case err != nil:
			missed.miss(username, err)
		case len(found) == 0:
			missed.miss(username, ErrNoUser)
		default:
			ids = append(ids, found[0].ID)
		}
	}

	return ids
}

// gitlabUsersNamed looks a user up by username: one user, or none when GitLab
// knows nobody by that name.
func gitlabUsersNamed(ctx context.Context, client Client, username string) ([]gitlabUser, error) {
	return call[[]gitlabUser](ctx, client, http.MethodGet, "/users?"+url.Values{"username": {username}}.Encode(), nil)
}
