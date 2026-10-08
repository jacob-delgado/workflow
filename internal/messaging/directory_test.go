// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package messaging_test

import (
	"errors"
	"net/http"
	"net/url"
	"slices"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/httpx"
	"github.com/jacob-delgado/workflow/internal/messaging"
)

// pages answers each Slack read from a body keyed by the request's cursor, and
// records every request it saw.
func pages(t *testing.T, byCursor map[string]string, seen *[]*http.Request) messaging.Client {
	t.Helper()

	var lock sync.Mutex

	return serve(t, func(writer http.ResponseWriter, request *http.Request) {
		lock.Lock()

		*seen = append(*seen, request.Clone(request.Context()))
		lock.Unlock()

		_, _ = writer.Write([]byte(byCursor[request.URL.Query().Get("cursor")]))
	})
}

// slackAnswering is a Slack API that gives every request body.
func slackAnswering(t *testing.T, body string) messaging.Client {
	t.Helper()

	return serve(t, func(writer http.ResponseWriter, _ *http.Request) {
		_, _ = writer.Write([]byte(body))
	})
}

func TestUsersReadsEveryPageKeepingOnlyPeople(t *testing.T) {
	t.Parallel()

	// Arrange
	var seen []*http.Request

	client := pages(t, map[string]string{
		"": `{"ok":true,"members":[
			{"id":"U01","name":"ada","real_name":"Ada L","profile":{"display_name":"ada.l"}},
			{"id":"U02","name":"bob","real_name":"Bob B","profile":{"display_name":""}},
			{"id":"U03","name":"gone","deleted":true,"profile":{}},
			{"id":"B04","name":"robot","is_bot":true,"profile":{}},
			{"id":"USLACKBOT","name":"slackbot","profile":{}}
		],"response_metadata":{"next_cursor":"page2"}}`,
		"page2": `{"ok":true,"members":[
			{"id":"W05","name":"cy","profile":{}},
			{"id":"bad id","name":"mallory","profile":{}}
		],"response_metadata":{"next_cursor":""}}`,
	}, &seen)

	// Act
	users, err := client.Users(t.Context())

	// Assert
	want := []messaging.SlackTarget{{ID: "U01", Label: "ada.l"}, {ID: "U02", Label: "Bob B"}, {ID: "W05", Label: "cy"}}
	if err != nil || !slices.Equal(users, want) {
		t.Fatalf("Users = %v, %v; want %v", users, err, want)
	}

	if len(seen) != 2 || seen[0].URL.Path != "/users.list" || seen[0].URL.Query().Get("limit") != "200" {
		t.Errorf("requests = %d, first %v; want two pages of users.list?limit=200", len(seen), seen[0].URL)
	}
}

func TestReadsSendTheTokenAsABearerGet(t *testing.T) {
	t.Parallel()

	// Arrange
	var seen []*http.Request

	client := pages(t, map[string]string{"": `{"ok":true,"members":[]}`}, &seen)

	// Act
	_, err := client.Users(t.Context())

	// Assert
	if err != nil || len(seen) != 1 {
		t.Fatalf("Users = %v after %d requests, want one clean read", err, len(seen))
	}

	if seen[0].Method != http.MethodGet || seen[0].Header.Get("Authorization") != "Bearer "+userToken {
		t.Errorf("request = %s with %q, want a GET carrying the token as a bearer",
			seen[0].Method, seen[0].Header.Get("Authorization"))
	}

	if seen[0].URL.Query().Has("token") {
		t.Errorf("query = %q, want the token kept out of the URL", seen[0].URL.RawQuery)
	}
}

func TestChannelMembersReadsTheChannelsUserIDs(t *testing.T) {
	t.Parallel()

	// Arrange
	var seen []*http.Request

	client := pages(t, map[string]string{
		"":   `{"ok":true,"members":["U01","U02"],"response_metadata":{"next_cursor":"c2"}}`,
		"c2": `{"ok":true,"members":["W03","<!channel>"]}`,
	}, &seen)

	// Act
	members, err := client.ChannelMembers(t.Context(), "C0123")

	// Assert
	got := make([]string, 0, len(members))
	for _, member := range members {
		got = append(got, member.String())
	}

	if err != nil || !slices.Equal(got, []string{"U01", "U02", "W03"}) {
		t.Fatalf("ChannelMembers = %v, %v; want the three valid user IDs", got, err)
	}

	query := seen[0].URL.Query()
	if seen[0].URL.Path != "/conversations.members" || query.Get("channel") != "C0123" || query.Get("limit") != "1000" {
		t.Errorf("first request = %v, want conversations.members?channel=C0123&limit=1000", seen[0].URL)
	}
}

