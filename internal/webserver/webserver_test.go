// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package webserver_test

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jacob-delgado/workflow/internal/api"
	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/gitrepo"
	"github.com/jacob-delgado/workflow/internal/jira"
	"github.com/jacob-delgado/workflow/internal/webserver"
)

// errSeam is what a failing seam returns; the server maps every seam error to a
// generic 500 so the wire message carries no detail.
var errSeam = errors.New("the seam failed")

// Fixtures the tests share.
const (
	testKey        = "PROJ-412"
	testReporter   = "Ana Lopez"
	testBranchName = "fix/PROJ-412"
	testAuthor     = "octocat"
	testVersion    = "1.2.3"
	testBugJQL     = "type = Bug"
	testBugView    = "Bugs"
	testBase       = "origin/main"
	testChannel    = "#dev-workflow"
	// testCommitSubject and testCommitHash are the branch's one commit, shared by
	// the write-action tests that need a branch with a commit on it.
	testCommitSubject = "feat: redact"
	testCommitHash    = "abc1234"
	// noBranchSeam is the shared name for the "no branch seam" case the write
	// handlers' unavailability tables each exercise.
	noBranchSeam = "no branch seam"
	// noForgeSeam is its twin for the reads and writes that ask the forge.
	noForgeSeam = "no forge seam"
	// waitAndTryAgain is what an answer to a service limiting requests says to
	// do, whichever service it is.
	waitAndTryAgain = "wait and try again"
	// tryAgain is what the internal problem's detail says to do, whichever
	// request met it.
	tryAgain = "try again"
	// repoPath is where the repository sits on disk, which a git error can
	// carry and no answer may repeat.
	repoPath = "/home/dev/src/acme"
	// loopbackHost is the Host the shared request helpers send, so requests pass
	// the loopback guard the same way a browser on 127.0.0.1 does. A test that
	// exercises the guard sets its own Host instead.
	loopbackHost = "127.0.0.1:7000"
)

// filledDeps is a Deps with every seam populated with canned answers. A test
// nils a seam to exercise the not-configured path, or replaces one to fail.
func filledDeps() webserver.Deps {
	return webserver.Deps{
		Search: func(string, int) (jira.SearchResult, error) {
			return jira.SearchResult{
				Issues: []jira.Issue{{
					Key: testKey, Summary: "Fix token redaction", Status: "In Progress",
					StatusCategory: "indeterminate", Type: "Bug", Priority: "High",
				}},
				Total: 1,
			}, nil
		},
		Issue: func(key jira.Key) (jira.IssueDetail, error) {
			return jira.IssueDetail{
				Issue: jira.Issue{
					Key: key, Summary: "Fix token redaction", Status: "In Progress",
					StatusCategory: "indeterminate", Type: "Bug",
				},
				Reporter:     testReporter,
				Description:  "Tokens reach the log.",
				Comments:     []jira.Comment{{Author: testReporter, Body: "Repro'd", Created: time.Unix(0, 0).UTC()}},
				CommentTotal: 1,
			}, nil
		},
		Branch: func() (gitrepo.Branch, error) {
			return gitrepo.Branch{
				Name: testBranchName, Base: testBase, Ahead: 2, Head: "abc123", PushRemote: gitrepo.DefaultRemote,
				Commits: []gitrepo.Commit{{Hash: "abc123", Subject: testCommitSubject}},
			}, nil
		},
		Changes: func() ([]gitrepo.Change, error) {
			return []gitrepo.Change{{Path: "internal/config/config.go", Staged: 'M'}}, nil
		},
		FindPull: func(string) (forge.PullRequest, bool, error) {
			pull := forge.PullRequest{Number: 42, URL: "https://x/42", Title: "redact", Mergeable: forge.MergeClean}

			return pull, true, nil
		},
		CheckCI: func(forge.PullRequest, string) (forge.CI, error) {
			return forge.CI{
				State: forge.CIPassed, Total: 3, Done: 3,
				Checks: []forge.Check{{Name: "build", State: forge.CIPassed}},
			}, nil
		},
		Author: func() (string, error) { return testAuthor, nil },
	}
}

