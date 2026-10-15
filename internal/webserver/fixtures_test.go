// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package webserver_test

import (
	"context"
	"errors"
	"time"

	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/gitrepo"
	"github.com/jacob-delgado/workflow/internal/jira"
	"github.com/jacob-delgado/workflow/internal/seams"
	"github.com/jacob-delgado/workflow/internal/webserver"
)

// errSeam is what a failing seam returns: an error in none of fault's classes,
// so the server answers it with the opaque internal 500, whose detail carries
// none of the error's own text.
var errSeam = errors.New("the seam failed")

// Fixtures the tests share.
const (
	testKey        = "PROJ-412"
	testSummary    = "Fix token redaction"
	testProject    = "PROJ"
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
	loopbackHost = "127.0.0.1:13579"
)

// filledHead is the head commit of filledDeps' branch, its one commit.
const filledHead = "abc123"

// filledDeps is a Deps with every seam populated with canned answers. A test
// nils a seam to exercise the not-configured path, or replaces one to fail.
func filledDeps() webserver.Deps {
	return webserver.Deps{
		Jira: seams.Jira{
			Search: func(context.Context, string, int) (jira.SearchResult, error) {
				return jira.SearchResult{
					Issues: []jira.Issue{{
						Key: testKey, Summary: testSummary, Status: "In Progress",
						StatusCategory: "indeterminate", Type: "Bug", Priority: "High",
					}},
					Total: 1,
				}, nil
			},
			Issue: func(_ context.Context, key jira.Key) (jira.IssueDetail, error) {
				return jira.IssueDetail{
					Issue: jira.Issue{
						Key: key, Summary: testSummary, Status: "In Progress",
						StatusCategory: "indeterminate", Type: "Bug",
					},
					Reporter:     testReporter,
					Description:  "Tokens reach the log.",
					Comments:     []jira.Comment{{Author: testReporter, Body: "Repro'd", Created: time.Unix(0, 0).UTC()}},
					CommentTotal: 1,
				}, nil
			},
		},
		Git: seams.Git{
			Branch: func(context.Context) (gitrepo.Branch, error) {
				return gitrepo.Branch{
					Name: testBranchName, Base: testBase, Ahead: 2, Head: filledHead, PushRemote: gitrepo.DefaultRemote,
					BaseRemote: gitrepo.DefaultRemote,
					Commits:    []gitrepo.Commit{{Hash: filledHead, Subject: testCommitSubject}},
				}, nil
			},
			Changes: func(context.Context) ([]gitrepo.Change, error) {
				return []gitrepo.Change{{Path: "internal/config/config.go", Staged: 'M'}}, nil
			},
		},
		Forge: seams.Forge{
			FindPullRequest: func(context.Context, string) (forge.PullRequest, bool, error) {
				pull := forge.PullRequest{Number: 42, URL: "https://x/42", Title: "redact", Mergeable: forge.MergeClean}

				return pull, true, nil
			},
			CheckStatus: func(context.Context, forge.PullRequest, string) (forge.CI, error) {
				return forge.CI{
					State: forge.CIPassed, Total: 3, Done: 3,
					Checks: []forge.Check{{Name: "build", State: forge.CIPassed}},
				}, nil
			},
			Author: func(context.Context) (string, error) { return testAuthor, nil },
		},
	}
}

const (
	prTitle = "fix: redact tokens"
	prBase  = "main"
	prURL   = "https://x/7"
)

// The writes the contract's tests ask for, and the body that names no issue.
const (
	checkoutPath = "/api/checkout"
	noIssueKey   = `{"issue_key":""}`
)

// hearing is deps with Unexpected spied on, and what it heard.
func hearing(deps webserver.Deps) (webserver.Deps, *[]error) {
	var heard []error

	deps.Unexpected = func(_ config.Config, err error) { heard = append(heard, err) }

	return deps, &heard
}
