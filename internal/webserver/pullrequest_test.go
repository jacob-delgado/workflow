// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package webserver_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jacob-delgado/workflow/internal/api"
	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/gitrepo"
	"github.com/jacob-delgado/workflow/internal/proc"
	"github.com/jacob-delgado/workflow/internal/webserver"
)

const (
	prTitle = "fix: redact tokens"
	prBase  = "main"
	prURL   = "https://x/7"
)

// openRequestBody is a complete request opening a pull request titled prTitle
// into prBase.
const openRequestBody = `{"title":"` + prTitle + `","base":"` + prBase + `","body":"why"}`

// pushedBranch is the checked-out branch already published to its remote, with a
// commit and no open pull request, so opening one needs no push first.
func pushedBranch() gitrepo.Branch {
	return gitrepo.Branch{
		Name: testBranchName, Base: testBase, Upstream: "origin/" + testBranchName, Ahead: 0,
		Commits: []gitrepo.Commit{{Hash: testCommitHash, Subject: testCommitSubject}},
	}
}

// openableDeps has an unpushed branch with commits and no open pull request, so
// a pull request can be composed and opened; the push and create seams record
// what they were asked to do.
func openableDeps() webserver.Deps {
	deps := filledDeps()
	deps.FindPull = func(string) (forge.PullRequest, bool, error) { return forge.PullRequest{}, false, nil }
	deps.Push = func(string) (proc.Output, error) { return fakeOutput(nil, nil), nil }
	deps.CreatePull = func(forge.NewPullRequest) (forge.PullRequest, error) {
		return forge.PullRequest{Number: 7, URL: prURL, Title: "opened", Mergeable: forge.MergeClean}, nil
	}

	return deps
}

func TestOpenPullRequestForwardsReviewersAssigneesAndLabels(t *testing.T) {
	t.Parallel()

	// Arrange
	var request forge.NewPullRequest

	deps := openableDeps()
	deps.CreatePull = func(newPull forge.NewPullRequest) (forge.PullRequest, error) {
		request = newPull

		return forge.PullRequest{Number: 7, URL: prURL, Title: newPull.Title}, nil
	}

	// A padded reviewer and a blank label are cleaned rather than sent on.
	body := `{"title":"` + prTitle + `","base":"` + prBase +
		`","reviewers":["ana"," ben "],"assignees":["cass"],"labels":["bug",""]}`

	// Act
	recorder := doOpen(t, deps, body)

	// Assert
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", recorder.Code)
	}

	if got := strings.Join(request.Reviewers, ","); got != "ana,ben" {
		t.Errorf("reviewers = %q, want ana,ben trimmed", got)
	}

	if got := strings.Join(request.Assignees, ","); got != "cass" {
		t.Errorf("assignees = %q, want cass", got)
	}

	if got := strings.Join(request.Labels, ","); got != "bug" {
		t.Errorf("labels = %q, want bug with the blank dropped", got)
	}
}

func TestOpenPullRequestKeepsAPullWhoseReviewersCouldNotBeAdded(t *testing.T) {
	t.Parallel()

	// Arrange
	// The pull opens, but the token cannot add its reviewers.
	deps := openableDeps()
	deps.CreatePull = func(newPull forge.NewPullRequest) (forge.PullRequest, error) {
		return forge.PullRequest{Number: 7, URL: prURL, Title: newPull.Title}, forge.ErrRefused
	}

	// Act
	recorder := doOpen(t, deps, openRequestBody)

	// Assert
	// It is reported open, not lost to a 422, and carries the warning.
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 for a pull that opened", recorder.Code)
	}

	opened := decode[api.OpenedPullRequest](t, recorder)
	if opened.Pull.Number != 7 {
		t.Errorf("pull number = %d, want the opened 7", opened.Pull.Number)
	}

	if opened.Warning == nil || !strings.Contains(*opened.Warning, "could not all be added") {
		t.Errorf("warning = %v, want a note that the reviewers were not added", opened.Warning)
	}
}

