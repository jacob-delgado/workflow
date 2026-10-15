// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package pr_test

// `workflow announce` tags the code owners already linked to Slack and the
// groups that start checked, never asking, and names who it leaves untagged —
// in the Slack workspace the token is for, which only Slack can say, so
// nobody is tagged without one. Slack is a fake on this machine (clitest.FakeSlack).

import (
	"net/http"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/jacob-delgado/workflow/internal/cli/clitest"
	"github.com/jacob-delgado/workflow/internal/store"
)

// slackUserConfig is clitest.OwnedRepo's forge, announcing to Slack with a user
// token whose credentials the file keeps, so no keychain is read.
const slackUserConfig = `{"forge":{"cli":true,"kind":"github","host":"github.com"},` +
	`"messaging":{"client_id":"1234.5678","client_secret":"client-secret-9999","channel":"#dev"}}`

// keptLinks is a home whose kept store links ana on github.com and gives the
// repository a group, chosen the last time, in a Slack workspace.
func keptLinks(t *testing.T) string {
	t.Helper()

	home := t.TempDir()
	env := clitest.IsolatedEnvironment(clitest.Place{Home: home})

	dir, err := store.Dir(runtime.GOOS, home, func(name string) (string, bool) {
		value, ok := env[name]

		return value, ok
	})
	if err != nil {
		t.Fatalf("finding the store under %s: %v", home, err)
	}

	kept, repo, workspace, now := store.New(dir, false), "github.com/acme/repo", clitest.SlackWorkspace, time.Now()
	linked := store.OwnerLink{
		Owner: "ana", Team: false, OnSlack: true, Slack: store.SlackTarget{ID: "U0ANA", Label: "Ana Souza"},
	}

	for _, seed := range []error{
		kept.LinkOwner(t.Context(), "github.com", workspace, linked, now),
		kept.SetRepoGroups(t.Context(), repo, workspace, []store.SlackTarget{{ID: "S0API", Label: "api-reviewers"}}, now),
		kept.RecordGroups(t.Context(), repo, workspace, []string{"S0API"}, now),
	} {
		if seed != nil {
			t.Fatalf("seeding the kept store: %v", seed)
		}
	}

	return home
}

func TestAnnounceTagsNoOneWithoutAWorkspaceToReadLinksIn(t *testing.T) {
	t.Parallel()

	// Arrange
	// The token is set up but not logged in, so Slack cannot be asked which
	// workspace it is for, and links kept in any workspace are left alone.
	clitest.FakeGh(t, clitest.GhResponses{Pulls: clitest.OpenPull("Add login")})
	repo := clitest.OwnedRepo(t)
	clitest.WriteFile(t, repo, slackUserConfig)
	home := keptLinks(t)

	// Act
	printed, err := clitest.RunStreamsAt(
		t, clitest.Place{Dir: repo, Home: home}, clitest.UnusedPrompt(t), "announce", "--dry-run",
	)
	// Assert
	if err != nil {
		t.Fatalf("announce --dry-run: %v (%+v)", err, printed)
	}

	if strings.Contains(printed.Stdout, "Ana") || strings.Contains(printed.Stdout, "api-reviewers") {
		t.Errorf("announce previewed %q, want no one tagged without a Slack workspace", printed.Stdout)
	}
}

func TestAnnounceTagsNoOneThroughAWebhook(t *testing.T) {
	t.Parallel()

	// Arrange
	clitest.FakeGh(t, clitest.GhResponses{Pulls: clitest.OpenPull("Add login")})
	repo := clitest.OwnedRepo(t)
	clitest.WriteFile(t, repo, `{"forge":{"cli":true,"kind":"github","host":"github.com"},`+
		`"messaging":{"webhook_url":"https://hooks.slack.example/services/x","channel":"#dev"}}`)
	home := keptLinks(t)

	// Act
	printed, err := clitest.RunStreamsAt(
		t, clitest.Place{Dir: repo, Home: home}, clitest.UnusedPrompt(t), "announce", "--dry-run",
	)
	// Assert
	if err != nil {
		t.Fatalf("announce --dry-run: %v (%+v)", err, printed)
	}

	if strings.Contains(printed.Stdout, "tags ") || strings.Contains(printed.Stdout, "not tagged") {
		t.Errorf("announce through a webhook previewed %q, want no tags", printed.Stdout)
	}
}