func TestUserGroupsLabelsEachByItsHandle(t *testing.T) {
	t.Parallel()

	// Arrange
	var seen []*http.Request

	client := pages(t, map[string]string{"": `{"ok":true,"usergroups":[
		{"id":"S01","handle":"control-plane-pod","name":"Control Plane"},
		{"id":"S02","handle":"","name":"Design"},
		{"id":"X03","handle":"nope","name":"Not a group"}
	]}`}, &seen)

	// Act
	groups, err := client.UserGroups(t.Context())

	// Assert
	want := []messaging.SlackTarget{{ID: "S01", Label: "control-plane-pod"}, {ID: "S02", Label: "Design"}}
	if err != nil || !slices.Equal(groups, want) {
		t.Fatalf("UserGroups = %v, %v; want %v", groups, err, want)
	}

	if seen[0].URL.Path != "/usergroups.list" || seen[0].URL.Query().Get("include_disabled") != "false" {
		t.Errorf("request = %v, want usergroups.list?include_disabled=false", seen[0].URL)
	}
}

func TestChannelIDFindsTheNamedChannel(t *testing.T) {
	t.Parallel()

	// Arrange
	var seen []*http.Request

	client := pages(t, map[string]string{
		"":  `{"ok":true,"channels":[{"id":"C01","name":"general"}],"response_metadata":{"next_cursor":"n"}}`,
		"n": `{"ok":true,"channels":[{"id":"G02","name":"dev"}]}`,
	}, &seen)

	// Act
	id, err := client.ChannelID(t.Context(), "#dev")

	// Assert
	if err != nil || id != "G02" {
		t.Fatalf("ChannelID = %q, %v; want G02", id, err)
	}

	want := url.Values{
		"types": {"public_channel,private_channel"}, "exclude_archived": {"true"}, "limit": {"200"},
	}
	query := seen[0].URL.Query()

	if seen[0].URL.Path != "/users.conversations" || query.Encode() != want.Encode() {
		t.Errorf("first request = %v, want users.conversations?%s", seen[0].URL, want.Encode())
	}
}

func TestChannelIDTakesAnIDAsItIs(t *testing.T) {
	t.Parallel()

	for _, value := range []string{"C0123ABC", "#G0123ABC"} {
		t.Run(value, func(t *testing.T) {
			t.Parallel()

			// Arrange
			var sent atomic.Bool

			client := messaging.New(counting(&sent), messaging.APIBase, userCredentials()).WithToken(heldToken)

			// Act
			id, err := client.ChannelID(t.Context(), value)

			// Assert
			if err != nil || "#"+id != value && id != value || sent.Load() {
				t.Errorf("ChannelID(%q) = %q, %v (sent %t); want the ID itself, nothing asked",
					value, id, err, sent.Load())
			}
		})
	}
}

func TestChannelIDReportsAChannelItCannotFind(t *testing.T) {
	t.Parallel()

	// Arrange
	client := slackAnswering(t, `{"ok":true,"channels":[{"id":"C01","name":"general"}]}`)

	// Act
	_, err := client.ChannelID(t.Context(), "dev")

	// Assert
	if !errors.Is(err, messaging.ErrChannelNotFound) {
		t.Errorf("ChannelID = %v, want ErrChannelNotFound", err)
	}
}

func TestAReadWithoutItsScopeNamesTheScopeNeeded(t *testing.T) {
	t.Parallel()

	// Arrange
	client := slackAnswering(t, `{"ok":false,"error":"missing_scope","needed":"usergroups:read","provided":"chat:write"}`)

	// Act
	_, err := client.UserGroups(t.Context())

	// Assert
	var missing *messaging.MissingScopeError
	if !errors.Is(err, messaging.ErrMissingScope) || !errors.As(err, &missing) || missing.Needed != "usergroups:read" {
		t.Errorf("UserGroups = %v, want ErrMissingScope needing usergroups:read", err)
	}

	if errors.Is(err, messaging.ErrRejected) {
		t.Errorf("UserGroups = %v, want a missing scope kept apart from a rejected token", err)
	}
}

func TestAWorkspaceWithoutUserGroupsSaysSo(t *testing.T) {
	t.Parallel()

	for _, code := range []string{"paid_teams_only", "not_allowed_token_type"} {
		t.Run(code, func(t *testing.T) {
			t.Parallel()

			// Arrange
			client := slackAnswering(t, `{"ok":false,"error":"`+code+`"}`)

			// Act
			_, err := client.UserGroups(t.Context())

			// Assert
			if !errors.Is(err, messaging.ErrNoUserGroups) {
				t.Errorf("UserGroups = %v, want ErrNoUserGroups", err)
			}
		})
	}
}

func TestAReadSlackRefusesIsReported(t *testing.T) {
	t.Parallel()

	// Arrange
	client := slackAnswering(t, `{"ok":false,"error":"channel_not_found"}`)

	// Act
	_, err := client.ChannelMembers(t.Context(), "C0123")

	// Assert
	if !errors.Is(err, messaging.ErrLookupRefused) {
		t.Errorf("ChannelMembers = %v, want ErrLookupRefused", err)
	}
}