// doOpen posts an open-pull-request request with the given JSON body.
func doOpen(t *testing.T, deps webserver.Deps, body string) *httptest.ResponseRecorder {
	t.Helper()

	return send(t, serve(t, deps, config.Default()), http.MethodPost, "/api/pull-request", body)
}

func TestGetPullRequestDraftComposesTheProposal(t *testing.T) {
	t.Parallel()

	// Arrange
	deps := openableDeps()

	// Act
	recorder := get(t, serve(t, deps, config.Default()), "/api/pull-request/draft")

	// Assert
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", recorder.Code)
	}

	draft := decode[api.PullRequestDraft](t, recorder)
	if draft.Title != testCommitSubject || draft.Base != prBase || draft.Head != testBranchName {
		t.Errorf("draft = %+v, want the first commit's title, base %q, head %q", draft, prBase, testBranchName)
	}

	if !draft.NeedsPush || !strings.Contains(draft.Body, testCommitSubject) {
		t.Errorf("draft = %+v, want needs_push and the commit in the body", draft)
	}
}

func TestGetPullRequestDraftTakesItsTitleFromTheConfiguredSource(t *testing.T) {
	t.Parallel()

	// Arrange
	// With the issue as the title source, the draft title is the issue rather than
	// the branch's oldest commit (testCommitSubject).
	deps := openableDeps()
	cfg := config.Default()
	cfg.PullRequest.TitleSource = "issue"

	// Act
	recorder := get(t, serve(t, deps, cfg), "/api/pull-request/draft")

	// Assert
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", recorder.Code)
	}

	if draft := decode[api.PullRequestDraft](t, recorder); draft.Title != "PROJ-412: Fix token redaction" {
		t.Errorf("draft title = %q, want the configured issue source", draft.Title)
	}
}

func TestGetPullRequestDraftBuildsTheBodyFromTheRepositoryTemplate(t *testing.T) {
	t.Parallel()

	// Arrange
	const marker = "<!-- repository pull request template -->"

	deps := openableDeps()
	deps.Templates = func() []forge.Template {
		return []forge.Template{{Name: "default", Path: ".github/pull_request_template.md", Body: marker}}
	}

	// Act
	recorder := get(t, serve(t, deps, config.Default()), "/api/pull-request/draft")

	// Assert
	if body := decode[api.PullRequestDraft](t, recorder).Body; !strings.Contains(body, marker) {
		t.Errorf("body = %q, want the repository template used", body)
	}
}

func TestGetPullRequestDraftDoesNotNeedPushForAPublishedBranch(t *testing.T) {
	t.Parallel()

	// Arrange
	deps := openableDeps()
	deps.Branch = func() (gitrepo.Branch, error) { return pushedBranch(), nil }

	// Act
	recorder := get(t, serve(t, deps, config.Default()), "/api/pull-request/draft")

	// Assert
	if decode[api.PullRequestDraft](t, recorder).NeedsPush {
		t.Error("needs_push = true for an already-published branch; want false")
	}
}

func TestGetPullRequestDraftComposesOverAMergedPull(t *testing.T) {
	t.Parallel()

	// Arrange
	// The branch's earlier pull request has merged, and it still carries commits,
	// so a merged pull is not a conflict — a fresh one can be proposed.
	deps := openableDeps()
	deps.FindPull = func(string) (forge.PullRequest, bool, error) {
		return forge.PullRequest{Number: 1, State: forge.StateMerged}, true, nil
	}

	// Act
	recorder := get(t, serve(t, deps, config.Default()), "/api/pull-request/draft")

	// Assert
	if recorder.Code != http.StatusOK {
		t.Errorf("status = %d, want 200 when the branch's old pull has merged, not a 409", recorder.Code)
	}
}

