// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package cli

// runStandup reads from a repository, a forge, Jira and Slack, none of which a
// black-box test can point at a fake through the wiring the command builds. So
// these drive runStandup directly with fake seams, which is what the seams are
// for. errServiceDown is shared with the status seam tests.

import (
	"bytes"
	"strconv"
	"strings"
	"testing"

	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/gitrepo"
	"github.com/jacob-delgado/workflow/internal/jira"
)

// sampleKey and sampleSummary are the fixture issue's key and summary, shared
// so the seam values and their assertions cannot drift apart.
const (
	sampleKey     = "PROJ-7"
	sampleSummary = "Fix the login"
	samplePullURL = "https://forge/pr/3"
)

// standupFixture is a day with one commit, one issue and one open pull request,
// no Slack configured, and an editor that leaves the draft as it found it.
func standupFixture() standupSeams {
	return standupSeams{
		Commits: func(string) ([]gitrepo.Commit, error) {
			return []gitrepo.Commit{{Hash: "abc1234", Subject: sampleSummary}}, nil
		},
		Branches: func() ([]string, error) { return []string{"fix/PROJ-7-login"}, nil },
		FindPull: func(string) (forge.PullRequest, bool, error) {
			return forge.PullRequest{Number: 3, Title: sampleSummary, URL: samplePullURL}, true, nil
		},
		Search: func(string, int) (jira.SearchResult, error) {
			return jira.SearchResult{Issues: []jira.Issue{{Key: sampleKey, Summary: sampleSummary, Status: "In Progress"}}}, nil
		},
		Compose: func(draft string) (string, error) { return draft, nil },
		Post:    func(string) error { return nil },
		Confirm: func() (bool, error) { return true, nil },
		Slack:   false,
	}
}

func TestStandupDraftsCommitsIssuesAndPulls(t *testing.T) {
	t.Parallel()

	// Arrange
	var out bytes.Buffer

	// Act
	err := runStandup(&out, standupFixture(), 1, true)
	if err != nil {
		t.Fatalf("runStandup: %v", err)
	}

	// Assert
	draft := out.String()
	for _, want := range []string{
		"# Standup", "Fix the login (abc1234)",
		"PROJ-7 Fix the login (In Progress)", "#3 Fix the login https://forge/pr/3",
	} {
		if !strings.Contains(draft, want) {
			t.Errorf("standup draft missing %q:\n%s", want, draft)
		}
	}
}

func TestStandupOmitsAMergedPullRequest(t *testing.T) {
	t.Parallel()

	// Arrange
	// The branch's pull request has merged, so it is no longer open work to list.
	seams := standupFixture()
	seams.FindPull = func(string) (forge.PullRequest, bool, error) {
		return forge.PullRequest{
			Number: 3, Title: sampleSummary, URL: samplePullURL, State: forge.StateMerged,
		}, true, nil
	}

	var out bytes.Buffer

	// Act
	err := runStandup(&out, seams, 1, true)
	if err != nil {
		t.Fatalf("runStandup: %v", err)
	}

	// Assert
	if draft := out.String(); strings.Contains(draft, "#3") {
		t.Errorf("standup listed a merged pull request as open:\n%s", draft)
	}
}

func TestStandupWithNoWorkSaysEachSectionIsEmpty(t *testing.T) {
	t.Parallel()

	// Arrange
	seams := standupSeams{
		Commits:  func(string) ([]gitrepo.Commit, error) { return nil, nil },
		Branches: func() ([]string, error) { return nil, nil },
		Search:   func(string, int) (jira.SearchResult, error) { return jira.SearchResult{}, nil },
	}

	var out bytes.Buffer

	// Act
	err := runStandup(&out, seams, 1, true)
	if err != nil {
		t.Fatalf("runStandup: %v", err)
	}

	// Assert
	if got := strings.Count(out.String(), "- none"); got != 3 {
		t.Errorf("empty standup listed %d empty sections, want 3:\n%s", got, out.String())
	}
}

