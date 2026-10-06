// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package webserver_test

import (
	"fmt"
	"net/http"
	"slices"
	"strings"
	"testing"

	"github.com/jacob-delgado/workflow/internal/api"
	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/gitrepo"
	"github.com/jacob-delgado/workflow/internal/webserver"
)

// Where the review's writes go.
const (
	pullAt   = "/api/pull-request"
	mergeAt  = "/api/pull-request/merge"
	finishAt = "/api/branch/finish"
	rerunAt  = "/api/review/rerun"
)

// The bodies of an edit and of a merge by squash.
const (
	anEdit   = `{"title":"t","body":""}`
	bySquash = `{"method":"squash"}`
)

// pullTitle is #42's title.
const pullTitle = "redact"

// bugfixTemplate is a repository's second pull request template.
const bugfixTemplate = "bugfix"

// reviewWrites records what the review's write seams were asked.
type reviewWrites struct {
	edits    []forge.PullRequestEdit
	merges   []forge.MergeMethod
	finishes []string
	reruns   []string
}

// sent is how many writes were made.
func (w *reviewWrites) sent() int {
	return len(w.edits) + len(w.merges) + len(w.finishes) + len(w.reruns)
}

// reviewDeps is filledDeps with #42 approved, green and clean — mergeable —
// and the review's write seams recording into writes.
func reviewDeps(writes *reviewWrites) webserver.Deps {
	deps := filledDeps()
	deps.FindPull = func(string) (forge.PullRequest, bool, error) {
		return forge.PullRequest{
			Number: 42, Title: pullTitle, Body: "Why: tokens leak.",
			Mergeable: forge.MergeClean, Approvals: 1,
		}, true, nil
	}
	deps.EditPull = func(pull forge.PullRequest, edit forge.PullRequestEdit) (forge.PullRequest, error) {
		writes.edits = append(writes.edits, edit)
		pull.Title, pull.Body = edit.Title, edit.Body

		return pull, nil
	}
	deps.MergeMethods = func() ([]forge.MergeMethod, error) {
		return []forge.MergeMethod{forge.MergeSquash, forge.MergeRebase}, nil
	}
	deps.Merge = func(_ forge.PullRequest, method forge.MergeMethod) error {
		writes.merges = append(writes.merges, method)

		return nil
	}
	deps.Finish = func(branch, base string) error {
		writes.finishes = append(writes.finishes, branch+" onto "+base)

		return nil
	}
	deps.Rerun = func(pull forge.PullRequest, head string) (bool, error) {
		writes.reruns = append(writes.reruns, fmt.Sprintf("#%d at %s", pull.Number, head))

		return true, nil
	}

	return deps
}

// merged makes deps' pull request one that has merged.
func merged(deps webserver.Deps) webserver.Deps {
	deps.FindPull = func(string) (forge.PullRequest, bool, error) {
		return forge.PullRequest{Number: 42, Title: pullTitle, State: forge.StateMerged}, true, nil
	}

	return deps
}

// failedCI makes deps' CI one that failed.
func failedCI(deps webserver.Deps) webserver.Deps {
	deps.CheckCI = func(forge.PullRequest, string) (forge.CI, error) {
		return forge.CI{State: forge.CIFailed, Total: 1, Done: 1, Failed: 1}, nil
	}

	return deps
}

// refusedByForge is the forge refusing a write the token may not make, its
// reason naming its own address.
func refusedByForge() error {
	return &forge.RefusalError{
		Kind: forge.KindGitHub, Status: forge.ErrRefused,
		Reason: "Resource not accessible by integration at https://" + forgeHost + "/api",
	}
}

func TestEditReadsThePullRequestsTextAfresh(t *testing.T) {
	t.Parallel()

	// Act
	recorder := get(t, serve(t, reviewDeps(&reviewWrites{}), config.Default()), pullAt)

	// Assert
	text := decode[api.PullRequestText](t, recorder)
	if text.Title != pullTitle || text.Body != "Why: tokens leak." {
		t.Errorf("text = %+v, want the pull request's title and description", text)
	}
}