func TestGetPullRequestDraftIsAConflictWhenNothingToOpen(t *testing.T) {
	t.Parallel()

	// There is nothing to open a pull request for whenever the branch cannot
	// carry a new one.
	commit := []gitrepo.Commit{{Hash: testCommitHash, Subject: testCommitSubject}}
	cases := map[string]func(webserver.Deps) webserver.Deps{
		"a pull request is already open": func(deps webserver.Deps) webserver.Deps {
			deps.FindPull = func(string) (forge.PullRequest, bool, error) {
				return forge.PullRequest{Number: 1}, true, nil
			}

			return deps
		},
		"the branch has no commits": func(deps webserver.Deps) webserver.Deps {
			deps.Branch = func() (gitrepo.Branch, error) {
				return gitrepo.Branch{Name: testBranchName, Base: testBase}, nil
			}

			return deps
		},
		"the tree is not on a branch": func(deps webserver.Deps) webserver.Deps {
			deps.Branch = func() (gitrepo.Branch, error) {
				return gitrepo.Branch{Name: "", Detached: true, Commits: commit}, nil
			}

			return deps
		},
	}

	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			deps := mutate(openableDeps())

			// Act
			recorder := get(t, serve(t, deps, config.Default()), "/api/pull-request/draft")

			// Assert
			if recorder.Code != http.StatusConflict {
				t.Errorf("status = %d, want 409 when there is nothing to open", recorder.Code)
			}
		})
	}
}

func TestOpenPullRequestPushesThenOpens(t *testing.T) {
	t.Parallel()

	// Arrange
	var (
		calls   []string
		request forge.NewPullRequest
	)

	deps := openableDeps()
	deps.Push = func(string) (proc.Output, error) {
		calls = append(calls, "push")

		return fakeOutput(nil, nil), nil
	}
	deps.CreatePull = func(newPull forge.NewPullRequest) (forge.PullRequest, error) {
		calls = append(calls, "open")
		request = newPull

		return forge.PullRequest{Number: 7, URL: prURL, Title: newPull.Title, Mergeable: forge.MergeClean}, nil
	}

	// Act
	recorder := doOpen(t, deps, openRequestBody)

	// Assert
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", recorder.Code)
	}

	// The unpushed branch is pushed first, then the pull request is opened — the
	// order matters, since a pull request cannot open from an unpushed branch.
	if len(calls) != 2 || calls[0] != "push" || calls[1] != "open" {
		t.Errorf("calls = %v, want the branch pushed before the pull request is opened", calls)
	}

	if request.Head != testBranchName || request.Base != prBase || request.Title != prTitle {
		t.Errorf("opened %+v, want head %q, base %q, and the given title", request, testBranchName, prBase)
	}

	if number := decode[api.OpenedPullRequest](t, recorder).Pull.Number; number != 7 {
		t.Errorf("response number = %d, want the opened pull request's 7", number)
	}
}

func TestOpenPullRequestOpensADraftWhenAsked(t *testing.T) {
	t.Parallel()

	// Arrange
	var request forge.NewPullRequest

	deps := openableDeps()
	deps.CreatePull = func(newPull forge.NewPullRequest) (forge.PullRequest, error) {
		request = newPull

		return forge.PullRequest{Number: 7, URL: prURL, Title: newPull.Title, Draft: true}, nil
	}

	// Act
	recorder := doOpen(t, deps, `{"title":"`+prTitle+`","base":"`+prBase+`","draft":true}`)

	// Assert
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", recorder.Code)
	}

	if !request.Draft {
		t.Error("opened a non-draft pull request; want the requested draft flag carried through")
	}
}

func TestOpenPullRequestDoesNotPushAnAlreadyPublishedBranch(t *testing.T) {
	t.Parallel()

	// Arrange
	pushed := false

	deps := openableDeps()
	deps.Branch = func() (gitrepo.Branch, error) { return pushedBranch(), nil }
	deps.Push = func(string) (proc.Output, error) {
		pushed = true

		return fakeOutput(nil, nil), nil
	}

	// Act
	recorder := doOpen(t, deps, openRequestBody)

	// Assert
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", recorder.Code)
	}

	if pushed {
		t.Error("pushed an already-published branch; want the push skipped")
	}
}

func TestOpenPullRequestIsAConflictWhenOneIsAlreadyOpen(t *testing.T) {
	t.Parallel()

	// Arrange
	deps := openableDeps()
	deps.FindPull = func(string) (forge.PullRequest, bool, error) { return forge.PullRequest{Number: 1}, true, nil }

	// Act
	recorder := doOpen(t, deps, openRequestBody)

	// Assert
	if recorder.Code != http.StatusConflict {
		t.Errorf("status = %d, want 409 when a pull request is already open", recorder.Code)
	}
}