func TestAnnounceTagsTheOwnersAndGroupsLinkedInTheWorkspaceInUse(t *testing.T) {
	t.Parallel()

	// Arrange
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

	for _, line := range []string{
		"tags @Ana Souza @api-reviewers\n",
		"not tagged: acme/control-plane (not linked — associate in People and groups)\n",
	} {
		if !strings.Contains(printed.Stdout, line) {
			t.Errorf("announce previewed %q, want the line %q", printed.Stdout, line)
		}
	}

	if post := slack.Post(t); !strings.Contains(post, "<@U0ANA>") || !strings.Contains(post, "<!subteam^S0API>") {
		t.Errorf("announce posted %q, want ana and api-reviewers tagged", post)
	}
}

func TestAnnounceTagsNoOneLinkedOnlyInAnotherWorkspace(t *testing.T) {
	t.Parallel()

	// Arrange
	clitest.FakeGh(t, clitest.GhResponses{Pulls: clitest.OpenPull("Add login")})
	clitest.FakeSlack(t, map[string]clitest.SlackAnswer{
		clitest.SlackAuthTest: {Status: http.StatusOK, Body: `{"ok":true,"team":"Other","user":"ana","team_id":"T0OTHER"}`},
	})
	repo := clitest.OwnedRepo(t)
	clitest.WriteFile(t, repo, clitest.SlackLoggedInConfig())
	home := keptLinks(t)

	// Act
	printed, err := clitest.RunStreamsAt(
		t, clitest.Place{Dir: repo, Home: home}, clitest.UnusedPrompt(t), "announce", "--dry-run",
	)
	// Assert
	if err != nil {
		t.Fatalf("announce --dry-run: %v (%+v)", err, printed)
	}

	if !strings.Contains(printed.Stdout, "tags no one\n") ||
		!strings.Contains(printed.Stdout, "not tagged: ana (not linked") {
		t.Errorf("announce previewed %q, want ana, linked in another workspace, named as not linked here",
			printed.Stdout)
	}
}

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

// releaseBranchRepo is a repository whose branch left release, not main:
// release changed the ops paths ben owns, and the branch only the API ana
// owns. The open pull request merges into release.
func releaseBranchRepo(t *testing.T) string {
	t.Helper()

	repo := t.TempDir()
	clitest.GitInit(t, repo)
	clitest.WriteRepoFile(t, repo, ".github/CODEOWNERS", "/api/ @ana\n/ops/ @ben\n")
	clitest.Git(t, repo, "add", ".")
	clitest.Commit(t, repo, "init")
	clitest.Git(t, repo, "branch", "-M", "main")
	clitest.Git(t, repo, "checkout", "-b", "release")
	clitest.WriteRepoFile(t, repo, "ops/deploy.sh", "echo deploy\n")
	clitest.Git(t, repo, "add", ".")
	clitest.Commit(t, repo, "chore: deploy")
	clitest.Git(t, repo, "checkout", "-b", "fix/PROJ-2-thing")
	clitest.WriteRepoFile(t, repo, "api/pull.go", "package api\n")
	clitest.Git(t, repo, "add", ".")
	clitest.Commit(t, repo, "fix: guard the api")
	clitest.Git(t, repo, "remote", "add", "origin", "https://github.com/acme/repo.git")
	clitest.PretendPushed(t, repo)
	clitest.WriteFile(t, repo, clitest.SlackLoggedInConfig())

	return repo
}

func TestAnnounceReadsTheOwnersAgainstThePullRequestsBase(t *testing.T) {
	t.Parallel()

	// Arrange
	clitest.FakeGh(t, clitest.GhResponses{Pulls: `[{"number":7,"html_url":"https://github.com/acme/repo/pull/7",` +
		`"title":"Add login","state":"open","draft":false,"base":{"ref":"release"}}]`})
	clitest.FakeSlack(t, nil)
	repo := releaseBranchRepo(t)
	home := keptLinks(t)

	// Act
	printed, err := clitest.RunStreamsAt(
		t, clitest.Place{Dir: repo, Home: home}, clitest.UnusedPrompt(t), "announce", "--dry-run",
	)
	// Assert
	if err != nil {
		t.Fatalf("announce --dry-run: %v (%+v)", err, printed)
	}

	if !strings.Contains(printed.Stdout, "tags @Ana Souza") || strings.Contains(printed.Stdout, "ben") {
		t.Errorf("announce previewed %q, want ana tagged and ben, who owns only release's change, not named",
			printed.Stdout)
	}
}

// untagged reports a post that tags no one on Slack.
func untagged(post string) bool {
	return !strings.Contains(post, "<@") && !strings.Contains(post, "<!subteam^")
}