func TestAReadWithARejectedTokenIsRejected(t *testing.T) {
	t.Parallel()

	// Arrange
	client := slackAnswering(t, `{"ok":false,"error":"invalid_auth"}`)

	// Act
	_, err := client.Users(t.Context())

	// Assert
	if !errors.Is(err, messaging.ErrRejected) {
		t.Errorf("Users = %v, want ErrRejected", err)
	}
}

func TestAReadSlackCallsExpiredIsMadeAgainWithANewerToken(t *testing.T) {
	t.Parallel()

	// Arrange
	server, seen := slackExpiringOnce(t, `{"ok":true,"members":[{"id":"U01","name":"ada","profile":{}}]}`)
	source := &rotating{}
	client := messaging.New(server.Client().Do, server.URL, userCredentials()).WithToken(source.token)

	// Act
	users, err := client.Users(t.Context())

	// Assert
	if err != nil || len(users) != 1 || len(*seen) != 2 {
		t.Errorf("Users = %v, %v after %d requests; want the read made again", users, err, len(*seen))
	}
}

func TestARateLimitedReadSaysWhenToTryAgain(t *testing.T) {
	t.Parallel()

	// Arrange
	client := serve(t, func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Retry-After", "30")
		writer.WriteHeader(http.StatusTooManyRequests)
	})

	// Act
	_, err := client.Users(t.Context())

	// Assert
	if !errors.Is(err, httpx.ErrRateLimited) {
		t.Errorf("Users = %v, want httpx.ErrRateLimited", err)
	}
}

func TestAReadThatNeverEndsStopsAtThePageCap(t *testing.T) {
	t.Parallel()

	// Arrange
	var requests atomic.Int64

	client := serve(t, func(writer http.ResponseWriter, _ *http.Request) {
		requests.Add(1)

		_, _ = writer.Write([]byte(`{"ok":true,"members":[],"response_metadata":{"next_cursor":"again"}}`))
	})

	// Act
	_, err := client.Users(t.Context())

	// Assert
	if !errors.Is(err, messaging.ErrDirectoryTooLarge) || requests.Load() != messaging.UserListPages {
		t.Errorf("Users = %v after %d requests, want ErrDirectoryTooLarge after %d",
			err, requests.Load(), messaging.UserListPages)
	}
}

func TestAWebhookHasNoDirectory(t *testing.T) {
	t.Parallel()

	// Arrange
	var sent atomic.Bool

	creds := config.Messaging{Kind: config.KindSlack, WebhookURL: "https://hooks.example/x"}
	client := messaging.New(counting(&sent), messaging.APIBase, creds).WithToken(heldToken)

	// Act
	_, err := client.Users(t.Context())

	// Assert
	if !errors.Is(err, messaging.ErrNoCredential) || sent.Load() {
		t.Errorf("Users = %v (sent %t), want no credential and nothing sent", err, sent.Load())
	}
}

func TestParseSlackIDsAcceptsOnlyTheirShape(t *testing.T) {
	t.Parallel()

	cases := []struct {
		raw     string
		userOK  bool
		groupOK bool
	}{
		{raw: "U012AB", userOK: true},
		{raw: "W0ABC", userOK: true},
		{raw: "S0ABC", groupOK: true},
		{raw: "U1"},
		{raw: "u012ab"},
		{raw: "U012 AB"},
		{raw: "<!channel>"},
		{raw: ""},
		{raw: "S0abc"},
	}

	for _, test := range cases {
		t.Run(test.raw, func(t *testing.T) {
			t.Parallel()

			// Act
			user, userErr := messaging.ParseSlackUser(test.raw)
			group, groupErr := messaging.ParseSlackGroup(test.raw)

			// Assert
			if (userErr == nil) != test.userOK || test.userOK && user.String() != test.raw {
				t.Errorf("ParseSlackUser(%q) = %q, %v; want accepted %t", test.raw, user, userErr, test.userOK)
			}

			if (groupErr == nil) != test.groupOK || test.groupOK && group.String() != test.raw {
				t.Errorf("ParseSlackGroup(%q) = %q, %v; want accepted %t", test.raw, group, groupErr, test.groupOK)
			}
		})
	}
}

func TestADirectoryReadAnsweredWithA4xxIsNoMessageRefused(t *testing.T) {
	t.Parallel()

	// Arrange
	// Only a post has a message to refuse.
	client := serve(t, func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusBadRequest)
		_, _ = writer.Write([]byte("invalid_arguments"))
	})

	// Act
	_, err := client.Users(t.Context())

	// Assert
	if !errors.Is(err, messaging.ErrRejected) || errors.Is(err, messaging.ErrPostRefused) {
		t.Errorf("Users = %v, want the read rejected, not a message refused", err)
	}
}
