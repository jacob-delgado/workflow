// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package webserver_test

// Settings' People and groups area, and the announcement preview's tags:
// whom each code owner is on Slack, and the user groups a repository tags.

import (
	"fmt"
	"net/http"
	"reflect"
	"strings"
	"testing"

	"github.com/jacob-delgado/workflow/internal/api"
	"github.com/jacob-delgado/workflow/internal/codeowners"
	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/loop"
	"github.com/jacob-delgado/workflow/internal/messaging"
	"github.com/jacob-delgado/workflow/internal/webserver"
)

// The owners, scope, group and paths the tests name.
const (
	benOwner       = "ben"
	carlaOwner     = "carla"
	danOwner       = "dan"
	usersRead      = "users:read"
	apiGroupID     = "S0API"
	peoplePath     = "/api/people"
	repoGroupsPath = "/api/repo-groups"
	noStore        = "store is off"
	groupsPath     = "/api/slack/groups"
	groupsRead     = "usergroups:read"
)

// carla is a Slack member a user owner is linked to.
func carla() loop.SlackTarget { return loop.SlackTarget{ID: "U0CARLA", Label: "Carla Diaz"} }

// ben is a Slack member no owner is linked to yet.
func ben() loop.SlackTarget { return loop.SlackTarget{ID: "U0BEN", Label: "Ben Ito"} }

// podGroup() is a user group a team can be linked to.
func podGroup() loop.SlackTarget { return loop.SlackTarget{ID: "S0POD", Label: "control-plane-pod"} }

// apiReviews() is the repository's own user group.
func apiReviews() loop.SlackTarget { return loop.SlackTarget{ID: apiGroupID, Label: "api-reviewers"} }

// ownerLinkedTo is owner linked to target on Slack.
func ownerLinkedTo(owner string, target loop.SlackTarget) loop.OwnerLink {
	return loop.OwnerLink{Owner: owner, OnSlack: true, Slack: target}
}

// fakeKept is the kept associations and the Slack directory, as a server
// reads and writes them.
type fakeKept struct {
	links      []loop.OwnerLink
	repoGroups []loop.SlackTarget
	last       []string
	chosen     bool
	recorded   [][]string
	members    []loop.SlackTarget
	groups     []loop.SlackTarget
	membersErr error
	groupsErr  error
}

// newFakeKept holds carla linked, dan not on Slack, and the repository's two
// groups, with members ben and carla and both groups in the directory.
func newFakeKept() *fakeKept {
	return &fakeKept{
		links: []loop.OwnerLink{
			{Owner: carlaOwner, OnSlack: true, Slack: carla()},
			{Owner: danOwner, OnSlack: false, Slack: loop.SlackTarget{}},
		},
		repoGroups: []loop.SlackTarget{apiReviews()},
		members:    []loop.SlackTarget{ben(), carla()},
		groups:     []loop.SlackTarget{podGroup(), apiReviews()},
	}
}

// wire hands the fake to deps, with the branch's changes owned by ben, carla
// and the acme/control-plane team.
func (f *fakeKept) wire(deps *webserver.Deps) {
	deps.ChangedPaths = func(string) ([]string, error) { return []string{"internal/api/server.go"}, nil }
	deps.CodeOwnersAt = func(string) (codeowners.File, bool, error) {
		return codeowners.Parse("internal/ @ben @carla @acme/control-plane\n", codeowners.GitHub), true, nil
	}
	deps.OwnerLinks = func() ([]loop.OwnerLink, error) { return f.links, nil }
	deps.LinkOwner = f.link
	deps.ForgetOwner = func(owner string) error {
		f.links = f.without(owner)

		return nil
	}
	deps.RepoGroups = func() ([]loop.SlackTarget, error) { return f.repoGroups, nil }
	deps.SetRepoGroups = func(groups []loop.SlackTarget) error {
		f.repoGroups = groups

		return nil
	}
	deps.LastGroups = func() ([]string, bool) { return f.last, f.chosen }
	deps.RecordGroups = func(ids []string) error {
		f.recorded = append(f.recorded, ids)

		return nil
	}
	deps.ChannelMembers = func(string) ([]loop.SlackTarget, error) { return f.members, f.membersErr }
	deps.UserGroups = func() ([]loop.SlackTarget, error) { return f.groups, f.groupsErr }
}

// link records whom owner is, replacing what was decided.
func (f *fakeKept) link(owner string, target *loop.SlackTarget) error {
	link := loop.OwnerLink{Owner: owner, OnSlack: target != nil, Slack: loop.SlackTarget{}}
	if target != nil {
		link.Slack = *target
	}

	f.links = append(f.without(owner), link)

	return nil
}

