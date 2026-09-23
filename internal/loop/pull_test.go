// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package loop_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/jacob-delgado/workflow/internal/convention"
	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/gitrepo"
	"github.com/jacob-delgado/workflow/internal/jira"
	"github.com/jacob-delgado/workflow/internal/loop"
)

// The branch, its issue and the tracker these tests compose from.
const (
	branchName    = "fix/PROJ-7-redact-the-token"
	keylessBranch = "chore/cleanup"
	project       = "PROJ"
	issueKey      = "PROJ-7"
	subject       = "fix: redact the token"
	summary       = "Redact the token"
	browseURL     = "https://jira.example.com/browse/PROJ-7"
	headCommit    = "abc1234"
)

// errSeam is what a failing seam answers.
var errSeam = errors.New("the seam failed")

// openable is a branch with a commit and an upstream base, which a pull request
// can be opened for.
func openable(name string) gitrepo.Branch {
	return gitrepo.Branch{
		Name: name, Base: "origin/main", Head: headCommit,
		Commits: []gitrepo.Commit{{Hash: headCommit, Subject: subject}},
	}
}

// pullSeams answer as a repository on an openable branch with no pull request,
// no template, and an issue the tracker can read and link.
func pullSeams() loop.PullSeams {
	return loop.PullSeams{
		Branch:    func() (gitrepo.Branch, error) { return openable(branchName), nil },
		FindPull:  func(string) (forge.PullRequest, bool, error) { return forge.PullRequest{}, false, nil },
		Templates: func() []forge.Template { return nil },
		Issue: func(jira.Key) (jira.IssueDetail, error) {
			return jira.IssueDetail{Issue: jira.Issue{Key: issueKey, Summary: summary}}, nil
		},
		BrowseURL: func(key jira.Key) string { return "https://jira.example.com/browse/" + string(key) },
	}
}

// options compose for the tracker's project, titling from the oldest commit.
func options() loop.PullOptions {
	return loop.PullOptions{Project: project, TitleSource: convention.TitleFromCommit}
}

func TestComposePullRefusesAnOpenPull(t *testing.T) {
	t.Parallel()

	// Arrange
	seams := pullSeams()
	seams.FindPull = func(string) (forge.PullRequest, bool, error) {
		return forge.PullRequest{Number: 9, URL: "https://github.com/o/r/pull/9"}, true, nil
	}

	// Act
	_, _, err := loop.ComposePull(seams, options())

	// Assert
	var open loop.PullAlreadyOpenError
	if !errors.Is(err, loop.ErrPullAlreadyOpen) || !errors.As(err, &open) || open.Pull.Number != 9 {
		t.Errorf("ComposePull returned %v, want ErrPullAlreadyOpen carrying pull 9", err)
	}

	// The pull request rides on the error, not in its text: a web detail shows the
	// text, and a self-managed forge's link would name its host there.
	if err == nil || err.Error() != loop.ErrPullAlreadyOpen.Error() {
		t.Errorf("the refusal reads %v, want only %q", err, loop.ErrPullAlreadyOpen)
	}
}

func TestComposePullRefusesWhatCannotBeOpened(t *testing.T) {
	t.Parallel()

	cases := map[string]func(loop.PullSeams) loop.PullSeams{
		"no way to read the branch": func(seams loop.PullSeams) loop.PullSeams {
			seams.Branch = nil

			return seams
		},
		"no way to ask the forge": func(seams loop.PullSeams) loop.PullSeams {
			seams.FindPull = nil

			return seams
		},
		"a detached tree": func(seams loop.PullSeams) loop.PullSeams {
			seams.Branch = func() (gitrepo.Branch, error) { return openable(""), nil }

			return seams
		},
		"a branch with no commits": func(seams loop.PullSeams) loop.PullSeams {
			seams.Branch = func() (gitrepo.Branch, error) {
				return gitrepo.Branch{Name: branchName, Base: "origin/main"}, nil
			}

			return seams
		},
	}

	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			seams := mutate(pullSeams())

			// Act
			_, _, err := loop.ComposePull(seams, options())

			// Assert
			if !errors.Is(err, loop.ErrNothingToOpen) {
				t.Errorf("ComposePull returned %v, want ErrNothingToOpen", err)
			}
		})
	}
}

func TestComposePullReportsAnUnreadableBranch(t *testing.T) {
	t.Parallel()

	// Arrange
	seams := pullSeams()
	seams.Branch = func() (gitrepo.Branch, error) { return gitrepo.Branch{}, errSeam }

	// Act
	_, _, err := loop.ComposePull(seams, options())

	// Assert
	if !errors.Is(err, errSeam) || errors.Is(err, loop.ErrNothingToOpen) || !strings.Contains(err.Error(), "branch") {
		t.Errorf("ComposePull returned %v, want the branch read's own failure", err)
	}
}