func TestStandupOpensTheDraftInTheEditor(t *testing.T) {
	t.Parallel()

	// Arrange
	seams := standupFixture()

	var seen string

	seams.Compose = func(draft string) (string, error) {
		seen = draft

		return "# Standup\n\nwhat I actually did", nil
	}

	var out bytes.Buffer

	// Act
	err := runStandup(&out, seams, 1, false)
	if err != nil {
		t.Fatalf("runStandup: %v", err)
	}

	// Assert
	if !strings.Contains(seen, "PROJ-7") || !strings.Contains(out.String(), "what I actually did") {
		t.Errorf("the editor was not handed the draft, or its edit was dropped:\nseen=%q\nout=%s", seen, out.String())
	}
}

func TestStandupWithNothingLeftSaysSo(t *testing.T) {
	t.Parallel()

	// Arrange
	seams := standupFixture()
	seams.Slack = true
	seams.Compose = func(string) (string, error) { return "   \n\t", nil }

	posted := false
	seams.Post = func(string) error {
		posted = true

		return nil
	}

	var out bytes.Buffer

	// Act
	err := runStandup(&out, seams, 1, false)
	if err != nil {
		t.Fatalf("runStandup: %v", err)
	}

	// Assert
	if !strings.Contains(out.String(), "Nothing to share.") || posted {
		t.Errorf("an emptied draft was posted or not reported:\nposted=%v\n%s", posted, out.String())
	}
}

func TestStandupPostsToSlackOnceConfirmed(t *testing.T) {
	t.Parallel()

	// Arrange
	seams := standupFixture()
	seams.Slack = true

	var posted string

	seams.Post = func(text string) error {
		posted = text

		return nil
	}

	var out bytes.Buffer

	// Act
	err := runStandup(&out, seams, 1, true)
	if err != nil {
		t.Fatalf("runStandup: %v", err)
	}

	// Assert
	if !strings.Contains(posted, "PROJ-7") || !strings.Contains(out.String(), "Posted to Slack.") {
		t.Errorf("the standup was not posted:\nposted=%q\n%s", posted, out.String())
	}
}

func TestStandupIsNotPostedWhenDeclined(t *testing.T) {
	t.Parallel()

	// Arrange
	seams := standupFixture()
	seams.Slack = true
	seams.Confirm = func() (bool, error) { return false, nil }

	posted := false
	seams.Post = func(string) error {
		posted = true

		return nil
	}

	var out bytes.Buffer

	// Act
	err := runStandup(&out, seams, 1, true)
	if err != nil {
		t.Fatalf("runStandup: %v", err)
	}

	// Assert
	if posted || !strings.Contains(out.String(), "Not posted.") {
		t.Errorf("a declined standup was posted:\nposted=%v\n%s", posted, out.String())
	}
}

func TestStandupWindowsBothQueriesByDays(t *testing.T) {
	t.Parallel()

	// Arrange
	seams := standupFixture()

	var since, jql string

	seams.Commits = func(gitSince string) ([]gitrepo.Commit, error) {
		since = gitSince

		return nil, nil
	}
	seams.Search = func(query string, _ int) (jira.SearchResult, error) {
		jql = query

		return jira.SearchResult{}, nil
	}

	var out bytes.Buffer

	// Act
	err := runStandup(&out, seams, 3, true)
	if err != nil {
		t.Fatalf("runStandup: %v", err)
	}

	// Assert
	if since != "3 days ago" || !strings.Contains(jql, "-3d") {
		t.Errorf("standup did not window by days: since=%q jql=%q", since, jql)
	}
}