// without is the links but owner's.
func (f *fakeKept) without(owner string) []loop.OwnerLink {
	var kept []loop.OwnerLink

	for _, link := range f.links {
		if link.Owner != owner {
			kept = append(kept, link)
		}
	}

	return kept
}

// keptServer is a server over fake, under info.
func keptServer(t *testing.T, fake *fakeKept, info webserver.Info) http.Handler {
	t.Helper()

	deps := filledDeps()
	deps.Post = func(string, string) error { return nil }
	fake.wire(&deps)

	return serveWith(t, deps, slackUserConfig(), info)
}

// slackUserConfig is a configuration that posts with a Slack user token,
// which tagging and its directory need.
func slackUserConfig() config.Config {
	cfg := config.Default()
	cfg.Messaging.ClientID = slackApp
	cfg.Messaging.Channel = slackChannel

	return cfg
}

// slackTarget is a wire Slack target.
func slackTarget(target loop.SlackTarget) *api.SlackTarget {
	wire := api.SlackTarget(target)

	return &wire
}

func TestGetPeopleListsTheDecidedOwnersThenTheUndecidedOnes(t *testing.T) {
	t.Parallel()

	// Arrange
	handler := keptServer(t, newFakeKept(), webserver.Info{Version: testVersion})

	// Act
	recorder := get(t, handler, peoplePath)

	// Assert
	want := api.People{Owners: []api.OwnerTag{
		{Owner: carlaOwner, Kind: api.User, State: api.Linked, Slack: slackTarget(carla())},
		{Owner: danOwner, Kind: api.User, State: api.NotOnSlack, Slack: nil},
		{Owner: benOwner, Kind: api.User, State: api.Unlinked, Slack: nil},
		{Owner: ownedTeam, Kind: api.Team, State: api.Unlinked, Slack: nil},
	}}
	if got := decode[api.People](t, recorder); recorder.Code != http.StatusOK || !reflect.DeepEqual(got, want) {
		t.Errorf("GET /api/people = %d %+v, want 200 %+v", recorder.Code, got, want)
	}
}

func TestLinkPersonKeepsTheLabelSlacksDirectoryGives(t *testing.T) {
	t.Parallel()

	// Arrange
	fake := newFakeKept()
	handler := keptServer(t, fake, webserver.Info{Version: testVersion})

	// Act
	recorder := send(t, handler, http.MethodPut, peoplePath, `{"owner":"ben","slack_id":"U0BEN","not_on_slack":false}`)

	// Assert
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d (%s), want 200", recorder.Code, recorder.Body.String())
	}

	want := loop.OwnerLink{Owner: benOwner, OnSlack: true, Slack: ben()}
	if got := fake.links[len(fake.links)-1]; got != want {
		t.Errorf("kept %+v, want %+v labeled from the directory", got, want)
	}
}

func TestLinkPersonLinksATeamToAUserGroup(t *testing.T) {
	t.Parallel()

	// Arrange
	fake := newFakeKept()
	handler := keptServer(t, fake, webserver.Info{Version: testVersion})

	// Act
	recorder := send(t, handler, http.MethodPut, peoplePath,
		`{"owner":"acme/control-plane","slack_id":"S0POD","not_on_slack":false}`)

	// Assert
	people := decode[api.People](t, recorder)
	team := people.Owners[len(people.Owners)-1]

	linked := team.State == api.Linked && reflect.DeepEqual(team.Slack, slackTarget(podGroup()))
	if team.Owner != ownedTeam || !linked {
		t.Errorf("the team reads %+v after linking, want it linked to %+v", team, podGroup())
	}
}

func TestLinkPersonMarksAnOwnerNotOnSlack(t *testing.T) {
	t.Parallel()

	// Arrange
	fake := newFakeKept()
	handler := keptServer(t, fake, webserver.Info{Version: testVersion})

	// Act
	recorder := send(t, handler, http.MethodPut, peoplePath, `{"owner":"ben","not_on_slack":true}`)

	// Assert
	want := loop.OwnerLink{Owner: benOwner, OnSlack: false, Slack: loop.SlackTarget{}}
	if got := fake.links[len(fake.links)-1]; recorder.Code != http.StatusOK || got != want {
		t.Errorf("status %d, kept %+v; want 200 and %+v", recorder.Code, got, want)
	}
}