func TestComposePullComposesPastWhatDoesNotBlock(t *testing.T) {
	t.Parallel()

	// Only an open pull request stands in the way: a merged or closed one leaves
	// the branch free for a fresh one, and a forge that cannot answer leaves the
	// open to report the forge's own error.
	cases := map[string]func(string) (forge.PullRequest, bool, error){
		"no pull request": func(string) (forge.PullRequest, bool, error) {
			return forge.PullRequest{}, false, nil
		},
		"a merged pull request": func(string) (forge.PullRequest, bool, error) {
			return forge.PullRequest{Number: 3, State: forge.StateMerged}, true, nil
		},
		"a closed pull request": func(string) (forge.PullRequest, bool, error) {
			return forge.PullRequest{Number: 3, State: forge.StateClosed}, true, nil
		},
		"a forge that cannot be read": func(string) (forge.PullRequest, bool, error) {
			return forge.PullRequest{}, false, errSeam
		},
	}

	for name, find := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			seams := pullSeams()
			seams.FindPull = find

			// Act
			request, branch, err := loop.ComposePull(seams, options())

			// Assert
			if err != nil || request.Head != branchName || branch.Name != branchName {
				t.Errorf("ComposePull = %+v, %q, %v; want a draft for %q", request, branch.Name, err, branchName)
			}
		})
	}
}

func TestComposePullDraftsFromTheBranchAndItsIssue(t *testing.T) {
	t.Parallel()

	// Act
	request, _, err := loop.ComposePull(pullSeams(), options())
	// Assert
	if err != nil {
		t.Fatalf("ComposePull returned %v, want nil", err)
	}

	if request.Title != subject || request.Head != branchName || request.Base != "main" || request.Draft {
		t.Errorf("draft = %+v, want the commit's title, the branch into main, not a draft", request)
	}

	if !strings.Contains(request.Body, "- "+subject) || !strings.Contains(request.Body, "["+issueKey+"]("+browseURL+")") {
		t.Errorf("body = %q, want the commits listed and the issue linked", request.Body)
	}
}

func TestComposePullTitlesFromTheIssueWhenConfigured(t *testing.T) {
	t.Parallel()

	// Arrange
	opts := options()
	opts.TitleSource = convention.TitleFromIssue

	// Act
	request, _, err := loop.ComposePull(pullSeams(), opts)

	// Assert
	if err != nil || request.Title != issueKey+": "+summary {
		t.Errorf("title = %q (%v), want the issue's %q", request.Title, err, issueKey+": "+summary)
	}
}

func TestComposePullBodyTakesTheFirstTemplate(t *testing.T) {
	t.Parallel()

	// Arrange
	seams := pullSeams()
	seams.Templates = func() []forge.Template {
		return []forge.Template{{Name: "default", Body: "## Why\n"}, {Name: "other", Body: "## Other\n"}}
	}

	// Act
	request, _, err := loop.ComposePull(seams, options())

	// Assert
	if err != nil || !strings.HasPrefix(request.Body, "## Why") || strings.Contains(request.Body, "## Commits") {
		t.Errorf("body = %q (%v), want the first template in place of the commit list", request.Body, err)
	}
}

func TestComposePullListsTheCommitsWithoutATemplate(t *testing.T) {
	t.Parallel()

	cases := map[string]func() []forge.Template{
		"no way to read templates": nil,
		"a repository with none":   func() []forge.Template { return []forge.Template{} },
	}

	for name, templates := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			seams := pullSeams()
			seams.Templates = templates

			// Act
			request, _, err := loop.ComposePull(seams, options())

			// Assert
			if err != nil || !strings.HasPrefix(request.Body, "## Commits") {
				t.Errorf("body = %q (%v), want the commit list", request.Body, err)
			}
		})
	}
}

func TestComposePullReadsWithoutWhatTheTrackerCannotSay(t *testing.T) {
	t.Parallel()

	// The issue's summary and link are extras: without them the draft still
	// composes, titled from the commit and naming the key unlinked.
	cases := map[string]func(loop.PullSeams) loop.PullSeams{
		"no way to read the issue": func(seams loop.PullSeams) loop.PullSeams {
			seams.Issue, seams.BrowseURL = nil, nil

			return seams
		},
		"an issue read that fails": func(seams loop.PullSeams) loop.PullSeams {
			seams.Issue = func(jira.Key) (jira.IssueDetail, error) { return jira.IssueDetail{}, errSeam }
			seams.BrowseURL = nil

			return seams
		},
	}

	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			seams := mutate(pullSeams())
			opts := options()
			opts.TitleSource = convention.TitleFromIssue

			// Act
			request, _, err := loop.ComposePull(seams, opts)

			// Assert
			if err != nil || request.Title != subject || !strings.Contains(request.Body, "Jira: "+issueKey+"\n") {
				t.Errorf("draft = %+v (%v), want the commit's title and the key unlinked", request, err)
			}
		})
	}
}

func TestComposePullAsksTheTrackerNothingForAKeylessBranch(t *testing.T) {
	t.Parallel()

	// Arrange
	asked := false

	seams := pullSeams()
	seams.Branch = func() (gitrepo.Branch, error) { return openable(keylessBranch), nil }
	seams.Issue = func(jira.Key) (jira.IssueDetail, error) {
		asked = true

		return jira.IssueDetail{}, nil
	}
	seams.BrowseURL = func(jira.Key) string {
		asked = true

		return ""
	}

	// Act
	request, _, err := loop.ComposePull(seams, options())

	// Assert
	if err != nil || asked || strings.Contains(request.Body, "Jira:") {
		t.Errorf("draft = %+v (%v), asked = %t; want no issue looked up or named", request, err, asked)
	}
}

func TestSubjectsListsEachCommitOldestFirst(t *testing.T) {
	t.Parallel()

	// Arrange
	commits := []gitrepo.Commit{{Subject: "first"}, {Subject: "second"}}

	// Act
	got := loop.Subjects(commits)

	// Assert
	if strings.Join(got, "|") != "first|second" {
		t.Errorf("Subjects = %q, want first then second", got)
	}
}
