// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package messaging

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"net/http"
	"net/url"
	"slices"
	"strings"

	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/sanitize"
)

// MaxPages is the most pages one directory read follows. A cursor that never
// runs out — a workspace past it, or a server that loops — ends the read
// rather than the session's patience.
const MaxPages = 100

// UserListPages is the most pages Users reads before calling the workspace
// too large to list whole. users.list is Slack's Tier 2, about 20 requests a
// minute, so a workspace past 20 pages — 4,000 people at 200 a page — would
// meet the rate limit rather than the page cap; its channels are better
// labeled member by member through users.info, Tier 4 at 100 or more a
// minute and bounded by the channel's size.
const UserListPages = 20

// The directory reads, as the Web API names them.
const (
	conversationsPath = "/users.conversations"
	membersPath       = "/conversations.members"
	usersPath         = "/users.list"
	userPath          = "/users.info"
	userGroupsPath    = "/usergroups.list"
)

// slackbotID is the built-in bot users.list returns without marking it a bot.
const slackbotID = "USLACKBOT"

// limitParam is the query parameter that sizes a page.
const limitParam = "limit"

// Errors the directory reads return, beside the ones every Slack call shares.
var (
	// ErrMissingScope reports a token without the scope a read needs; a
	// MissingScopeError carrying the scope's name wraps it.
	ErrMissingScope = errors.New("the Slack token lacks a scope")
	// ErrNoUserGroups reports a workspace or token that has no user groups to
	// read: a free workspace, or a token Slack will not answer them for.
	ErrNoUserGroups = errors.New("this Slack workspace has no user groups")
	// ErrLookupRefused reports a directory read Slack would not answer, for a
	// reason that is neither the token nor its scopes.
	ErrLookupRefused = errors.New("the lookup was refused")
	// ErrChannelNotFound reports a channel name none of the user's channels has.
	ErrChannelNotFound = errors.New("no channel of yours has that name")
	// ErrDirectoryTooLarge reports a read still paging after its page cap:
	// MaxPages, or UserListPages for Users.
	ErrDirectoryTooLarge = errors.New("the Slack directory is too large to read")
	// ErrNotTaggable reports a user who is not a person to tag: deactivated,
	// a bot, or Slackbot.
	ErrNotTaggable = errors.New("not a Slack user who can be tagged")
	// ErrInvalidSlackID reports a value that is not a Slack user or group ID.
	ErrInvalidSlackID = errors.New("not a Slack ID")
)

// MissingScopeError is Slack's missing_scope, naming the scope it needs.
type MissingScopeError struct {
	// Needed is Slack's own name for the scope, such as "usergroups:read".
	Needed string
}

// Error names the scope the token lacks.
func (e *MissingScopeError) Error() string {
	return ErrMissingScope.Error() + ": " + e.Needed
}

// Unwrap makes every MissingScopeError an ErrMissingScope to errors.Is.
func (e *MissingScopeError) Unwrap() error {
	return ErrMissingScope
}

// SlackTarget is someone a message can tag: a user or a user group, by its ID
// and the name to show for it.
type SlackTarget struct {
	ID    string
	Label string
}

// SlackUserID is a validated Slack user ID. It is made only by ParseSlackUser,
// so one placed in a mention cannot carry markup.
type SlackUserID struct{ id string }

// String is the ID itself.
func (u SlackUserID) String() string { return u.id }

// SlackGroupID is a validated Slack user group ID. It is made only by
// ParseSlackGroup, so one placed in a mention cannot carry markup.
type SlackGroupID struct{ id string }

// String is the ID itself.
func (g SlackGroupID) String() string { return g.id }

// ParseSlackUser accepts a user ID: U or W, then two or more of A–Z and 0–9.
func ParseSlackUser(raw string) (SlackUserID, error) {
	if !slackID(raw, "UW") {
		return SlackUserID{}, fmt.Errorf("%w: a user ID is U or W then capitals and digits", ErrInvalidSlackID)
	}

	return SlackUserID{id: raw}, nil
}

// ParseSlackGroup accepts a user group ID: S, then two or more of A–Z and 0–9.
func ParseSlackGroup(raw string) (SlackGroupID, error) {
	if !slackID(raw, "S") {
		return SlackGroupID{}, fmt.Errorf("%w: a user group ID is S then capitals and digits", ErrInvalidSlackID)
	}

	return SlackGroupID{id: raw}, nil
}

// slackID reports whether raw is one of prefixes followed by at least two
// capitals or digits — the whole of what a Slack ID may hold.
func slackID(raw, prefixes string) bool {
	const shortestTail = 2

	if len(raw) < 1+shortestTail || !strings.ContainsRune(prefixes, rune(raw[0])) {
		return false
	}

	for _, char := range raw[1:] {
		if (char < 'A' || char > 'Z') && (char < '0' || char > '9') {
			return false
		}
	}

	return true
}