func TestLinkPersonRefusesWhatItCannotKeep(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name, body, detail string
	}{
		{"both an ID and not on Slack", `{"owner":"ben","slack_id":"U0BEN","not_on_slack":true}`, "one or the other"},
		{"neither", `{"owner":"ben","not_on_slack":false}`, "one or the other"},
		{"a group for a person", `{"owner":"ben","slack_id":"S0POD","not_on_slack":false}`, "a team links to"},
		{"a person for a team", `{"owner":"acme/control-plane","slack_id":"U0BEN","not_on_slack":false}`, "a team links to"},
		{"no such member", `{"owner":"ben","slack_id":"U0NOBODY","not_on_slack":false}`, "not among"},
		{"no such group", `{"owner":"acme/control-plane","slack_id":"S0NONE","not_on_slack":false}`, "not among"},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			fake := newFakeKept()
			handler := keptServer(t, fake, webserver.Info{Version: testVersion})

			// Act
			recorder := send(t, handler, http.MethodPut, peoplePath, testCase.body)

			// Assert
			failure := decode[api.Problem](t, recorder)
			if recorder.Code != http.StatusUnprocessableEntity || !strings.Contains(failure.Detail, testCase.detail) {
				t.Errorf("status/detail = %d/%q, want 422 saying %q", recorder.Code, failure.Detail, testCase.detail)
			}

			if len(fake.links) != 2 {
				t.Errorf("links = %+v, want nothing written", fake.links)
			}
		})
	}
}

func TestLinkPersonNamesTheScopeTheTokenLacks(t *testing.T) {
	t.Parallel()

	// Arrange
	fake := newFakeKept()
	fake.membersErr = &messaging.MissingScopeError{Needed: usersRead}
	handler := keptServer(t, fake, webserver.Info{Version: testVersion})

	// Act
	recorder := send(t, handler, http.MethodPut, peoplePath, `{"owner":"ben","slack_id":"U0BEN","not_on_slack":false}`)

	// Assert
	failure := decode[api.Problem](t, recorder)
	if recorder.Code != http.StatusUnprocessableEntity || !strings.Contains(failure.Detail, usersRead) {
		t.Errorf("status/detail = %d/%q, want 422 naming users:read", recorder.Code, failure.Detail)
	}
}

func TestPeopleNeedASlackUserTokenAndAStore(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name, method, target, body, detail string
	}{
		{"members", http.MethodGet, "/api/slack/members", "", "Slack user token"},
		{"groups", http.MethodGet, groupsPath, "", "Slack user token"},
		{"people", http.MethodGet, peoplePath, "", noStore},
		{"link", http.MethodPut, peoplePath, `{"owner":"ben","not_on_slack":true}`, noStore},
		{"forget", http.MethodDelete, "/api/people?owner=ben", "", noStore},
		{"repo groups", http.MethodGet, repoGroupsPath, "", noStore},
		{"set repo groups", http.MethodPut, repoGroupsPath, `{"ids":["S0POD"]}`, noStore},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			handler := serve(t, filledDeps(), config.Default())

			// Act
			recorder := send(t, handler, testCase.method, testCase.target, testCase.body)

			// Assert
			failure := decode[api.Problem](t, recorder)
			if recorder.Code != http.StatusUnprocessableEntity || !strings.Contains(failure.Detail, testCase.detail) {
				t.Errorf("status/detail = %d/%q, want 422 saying %q", recorder.Code, failure.Detail, testCase.detail)
			}
		})
	}
}

func TestForgetPersonAsksAboutTheOwnerAgain(t *testing.T) {
	t.Parallel()

	// Arrange
	handler := keptServer(t, newFakeKept(), webserver.Info{Version: testVersion})

	// Act
	recorder := send(t, handler, http.MethodDelete, "/api/people?owner=dan", "")

	// Assert
	for _, owner := range decode[api.People](t, recorder).Owners {
		if owner.Owner == danOwner {
			t.Errorf("dan is still listed as %+v after being forgotten", owner)
		}
	}
}

func TestSlackDirectoryReadsListTheirEntries(t *testing.T) {
	t.Parallel()

	cases := []struct {
		target string
		want   []api.SlackTarget
	}{
		{"/api/slack/members?channel=dev", []api.SlackTarget{api.SlackTarget(ben()), api.SlackTarget(carla())}},
		{groupsPath, []api.SlackTarget{api.SlackTarget(podGroup()), api.SlackTarget(apiReviews())}},
	}

	for _, testCase := range cases {
		t.Run(testCase.target, func(t *testing.T) {
			t.Parallel()

			// Arrange
			handler := keptServer(t, newFakeKept(), webserver.Info{Version: testVersion})

			// Act
			recorder := get(t, handler, testCase.target)

			// Assert
			want := api.SlackDirectory{Entries: testCase.want, MissingScope: nil}
			if got := decode[api.SlackDirectory](t, recorder); !reflect.DeepEqual(got, want) {
				t.Errorf("GET %s = %+v, want %+v", testCase.target, got, want)
			}
		})
	}
}

