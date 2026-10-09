// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package webserver_test

// People and groups when what it reads fails: the branch's owners, Slack's
// word on the workspace, or the kept store, before a write or after one.

import (
	"net/http"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/jacob-delgado/workflow/internal/api"
	"github.com/jacob-delgado/workflow/internal/codeowners"
	"github.com/jacob-delgado/workflow/internal/gitrepo"
	"github.com/jacob-delgado/workflow/internal/loop"
	"github.com/jacob-delgado/workflow/internal/messaging"
	"github.com/jacob-delgado/workflow/internal/webserver"
)

// keptDeps is filledDeps over fake, posting nowhere, as keptServer serves it.
func keptDeps(fake *fakeKept) webserver.Deps {
	deps := filledDeps()
	deps.Messaging.Post = func(string, string) error { return nil }
	fake.wire(&deps)

	return deps
}

// breaksOnceWritten has deps' kept store fail every read once it has taken a
// write, as a store whose file went away under the server would.
func breaksOnceWritten(deps *webserver.Deps) {
	var (
		lock   sync.Mutex
		broken bool
	)

	isBroken := func() bool {
		lock.Lock()
		defer lock.Unlock()

		return broken
	}
	breaking := func(err error) error {
		lock.Lock()
		defer lock.Unlock()

		broken = true

		return err
	}

	links, link, forget := deps.Store.OwnerLinks, deps.Store.LinkOwner, deps.Store.ForgetOwner
	groups, setGroups := deps.Store.RepoGroups, deps.Store.SetRepoGroups

	deps.Store.OwnerLinks = func(workspace string) ([]loop.OwnerLink, error) {
		if isBroken() {
			return nil, errSeam
		}

		return links(workspace)
	}
	deps.Store.RepoGroups = func(workspace string) ([]loop.SlackTarget, error) {
		if isBroken() {
			return nil, errSeam
		}

		return groups(workspace)
	}
	deps.Store.LinkOwner = func(workspace string, decided loop.OwnerLink) error {
		return breaking(link(workspace, decided))
	}
	deps.Store.ForgetOwner = func(workspace, owner string) error { return breaking(forget(workspace, owner)) }
	deps.Store.SetRepoGroups = func(workspace string, kept []loop.SlackTarget) error {
		return breaking(setGroups(workspace, kept))
	}
}

func TestGetPeopleListsTheDecidedAloneWhenTheOwnersOfTheBranchCannotBeRead(t *testing.T) {
	t.Parallel()

	cases := map[string]func(deps *webserver.Deps){
		"the branch git is on": func(deps *webserver.Deps) {
			deps.Git.Branch = func() (gitrepo.Branch, error) { return gitrepo.Branch{}, errSeam }
		},
		"the changed paths": func(deps *webserver.Deps) {
			deps.Git.ChangedPaths = func(string) ([]string, error) { return nil, errSeam }
		},
		"the code owners": func(deps *webserver.Deps) {
			deps.Git.CodeOwnersAt = func(string) (codeowners.File, bool, error) { return codeowners.File{}, false, errSeam }
		},
	}

	for name, unreadable := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			deps := keptDeps(newFakeKept())
			unreadable(&deps)

			// Act
			recorder := get(t, serve(t, deps, slackUserConfig()), peoplePath)

			// Assert
			want := api.People{Owners: []api.OwnerTag{
				{Owner: carlaOwner, Kind: api.OwnerTagKindUser, State: api.OwnerTagStateLinked, Slack: slackTarget(carla())},
				{Owner: danOwner, Kind: api.OwnerTagKindUser, State: api.OwnerTagStateNotOnSlack, Slack: nil},
			}}
			if got := decode[api.People](t, recorder); recorder.Code != http.StatusOK || !reflect.DeepEqual(got, want) {
				t.Errorf("GET /api/people = %d %+v, want 200 %+v", recorder.Code, got, want)
			}
		})
	}
}

func TestPeopleIsRefusedForADirectoryWhenSlackHasNoCredential(t *testing.T) {
	t.Parallel()

	// Arrange
	fake := newFakeKept()
	fake.workspaceErr = messaging.ErrNoCredential
	handler := keptServer(t, fake, webserver.Info{Version: testVersion})

	// Act
	recorder := get(t, handler, peoplePath)

	// Assert
	if detail := decode[api.Problem](t, recorder).Detail; recorder.Code != http.StatusUnprocessableEntity ||
		!strings.Contains(detail, "tagging needs a Slack user token") {
		t.Errorf("status %d, detail %q; want 422 saying tagging needs a Slack user token", recorder.Code, detail)
	}
}