// listing is a page of a directory read: Slack's verdict, the scope it wants
// when the verdict is missing_scope, and the cursor to the next page.
type listing struct {
	OK       bool   `json:"ok"`
	Error    string `json:"error"`
	Needed   string `json:"needed"`
	Metadata struct {
		NextCursor string `json:"next_cursor"`
	} `json:"response_metadata"`
}

// lookup is one directory read: the endpoint, its fixed query, the error
// codes it gives a meaning of its own, and the most pages it follows.
type lookup struct {
	path     string
	query    url.Values
	refusals map[string]error
	pages    int
}

// channelPage is a page of users.conversations.
type channelPage struct {
	Channels []struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"channels"`
}

// ChannelID is the ID of the channel the user is in by name, with or without
// its leading "#". A value already shaped like a channel ID is the answer
// itself, and nothing is asked.
func (c Client) ChannelID(ctx context.Context, name string) (string, error) {
	name = strings.TrimPrefix(name, "#")
	if slackID(name, "CG") {
		return name, nil
	}

	read := lookup{path: conversationsPath, query: url.Values{
		"types": {"public_channel,private_channel"}, "exclude_archived": {"true"}, limitParam: {"200"},
	}, refusals: nil, pages: MaxPages}

	ids, err := readAll(ctx, c, read, func(page channelPage) []string {
		named := make([]string, 0, 1)

		for _, channel := range page.Channels {
			if strings.EqualFold(channel.Name, name) && slackID(channel.ID, "CG") {
				named = append(named, channel.ID)
			}
		}

		return named
	})
	if err != nil {
		return "", err
	}

	if len(ids) == 0 {
		return "", ErrChannelNotFound
	}

	return ids[0], nil
}

// memberPage is a page of conversations.members.
type memberPage struct {
	Members []string `json:"members"`
}

// ChannelMembers is the user ID of everyone in the channel, leaving out any
// value Slack sent that is not shaped like one.
func (c Client) ChannelMembers(ctx context.Context, channelID string) ([]SlackUserID, error) {
	read := lookup{
		path: membersPath, query: url.Values{"channel": {channelID}, limitParam: {"1000"}}, refusals: nil, pages: MaxPages,
	}

	return readAll(ctx, c, read, func(page memberPage) []SlackUserID {
		users := make([]SlackUserID, 0, len(page.Members))

		for _, value := range page.Members {
			user, err := ParseSlackUser(value)
			if err == nil {
				users = append(users, user)
			}
		}

		return users
	})
}

// slackUser is a users.list member: the fields that say whether it is a person
// to tag, and the names it might go by.
type slackUser struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	RealName string `json:"real_name"`
	Deleted  bool   `json:"deleted"`
	IsBot    bool   `json:"is_bot"`
	Profile  struct {
		DisplayName string `json:"display_name"`
		RealName    string `json:"real_name"`
	} `json:"profile"`
}

// userPage is a page of users.list.
type userPage struct {
	Members []slackUser `json:"members"`
}

// Users is every person in the workspace who can be tagged, labeled by the
// name Slack shows for them. Deactivated users, bots and Slackbot are left out.
// A workspace past UserListPages pages is ErrDirectoryTooLarge.
func (c Client) Users(ctx context.Context) ([]SlackTarget, error) {
	read := lookup{path: usersPath, query: url.Values{limitParam: {"200"}}, refusals: nil, pages: UserListPages}

	return readAll(ctx, c, read, func(page userPage) []SlackTarget {
		targets := make([]SlackTarget, 0, len(page.Members))

		for _, member := range page.Members {
			if member.taggable() {
				targets = append(targets, SlackTarget{ID: member.ID, Label: member.label()})
			}
		}

		return targets
	})
}

// userAnswer is users.info's answer.
type userAnswer struct {
	User slackUser `json:"user"`
}

// User is one person by their ID, labeled as Users labels them. Someone Users
// would leave out — deactivated, a bot, Slackbot — is ErrNotTaggable.
func (c Client) User(ctx context.Context, user SlackUserID) (SlackTarget, error) {
	read := lookup{path: userPath, query: url.Values{"user": {user.String()}}, refusals: nil, pages: 1}

	found, err := readAll(ctx, c, read, func(answer userAnswer) []slackUser { return []slackUser{answer.User} })
	if err != nil {
		return SlackTarget{}, err
	}

	if !found[0].taggable() || found[0].ID != user.String() {
		return SlackTarget{}, fmt.Errorf("%w: %s", ErrNotTaggable, user)
	}

	return SlackTarget{ID: found[0].ID, Label: found[0].label()}, nil
}

// taggable reports whether the member is a person with a valid ID.
func (u slackUser) taggable() bool {
	return !u.Deleted && !u.IsBot && u.ID != slackbotID && slackID(u.ID, "UW")
}

// label is the first name the user has: the display name, the real name, then
// the account name.
func (u slackUser) label() string {
	return cmp.Or(u.Profile.DisplayName, u.RealName, u.Profile.RealName, u.Name)
}

// groupPage is usergroups.list's answer, which comes in one page.
type groupPage struct {
	Groups []struct {
		ID     string `json:"id"`
		Handle string `json:"handle"`
		Name   string `json:"name"`
	} `json:"usergroups"`
}

// UserGroups is every enabled user group in the workspace, labeled by its
// handle, or its name when it has none. A workspace without user groups is
// ErrNoUserGroups.
func (c Client) UserGroups(ctx context.Context) ([]SlackTarget, error) {
	read := lookup{path: userGroupsPath, query: url.Values{"include_disabled": {"false"}}, refusals: map[string]error{
		"paid_teams_only":        ErrNoUserGroups,
		"not_allowed_token_type": ErrNoUserGroups,
	}, pages: MaxPages}

	return readAll(ctx, c, read, func(page groupPage) []SlackTarget {
		groups := make([]SlackTarget, 0, len(page.Groups))

		for _, group := range page.Groups {
			if slackID(group.ID, "S") {
				groups = append(groups, SlackTarget{ID: group.ID, Label: cmp.Or(group.Handle, group.Name)})
			}
		}

		return groups
	})
}

// readAll follows read's pages to the end, collecting what pick finds in each.
func readAll[Page, Item any](ctx context.Context, client Client, read lookup, pick func(Page) []Item) ([]Item, error) {
	if client.creds.Mode() != config.MessagingUser {
		return nil, ErrNoCredential
	}

	var (
		all    []Item
		cursor string
	)

	for range read.pages {
		body, next, err := client.readPage(ctx, read, cursor)
		if err != nil {
			return nil, err
		}

		var page Page

		err = json.Unmarshal(body, &page)
		if err != nil {
			return nil, fmt.Errorf("reading the answer from Slack: %w", err)
		}

		all = append(all, pick(page)...)
		if next == "" {
			return all, nil
		}

		cursor = next
	}

	return nil, ErrDirectoryTooLarge
}

// readPage reads one page of read at cursor, with a newer token when Slack
// calls the first one expired. It returns the sanitized body and the next
// page's cursor.
func (c Client) readPage(ctx context.Context, read lookup, cursor string) ([]byte, string, error) {
	query := maps.Clone(read.query)

	if cursor != "" {
		query.Set("cursor", cursor)
	}

	var (
		body []byte
		page listing
	)

	err := c.withFreshToken(ctx, func(token config.Secret) error {
		var err error

		body, page, err = c.get(ctx, token, c.base+read.path+"?"+query.Encode(), read.refusals)

		return err
	})

	return body, page.Metadata.NextCursor, err
}

// get performs one directory GET with token and reads Slack's verdict. Slack
// answers a refusal with 200 and ok:false, so the status alone says nothing.
func (c Client) get(
	ctx context.Context, token config.Secret, address string, refusals map[string]error,
) ([]byte, listing, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, address, nil)
	if err != nil {
		// Unwrapped: the parse error quotes the whole URL.
		return nil, listing{}, fmt.Errorf("%w: building the request", ErrUnreachable)
	}

	request.Header.Set("Authorization", "Bearer "+token.Reveal())
	request.Header.Set("Accept", "application/json")

	raw, err := c.deliver(request, refusedRead)
	if err != nil {
		return nil, listing{}, err
	}

	// Names are about to be shown, and Slack's own error printed.
	body := sanitize.JSON(raw)

	var page listing

	err = json.Unmarshal(body, &page)
	if err != nil {
		return nil, listing{}, fmt.Errorf("reading the answer from Slack: %w", err)
	}

	if !page.OK {
		return nil, listing{}, lookupRefusal(page, refusals)
	}

	return body, page, nil
}

// lookupRefusal is Slack's ok:false to a read: a scope the token lacks, a
// meaning the read gives the code, the token itself, or any other refusal.
func lookupRefusal(page listing, refusals map[string]error) error {
	if page.Error == "missing_scope" {
		return &MissingScopeError{Needed: page.Needed}
	}

	if meaning, known := refusals[page.Error]; known {
		return fmt.Errorf("%w: %s", meaning, page.Error)
	}

	if slices.Contains(credentialCodes(), page.Error) {
		return credentialRefusal(page.Error)
	}

	return fmt.Errorf("%w: %s", ErrLookupRefused, page.Error)
}