func TestSlackDirectoryReadsNameAMissingScopeWithoutFailing(t *testing.T) {
	t.Parallel()

	// Arrange
	fake := newFakeKept()
	fake.groupsErr = fmt.Errorf("reading user groups: %w", &messaging.MissingScopeError{Needed: groupsRead})
	handler := keptServer(t, fake, webserver.Info{Version: testVersion})

	// Act
	recorder := get(t, handler, groupsPath)

	// Assert
	got := decode[api.SlackDirectory](t, recorder)
	if recorder.Code != http.StatusOK || got.MissingScope == nil || *got.MissingScope != groupsRead ||
		len(got.Entries) != 0 {
		t.Errorf("GET /api/slack/groups = %d %+v, want 200 with no entries naming usergroups:read", recorder.Code, got)
	}
}

func TestSlackGroupsAnswerNoneForAWorkspaceWithout(t *testing.T) {
	t.Parallel()

	// Arrange
	fake := newFakeKept()
	fake.groupsErr = messaging.ErrNoUserGroups
	handler := keptServer(t, fake, webserver.Info{Version: testVersion})

	// Act
	recorder := get(t, handler, groupsPath)

	// Assert
	if got := decode[api.SlackDirectory](t, recorder); recorder.Code != http.StatusOK || len(got.Entries) != 0 {
		t.Errorf("GET /api/slack/groups = %d %+v, want 200 and none", recorder.Code, got)
	}
}

func TestAnUnreachableSlackNamesNoHost(t *testing.T) {
	t.Parallel()

	// Arrange
	fake := newFakeKept()
	fake.membersErr = fmt.Errorf("%w: dial tcp slack.example.com:443", messaging.ErrUnreachable)
	handler := keptServer(t, fake, webserver.Info{Version: testVersion})

	// Act
	recorder := get(t, handler, "/api/slack/members")

	// Assert
	failure := decode[api.Problem](t, recorder)
	if recorder.Code != http.StatusBadGateway || strings.Contains(failure.Detail, "slack.example.com") {
		t.Errorf("status/detail = %d/%q, want 502 naming no host", recorder.Code, failure.Detail)
	}
}

func TestSetRepoGroupsLabelsEachGroupFromTheDirectory(t *testing.T) {
	t.Parallel()

	// Arrange
	fake := newFakeKept()
	handler := keptServer(t, fake, webserver.Info{Version: testVersion, Repository: "acme/widgets"})

	// Act
	recorder := send(t, handler, http.MethodPut, repoGroupsPath, `{"ids":["S0POD","S0API","S0POD"]}`)

	// Assert
	want := api.RepoGroups{
		Repository: "acme/widgets", Groups: []api.SlackTarget{api.SlackTarget(podGroup()), api.SlackTarget(apiReviews())},
	}
	if got := decode[api.RepoGroups](t, recorder); recorder.Code != http.StatusOK || !reflect.DeepEqual(got, want) {
		t.Errorf("PUT /api/repo-groups = %d %+v, want 200 %+v", recorder.Code, got, want)
	}
}

func TestSetRepoGroupsRefusesAGroupTheWorkspaceLacks(t *testing.T) {
	t.Parallel()

	// Arrange
	fake := newFakeKept()
	handler := keptServer(t, fake, webserver.Info{Version: testVersion})

	// Act
	recorder := send(t, handler, http.MethodPut, repoGroupsPath, `{"ids":["S0POD","S0GONE"]}`)

	// Assert
	unchanged := reflect.DeepEqual(fake.repoGroups, []loop.SlackTarget{apiReviews()})
	if recorder.Code != http.StatusUnprocessableEntity || !unchanged {
		t.Errorf("status %d, groups %+v; want 422 and nothing written", recorder.Code, fake.repoGroups)
	}
}

func TestPeopleWritesAreRefusedUnderDryRun(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name, method, target, body string
	}{
		{"link", http.MethodPut, peoplePath, `{"owner":"ben","not_on_slack":true}`},
		{"forget", http.MethodDelete, "/api/people?owner=dan", ""},
		{"repo groups", http.MethodPut, repoGroupsPath, `{"ids":["S0POD"]}`},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			fake := newFakeKept()
			handler := keptServer(t, fake, webserver.Info{Version: testVersion, DryRun: true})

			// Act
			recorder := send(t, handler, testCase.method, testCase.target, testCase.body)

			// Assert
			if recorder.Code != http.StatusForbidden || len(fake.links) != 2 || len(fake.repoGroups) != 1 {
				t.Errorf("status %d, links %+v, groups %+v; want 403 and nothing written",
					recorder.Code, fake.links, fake.repoGroups)
			}
		})
	}
}