// serve builds the API handler over deps and cfg, not in dry-run, so the write
// endpoints are reachable — the common case. A test that exercises dry-run's
// read-only guard passes its own Info. Handler fails only when the embedded spec
// cannot load, which is a build defect, so the test fails there.
func serve(t *testing.T, deps webserver.Deps, cfg config.Config) http.Handler {
	t.Helper()

	return serveWith(t, deps, cfg, webserver.Info{Version: testVersion})
}

// serveWith is serve with the caller's Info, for the tests that need a specific
// stream interval.
func serveWith(t *testing.T, deps webserver.Deps, cfg config.Config, info webserver.Info) http.Handler {
	t.Helper()

	handler, err := webserver.Handler(deps, cfg, info, nil)
	if err != nil {
		t.Fatalf("building the handler: %v", err)
	}

	return handler
}

// get sends a GET and returns the recorder.
func get(t *testing.T, handler http.Handler, target string) *httptest.ResponseRecorder {
	t.Helper()

	return send(t, handler, http.MethodGet, target, "")
}

// send sends a request with an optional body and returns the recorder.
func send(t *testing.T, handler http.Handler, method, target, body string) *httptest.ResponseRecorder {
	t.Helper()

	var reader io.Reader
	if body != "" {
		reader = strings.NewReader(body)
	}

	request := httptest.NewRequestWithContext(t.Context(), method, target, reader)
	request.Host = loopbackHost

	if body != "" {
		request.Header.Set("Content-Type", "application/json")
	}

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)

	return recorder
}

// decode unmarshals the recorder's JSON body into T, failing the test on error.
func decode[T any](t *testing.T, recorder *httptest.ResponseRecorder) T {
	t.Helper()

	var value T

	err := json.Unmarshal(recorder.Body.Bytes(), &value)
	if err != nil {
		t.Fatalf("decoding %T from %q: %v", value, recorder.Body.String(), err)
	}

	return value
}

func TestGetHealthReportsTheBuild(t *testing.T) {
	t.Parallel()

	// Arrange
	dryRun := webserver.Info{Version: testVersion, DryRun: true}

	// Act
	recorder := get(t, serveWith(t, webserver.Deps{}, config.Default(), dryRun), "/api/health")

	// Assert
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", recorder.Code)
	}

	health := decode[api.Health](t, recorder)
	if health.Version != testVersion || !health.DryRun {
		t.Errorf("health = %+v, want version 1.2.3 and dry_run true", health)
	}
}

func TestGetHealthNamesTheForgesOwnWords(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		kind                forge.Kind
		wantNoun, wantSigil string
	}{
		"GitLab":          {kind: forge.KindGitLab, wantNoun: "merge request", wantSigil: "!"},
		"GitHub":          {kind: forge.KindGitHub, wantNoun: "pull request", wantSigil: "#"},
		"a forge unnamed": {kind: forge.KindUnknown, wantNoun: "pull request", wantSigil: "#"},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			info := webserver.Info{Version: testVersion, ForgeKind: tt.kind}

			// Act
			recorder := get(t, serveWith(t, webserver.Deps{}, config.Default(), info), "/api/health")

			// Assert
			health := decode[api.Health](t, recorder)
			if health.ForgeNoun != tt.wantNoun || health.ForgeSigil != tt.wantSigil {
				t.Errorf("health = %+v, want forge_noun %q and forge_sigil %q", health, tt.wantNoun, tt.wantSigil)
			}
		})
	}
}