func TestOpenPullRequestRefusesAnIncompleteRequest(t *testing.T) {
	t.Parallel()

	// A title and a base are both required; a present-but-blank field is as
	// missing as an absent one.
	cases := map[string]string{
		"a blank title": `{"title":"  ","base":"main"}`,
		"a blank base":  `{"title":"fix: x","base":""}`,
	}

	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			deps := openableDeps()

			// Act
			recorder := doOpen(t, deps, body)

			// Assert
			if recorder.Code != http.StatusUnprocessableEntity {
				t.Errorf("status = %d, want 422 for %s", recorder.Code, name)
			}
		})
	}
}

func TestOpenPullRequestReportsAFailedPush(t *testing.T) {
	t.Parallel()

	// Arrange
	deps := openableDeps()
	deps.Push = func(string) (proc.Output, error) {
		return fakeOutput([]string{"! [rejected]"}, errSeam), nil
	}

	// Act
	recorder := doOpen(t, deps, openRequestBody)

	// Assert
	if recorder.Code != http.StatusUnprocessableEntity {
		t.Errorf("status = %d, want 422 when the push fails", recorder.Code)
	}
}

func TestOpenPullRequestReportsAFailedOpen(t *testing.T) {
	t.Parallel()

	// Arrange
	// The branch is already pushed, so the failure is the open itself, not a push.
	opened := false

	deps := openableDeps()
	deps.Branch = func() (gitrepo.Branch, error) { return pushedBranch(), nil }
	deps.CreatePull = func(forge.NewPullRequest) (forge.PullRequest, error) {
		opened = true

		return forge.PullRequest{}, forge.ErrRejected
	}

	// Act
	recorder := doOpen(t, deps, openRequestBody)

	// Assert
	if recorder.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422 when the open fails", recorder.Code)
	}

	if !opened || !strings.Contains(recorder.Body.String(), "rejected") {
		t.Errorf("body = %q, want the forge's reason surfaced", recorder.Body.String())
	}
}

func TestOpenPullRequestIsUnreachableAndHidesTheForgeHost(t *testing.T) {
	t.Parallel()

	// Arrange
	// An unreachable forge carries its host in the error; the answer must be a 502
	// whose detail does not leak that host.
	deps := openableDeps()
	deps.Branch = func() (gitrepo.Branch, error) { return pushedBranch(), nil }
	deps.CreatePull = func(forge.NewPullRequest) (forge.PullRequest, error) {
		return forge.PullRequest{}, fmt.Errorf("%w: https://git.internal.example", forge.ErrUnreachable)
	}

	// Act
	recorder := doOpen(t, deps, openRequestBody)

	// Assert
	failure := decode[api.Problem](t, recorder)
	if recorder.Code != http.StatusBadGateway || failure.Code != api.Unreachable {
		t.Errorf("status/code = %d/%s, want 502/unreachable", recorder.Code, failure.Code)
	}

	if strings.Contains(recorder.Body.String(), "git.internal.example") {
		t.Errorf("body = %q, leaks the forge host", recorder.Body.String())
	}
}

func TestOpenPullRequestIsUnavailableWithoutItsSeams(t *testing.T) {
	t.Parallel()

	// Opening needs the create, branch, and push seams; a nil one is not
	// available rather than a server error.
	cases := map[string]func(webserver.Deps) webserver.Deps{
		"no create seam": func(deps webserver.Deps) webserver.Deps {
			deps.CreatePull = nil

			return deps
		},
		"no push seam": func(deps webserver.Deps) webserver.Deps {
			deps.Push = nil

			return deps
		},
	}

	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			deps := mutate(openableDeps())

			// Act
			recorder := doOpen(t, deps, openRequestBody)

			// Assert
			if recorder.Code != http.StatusUnprocessableEntity {
				t.Errorf("status = %d, want 422 when opening is not available", recorder.Code)
			}
		})
	}
}