func TestEditSavesTheTitleAndDescription(t *testing.T) {
	t.Parallel()

	// Arrange
	writes := &reviewWrites{}
	handler := serve(t, reviewDeps(writes), config.Default())

	// Act
	recorder := send(t, handler, http.MethodPatch, pullAt, `{"title":"Redact every token","body":"Why: all of them."}`)

	// Assert
	pull := decode[api.PullRequest](t, recorder)
	if pull.Title != "Redact every token" {
		t.Errorf("answered %+v, want the pull request as edited", pull)
	}

	want := []forge.PullRequestEdit{{Title: "Redact every token", Body: "Why: all of them."}}
	if !slices.Equal(writes.edits, want) {
		t.Errorf("edits = %+v, want %+v", writes.edits, want)
	}
}

func TestReviewWritesAreRefusedWhenThereIsNothingToWriteTo(t *testing.T) {
	t.Parallel()

	noPull := func(deps webserver.Deps) webserver.Deps {
		deps.FindPull = func(string) (forge.PullRequest, bool, error) { return forge.PullRequest{}, false, nil }

		return deps
	}
	unapproved := func(deps webserver.Deps) webserver.Deps {
		deps.FindPull = func(string) (forge.PullRequest, bool, error) {
			return forge.PullRequest{Number: 42, Mergeable: forge.MergeClean}, true, nil
		}

		return deps
	}
	ahead := func(deps webserver.Deps) webserver.Deps {
		deps = merged(deps)
		deps.Branch = func() (gitrepo.Branch, error) {
			return gitrepo.Branch{
				Name: testBranchName, Base: testBase, Upstream: "origin/" + testBranchName, Ahead: 1,
				PushRemote: gitrepo.DefaultRemote,
			}, nil
		}

		return deps
	}

	cases := map[string]struct {
		method, path, body string
		shape              func(webserver.Deps) webserver.Deps
		saying             string
	}{
		"reading the text of no pull request": {http.MethodGet, pullAt, "", noPull, "no open"},
		"editing a merged pull request":       {http.MethodPatch, pullAt, anEdit, merged, "no open"},
		"merging an unapproved one": {
			http.MethodPost, mergeAt, bySquash, unapproved, "cannot be merged",
		},
		"offering to merge an unapproved one": {http.MethodGet, mergeAt, "", unapproved, "cannot be merged"},
		"finishing an open one":               {http.MethodPost, finishAt, "", nil, "cannot be finished"},
		"finishing with a commit unpushed":    {http.MethodPost, finishAt, "", ahead, "cannot be finished"},
		"re-running green CI":                 {http.MethodPost, rerunAt, "", nil, "nothing to re-run"},
		"re-running a merged one's CI": {
			http.MethodPost, rerunAt, "", func(d webserver.Deps) webserver.Deps { return failedCI(merged(d)) },
			"nothing to re-run",
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			writes := &reviewWrites{}
			deps := reviewDeps(writes)

			if tt.shape != nil {
				deps = tt.shape(deps)
			}

			// Act
			recorder := send(t, serve(t, deps, config.Default()), tt.method, tt.path, tt.body)

			// Assert
			assertProblem(t, recorder, http.StatusConflict, tt.saying)

			if writes.sent() != 0 {
				t.Errorf("writes = %+v, want none", writes)
			}
		})
	}
}

func TestEditRefusesAnEmptyTitle(t *testing.T) {
	t.Parallel()

	// Arrange
	writes := &reviewWrites{}

	// Act
	recorder := send(t, serve(t, reviewDeps(writes), config.Default()), http.MethodPatch, pullAt,
		`{"title":"  ","body":"x"}`)

	// Assert
	assertProblem(t, recorder, http.StatusUnprocessableEntity, "a title is required")

	if writes.sent() != 0 {
		t.Errorf("writes = %+v, want none", writes)
	}
}

func TestMergeOffersTheMethodsTheRepositoryPermits(t *testing.T) {
	t.Parallel()

	// Act
	recorder := get(t, serve(t, reviewDeps(&reviewWrites{}), config.Default()), mergeAt)

	// Assert
	offer := decode[api.MergeOffer](t, recorder)
	if !slices.Equal(offer.Methods, []api.MergeMethod{api.MergeMethodSquash, api.MergeMethodRebase}) ||
		offer.Pull.Number != 42 {
		t.Errorf("offer = %+v, want #42 by squash or rebase", offer)
	}
}

