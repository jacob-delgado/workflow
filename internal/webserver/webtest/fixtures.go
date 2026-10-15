// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package webtest

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

// ErrSeam is what a failing seam returns: an error in none of fault's classes,
// so the server answers it with the opaque internal 500, whose detail carries
// none of the error's own text.
var ErrSeam = errors.New("the seam failed")

// Fixtures the tests share.
const (
	TestKey        = "PROJ-412"
	TestSummary    = "Fix token redaction"
	TestProject    = "PROJ"
	TestReporter   = "Ana Lopez"
	TestBranchName = "fix/PROJ-412"
	TestAuthor     = "octocat"
	TestVersion    = "1.2.3"
	TestBugJQL     = "type = Bug"
	TestBugView    = "Bugs"
	TestBase       = "origin/main"
	TestChannel    = "#dev-workflow"
	// TestCommitSubject and TestCommitHash are the branch's one commit, shared by
	// the write-action tests that need a branch with a commit on it.
	TestCommitSubject = "feat: redact"
	TestCommitHash    = "abc1234"
	// NoBranchSeam is the shared name for the "no branch seam" case the write
	// handlers' unavailability tables each exercise.
	NoBranchSeam = "no branch seam"
	// NoForgeSeam is its twin for the reads and writes that ask the forge.
	NoForgeSeam = "no forge seam"
	// WaitAndTryAgain is what an answer to a service limiting requests says to
	// do, whichever service it is.
	WaitAndTryAgain = "wait and try again"
	// TryAgain is what the internal problem's detail says to do, whichever
	// request met it.
	TryAgain = "try again"
	// RepoPath is where the repository sits on disk, which a git error can
	// carry and no answer may repeat.
	RepoPath = "/home/dev/src/acme"
	// LoopbackHost is the Host the shared request helpers send, so requests pass
	// the loopback guard the same way a browser on 127.0.0.1 does. A test that
	// exercises the guard sets its own Host instead.
	LoopbackHost = "127.0.0.1:13579"
)

// FilledHead is the head commit of FilledDeps' branch, its one commit.
const FilledHead = "abc123"

// FilledDeps is a Deps with every seam populated with canned answers. A test
// nils a seam to exercise the not-configured path, or replaces one to fail.
func FilledDeps() webserver.Deps {
	return webserver.Deps{
		Jira: seams.Jira{
			Search: func(context.Context, string, int) (jira.SearchResult, error) {
				return jira.SearchResult{
					Issues: []jira.Issue{{
						Key: TestKey, Summary: TestSummary, Status: "In Progress",
						StatusCategory: "indeterminate", Type: "Bug", Priority: "High",
					}},
					Total: 1,
				}, nil
			},
			Issue: func(_ context.Context, key jira.Key) (jira.IssueDetail, error) {
				return jira.IssueDetail{
					Issue: jira.Issue{
						Key: key, Summary: TestSummary, Status: "In Progress",
						StatusCategory: "indeterminate", Type: "Bug",
					},
					Reporter:     TestReporter,
					Description:  "Tokens reach the log.",
					Comments:     []jira.Comment{{Author: TestReporter, Body: "Repro'd", Created: time.Unix(0, 0).UTC()}},
					CommentTotal: 1,
				}, nil
			},
		},
		Git: seams.Git{
			Branch: func(context.Context) (gitrepo.Branch, error) {
				return gitrepo.Branch{
					Name: TestBranchName, Base: TestBase, Ahead: 2, Head: FilledHead, PushRemote: gitrepo.DefaultRemote,
					BaseRemote: gitrepo.DefaultRemote,
					Commits:    []gitrepo.Commit{{Hash: FilledHead, Subject: TestCommitSubject}},
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
			Author: func(context.Context) (string, error) { return TestAuthor, nil },
		},
	}
}

// The pull request the tests open: its title, the base it targets and its
// address.
const (
	PRTitle = "fix: redact tokens"
	PRBase  = "main"
	PRURL   = "https://x/7"
)

// The writes the contract's tests ask for, and the body that names no issue.
const (
	CheckoutPath = "/api/checkout"
	NoIssueKey   = `{"issue_key":""}`
)

// Hearing is deps with Unexpected spied on, and what it heard.
func Hearing(deps webserver.Deps) (webserver.Deps, *[]error) {
	var heard []error

	deps.Unexpected = func(_ config.Config, err error) { heard = append(heard, err) }

	return deps, &heard
}
