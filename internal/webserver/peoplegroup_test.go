// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package webserver_test

// CODEOWNERS spells a top-level GitLab group @acme, as it spells a user, so
// the forge says which a bare name is: a group is listed as a team and links
// to a Slack user group.

import (
	"context"
	"net/http"
	"testing"

	"github.com/jacob-delgado/workflow/internal/api"
	"github.com/jacob-delgado/workflow/internal/codeowners"
	"github.com/jacob-delgado/workflow/internal/loop"
	"github.com/jacob-delgado/workflow/internal/webserver"
	"github.com/jacob-delgado/workflow/internal/webserver/webtest"
)

// acmeOwnedServer is a server whose branch's changes only the bare name acme
// owns, on a GitLab that knows acme as a group.
func acmeOwnedServer(t *testing.T, fake *fakeKept) http.Handler {
	t.Helper()

	deps := webtest.FilledDeps()
	deps.Messaging.Post = func(context.Context, string, string) error { return nil }
	fake.wire(&deps)
	deps.Git.CodeOwnersAt = func(context.Context, string) (codeowners.File, bool, error) {
		return codeowners.Parse("* @acme\n", codeowners.GitLab), true, nil
	}
	deps.Forge.IsGroup = func(_ context.Context, name string) (bool, error) { return name == "acme", nil }

	return webtest.ServeWith(t, deps, slackUserConfig(), webserver.Info{Version: webtest.TestVersion})
}

func TestLinkPersonLinksABareNamedGitLabGroupToAUserGroup(t *testing.T) {
	t.Parallel()

	// Arrange
	fake := newFakeKept()
	fake.links = nil
	handler := acmeOwnedServer(t, fake)

	// Act
	recorder := webtest.Send(t, handler, http.MethodPut, peoplePath,
		`{"owner":"acme","slack_id":"S0POD","not_on_slack":false}`)

	// Assert
	people := webtest.Decode[api.People](t, recorder)
	if recorder.Code != http.StatusOK || len(people.Owners) != 1 || people.Owners[0].Kind != api.OwnerTagKindTeam ||
		people.Owners[0].State != api.OwnerTagStateLinked {
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
	recorder := webtest.Send(t, handler, http.MethodPut, peoplePath,
		`{"owner":"acme","slack_id":"S0POD","not_on_slack":false}`)

	// Assert
	people := webtest.Decode[api.People](t, recorder)
	if recorder.Code != http.StatusOK || len(people.Owners) != 1 || people.Owners[0].Kind != api.OwnerTagKindTeam ||
		people.Owners[0].State != api.OwnerTagStateLinked {
		t.Errorf("status %d, people %+v; want acme a team linked to the pod's group", recorder.Code, people)
	}
}