func TestLinkPersonIsRefusedWithWhyInAnUnknownWorkspace(t *testing.T) {
	t.Parallel()

	// Arrange
	fake := newFakeKept()
	fake.workspaceErr = messaging.ErrNoWorkspace
	handler := keptServer(t, fake, webserver.Info{Version: testVersion})

	// Act
	recorder := send(t, handler, http.MethodPut, peoplePath, `{"owner":"ben","slack_id":"U0BEN","not_on_slack":false}`)

	// Assert
	if detail := decode[api.Problem](t, recorder).Detail; recorder.Code != http.StatusUnprocessableEntity ||
		!strings.Contains(detail, unknownWorkspace) {
		t.Errorf("status %d, detail %q; want 422 saying %q", recorder.Code, detail, unknownWorkspace)
	}

	if len(fake.links) != len(newFakeKept().links) {
		t.Errorf("links = %+v, want them left as they were", fake.links)
	}
}

func TestAPersonDecidedThatTheStoreCannotListBackIsAFault(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		method, target, body string
		// decided reports the store holding the decision.
		decided func(links []loop.OwnerLink) bool
	}{
		"a person linked": {
			method: http.MethodPut, target: peoplePath, body: `{"owner":"ben","slack_id":"U0BEN","not_on_slack":false}`,
			decided: func(links []loop.OwnerLink) bool { return len(links) == 3 },
		},
		"a person forgotten": {
			method: http.MethodDelete, target: peoplePath + "?owner=dan",
			decided: func(links []loop.OwnerLink) bool { return len(links) == 1 },
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			fake := newFakeKept()
			deps := keptDeps(fake)
			breaksOnceWritten(&deps)

			// Act
			recorder := send(t, serve(t, deps, slackUserConfig()), tt.method, tt.target, tt.body)

			// Assert
			assertProblem(t, recorder, http.StatusInternalServerError, tryAgain)

			if !tt.decided(fake.links) {
				t.Errorf("links = %+v, want the decision kept before the listing failed", fake.links)
			}
		})
	}
}

func TestRepoGroupsAreThoseOfTheRepository(t *testing.T) {
	t.Parallel()

	// Arrange
	handler := keptServer(t, newFakeKept(), webserver.Info{Version: testVersion, Repository: "acme/kept"})

	// Act
	recorder := get(t, handler, repoGroupsPath)

	// Assert
	want := api.RepoGroups{Repository: "acme/kept", Groups: []api.SlackTarget{api.SlackTarget(apiReviews())}}
	if got := decode[api.RepoGroups](t, recorder); recorder.Code != http.StatusOK || !reflect.DeepEqual(got, want) {
		t.Errorf("GET %s = %d %+v, want 200 %+v", repoGroupsPath, recorder.Code, got, want)
	}
}

func TestRepoGroupsAreRefusedWithWhyInAnUnknownWorkspace(t *testing.T) {
	t.Parallel()

	// Arrange
	fake := newFakeKept()
	fake.workspaceErr = messaging.ErrNoWorkspace
	handler := keptServer(t, fake, webserver.Info{Version: testVersion})

	// Act
	recorder := get(t, handler, repoGroupsPath)

	// Assert
	if detail := decode[api.Problem](t, recorder).Detail; recorder.Code != http.StatusUnprocessableEntity ||
		!strings.Contains(detail, unknownWorkspace) {
		t.Errorf("status %d, detail %q; want 422 saying %q", recorder.Code, detail, unknownWorkspace)
	}
}

func TestSettingRepoGroupsTheStoreCannotReadIsAFault(t *testing.T) {
	t.Parallel()

	// Arrange
	fake := newFakeKept()
	deps := keptDeps(fake)
	deps.Store.RepoGroups = func(string) ([]loop.SlackTarget, error) { return nil, errSeam }

	// Act
	recorder := send(t, serve(t, deps, slackUserConfig()), http.MethodPut, repoGroupsPath, `{"ids":["S0POD"]}`)

	// Assert
	assertProblem(t, recorder, http.StatusInternalServerError, tryAgain)

	if !reflect.DeepEqual(fake.repoGroups, newFakeKept().repoGroups) {
		t.Errorf("repository groups = %+v, want them left as they were", fake.repoGroups)
	}
}

func TestSettingRepoGroupsTheStoreCannotListBackIsAFault(t *testing.T) {
	t.Parallel()

	// Arrange
	fake := newFakeKept()
	deps := keptDeps(fake)
	breaksOnceWritten(&deps)

	// Act
	recorder := send(t, serve(t, deps, slackUserConfig()), http.MethodPut, repoGroupsPath, `{"ids":["S0POD"]}`)

	// Assert
	assertProblem(t, recorder, http.StatusInternalServerError, tryAgain)

	if !reflect.DeepEqual(fake.repoGroups, []loop.SlackTarget{podGroup()}) {
		t.Errorf("repository groups = %+v, want the pod's group kept before the listing failed", fake.repoGroups)
	}
}