func TestWhatThePageIsToldSaysMergeRequestOnGitLab(t *testing.T) {
	t.Parallel()

	nothingOpen := func(deps webserver.Deps) webserver.Deps {
		deps.FindPull = func(string) (forge.PullRequest, bool, error) { return forge.PullRequest{}, false, nil }

		return deps
	}
	noCommits := func(deps webserver.Deps) webserver.Deps {
		deps.Branch = func() (gitrepo.Branch, error) { return gitrepo.Branch{Name: testBranchName, Base: testBase}, nil }

		return deps
	}
	reviewersRefused := func(deps webserver.Deps) webserver.Deps {
		deps.CreatePull = func(forge.NewPullRequest) (forge.PullRequest, error) {
			return forge.PullRequest{Number: 7, URL: prURL, Title: prTitle}, forge.ErrRefused
		}

		return deps
	}
	noCreate := func(deps webserver.Deps) webserver.Deps {
		deps.CreatePull = nil

		return deps
	}
	unchanged := func(deps webserver.Deps) webserver.Deps { return deps }
	linkable := func(deps webserver.Deps) webserver.Deps {
		deps.LinkPullRequest = func(jira.Key, string, string) error { return nil }

		return deps
	}

	// Each case is a request the page makes and the answer it shows, on GitLab.
	const openPath = "/api/pull-request"

	announceBody := `{"channel":"#dev"}`
	cases := map[string]struct {
		mutate       func(webserver.Deps) webserver.Deps
		method, path string
		body         string
	}{
		"nothing to draft":         {noCommits, http.MethodGet, "/api/pull-request/draft", ""},
		"nothing to open":          {noCommits, http.MethodPost, openPath, openRequestBody},
		"opening is not available": {noCreate, http.MethodPost, openPath, openRequestBody},
		"reviewers were not added": {reviewersRefused, http.MethodPost, openPath, openRequestBody},
		"nothing to preview":       {nothingOpen, http.MethodGet, "/api/announcement", ""},
		"nothing to announce":      {nothingOpen, http.MethodPost, "/api/announce", announceBody},
		"linking is not available": {unchanged, http.MethodPost, linkPath, ""},
		"nothing to link":          {linkable, http.MethodPost, linkPath, ""},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			deps := tt.mutate(openableDeps())
			deps.Post = func(string, string) error { return nil }
			info := webserver.Info{Version: testVersion, ForgeKind: forge.KindGitLab}

			// Act
			recorder := send(t, serveWith(t, deps, config.Default(), info), tt.method, tt.path, tt.body)

			// Assert
			told := recorder.Body.String()
			if !strings.Contains(told, "merge request") || strings.Contains(told, "pull request") {
				t.Errorf("answer = %q, want it in GitLab's words: a merge request", told)
			}
		})
	}
}

func TestListViewsFallsBackToTheBuiltInList(t *testing.T) {
	t.Parallel()

	// Act
	recorder := get(t, serve(t, filledDeps(), config.Default()), "/api/views")

	// Assert
	views := decode[api.ViewList](t, recorder)
	if len(views.Views) != 1 || views.Views[0].Name != "Assigned to me" {
		t.Errorf("views = %+v, want the one built-in list", views.Views)
	}
}

func TestListViewsListsTheConfiguredViews(t *testing.T) {
	t.Parallel()

	// Arrange
	cfg := config.Default()
	cfg.Jira.Views = []config.JiraView{{Name: "Sprint", JQL: "sprint in openSprints()"}}

	// Act
	views := decode[api.ViewList](t, get(t, serve(t, filledDeps(), cfg), "/api/views"))

	// Assert
	if len(views.Views) != 1 || views.Views[0].Name != "Sprint" || views.Views[0].Jql != "sprint in openSprints()" {
		t.Errorf("views = %+v, want the configured Sprint view", views.Views)
	}
}

func TestAnErrorIsAnRFC9457Problem(t *testing.T) {
	t.Parallel()

	// Arrange
	// Any failure is answered as application/problem+json with the problem's type,
	// title and status populated — the RFC 9457 shape, not the old code+message.
	deps := filledDeps()
	deps.Issue = func(jira.Key) (jira.IssueDetail, error) { return jira.IssueDetail{}, errSeam }

	// Act
	recorder := get(t, serve(t, deps, config.Default()), "/api/issues/PROJ-1")

	// Assert
	failure := decode[api.Problem](t, recorder)
	if ct := recorder.Header().Get("Content-Type"); ct != "application/problem+json" {
		t.Errorf("Content-Type = %q, want application/problem+json", ct)
	}

	if failure.Type == "" || failure.Title == "" || failure.Status != http.StatusInternalServerError {
		t.Errorf("problem = %+v, want type, title and status 500 populated", failure)
	}
}

