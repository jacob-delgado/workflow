// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package webserver_test

// People and groups when what it reads fails: the branch's owners, Slack's
// word on the workspace, or the kept store, before a write or after one.

import (
	"context"
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
	"github.com/jacob-delgado/workflow/internal/webserver/webtest"
)

// keptDeps is webtest.FilledDeps over fake, posting nowhere, as keptServer
// serves it.
func keptDeps(fake *fakeKept) webserver.Deps {
	deps := webtest.FilledDeps()
	deps.Messaging.Post = func(context.Context, string, string) error { return nil }
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

	deps.Store.OwnerLinks = func(ctx context.Context, workspace string) ([]loop.OwnerLink, error) {
		if isBroken() {
			return nil, webtest.ErrSeam
		}

		return links(ctx, workspace)
	}
	deps.Store.RepoGroups = func(ctx context.Context, workspace string) ([]loop.SlackTarget, error) {
		if isBroken() {
			return nil, webtest.ErrSeam
		}

		return groups(ctx, workspace)
	}
	deps.Store.LinkOwner = func(ctx context.Context, workspace string, decided loop.OwnerLink) error {
		return breaking(link(ctx, workspace, decided))
	}
	deps.Store.ForgetOwner = func(ctx context.Context, workspace, owner string) error {
		return breaking(forget(ctx, workspace, owner))
	}
	deps.Store.SetRepoGroups = func(ctx context.Context, workspace string, kept []loop.SlackTarget) error {
		return breaking(setGroups(ctx, workspace, kept))
	}
}

func TestGetPeopleListsTheDecidedAloneWhenTheOwnersOfTheBranchCannotBeRead(t *testing.T) {
	t.Parallel()

	cases := map[string]func(deps *webserver.Deps){
		"the branch git is on": func(deps *webserver.Deps) {
			deps.Git.Branch = func(context.Context) (gitrepo.Branch, error) { return gitrepo.Branch{}, webtest.ErrSeam }
		},
		"the changed paths": func(deps *webserver.Deps) {
			deps.Git.ChangedPaths = func(context.Context, string) ([]string, error) { return nil, webtest.ErrSeam }
		},
		"the code owners": func(deps *webserver.Deps) {
			deps.Git.CodeOwnersAt = func(context.Context, string) (codeowners.File, bool, error) {
				return codeowners.File{}, false, webtest.ErrSeam
			}
		},
	}

	for name, unreadable := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			deps := keptDeps(newFakeKept())
			unreadable(&deps)

			// Act
			recorder := webtest.Get(t, webtest.Serve(t, deps, slackUserConfig()), peoplePath)

			// Assert
			want := api.People{Owners: []api.OwnerTag{
				{Owner: carlaOwner, Kind: api.OwnerTagKindUser, State: api.OwnerTagStateLinked, Slack: slackTarget(carla())},
				{Owner: danOwner, Kind: api.OwnerTagKindUser, State: api.OwnerTagStateNotOnSlack, Slack: nil},
			}}
			if got := webtest.Decode[api.People](t, recorder); recorder.Code != http.StatusOK || !reflect.DeepEqual(got, want) {
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
	handler := keptServer(t, fake, webserver.Info{Version: webtest.TestVersion})

	// Act
	recorder := webtest.Get(t, handler, peoplePath)

	// Assert
	if detail := webtest.Decode[api.Problem](t, recorder).Detail; recorder.Code != http.StatusUnprocessableEntity ||
		!strings.Contains(detail, "tagging needs a Slack user token") {
		t.Errorf("status %d, detail %q; want 422 saying tagging needs a Slack user token", recorder.Code, detail)
	}
}

func TestLinkPersonIsRefusedWithWhyInAnUnknownWorkspace(t *testing.T) {
	t.Parallel()

	// Arrange
	fake := newFakeKept()
	fake.workspaceErr = messaging.ErrNoWorkspace
	handler := keptServer(t, fake, webserver.Info{Version: webtest.TestVersion})

	// Act
	recorder := webtest.Send(t, handler, http.MethodPut, peoplePath,
		`{"owner":"ben","slack_id":"U0BEN","not_on_slack":false}`)

	// Assert
	if detail := webtest.Decode[api.Problem](t, recorder).Detail; recorder.Code != http.StatusUnprocessableEntity ||
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
			recorder := webtest.Send(t, webtest.Serve(t, deps, slackUserConfig()), tt.method, tt.target, tt.body)

			// Assert
			assertProblem(t, recorder, http.StatusInternalServerError, webtest.TryAgain)

			if !tt.decided(fake.links) {
				t.Errorf("links = %+v, want the decision kept before the listing failed", fake.links)
			}
		})
	}
}

func TestRepoGroupsAreThoseOfTheRepository(t *testing.T) {
	t.Parallel()

	// Arrange
	handler := keptServer(t, newFakeKept(), webserver.Info{Version: webtest.TestVersion, Repository: "acme/kept"})

	// Act
	recorder := webtest.Get(t, handler, repoGroupsPath)

	// Assert
	want := api.RepoGroups{Repository: "acme/kept", Groups: []api.SlackTarget{api.SlackTarget(apiReviews())}}
	if got := webtest.Decode[api.RepoGroups](t, recorder); recorder.Code != http.StatusOK || !reflect.DeepEqual(got,
		want) {
		t.Errorf("GET %s = %d %+v, want 200 %+v", repoGroupsPath, recorder.Code, got, want)
	}
}

func TestRepoGroupsAreRefusedWithWhyInAnUnknownWorkspace(t *testing.T) {
	t.Parallel()

	// Arrange
	fake := newFakeKept()
	fake.workspaceErr = messaging.ErrNoWorkspace
	handler := keptServer(t, fake, webserver.Info{Version: webtest.TestVersion})

	// Act
	recorder := webtest.Get(t, handler, repoGroupsPath)

	// Assert
	if detail := webtest.Decode[api.Problem](t, recorder).Detail; recorder.Code != http.StatusUnprocessableEntity ||
		!strings.Contains(detail, unknownWorkspace) {
		t.Errorf("status %d, detail %q; want 422 saying %q", recorder.Code, detail, unknownWorkspace)
	}
}

func TestSettingRepoGroupsTheStoreCannotReadIsAFault(t *testing.T) {
	t.Parallel()

	// Arrange
	fake := newFakeKept()
	deps := keptDeps(fake)
	deps.Store.RepoGroups = func(context.Context, string) ([]loop.SlackTarget, error) { return nil, webtest.ErrSeam }

	// Act
	recorder := webtest.Send(t, webtest.Serve(t, deps, slackUserConfig()), http.MethodPut, repoGroupsPath,
		`{"ids":["S0POD"]}`)

	// Assert
	assertProblem(t, recorder, http.StatusInternalServerError, webtest.TryAgain)

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
	recorder := webtest.Send(t, webtest.Serve(t, deps, slackUserConfig()), http.MethodPut, repoGroupsPath,
		`{"ids":["S0POD"]}`)

	// Assert
	assertProblem(t, recorder, http.StatusInternalServerError, webtest.TryAgain)

	if !reflect.DeepEqual(fake.repoGroups, []loop.SlackTarget{podGroup()}) {
		t.Errorf("repository groups = %+v, want the pod's group kept before the listing failed", fake.repoGroups)
	}
}