func TestStandupNeutralizesHostileServiceText(t *testing.T) {
	t.Parallel()

	// Arrange
	seams := standupFixture()
	seams.Search = func(string, int) (jira.SearchResult, error) {
		return jira.SearchResult{Issues: []jira.Issue{
			{Key: sampleKey, Summary: "evil\x1b[2Jcleared", Status: "Open"},
		}}, nil
	}
	seams.FindPull = func(string) (forge.PullRequest, bool, error) {
		return forge.PullRequest{Number: 3, Title: "title\x1b[1mbold", URL: samplePullURL}, true, nil
	}

	var out bytes.Buffer

	// Act
	err := runStandup(&out, seams, 1, true)
	if err != nil {
		t.Fatalf("runStandup: %v", err)
	}

	// Assert
	if strings.ContainsRune(out.String(), 0x1b) {
		t.Errorf("a hostile summary or title reached the draft as a control sequence:\n%q", out.String())
	}
}

func TestStandupReportsAnUnreadableRepository(t *testing.T) {
	t.Parallel()

	// Arrange
	seams := standupFixture()
	seams.Commits = func(string) ([]gitrepo.Commit, error) { return nil, errServiceDown }

	var out bytes.Buffer

	// Act
	err := runStandup(&out, seams, 1, true)

	// Assert
	if err == nil || !strings.Contains(err.Error(), "recent commits") {
		t.Errorf("runStandup returned %v, want the commit read failure", err)
	}
}

func TestStandupReportsAFailedPost(t *testing.T) {
	t.Parallel()

	// Arrange
	seams := standupFixture()
	seams.Slack = true
	seams.Post = func(string) error { return errServiceDown }

	var out bytes.Buffer

	// Act
	err := runStandup(&out, seams, 1, true)

	// Assert
	if err == nil || !strings.Contains(err.Error(), "posting to Slack") {
		t.Errorf("runStandup returned %v, want the post failure", err)
	}
}

func TestStandupDegradesWhenBranchesCannotBeRead(t *testing.T) {
	t.Parallel()

	// Arrange
	seams := standupFixture()
	seams.Branches = func() ([]string, error) { return nil, errServiceDown }

	var out bytes.Buffer

	// Act
	err := runStandup(&out, seams, 1, true)
	if err != nil {
		t.Fatalf("standup should degrade, not fail: %v", err)
	}

	// Assert
	if !strings.Contains(out.String(), "## Pull requests\n- none") {
		t.Errorf("unreadable branches did not degrade to no pull requests:\n%s", out.String())
	}
}

func TestStandupSkipsBranchesWithoutAPull(t *testing.T) {
	t.Parallel()

	// Arrange
	seams := standupFixture()
	seams.Branches = func() ([]string, error) { return []string{"quiet", "loud"}, nil }
	seams.FindPull = func(branch string) (forge.PullRequest, bool, error) {
		if branch == "quiet" {
			return forge.PullRequest{}, false, errServiceDown
		}

		return forge.PullRequest{Number: 9, Title: "Loud", URL: "https://forge/pr/9"}, true, nil
	}

	var out bytes.Buffer

	// Act
	err := runStandup(&out, seams, 1, true)
	if err != nil {
		t.Fatalf("runStandup: %v", err)
	}

	// Assert
	if !strings.Contains(out.String(), "#9 Loud") || strings.Contains(out.String(), "quiet") {
		t.Errorf("standup did not skip the branch without a pull request:\n%s", out.String())
	}
}

func TestStandupCapsBranchesItAsksAbout(t *testing.T) {
	t.Parallel()

	// Arrange
	seams := standupFixture()

	many := make([]string, standupBranchLimit+5)
	for index := range many {
		many[index] = "branch-" + strconv.Itoa(index)
	}

	calls := 0

	seams.Branches = func() ([]string, error) { return many, nil }
	seams.FindPull = func(string) (forge.PullRequest, bool, error) {
		calls++

		return forge.PullRequest{}, false, nil
	}

	var out bytes.Buffer

	// Act
	err := runStandup(&out, seams, 1, true)
	if err != nil {
		t.Fatalf("runStandup: %v", err)
	}

	// Assert
	if calls != standupBranchLimit {
		t.Errorf("standup asked about %d branches, want at most %d", calls, standupBranchLimit)
	}
}
