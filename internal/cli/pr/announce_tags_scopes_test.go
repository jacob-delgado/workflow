// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package pr_test

// `workflow announce` still posts when Slack cannot read what tagging needs —
// a token missing a scope, or a workspace Slack cannot name — untagged, and
// says why; and it tells its scopes without reading the workspace's directory.
// The fixtures are announce_tags_test.go's.

import (
	"net/http"
	"strings"
	"testing"

	"github.com/jacob-delgado/workflow/internal/cli/clitest"
)

func TestAnnounceWithATokenMissingAScopePostsUntaggedAndNamesIt(t *testing.T) {
	t.Parallel()

	for _, scope := range []string{"users:read", "channels:read", "groups:read", "usergroups:read"} {
		t.Run(scope, func(t *testing.T) {
			t.Parallel()

			// Arrange
			clitest.FakeGh(t, clitest.GhResponses{Pulls: clitest.OpenPull("Add login")})
			slack := clitest.FakeSlack(t, nil)
			slack.Grant(strings.ReplaceAll(clitest.TaggingScopes, ","+scope, ""))

			repo := clitest.OwnedRepo(t)
			clitest.WriteFile(t, repo, clitest.SlackLoggedInConfig())
			home := keptLinks(t)

			// Act
			printed, err := clitest.RunStreamsAt(
				t, clitest.Place{Dir: repo, Home: home}, clitest.UnusedPrompt(t), "announce", "--yes",
			)
			// Assert
			if err != nil {
				t.Fatalf("announce --yes: %v (%+v)", err, printed)
			}

			if !strings.Contains(printed.Stderr, "WARN Not tagging anyone: the Slack token lacks the "+scope+" scope") {
				t.Errorf("announce said %q, want the missing %s scope named", printed.Stderr, scope)
			}

			if post := slack.Post(t); !untagged(post) {
				t.Errorf("announce posted %q, want it untagged", post)
			}
		})
	}
}

func TestAnnounceReadsNoDirectoryToTellItsScopes(t *testing.T) {
	t.Parallel()

	// Arrange
	// In a large workspace the directory is pages of users.list and a
	// users.info per member; the announcement tags only whom is linked
	// already, so it needs none of it.
	clitest.FakeGh(t, clitest.GhResponses{Pulls: clitest.OpenPull("Add login")})
	slack := clitest.FakeSlack(t, nil)
	repo := clitest.OwnedRepo(t)
	clitest.WriteFile(t, repo, clitest.SlackLoggedInConfig())
	home := keptLinks(t)

	// Act
	printed, err := clitest.RunStreamsAt(
		t, clitest.Place{Dir: repo, Home: home}, clitest.UnusedPrompt(t), "announce", "--yes",
	)
	// Assert
	if err != nil {
		t.Fatalf("announce --yes: %v (%+v)", err, printed)
	}

	for _, path := range []string{"/users.conversations", "/conversations.members", "/users.list", "/usergroups.list"} {
		if asked := slack.Count(path); asked != 0 {
			t.Errorf("announce asked %s %d times, want none", path, asked)
		}
	}

	if post := slack.Post(t); untagged(post) {
		t.Errorf("announce posted %q, want ana and api-reviewers tagged", post)
	}
}

func TestAnnounceWhenSlackCannotNameTheWorkspacePostsUntaggedAndSaysWhy(t *testing.T) {
	t.Parallel()

	// One auth.test answers both the scopes the token was granted and the
	// workspace it is for. It can fail, when neither is read, though a
	// second one asked would have answered; or name no workspace, though it
	// lists the scopes.
	cases := map[string]func(t *testing.T) *clitest.SlackFake{
		"an auth.test that fails": func(t *testing.T) *clitest.SlackFake {
			t.Helper()

			slack := clitest.FakeSlack(t, nil)
			slack.AnswerFirst(clitest.SlackAuthTest, clitest.SlackAnswer{Status: http.StatusServiceUnavailable, Body: ""})

			return slack
		},
		"an auth.test that names no workspace": func(t *testing.T) *clitest.SlackFake {
			t.Helper()

			return clitest.FakeSlack(t, map[string]clitest.SlackAnswer{
				clitest.SlackAuthTest: {Status: http.StatusOK, Body: `{"ok":true,"team":"Acme","user":"ana"}`},
			})
		},
	}

	for name, answering := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			clitest.FakeGh(t, clitest.GhResponses{Pulls: clitest.OpenPull("Add login")})
			slack := answering(t)
			repo := clitest.OwnedRepo(t)
			clitest.WriteFile(t, repo, clitest.SlackLoggedInConfig())
			home := keptLinks(t)

			// Act
			printed, err := clitest.RunStreamsAt(
				t, clitest.Place{Dir: repo, Home: home}, clitest.UnusedPrompt(t), "announce", "--yes",
			)
			// Assert
			if err != nil {
				t.Fatalf("announce --yes: %v (%+v)", err, printed)
			}

			if !strings.Contains(printed.Stderr, "can't tell which Slack workspace this token is for") ||
				!strings.Contains(printed.Stderr, "posting untagged") {
				t.Errorf("announce said %q, want the unknown workspace named and the post said to go untagged",
					printed.Stderr)
			}

			if post := slack.Post(t); !untagged(post) {
				t.Errorf("announce posted %q, want it untagged", post)
			}
		})
	}
}
