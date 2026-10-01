// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package cli_test

// `workflow announce` tags the code owners already linked to Slack and the
// groups that start checked, never asking, and names who it leaves untagged —
// in the Slack workspace the token is for, which only Slack can say, so
// nobody is tagged without one. Slack is a fake on this machine (fakeSlack).

import (
	"net/http"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/jacob-delgado/workflow/internal/store"
)

// slackUserConfig is ownedRepo's forge, announcing to Slack with a user token
// whose credentials the file keeps, so no keychain is read.
const slackUserConfig = `{"forge":{"cli":true,"kind":"github","host":"github.com"},` +
	`"messaging":{"client_id":"1234.5678","client_secret":"client-secret-9999","channel":"#dev"}}`

// keptLinks is a home whose kept store links ana on github.com and gives the
// repository a group, chosen the last time, in a Slack workspace.
func keptLinks(t *testing.T) string {
	t.Helper()

	home := t.TempDir()
	env := isolatedEnvironment(home)

	dir, err := store.Dir(runtime.GOOS, home, func(name string) (string, bool) {
		value, ok := env[name]

		return value, ok
	})
	if err != nil {
		t.Fatalf("finding the store under %s: %v", home, err)
	}

	kept, repo, workspace, now := store.New(dir, false), "github.com/acme/repo", slackWorkspace, time.Now()
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
	// Arrange
	// The token is set up but not logged in, so Slack cannot be asked which
	// workspace it is for, and links kept in any workspace are left alone.
	fakeGh(t, ghResponses{pulls: openPull("Add login")})
	repo := ownedRepo(t)
	writeFile(t, repo, slackUserConfig)
	home := keptLinks(t)

	// Act
	printed, err := runStreamsAt(t, place{dir: repo, home: home}, unusedPrompt(t), "announce", "--dry-run")
	// Assert
	if err != nil {
		t.Fatalf("announce --dry-run: %v (%+v)", err, printed)
	}

	if strings.Contains(printed.stdout, "Ana") || strings.Contains(printed.stdout, "api-reviewers") {
		t.Errorf("announce previewed %q, want no one tagged without a Slack workspace", printed.stdout)
	}
}

func TestAnnounceTagsNoOneThroughAWebhook(t *testing.T) {
	// Arrange
	fakeGh(t, ghResponses{pulls: openPull("Add login")})
	repo := ownedRepo(t)
	writeFile(t, repo, `{"forge":{"cli":true,"kind":"github","host":"github.com"},`+
		`"messaging":{"webhook_url":"https://hooks.slack.example/services/x","channel":"#dev"}}`)
	home := keptLinks(t)

	// Act
	printed, err := runStreamsAt(t, place{dir: repo, home: home}, unusedPrompt(t), "announce", "--dry-run")
	// Assert
	if err != nil {
		t.Fatalf("announce --dry-run: %v (%+v)", err, printed)
	}

	if strings.Contains(printed.stdout, "tags ") || strings.Contains(printed.stdout, "not tagged") {
		t.Errorf("announce through a webhook previewed %q, want no tags", printed.stdout)
	}
}

func TestAnnounceTagsTheOwnersAndGroupsLinkedInTheWorkspaceInUse(t *testing.T) {
	// Arrange
	fakeGh(t, ghResponses{pulls: openPull("Add login")})
	slack := fakeSlack(t, nil)
	repo := ownedRepo(t)
	writeFile(t, repo, slackLoggedInConfig())
	home := keptLinks(t)

	// Act
	printed, err := runStreamsAt(t, place{dir: repo, home: home}, unusedPrompt(t), "announce", "--yes")
	// Assert
	if err != nil {
		t.Fatalf("announce --yes: %v (%+v)", err, printed)
	}

	for _, line := range []string{
		"tags @Ana Souza @api-reviewers\n",
		"not tagged: acme/control-plane (not linked — associate in People and groups)\n",
	} {
		if !strings.Contains(printed.stdout, line) {
			t.Errorf("announce previewed %q, want the line %q", printed.stdout, line)
		}
	}

	if post := slack.post(t); !strings.Contains(post, "<@U0ANA>") || !strings.Contains(post, "<!subteam^S0API>") {
		t.Errorf("announce posted %q, want ana and api-reviewers tagged", post)
	}
}

func TestAnnounceTagsNoOneLinkedOnlyInAnotherWorkspace(t *testing.T) {
	// Arrange
	fakeGh(t, ghResponses{pulls: openPull("Add login")})
	fakeSlack(t, map[string]slackAnswer{
		"/auth.test": {http.StatusOK, `{"ok":true,"team":"Other","user":"ana","team_id":"T0OTHER"}`},
	})
	repo := ownedRepo(t)
	writeFile(t, repo, slackLoggedInConfig())
	home := keptLinks(t)

	// Act
	printed, err := runStreamsAt(t, place{dir: repo, home: home}, unusedPrompt(t), "announce", "--dry-run")
	// Assert
	if err != nil {
		t.Fatalf("announce --dry-run: %v (%+v)", err, printed)
	}

	if !strings.Contains(printed.stdout, "tags no one\n") ||
		!strings.Contains(printed.stdout, "not tagged: ana (not linked") {
		t.Errorf("announce previewed %q, want ana, linked in another workspace, named as not linked here",
			printed.stdout)
	}
}

func TestAnnounceWithATokenMissingAScopePostsUntaggedAndNamesIt(t *testing.T) {
	for scope, path := range map[string]string{
		"channels:read":   "/conversations.members",
		"usergroups:read": "/usergroups.list",
	} {
		t.Run(scope, func(t *testing.T) {
			// Arrange
			fakeGh(t, ghResponses{pulls: openPull("Add login")})
			slack := fakeSlack(t, map[string]slackAnswer{
				path: {http.StatusOK, `{"ok":false,"error":"missing_scope","needed":"` + scope + `"}`},
			})
			repo := ownedRepo(t)
			writeFile(t, repo, slackLoggedInConfig())
			home := keptLinks(t)

			// Act
			printed, err := runStreamsAt(t, place{dir: repo, home: home}, unusedPrompt(t), "announce", "--yes")
			// Assert
			if err != nil {
				t.Fatalf("announce --yes: %v (%+v)", err, printed)
			}

			if !strings.Contains(printed.stderr, "lacks the "+scope+" scope") {
				t.Errorf("announce said %q, want the missing %s scope named", printed.stderr, scope)
			}

			if post := slack.post(t); !untagged(post) {
				t.Errorf("announce posted %q, want it untagged", post)
			}
		})
	}
}
