// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package cli_test

// `workflow announce` tags the code owners already linked to Slack and the
// groups that start checked, never asking, and names who it leaves untagged.

import (
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
// repository a group, chosen the last time.
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

	kept, repo, now := store.New(dir, false), "github.com/acme/repo", time.Now()
	ana := store.SlackTarget{ID: "U0ANA", Label: "Ana Souza"}

	for _, seed := range []error{
		kept.LinkOwner(t.Context(), "github.com", "ana", &ana, now),
		kept.SetRepoGroups(t.Context(), repo, []store.SlackTarget{{ID: "S0API", Label: "api-reviewers"}}, now),
		kept.RecordGroups(t.Context(), repo, []string{"S0API"}, now),
	} {
		if seed != nil {
			t.Fatalf("seeding the kept store: %v", seed)
		}
	}

	return home
}

func TestAnnouncePreviewsWhomItTagsAndWhomNot(t *testing.T) {
	// Arrange
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

	for _, line := range []string{
		"tags @Ana Souza @api-reviewers\n",
		"not tagged: acme/control-plane (not linked — associate in People and groups)\n",
	} {
		if !strings.Contains(printed.stdout, line) {
			t.Errorf("announce previewed %q, want the line %q", printed.stdout, line)
		}
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

// releaseBranchRepo is a repository whose branch left release, not main:
// release changed the ops paths ben owns, and the branch only the API ana
// owns. The open pull request merges into release.
func releaseBranchRepo(t *testing.T) string {
	t.Helper()

	repo := t.TempDir()
	gitInit(t, repo)
	writeRepoFile(t, repo, ".github/CODEOWNERS", "/api/ @ana\n/ops/ @ben\n")
	git(t, repo, "add", ".")
	commit(t, repo, "init")
	git(t, repo, "branch", "-M", "main")
	git(t, repo, "checkout", "-b", "release")
	writeRepoFile(t, repo, "ops/deploy.sh", "echo deploy\n")
	git(t, repo, "add", ".")
	commit(t, repo, "chore: deploy")
	git(t, repo, "checkout", "-b", "fix/PROJ-2-thing")
	writeRepoFile(t, repo, "api/pull.go", "package api\n")
	git(t, repo, "add", ".")
	commit(t, repo, "fix: guard the api")
	git(t, repo, "remote", "add", "origin", "https://github.com/acme/repo.git")
	pretendPushed(t, repo)
	writeFile(t, repo, slackUserConfig)

	return repo
}

func TestAnnounceReadsTheOwnersAgainstThePullRequestsBase(t *testing.T) {
	// Arrange
	fakeGh(t, ghResponses{pulls: `[{"number":7,"html_url":"https://github.com/acme/repo/pull/7",` +
		`"title":"Add login","state":"open","draft":false,"base":{"ref":"release"}}]`})
	repo := releaseBranchRepo(t)
	home := keptLinks(t)

	// Act
	printed, err := runStreamsAt(t, place{dir: repo, home: home}, unusedPrompt(t), "announce", "--dry-run")
	// Assert
	if err != nil {
		t.Fatalf("announce --dry-run: %v (%+v)", err, printed)
	}

	if !strings.Contains(printed.stdout, "tags @Ana Souza") || strings.Contains(printed.stdout, "ben") {
		t.Errorf("announce previewed %q, want ana tagged and ben, who owns only release's change, not named",
			printed.stdout)
	}
}