func TestGetBranchReturnsTheBranch(t *testing.T) {
	t.Parallel()

	// Act
	branch := decode[api.Branch](t, get(t, serve(t, filledDeps(), config.Default()), "/api/branch"))

	// Assert
	if branch.Name != testBranchName || branch.Base != testBase || branch.Ahead != 2 {
		t.Errorf("branch = %+v, want the current branch", branch)
	}
}

func TestGetBranchNamesTheRemoteItsPushGoesTo(t *testing.T) {
	t.Parallel()

	// Arrange
	const forkRemote = "fork"

	deps := filledDeps()
	deps.Branch = func() (gitrepo.Branch, error) {
		return gitrepo.Branch{Name: testBranchName, PushRemote: forkRemote}, nil
	}

	// Act
	branch := decode[api.Branch](t, get(t, serve(t, deps, config.Default()), "/api/branch"))

	// Assert
	if branch.PushRemote != forkRemote {
		t.Errorf("push_remote = %q, want %q", branch.PushRemote, forkRemote)
	}
}

func TestGetBranchIsEmptyOutsideARepository(t *testing.T) {
	t.Parallel()

	// Arrange
	deps := filledDeps()
	deps.Branch = nil

	// Act
	branch := decode[api.Branch](t, get(t, serve(t, deps, config.Default()), "/api/branch"))

	// Assert
	if branch.Name != "" || len(branch.Commits) != 0 {
		t.Errorf("branch = %+v, want an empty branch", branch)
	}
}

func TestListChangesReturnsTheWorkingTree(t *testing.T) {
	t.Parallel()

	// Act
	changes := decode[api.ChangeList](t, get(t, serve(t, filledDeps(), config.Default()), "/api/changes"))

	// Assert
	if len(changes.Changes) != 1 || changes.Changes[0].Path != "internal/config/config.go" {
		t.Errorf("changes = %+v, want the one staged file", changes.Changes)
	}
}

func TestGetMessagingReturnsTheDestination(t *testing.T) {
	t.Parallel()

	// Arrange
	cfg := config.Default()
	cfg.Messaging.Token = "xoxb-t"
	cfg.Messaging.Channel = "#dev"

	// Act
	destination := decode[api.MessagingDestination](t, get(t, serve(t, filledDeps(), cfg), "/api/messaging"))

	// Assert
	if destination.Service != "Slack" || !destination.Configured ||
		destination.Channel != "#dev" || destination.Author != testAuthor {
		t.Errorf("destination = %+v, want configured Slack, #dev and octocat", destination)
	}
}

func TestGetMessagingMarksAWebhookServiceConfigured(t *testing.T) {
	t.Parallel()

	// Arrange
	// A Teams webhook is fully configured yet carries no channel, so the
	// destination must report it configured without one.
	cfg := config.Default()
	cfg.Messaging.Kind = "teams"
	cfg.Messaging.WebhookURL = "https://example.com/hook"

	// Act
	destination := decode[api.MessagingDestination](t, get(t, serve(t, filledDeps(), cfg), "/api/messaging"))

	// Assert
	if destination.Service != "Teams" || !destination.Configured || destination.Channel != "" {
		t.Errorf("destination = %+v, want a configured Teams with no channel", destination)
	}
}

func TestGetMessagingHasNoAuthorWhenTheForgeCannotSay(t *testing.T) {
	t.Parallel()

	// Arrange
	deps := filledDeps()
	deps.Author = func() (string, error) { return "", errSeam }

	// Act
	destination := decode[api.MessagingDestination](t, get(t, serve(t, deps, config.Default()), "/api/messaging"))

	// Assert
	if destination.Author != "" {
		t.Errorf("author = %q, want empty when the forge cannot say", destination.Author)
	}
}
