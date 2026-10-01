// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package webserver_test

// CODEOWNERS spells a top-level GitLab group @acme, as it spells a user, so
// the forge says which a bare name is: a group is listed as a team and links
// to a Slack user group.

import (
	"net/http"
	"testing"

	"github.com/jacob-delgado/workflow/internal/api"
	"github.com/jacob-delgado/workflow/internal/codeowners"
	"github.com/jacob-delgado/workflow/internal/loop"
	"github.com/jacob-delgado/workflow/internal/webserver"
)

// acmeOwnedServer is a server whose branch's changes only the bare name acme
// owns, on a GitLab that knows acme as a group.
func acmeOwnedServer(t *testing.T, fake *fakeKept) http.Handler {
	t.Helper()

	deps := filledDeps()
	deps.Post = func(string, string) error { return nil }
	fake.wire(&deps)
	deps.CodeOwnersAt = func(string) (codeowners.File, bool, error) {
		return codeowners.Parse("* @acme\n", codeowners.GitLab), true, nil
	}
	deps.IsGroup = func(name string) (bool, error) { return name == "acme", nil }

	return serveWith(t, deps, slackUserConfig(), webserver.Info{Version: testVersion})
}

func TestLinkPersonLinksABareNamedGitLabGroupToAUserGroup(t *testing.T) {
	t.Parallel()

	// Arrange
	fake := newFakeKept()
	fake.links = nil
	handler := acmeOwnedServer(t, fake)

	// Act
	recorder := send(t, handler, http.MethodPut, peoplePath, `{"owner":"acme","slack_id":"S0POD","not_on_slack":false}`)

	// Assert
	people := decode[api.People](t, recorder)
	if recorder.Code != http.StatusOK || len(people.Owners) != 1 || people.Owners[0].Kind != api.Team ||
		people.Owners[0].State != api.Linked {
		t.Errorf("status %d, people %+v; want acme a team linked to the pod's group", recorder.Code, people)
	}
}

func TestLinkPersonLinksAGroupDecidedAsAPersonBeforeTheForgeCouldTell(t *testing.T) {
	t.Parallel()

	// Arrange
	// Before the forge told bare names apart, acme was decided as a person
	// not on Slack.
	fake := newFakeKept()
	fake.links = []loop.OwnerLink{{Owner: "acme", Team: false, OnSlack: false, Slack: loop.SlackTarget{}}}
	handler := acmeOwnedServer(t, fake)

	// Act
	recorder := send(t, handler, http.MethodPut, peoplePath, `{"owner":"acme","slack_id":"S0POD","not_on_slack":false}`)

	// Assert
	people := decode[api.People](t, recorder)
	if recorder.Code != http.StatusOK || len(people.Owners) != 1 || people.Owners[0].Kind != api.Team ||
		people.Owners[0].State != api.Linked {
		t.Errorf("status %d, people %+v; want acme a team linked to the pod's group", recorder.Code, people)
	}
}
