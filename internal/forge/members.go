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

// gitlabMember is a group member as GitLab lists one: the user's name and
// state, and where the membership itself stands.
type gitlabMember struct {
	Username        string `json:"username"`
	State           string `json:"state"`
	MembershipState string `json:"membership_state"`
}

// active reports whether the member can review: an active user whose
// membership, when GitLab says, is active too.
func (m gitlabMember) active() bool {
	return m.State == gitlabActive && (m.MembershipState == "" || m.MembershipState == gitlabActive)
}

// GroupMembers lists the usernames of a GitLab group's active direct members,
// the group named by its full path, such as "acme/control-plane". Inherited
// members are left out: a CODEOWNERS group means the people put in it. GitHub
// has no such list, so there it is ErrNotSupported.
func (c Client) GroupMembers(ctx context.Context, group string) ([]string, error) {
	if c.kind != KindGitLab {
		return nil, ErrNotSupported
	}

	return gitlabGroupMembers(ctx, c, group)
}

// gitlabGroupMembers reads every page of a group's direct members, as far as
// the page cap, and keeps the active ones' usernames.
func gitlabGroupMembers(ctx context.Context, client Client, group string) ([]string, error) {
	path := "/groups/" + url.PathEscape(group) + "/members?"

	members, err := readPages(func(page int) ([]gitlabMember, int, error) {
		one, err := call[[]gitlabMember](ctx, client, http.MethodGet, path+pageQuery(nil, page), nil)

		return one, uncounted, err
	})
	if err != nil {
		return nil, err
	}

	usernames := make([]string, 0, len(members))

	for _, member := range members {
		if member.active() {
			usernames = append(usernames, member.Username)
		}
	}

	return usernames, nil
}

// gitlabTeamsExpanded is the reviewers named with each team's active members
// added after them, each once. A team whose members cannot be read is left
// out and named in unread, with why the first one could not be.
func gitlabTeamsExpanded(ctx context.Context, client Client, request NewPullRequest) ([]string, []string, error) {
	names := slices.Clone(request.Reviewers)

	var (
		unread []string
		cause  error
	)

	for _, team := range request.TeamReviewers {
		members, err := gitlabGroupMembers(ctx, client, team)
		if err == nil {
			names = withNew(names, members)

			continue
		}

		unread = append(unread, team)

		if cause == nil {
			cause = err
		}
	}

	return names, unread, cause
}

// withNew is names with each of more not already in it added, in order.
func withNew(names, more []string) []string {
	for _, name := range more {
		if !slices.Contains(names, name) {
			names = append(names, name)
		}
	}

	return names
}

// gitlabUser is a user as GitLab's user lookup sends one; only the id is read,
// which is what a reviewer or assignee is set by.
type gitlabUser struct {
	ID int64 `json:"id"`
}

// ErrNoUser reports a username GitLab does not know, so a reviewer or assignee
// named for a pull request cannot be set rather than being dropped in silence.
var ErrNoUser = errors.New("no such user")

// gitlabUserIDs resolves usernames to the ids GitLab wants for assignees. An
// unknown name is an error, not a silently missing assignee.
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
		found, err := call[[]gitlabUser](ctx, client, http.MethodGet,
			"/users?"+url.Values{"username": {username}}.Encode(), nil)
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

// gitlabMissedReviewers is ErrSomeReviewersNotAdded for the teams whose
// members could not be read, for teamCause, and the reviewers GitLab does not
// know; or nil when every one was added.
func gitlabMissedReviewers(unread []string, teamCause error, unknown []string) error {
	var noUser error
	if len(unknown) > 0 {
		noUser = ErrNoUser
	}

	cause := errors.Join(teamCause, noUser)
	if cause == nil {
		return nil
	}

	return reviewersNotAdded(cause, append(unread, unknown...))
}
