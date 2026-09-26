// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package cli_test

import (
	"net/http"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/jira"
	"github.com/jacob-delgado/workflow/internal/wiring"
)

// A command run under --dry-run reads what the store kept, when there is one,
// and never makes one: each of these runs in a fresh home, with no store yet,
// on a branch whose open pull request sends it to ask the store what was
// announced.

func TestAnnounceDryRunMakesNoStore(t *testing.T) {
	// Arrange
	fakeGh(t, ghResponses{pulls: openPull("Add login")})
	repo := githubRepo(t, "fix/PROJ-2-thing")
	writeFile(t, repo, forgeCLIConfig)

	// Act
	printed, err := runStreams(t, repo, unusedPrompt(t), "announce", "--dry-run")
	// Assert
	if err != nil {
		t.Fatalf("announce --dry-run: %v (%+v)", err, printed)
	}

	if kept, found := storeKept(t); found {
		t.Errorf("announce --dry-run made the store at %s, want nothing on disk", kept)
	}
}

func TestStatusOfARepositoryUnderDryRunMakesNoStore(t *testing.T) {
	// Arrange
	server := jiraServer(t, http.StatusOK, issueFixture("PROJ-7", "Bug", "Fix login"), new(atomic.Bool))
	fakeGh(t, ghResponses{pulls: openPull("Add login"), status: runningStatus()})
	repo := statusFeatureRepo(t, server.URL)

	// Act
	// Naming the repository reads it the way status reads several at once.
	printed, err := runStreams(t, t.TempDir(), unusedPrompt(t), "status", "--dry-run", repo)
	// Assert
	if err != nil {
		t.Fatalf("status --dry-run: %v (%+v)", err, printed)
	}

	if kept, found := storeKept(t); found {
		t.Errorf("status --dry-run made the store at %s, want nothing on disk", kept)
	}
}

// keptView is the one issue view the interface's configuration names, and
// keptSummary the summary of the one issue a store holds for it.
const (
	keptView         = "project = PROJ ORDER BY updated DESC"
	keptViewSettings = `{"jira": {"base_url": "https://jira.example.net", ` +
		`"views": [{"name": "Recent", "jql": "` + keptView + `"}]}}`
	keptSummary = "Seeded from the store"
)

// issueListKept is a home whose store holds the issue list of keptView, as a
// live session of the interface leaves it, for a run in dir, whose
// configuration names that view.
func issueListKept(t *testing.T, dir string) place {
	t.Helper()

	where := place{dir: dir, home: t.TempDir()}
	for name, value := range isolatedEnvironment(where.home) {
		t.Setenv(name, value)
	}

	cfg, err := config.Load(where.dir, where.home)
	if err != nil {
		t.Fatalf("loading the configuration: %v", err)
	}

	deps, _ := wiring.Deps(t.Context(), cfg, wiring.Locate(t.Context(), where.dir), nil)
	deps.Store.CacheIssues(keptView, []jira.Issue{{Key: "PROJ-9", Summary: keptSummary}})

	return where
}

func TestADryRunInterfaceOpensWithoutTheKeptIssueList(t *testing.T) {
	// Arrange
	dir := t.TempDir()
	writeFile(t, dir, keptViewSettings)
	where := issueListKept(t, dir)

	// Act
	ran := runRootAt(t, where, "--dry-run")

	// Assert
	if ran.err != nil || ran.interfaces != 1 {
		t.Fatalf("workflow --dry-run = %v, opened %d interfaces; want the interface", ran.err, ran.interfaces)
	}

	if view := ansi.Strip(ran.model.View().Content); strings.Contains(view, keptSummary) {
		t.Errorf("workflow --dry-run opened on the store's issue list, want no store:\n%s", view)
	}
}

// The twin of the test above, so its Assert is seen to fail when the store is
// read: the same interface, its writes live, opens on the list the store kept.
func TestTheInterfaceOpensOnTheKeptIssueList(t *testing.T) {
	// Arrange
	dir := t.TempDir()
	writeFile(t, dir, keptViewSettings)
	where := issueListKept(t, dir)

	// Act
	ran := runRootAt(t, where)

	// Assert
	if ran.err != nil || ran.interfaces != 1 {
		t.Fatalf("workflow = %v, opened %d interfaces; want the interface", ran.err, ran.interfaces)
	}

	if view := ansi.Strip(ran.model.View().Content); !strings.Contains(view, keptSummary) {
		t.Errorf("workflow opened without the store's issue list, want it seeded from the store:\n%s", view)
	}
}