func TestMergeOffersNothingWhenTheRepositoryPermitsNothing(t *testing.T) {
	t.Parallel()

	// Arrange
	deps := reviewDeps(&reviewWrites{})
	deps.MergeMethods = func() ([]forge.MergeMethod, error) { return nil, nil }

	// Act
	recorder := get(t, serve(t, deps, config.Default()), mergeAt)

	// Assert
	assertProblem(t, recorder, http.StatusUnprocessableEntity, "permits no merge method")
}

func TestMergeMergesByTheMethodChosen(t *testing.T) {
	t.Parallel()

	// Arrange
	writes := &reviewWrites{}

	// Act
	recorder := send(t, serve(t, reviewDeps(writes), config.Default()), http.MethodPost, mergeAt, `{"method":"rebase"}`)

	// Assert
	if recorder.Code != http.StatusOK || !slices.Equal(writes.merges, []forge.MergeMethod{forge.MergeRebase}) {
		t.Errorf("status %d, merges %v; want #42 merged by rebase", recorder.Code, writes.merges)
	}
}

func TestMergeRefusesAMethodTheRepositoryDoesNotPermit(t *testing.T) {
	t.Parallel()

	// Arrange
	writes := &reviewWrites{}

	// Act
	recorder := send(t, serve(t, reviewDeps(writes), config.Default()), http.MethodPost, mergeAt, `{"method":"merge"}`)

	// Assert
	assertProblem(t, recorder, http.StatusUnprocessableEntity, "does not permit")

	if writes.sent() != 0 {
		t.Errorf("merges = %v, want none", writes.merges)
	}
}

func TestARefusedMergeNamesTheMissingScopeAndNoHost(t *testing.T) {
	t.Parallel()

	// Arrange
	deps := reviewDeps(&reviewWrites{})
	deps.Merge = func(forge.PullRequest, forge.MergeMethod) error { return refusedByForge() }

	// Act
	recorder := send(t, serve(t, deps, config.Default()), http.MethodPost, mergeAt, bySquash)

	// Assert
	assertProblem(t, recorder, http.StatusUnprocessableEntity, "the token may lack the repo scope")

	if strings.Contains(recorder.Body.String(), forgeHost) {
		t.Errorf("the refusal %s names the forge's host", recorder.Body)
	}
}

func TestFinishFinishesAMergedBranch(t *testing.T) {
	t.Parallel()

	// Arrange
	writes := &reviewWrites{}

	// Act
	recorder := send(t, serve(t, merged(reviewDeps(writes)), config.Default()), http.MethodPost, finishAt, "")

	// Assert
	if recorder.Code != http.StatusOK || !slices.Equal(writes.finishes, []string{testBranchName + " onto main"}) {
		t.Errorf("status %d, finishes %v; want %s finished onto main", recorder.Code, writes.finishes, testBranchName)
	}
}

func TestAFinishGitRefusesKeepsItsWordsOff(t *testing.T) {
	t.Parallel()

	// Arrange
	deps := merged(reviewDeps(&reviewWrites{}))
	deps.Finish = func(string, string) error {
		return fmt.Errorf("git pull from https://%s/acme.git: %w", forgeHost, errSeam)
	}

	// Act
	recorder := send(t, serve(t, deps, config.Default()), http.MethodPost, finishAt, "")

	// Assert
	assertProblem(t, recorder, http.StatusUnprocessableEntity, "git would not finish")

	if strings.Contains(recorder.Body.String(), forgeHost) {
		t.Errorf("the refusal %s names origin", recorder.Body)
	}
}

func TestRerunRestartsTheFailedChecks(t *testing.T) {
	t.Parallel()

	// Arrange
	writes := &reviewWrites{}

	// Act
	recorder := send(t, serve(t, failedCI(reviewDeps(writes)), config.Default()), http.MethodPost, rerunAt, "")

	// Assert
	if !decode[api.Rerun](t, recorder).Reran || !slices.Equal(writes.reruns, []string{"#42 at " + filledHead}) {
		t.Errorf("reruns = %v, want #42's failed checks re-run at its head", writes.reruns)
	}
}

func TestARefusedRerunNamesTheScopeAndNoHost(t *testing.T) {
	t.Parallel()

	// Arrange
	deps := failedCI(reviewDeps(&reviewWrites{}))
	deps.Rerun = func(forge.PullRequest, string) (bool, error) { return false, refusedByForge() }

	// Act
	recorder := send(t, serve(t, deps, config.Default()), http.MethodPost, rerunAt, "")

	// Assert
	assertProblem(t, recorder, http.StatusUnprocessableEntity, "the token may lack")

	if strings.Contains(recorder.Body.String(), forgeHost) {
		t.Errorf("the refusal %s names the forge's host", recorder.Body)
	}
}

func TestReviewWritesWithoutTheirSeamAreNotAvailable(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		method, path, body string
		unset              func(*webserver.Deps)
	}{
		"an edit":     {http.MethodPatch, pullAt, anEdit, func(d *webserver.Deps) { d.EditPull = nil }},
		"a merge":     {http.MethodPost, mergeAt, bySquash, func(d *webserver.Deps) { d.Merge = nil }},
		"the methods": {http.MethodGet, mergeAt, "", func(d *webserver.Deps) { d.MergeMethods = nil }},
		"a finish":    {http.MethodPost, finishAt, "", func(d *webserver.Deps) { d.Finish = nil }},
		"a re-run":    {http.MethodPost, rerunAt, "", func(d *webserver.Deps) { d.Rerun = nil }},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			deps := failedCI(reviewDeps(&reviewWrites{}))
			tt.unset(&deps)

			// Act
			recorder := send(t, serve(t, deps, config.Default()), tt.method, tt.path, tt.body)

			// Assert
			assertProblem(t, recorder, http.StatusUnprocessableEntity, "not available")
		})
	}
}

func TestReviewWritesAreHeldBackUnderADryRun(t *testing.T) {
	t.Parallel()

	// Arrange
	writes := &reviewWrites{}
	handler := serveWith(t, failedCI(reviewDeps(writes)), config.Default(),
		webserver.Info{Version: testVersion, DryRun: true})

	// Act
	codes := []int{
		send(t, handler, http.MethodPatch, pullAt, anEdit).Code,
		send(t, handler, http.MethodPost, mergeAt, bySquash).Code,
		send(t, handler, http.MethodPost, finishAt, "").Code,
		send(t, handler, http.MethodPost, rerunAt, "").Code,
	}

	// Assert
	if slices.ContainsFunc(codes, func(code int) bool { return code != http.StatusForbidden }) || writes.sent() != 0 {
		t.Errorf("codes = %v after %+v, want every write refused with 403 and none made", codes, writes)
	}
}

func TestTheDraftStartsFromTheTemplateChosen(t *testing.T) {
	t.Parallel()

	// Arrange
	deps := filledDeps()
	deps.FindPull = func(string) (forge.PullRequest, bool, error) { return forge.PullRequest{}, false, nil }
	deps.Templates = func() []forge.Template {
		return []forge.Template{{Name: "feature", Body: "## Feature\n"}, {Name: bugfixTemplate, Body: "## Bug\n"}}
	}
	handler := serve(t, deps, config.Default())

	// Act
	recorder := get(t, handler, "/api/pull-request/draft?template=bugfix")

	// Assert
	draft := decode[api.PullRequestDraft](t, recorder)
	if !strings.HasPrefix(draft.Body, "## Bug") || draft.Template != bugfixTemplate ||
		!slices.Equal(draft.Templates, []string{"feature", bugfixTemplate}) {
		t.Errorf("draft = %+v, want the bugfix template, both listed", draft)
	}

	unknown := get(t, handler, "/api/pull-request/draft?template=nope")
	assertProblem(t, unknown, http.StatusNotFound, "no pull request template")
}

// everyReviewWrite is each of the review's reads and writes, by method, path
// and body.
func everyReviewWrite() map[string][3]string {
	return map[string][3]string{
		"reading the text":  {http.MethodGet, pullAt, ""},
		"editing":           {http.MethodPatch, pullAt, anEdit},
		"reading the offer": {http.MethodGet, mergeAt, ""},
		"merging":           {http.MethodPost, mergeAt, bySquash},
		"finishing":         {http.MethodPost, finishAt, ""},
		"re-running":        {http.MethodPost, rerunAt, ""},
	}
}

func TestReviewWritesWhoseReadsFailSayToTryAgain(t *testing.T) {
	t.Parallel()

	failures := map[string]func(*webserver.Deps){
		"the branch": func(deps *webserver.Deps) {
			deps.Branch = func() (gitrepo.Branch, error) { return gitrepo.Branch{}, errSeam }
		},
		"the pull request": func(deps *webserver.Deps) {
			deps.FindPull = func(string) (forge.PullRequest, bool, error) { return forge.PullRequest{}, false, errSeam }
		},
	}

	for failing, fail := range failures {
		for name, write := range everyReviewWrite() {
			t.Run(name+" when "+failing+" cannot be read", func(t *testing.T) {
				t.Parallel()

				// Arrange
				writes := &reviewWrites{}
				deps := failedCI(merged(reviewDeps(writes)))
				fail(&deps)

				// Act
				recorder := send(t, serve(t, deps, config.Default()), write[0], write[1], write[2])

				// Assert
				assertProblem(t, recorder, http.StatusInternalServerError, tryAgain)

				if writes.sent() != 0 {
					t.Errorf("writes = %+v, want none", writes)
				}
			})
		}
	}
}

func TestReviewWritesWithNoPullRequestHaveNothingToWriteTo(t *testing.T) {
	t.Parallel()

	shapes := map[string]func(*webserver.Deps){
		"none found": func(deps *webserver.Deps) {
			deps.FindPull = func(string) (forge.PullRequest, bool, error) { return forge.PullRequest{}, false, nil }
		},
		"no forge to ask":  func(deps *webserver.Deps) { deps.FindPull = nil },
		"no branch to ask": func(deps *webserver.Deps) { deps.Branch = nil },
	}

	for shape, unset := range shapes {
		for name, write := range everyReviewWrite() {
			t.Run(name+" with "+shape, func(t *testing.T) {
				t.Parallel()

				// Arrange
				writes := &reviewWrites{}
				deps := failedCI(reviewDeps(writes))
				unset(&deps)

				// Act
				recorder := send(t, serve(t, deps, config.Default()), write[0], write[1], write[2])

				// Assert
				if recorder.Code != http.StatusConflict || writes.sent() != 0 {
					t.Errorf("status %d after %+v, want 409 and nothing written: %s", recorder.Code, writes, recorder.Body)
				}
			})
		}
	}
}

func TestAMergeWithNoCIToReadCannotGo(t *testing.T) {
	t.Parallel()

	// Arrange
	writes := &reviewWrites{}
	deps := reviewDeps(writes)
	deps.CheckCI = nil

	// Act
	recorder := send(t, serve(t, deps, config.Default()), http.MethodPost, mergeAt, bySquash)

	// Assert
	assertProblem(t, recorder, http.StatusConflict, "cannot be merged yet")
}

func TestReviewWritesTheForgeRefusesSayWhy(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		method, path, body string
		refuse             func(*webserver.Deps)
	}{
		"an edit": {http.MethodPatch, pullAt, anEdit, func(d *webserver.Deps) {
			d.EditPull = func(forge.PullRequest, forge.PullRequestEdit) (forge.PullRequest, error) {
				return forge.PullRequest{}, refusedByForge()
			}
		}},
		"the methods' read": {http.MethodGet, mergeAt, "", func(d *webserver.Deps) {
			d.MergeMethods = func() ([]forge.MergeMethod, error) { return nil, refusedByForge() }
		}},
		"a merge's methods": {http.MethodPost, mergeAt, bySquash, func(d *webserver.Deps) {
			d.MergeMethods = func() ([]forge.MergeMethod, error) { return nil, refusedByForge() }
		}},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			deps := reviewDeps(&reviewWrites{})
			tt.refuse(&deps)

			// Act
			recorder := send(t, serve(t, deps, config.Default()), tt.method, tt.path, tt.body)

			// Assert
			assertProblem(t, recorder, http.StatusUnprocessableEntity, "the token may lack")
		})
	}
}
